package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Model commands have exactly two scopes: the verified chat, or the project's
// global default.
func (e *Engine) cmdModel(p Platform, msg *Message, args []string) {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "default":
			e.cmdDefaultModel(p, msg, args[1:])
			return
		}
	}
	agent, _, _, err := e.commandContext(p, msg)
	if err != nil {
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgWsResolutionError, err))
		return
	}
	e.cmdChatModel(p, msg, agent, args)
}

func (e *Engine) cmdDefaultModel(p Platform, msg *Message, args []string) {
	if len(args) == 0 {
		e.replyModelCard(p, msg.ReplyCtx, e.renderModelDefaultCard())
		return
	}
	if !e.isAdmin(msg.UserID) {
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgAdminRequired, "/model default"))
		return
	}
	if len(args) != 1 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgModelDefaultUsage))
		return
	}
	switcher, ok := e.agent.(ModelSwitcher)
	if !ok {
		e.replyModelCard(p, msg.ReplyCtx, e.renderModelDefaultCard())
		return
	}
	target := strings.TrimSpace(args[0])
	if target == "" {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgModelDefaultUsage))
		return
	}
	if modelSwitchNeedsLookup(target) {
		ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
		models := selectableModels(ctx, e.agent, switcher)
		cancel()
		if number, err := strconv.Atoi(target); err == nil && (number < 1 || number > len(models)) {
			e.reply(p, msg.ReplyCtx, e.i18n.T(MsgModelDefaultUsage))
			return
		}
		target = resolveModelSwitchTarget(target, models)
	}
	target, err := e.switchModel(target)
	message := e.i18n.Tf(MsgModelDefaultChanged, target)
	if err != nil {
		message = e.i18n.Tf(MsgModelChangeFailed, err)
	} else {
		e.interactiveMu.Lock()
		pool := e.workspacePool
		e.interactiveMu.Unlock()
		if pool != nil {
			for _, workspace := range pool.All() {
				workspace.mu.Lock()
				if workspace.agent != nil {
					e.syncModelDefault(workspace.agent)
				}
				workspace.mu.Unlock()
			}
		}
	}
	e.replyModelCard(p, msg.ReplyCtx, modelResultCard(e.renderModelDefaultCard(), message))
}

// Workspaces do not own a model default. Refresh their runtime provider entry
// as well as SetModel, since provider-backed agents may prefer provider.Model.
func (e *Engine) syncModelDefault(agent Agent) {
	if agent == e.agent || modelProvider(agent) != modelProvider(e.agent) {
		return
	}
	source, sourceOK := e.agent.(ModelSwitcher)
	target, targetOK := agent.(ModelSwitcher)
	if !sourceOK || !targetOK {
		return
	}
	model := source.GetModel()
	if target.GetModel() == model {
		return
	}
	if ps, ok := agent.(ProviderSwitcher); ok {
		if active := ps.GetActiveProvider(); active != nil {
			if providers, found := SetProviderModel(ps.ListProviders(), active.Name, model); found {
				ps.SetProviders(providers)
			}
		}
	}
	target.SetModel(model)
}

// Called only between turns, after the previous Send has returned. The regular
// startup path can resume a process-bound backend with its new default while
// retaining this state and its queue. Per-turn backends need no reconnect.
func (e *Engine) ensureBackendForTurn(state *interactiveState, key string, session *Session, sessions *SessionManager, queued queuedMessage) error {
	state.mu.Lock()
	agent := state.agent
	stopped := state.stopped
	state.mu.Unlock()
	if stopped {
		return context.Canceled
	}
	if agent == nil || supportsChatModel(agent) {
		return nil
	}
	current, _ := e.getOrCreateInteractiveState(key, queued.platform, queued.replyCtx, session, sessions, agent, queued.msgSessionKey, state)
	if current != state {
		return context.Canceled
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.stopped {
		return context.Canceled
	}
	if state.startError != nil {
		return state.startError
	}
	if state.agentSession == nil {
		return fmt.Errorf("model refresh did not start a backend session")
	}
	return nil
}

func (e *Engine) reportModelResumeFailure(state *interactiveState, queued queuedMessage, err error) {
	if !errors.Is(err, context.Canceled) {
		e.reply(queued.platform, queued.replyCtx, e.i18n.T(MsgFailedToStartAgentSession))
	}
	e.notifyDroppedQueuedMessages(state, err)
}

func modelResultCard(card *Card, message string) *Card {
	card.Elements = append([]CardElement{CardMarkdown{Content: message}}, card.Elements...)
	return card
}

// Retain selectable controls on platforms with inline buttons but no rich cards.
// Both projections dispatch the same stable, permission-checked commands.
func (e *Engine) replyModelCard(p Platform, replyCtx any, card *Card) {
	if supportsCards(p) {
		e.replyWithCard(p, replyCtx, card)
		return
	}
	var buttons [][]ButtonOption
	for _, element := range card.Elements {
		switch item := element.(type) {
		case CardSelect:
			for _, choice := range item.Options {
				buttons = append(buttons, []ButtonOption{{Text: choice.Text, Data: choice.Value}})
			}
		case CardActions:
			var row []ButtonOption
			for _, button := range item.Buttons {
				row = append(row, ButtonOption{Text: button.Text, Data: button.Value})
			}
			buttons = append(buttons, row)
		}
	}
	e.replyWithButtons(p, replyCtx, card.RenderText(), buttons)
}

func (e *Engine) renderModelCard(sessionKey string) *Card {
	agent := e.agent
	if sessionKey != "" {
		agent, _ = e.sessionContextForKey(sessionKey)
	}
	return e.renderModelCardForAgent(sessionKey, agent)
}

func (e *Engine) renderModelCardForAgent(sessionKey string, agent Agent) *Card {
	e.syncModelDefault(agent)
	cb := NewCard().Title(e.i18n.T(MsgCardTitleModel), "indigo")
	switcher, ok := agent.(ModelSwitcher)
	if !ok {
		cb.Markdown(e.i18n.T(MsgModelNotSupported))
	} else if e.chatKey(sessionKey) == "" {
		cb.Markdown(e.i18n.T(MsgModelChatOnly))
	} else {
		ctx, cancel := context.WithTimeout(e.ctx, 3*time.Second)
		models := selectableModels(ctx, agent, switcher)
		cancel()
		e.chatModelCard(cb, sessionKey, agent, models)
	}
	return cb.Buttons(DefaultBtn(e.i18n.T(MsgModelDefaultButton), "cmd:/model default"), e.cardBackButton()).
		Note(e.i18n.T(MsgModelUsage)).Build()
}

func (e *Engine) renderModelDefaultCard() *Card {
	cb := NewCard().Title(e.i18n.T(MsgModelDefaultTitle), "indigo").Markdown(e.i18n.T(MsgModelScopeProject))
	switcher, ok := e.agent.(ModelSwitcher)
	if !ok {
		cb.Markdown(e.i18n.T(MsgModelNotSupported))
	} else {
		ctx, cancel := context.WithTimeout(e.ctx, 3*time.Second)
		models := selectableModels(ctx, e.agent, switcher)
		cancel()
		current := switcher.GetModel()
		if current == "" {
			cb.Markdown(e.i18n.T(MsgModelDefault))
		} else {
			cb.Markdown(e.i18n.Tf(MsgModelCurrent, current))
		}
		var choices []CardSelectOption
		selected := ""
		for _, model := range models {
			value := "cmd:/model default " + model.Name
			label := model.Name
			if model.Alias != "" {
				label = model.Alias + " - " + model.Name
			}
			choices = append(choices, CardSelectOption{Text: label, Value: value})
			if current == model.Name {
				selected = value
			}
		}
		if len(choices) > 0 {
			cb.Select(e.i18n.T(MsgModelSelectPlaceholder), choices, selected)
		}
	}
	return cb.Buttons(DefaultBtn(e.i18n.T(MsgCardBack), "cmd:/model")).
		Note(e.i18n.T(MsgModelDefaultUsage)).Build()
}
