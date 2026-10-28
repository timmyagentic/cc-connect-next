package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Only trusted, localized routing failures may bypass generic backend-error
// redaction to explain how to recover on a rich-card platform.
type chatModelError struct{ message string }

func (err *chatModelError) Error() string { return err.message }

func (e *Engine) chatModelFailure(key MsgKey) error {
	return &chatModelError{message: e.i18n.T(key)}
}

// rememberChatScope accepts only platform-verified chat identity. Keep
// groups and direct messages in separate identity namespaces.
// The project owns this metadata, so bots sharing a directory remain isolated.
func (e *Engine) rememberChatScope(p Platform, msg *Message) error {
	if msg.IsDirect {
		key := ""
		if strings.TrimSpace(msg.UserID) != "" {
			key = p.Name() + ":dm:" + msg.UserID
		}
		return e.sessions.UpdateChatKey(msg.SessionKey, key)
	}
	if msg.GroupID == "" {
		// Only the verified owner may reuse a persisted DM scope; a
		// forwarded card cannot borrow it.
		if key := e.chatKey(msg.SessionKey); msg.IsCardAction && strings.HasPrefix(key, p.Name()+":dm:") && key != p.Name()+":dm:"+msg.UserID {
			return e.chatModelFailure(MsgModelChatOnly)
		}
		return nil // callbacks may use previously verified, persisted routing
	}
	return e.sessions.UpdateChatKey(msg.SessionKey, p.Name()+":"+msg.GroupID)
}

func (e *Engine) chatKey(sessionKey string) string {
	if meta := e.sessions.GetUserMeta(sessionKey); meta != nil {
		return meta.ChatKey
	}
	return ""
}

func (e *Engine) chatModelBinding(sessionKey string) (ChatModelBinding, bool) {
	if e.projectState == nil {
		return ChatModelBinding{}, false
	}
	key := e.chatKey(sessionKey)
	if key == "" {
		return ChatModelBinding{}, false
	}
	return e.projectState.ChatModelBinding(key)
}

func modelProvider(agent Agent) string {
	provider := ""
	if ps, ok := agent.(ProviderSwitcher); ok {
		if active := ps.GetActiveProvider(); active != nil {
			provider = active.Name
		}
	}
	return agent.Name() + ":" + provider
}

func supportsChatModel(agent Agent) bool {
	supporter, ok := agent.(PerTurnModelSupporter)
	return ok && supporter.SupportsPerTurnModel()
}

func (e *Engine) chatDefaultModel(agent Agent, sessionKey string, backend AgentSession) (string, error) {
	binding, ok := e.chatModelBinding(sessionKey)
	if !ok {
		return "", nil
	}
	if !supportsChatModel(agent) {
		return "", e.chatModelFailure(MsgModelChatUnsupported)
	}
	if binding.Provider != modelProvider(agent) {
		return "", e.chatModelFailure(MsgModelChatProviderChanged)
	}
	if backend != nil {
		if _, ok := backend.(TurnOptionsSession); !ok {
			return "", e.chatModelFailure(MsgModelChatUnsupported)
		}
		if actual, ok := backend.(ModelProviderSession); ok {
			if binding.Provider != agent.Name()+":"+actual.ModelProviderName() {
				return "", e.chatModelFailure(MsgModelChatSessionProviderChanged)
			}
		} else if _, canSwitch := agent.(ProviderSwitcher); canSwitch {
			return "", e.chatModelFailure(MsgModelChatUnsupported)
		}
	}
	return binding.Model, nil
}

// A chat selection is a default layer, not an explicit task override. Resolve
// it for each dequeued turn, preserving explicit task/profile model precedence.
func (e *Engine) prepareSessionTurnOverrides(agent Agent, sessions *SessionManager, session *Session, backend AgentSession, sessionKey string, profile AnswerProfileName, model, reasoning, tier string) (agentTurnOverrides, error) {
	profileModel, profileReasoning := "", ""
	if configured, ok := e.answerProfile(profile); ok {
		profileModel = strings.TrimSpace(configured.Model)
		profileReasoning = strings.TrimSpace(configured.ReasoningEffort)
	}
	if strings.TrimSpace(model) == "" && profileModel == "" {
		var err error
		model, err = e.chatDefaultModel(agent, sessionKey, backend)
		if err != nil {
			return agentTurnOverrides{}, err
		}
	}
	if strings.TrimSpace(reasoning) == "" && profileReasoning == "" {
		var err error
		reasoning, err = e.groupDefaultReasoning(agent, sessionKey, backend)
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
		state.activeReasoningEffortOverride = overrides.reasoningEffort
		state.mu.Unlock()
	}
	return overrides, err
}

func (e *Engine) setChatModel(agent Agent, sessionKey, input string) (string, error) {
	key := e.chatKey(sessionKey)
	if key == "" {
		return "", fmt.Errorf("%s", e.i18n.T(MsgModelChatOnly))
	}
	if e.projectState == nil {
		return "", fmt.Errorf("%s", e.i18n.T(MsgModelChatStorageUnavailable))
	}
	if strings.EqualFold(input, "reset") {
		return "", e.projectState.SaveChatModelBinding(key, ChatModelBinding{})
	}
	switcher, ok := agent.(ModelSwitcher)
	if !ok || !supportsChatModel(agent) {
		return "", fmt.Errorf("%s", e.i18n.T(MsgModelChatUnsupported))
	}
	ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	defer cancel()
	provider := modelProvider(agent)
	models := selectableModels(ctx, agent, switcher)
	if provider != modelProvider(agent) {
		return "", fmt.Errorf("%s", e.i18n.T(MsgModelChatProviderChanged))
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
		return "", fmt.Errorf("%s", e.i18n.Tf(MsgModelChatUnknown, input))
	}
	if err := e.projectState.SaveChatModelBinding(key, ChatModelBinding{Model: target, Provider: provider}); err != nil {
		return "", err
	}
	return target, nil
}

// AvailableModels can be a native catalog snapshot. Include explicit provider
// models as well, so a local catalog cannot hide configured custom models.
func selectableModels(ctx context.Context, agent Agent, switcher ModelSwitcher) []ModelOption {
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

func (e *Engine) cmdChatModel(p Platform, msg *Message, agent Agent, args []string) {
	if len(args) == 0 {
		e.replyModelCard(p, msg.ReplyCtx, e.renderModelCardForAgent(msg.SessionKey, agent))
		return
	}
	if len(args) != 1 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgModelChatUsage))
		return
	}
	model, err := e.setChatModel(agent, msg.SessionKey, args[0])
	if err != nil {
		e.replyModelCard(p, msg.ReplyCtx, modelResultCard(e.renderModelCardForAgent(msg.SessionKey, agent), e.i18n.Tf(MsgModelChangeFailed, err)))
		return
	}
	message := e.i18n.T(MsgModelChatReset)
	if model != "" {
		message = e.i18n.Tf(MsgModelChatChanged, model)
	}
	card := e.renderModelCardForAgent(msg.SessionKey, agent)
	e.replyModelCard(p, msg.ReplyCtx, modelResultCard(card, message))
}

// chatModelCard renders the only model selection on the current-chat card.
func (e *Engine) chatModelCard(cb *CardBuilder, sessionKey string, agent Agent, models []ModelOption) {
	if e.chatKey(sessionKey) == "" {
		return
	}
	if binding, ok := e.chatModelBinding(sessionKey); ok {
		cb.Markdown(e.i18n.Tf(MsgModelChatCurrent, binding.Model))
		if binding.Provider != modelProvider(agent) {
			cb.Markdown(e.i18n.T(MsgModelChatProviderChanged))
		}
	} else {
		current := agent.(ModelSwitcher).GetModel()
		if current == "" {
			current = e.i18n.T(MsgModelDefault)
		}
		cb.Markdown(e.i18n.Tf(MsgModelChatDefault, current))
	}
	if supportsChatModel(agent) {
		choices := []CardSelectOption{{Text: e.i18n.T(MsgModelChatResetButton), Value: "cmd:/model reset"}}
		selected := choices[0].Value
		binding, bound := e.chatModelBinding(sessionKey)
		if bound {
			selected = "" // a removed model must never look like inheritance
		}
		for _, model := range models {
			value := "cmd:/model " + model.Name
			choices = append(choices, CardSelectOption{Text: model.Name, Value: value})
			if binding.Model == model.Name {
				selected = value
			}
		}
		cb.Select(e.i18n.T(MsgModelChatSelect), choices, selected)
	} else {
		cb.Markdown(e.i18n.T(MsgModelChatUnsupported))
		cb.Buttons(CardButton{Text: e.i18n.T(MsgModelChatResetButton), Value: "cmd:/model reset"})
	}
}
