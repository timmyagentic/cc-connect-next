package core

import (
	"fmt"
	"strings"
)

func (e *Engine) cmdSpeed(p Platform, msg *Message, args []string) {
	agent, sessions, key, err := e.commandContext(p, msg)
	if err != nil {
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgWsResolutionError, err))
		return
	}
	catalog, ok := agent.(ServiceTierCatalog)
	if !ok {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSpeedNotSupported))
		return
	}
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	// Serialize a change with steer and turn admission. No process is stopped
	// and the running turn retains the override it captured on admission.
	if state != nil {
		state.steerMu.Lock()
		defer state.steerMu.Unlock()
	}
	var as AgentSession
	if state != nil {
		state.mu.Lock()
		as = state.agentSession
		state.mu.Unlock()
	}
	if as != nil {
		if _, ok := as.(TurnOptionsSession); !ok {
			e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSpeedNotSupported))
			return
		}
	}
	// A live backend knows its account, configuration layers and model catalog.
	// Prefer that evidence over local bootstrap metadata whenever available.
	if live, supported := serviceTierCatalog(agent, as); supported {
		catalog = live
	}
	model := turnDefaults(agent, as).Model
	if model == "" && as != nil {
		if getter, ok := as.(interface{ GetModel() string }); ok {
			model = getter.GetModel()
		}
	}
	caps, err := catalog.ServiceTierCapabilities(model)
	if err != nil || len(caps.Options) == 0 {
		if err == nil {
			err = fmt.Errorf("model catalog declares no service tiers")
		}
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSpeedUnavailable, redactFeedbackText(err.Error())))
		return
	}
	session := sessions.FindByID(sessions.ActiveSessionID(msg.SessionKey))
	var choices []string
	for _, option := range caps.Options {
		choices = append(choices, option.Name)
	}
	if len(args) > 0 {
		choice := strings.ToLower(strings.TrimSpace(args[0]))
		tier := ""
		if len(args) == 1 {
			for _, option := range caps.Options {
				if choice == option.Name {
					tier = option.Value
				}
			}
		}
		if tier == "" {
			e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSpeedInvalid, strings.Join(choices, ", ")))
			return
		}
		if session == nil {
			session = sessions.GetOrCreateActive(msg.SessionKey)
		}
		if err := sessions.SetServiceTier(session, tier); err != nil {
			e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSpeedSaveFailed, redactFeedbackText(err.Error())))
			return
		}
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSpeedChanged, choice)+"\n"+e.speedScope(sessions))
		return
	}
	tier := ""
	if session != nil {
		tier = sessions.ServiceTier(session)
	}
	if tier == "" {
		tier = caps.Default
		if defaults := turnDefaults(agent, as); defaults.ServiceTier != "" {
			tier = defaults.ServiceTier
		}
	}
	text := e.i18n.Tf(MsgSpeedCurrent, speedName(caps, tier, e.i18n.T(MsgSpeedInherited)), caps.Model, strings.Join(choices, ", "))
	if state != nil && session != nil && session.Busy() {
		state.mu.Lock()
		profile, active := state.activeAnswerProfile, state.activeServiceTier
		state.mu.Unlock()
		options, _ := e.applyTurnOverrides(turnDefaults(agent, as), profile, active)
		if options.ServiceTier == "" {
			options.ServiceTier = caps.Default
		}
		text += "\n" + e.i18n.Tf(MsgSpeedActive, speedName(caps, options.ServiceTier, e.i18n.T(MsgSpeedInherited)))
	}
	e.reply(p, msg.ReplyCtx, text+"\n"+e.speedScope(sessions))
}

func (e *Engine) speedScope(sessions *SessionManager) string {
	if sessions.storePath == "" {
		return e.i18n.T(MsgSpeedScopeMemory)
	}
	return e.i18n.T(MsgSpeedScopeSaved)
}

func speedName(caps ServiceTierCapabilities, tier, inherited string) string {
	for _, option := range caps.Options {
		if tier == option.Value || tier == option.Name {
			return option.Name
		}
	}
	if tier == "" {
		return inherited
	}
	return tier
}
