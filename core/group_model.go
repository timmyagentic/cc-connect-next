package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Only trusted, localized routing failures may bypass generic backend-error
// redaction to explain how to recover on a rich-card platform.
type groupModelError struct{ message string }

func (err *groupModelError) Error() string { return err.message }

func (e *Engine) groupModelFailure(key MsgKey) error {
	return &groupModelError{message: e.i18n.T(key)}
}

// rememberGroupModelScope accepts only platform-verified group identity. The
// project owns this metadata, so two bots sharing a directory remain isolated.
func (e *Engine) rememberGroupModelScope(p Platform, msg *Message) error {
	if msg.IsDirect {
		return e.sessions.UpdateGroupModelKey(msg.SessionKey, "")
	}
	if msg.GroupID == "" {
		return nil // callbacks may use previously verified, persisted routing
	}
	return e.sessions.UpdateGroupModelKey(msg.SessionKey, p.Name()+":"+msg.GroupID)
}

func (e *Engine) groupModelKey(sessionKey string) string {
	if meta := e.sessions.GetUserMeta(sessionKey); meta != nil {
		return meta.GroupModelKey
	}
	return ""
}

func (e *Engine) groupModelBinding(sessionKey string) (GroupModelBinding, bool) {
	if e.projectState == nil {
		return GroupModelBinding{}, false
	}
	key := e.groupModelKey(sessionKey)
	if key == "" {
		return GroupModelBinding{}, false
	}
	return e.projectState.GroupModelBinding(key)
}

func groupModelProvider(agent Agent) string {
	provider := ""
	if ps, ok := agent.(ProviderSwitcher); ok {
		if active := ps.GetActiveProvider(); active != nil {
			provider = active.Name
		}
	}
	return agent.Name() + ":" + provider
}

func supportsGroupModel(agent Agent) bool {
	supporter, ok := agent.(PerTurnModelSupporter)
	return ok && supporter.SupportsPerTurnModel()
}

func (e *Engine) groupDefaultModel(agent Agent, sessionKey string, backend AgentSession) (string, error) {
	binding, ok := e.groupModelBinding(sessionKey)
	if !ok {
		return "", nil
	}
	if !supportsGroupModel(agent) {
		return "", e.groupModelFailure(MsgModelGroupUnsupported)
	}
	if binding.Provider != groupModelProvider(agent) {
		return "", e.groupModelFailure(MsgModelGroupProviderChanged)
	}
	if backend != nil {
		if _, ok := backend.(TurnOptionsSession); !ok {
			return "", e.groupModelFailure(MsgModelGroupUnsupported)
		}
		if actual, ok := backend.(ModelProviderSession); ok {
			if binding.Provider != agent.Name()+":"+actual.ModelProviderName() {
				return "", e.groupModelFailure(MsgModelGroupSessionProviderChanged)
			}
		} else if _, canSwitch := agent.(ProviderSwitcher); canSwitch {
			return "", e.groupModelFailure(MsgModelGroupUnsupported)
		}
	}
	return binding.Model, nil
}

// A group selection is a default layer, not an explicit task override. Resolve
// it for each dequeued turn, preserving explicit task/profile model precedence.
func (e *Engine) prepareSessionTurnOverrides(agent Agent, sessions *SessionManager, session *Session, backend AgentSession, sessionKey string, profile AnswerProfileName, model, reasoning, tier string) (agentTurnOverrides, error) {
	profileModel := ""
	if configured, ok := e.answerProfile(profile); ok {
		profileModel = strings.TrimSpace(configured.Model)
	}
	if strings.TrimSpace(model) == "" && profileModel == "" {
		var err error
		model, err = e.groupDefaultModel(agent, sessionKey, backend)
		if err != nil {
			return agentTurnOverrides{}, err
		}
	}
	return prepareAgentTurnOverrides(sessions, session, backend, profile, model, reasoning, tier)
}

// Publish the resolved model atomically with steering admission. A binding can
// be set and reset between queue admission and dispatch; steering must observe
// the model actually captured for this turn, not just the current binding.
func (e *Engine) prepareInteractiveTurnOverrides(state *interactiveState, agent Agent, sessions *SessionManager, session *Session, backend AgentSession, sessionKey string, profile AnswerProfileName, model, reasoning, tier string) (agentTurnOverrides, error) {
	state.steerMu.Lock()
	defer state.steerMu.Unlock()
	overrides, err := e.prepareSessionTurnOverrides(agent, sessions, session, backend, sessionKey, profile, model, reasoning, tier)
	if err == nil {
		state.mu.Lock()
		state.activeModelOverride = overrides.model
		state.mu.Unlock()
	}
	return overrides, err
}

func (e *Engine) setGroupModel(agent Agent, sessionKey, input string) (string, error) {
	key := e.groupModelKey(sessionKey)
	if key == "" {
		return "", fmt.Errorf("%s", e.i18n.T(MsgModelGroupOnly))
	}
	if e.projectState == nil {
		return "", fmt.Errorf("%s", e.i18n.T(MsgModelGroupStorageUnavailable))
	}
	if strings.EqualFold(input, "reset") {
		return "", e.projectState.SaveGroupModelBinding(key, GroupModelBinding{})
	}
	switcher, ok := agent.(ModelSwitcher)
	if !ok || !supportsGroupModel(agent) {
		return "", fmt.Errorf("%s", e.i18n.T(MsgModelGroupUnsupported))
	}
	ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	defer cancel()
	provider := groupModelProvider(agent)
	models := groupSelectableModels(ctx, agent, switcher)
	if provider != groupModelProvider(agent) {
		return "", fmt.Errorf("%s", e.i18n.T(MsgModelGroupProviderChanged))
	}
	target := resolveModelSwitchTarget(input, models)
	found := false
	for _, candidate := range models {
		if candidate.Name == target {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("%s", e.i18n.Tf(MsgModelGroupUnknown, input))
	}
	if err := e.projectState.SaveGroupModelBinding(key, GroupModelBinding{Model: target, Provider: provider}); err != nil {
		return "", err
	}
	return target, nil
}

// AvailableModels can be a native catalog snapshot. Include explicit provider
// models as well, so a local catalog cannot hide configured custom models.
func groupSelectableModels(ctx context.Context, agent Agent, switcher ModelSwitcher) []ModelOption {
	var models []ModelOption
	seen := make(map[string]bool)
	appendModels := func(entries []ModelOption) {
		for _, entry := range entries {
			if entry.Name != "" && !seen[entry.Name] {
				models = append(models, entry)
				seen[entry.Name] = true
			}
		}
	}
	if ps, ok := agent.(ProviderSwitcher); ok {
		if active := ps.GetActiveProvider(); active != nil {
			appendModels(active.Models)
			appendModels([]ModelOption{{Name: active.Model}})
		}
	}
	appendModels(switcher.AvailableModels(ctx))
	return models
}

func (e *Engine) cmdGroupModel(p Platform, msg *Message, agent Agent, args []string) {
	if e.groupModelKey(msg.SessionKey) == "" {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgModelGroupOnly))
		return
	}
	if len(args) == 0 {
		e.replyWithCard(p, msg.ReplyCtx, e.renderModelCardForAgent(msg.SessionKey, agent))
		return
	}
	if len(args) != 1 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgModelGroupUsage))
		return
	}
	model, err := e.setGroupModel(agent, msg.SessionKey, args[0])
	if err != nil {
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgModelChangeFailed, err))
		return
	}
	message := e.i18n.T(MsgModelGroupReset)
	if model != "" {
		message = e.i18n.Tf(MsgModelGroupChanged, model)
	}
	card := e.renderModelCardForAgent(msg.SessionKey, agent)
	card.Elements = append([]CardElement{CardMarkdown{Content: message}}, card.Elements...)
	e.replyWithCard(p, msg.ReplyCtx, card)
}

// groupModelCard adds an explicit group section while leaving the legacy
// default selection available and clearly labelled on the same card.
func (e *Engine) groupModelCard(cb *CardBuilder, sessionKey string, agent Agent, models []ModelOption) {
	if e.groupModelKey(sessionKey) == "" {
		return
	}
	if binding, ok := e.groupModelBinding(sessionKey); ok {
		cb.Markdown(e.i18n.Tf(MsgModelGroupCurrent, binding.Model))
		if binding.Provider != groupModelProvider(agent) {
			cb.Markdown(e.i18n.T(MsgModelGroupProviderChanged))
		}
	} else {
		cb.Markdown(e.i18n.T(MsgModelGroupDefault))
	}
	if supportsGroupModel(agent) {
		choices := []CardSelectOption{{Text: e.i18n.T(MsgModelGroupResetButton), Value: "cmd:/model group reset"}}
		selected := choices[0].Value
		binding, bound := e.groupModelBinding(sessionKey)
		if bound {
			selected = "" // a removed model must never look like inheritance
		}
		for _, model := range models {
			value := "cmd:/model group " + model.Name
			choices = append(choices, CardSelectOption{Text: model.Name, Value: value})
			if binding.Model == model.Name {
				selected = value
			}
		}
		cb.Select(e.i18n.T(MsgModelGroupSelect), choices, selected)
	} else {
		cb.Markdown(e.i18n.T(MsgModelGroupUnsupported))
		cb.Buttons(CardButton{Text: e.i18n.T(MsgModelGroupResetButton), Value: "cmd:/model group reset"})
	}
	cb.Divider()
}
