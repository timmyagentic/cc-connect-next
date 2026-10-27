package core

import (
	"fmt"
	"slices"
	"strings"
)

func (e *Engine) groupReasoningBinding(sessionKey string) (GroupReasoningBinding, bool) {
	if e.projectState == nil || e.groupModelKey(sessionKey) == "" {
		return GroupReasoningBinding{}, false
	}
	return e.projectState.GroupReasoningBinding(e.groupModelKey(sessionKey))
}

func supportsGroupReasoning(agent Agent) bool {
	supporter, ok := agent.(PerTurnReasoningEffortSupporter)
	switcher, hasCatalog := agent.(ReasoningEffortSwitcher)
	return ok && supporter.SupportsPerTurnReasoningEffort() && hasCatalog && len(switcher.AvailableReasoningEfforts()) > 0
}

func (e *Engine) groupDefaultReasoning(agent Agent, sessionKey string, backend AgentSession) (string, error) {
	binding, ok := e.groupReasoningBinding(sessionKey)
	if !ok {
		return "", nil
	}
	fail := func(key MsgKey) (string, error) { return "", e.groupModelFailure(key) }
	if !supportsGroupReasoning(agent) {
		return fail(MsgReasoningGroupUnsupported)
	}
	if binding.Provider != groupModelProvider(agent) {
		return fail(MsgReasoningGroupProviderChanged)
	}
	if !slices.Contains(agent.(ReasoningEffortSwitcher).AvailableReasoningEfforts(), binding.Effort) {
		return fail(MsgReasoningGroupUnsupported)
	}
	if backend != nil {
		if _, ok := backend.(TurnOptionsSession); !ok {
			return fail(MsgReasoningGroupUnsupported)
		}
		if actual, ok := backend.(ModelProviderSession); ok {
			if binding.Provider != agent.Name()+":"+actual.ModelProviderName() {
				return fail(MsgReasoningGroupProviderChanged)
			}
		} else if _, canSwitch := agent.(ProviderSwitcher); canSwitch {
			return fail(MsgReasoningGroupUnsupported)
		}
	}
	return binding.Effort, nil
}

func (e *Engine) setGroupReasoning(agent Agent, sessionKey, input string) (string, error) {
	key := e.groupModelKey(sessionKey)
	if key == "" {
		return "", fmt.Errorf("%s", e.i18n.T(MsgReasoningGroupOnly))
	}
	if e.projectState == nil {
		return "", fmt.Errorf("%s", e.i18n.T(MsgReasoningGroupStorageUnavailable))
	}
	target := strings.ToLower(strings.TrimSpace(input))
	if target == "reset" {
		return "", e.projectState.SaveGroupReasoningBinding(key, GroupReasoningBinding{})
	}
	if !supportsGroupReasoning(agent) {
		return "", fmt.Errorf("%s", e.i18n.T(MsgReasoningGroupUnsupported))
	}
	if !slices.Contains(agent.(ReasoningEffortSwitcher).AvailableReasoningEfforts(), target) {
		return "", fmt.Errorf("%s", e.i18n.T(MsgReasoningGroupUsage))
	}
	if err := e.projectState.SaveGroupReasoningBinding(key, GroupReasoningBinding{Effort: target, Provider: groupModelProvider(agent)}); err != nil {
		return "", err
	}
	return target, nil
}

func (e *Engine) cmdGroupReasoning(p Platform, msg *Message, agent Agent, args []string) {
	if e.groupModelKey(msg.SessionKey) == "" {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgReasoningGroupOnly))
		return
	}
	if len(args) > 1 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgReasoningGroupUsage))
		return
	}
	var notice string
	if len(args) == 1 {
		effort, err := e.setGroupReasoning(agent, msg.SessionKey, args[0])
		if err != nil {
			e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgReasoningGroupFailed, err))
			return
		}
		notice = e.i18n.T(MsgReasoningGroupReset)
		if effort != "" {
			notice = e.i18n.Tf(MsgReasoningGroupChanged, effort)
		}
	}
	card := e.renderReasoningCardForAgent(msg.SessionKey, agent)
	if notice != "" {
		card.Elements = append([]CardElement{CardMarkdown{Content: notice}}, card.Elements...)
	}
	e.replyWithCard(p, msg.ReplyCtx, card)
}

func (e *Engine) groupReasoningCard(cb *CardBuilder, sessionKey string, agent Agent) {
	if e.groupModelKey(sessionKey) == "" {
		return
	}
	binding, bound := e.groupReasoningBinding(sessionKey)
	if bound {
		cb.Markdown(e.i18n.Tf(MsgReasoningGroupCurrent, binding.Effort))
		if binding.Provider != groupModelProvider(agent) {
			cb.Markdown(e.i18n.T(MsgReasoningGroupProviderChanged))
		}
	} else {
		cb.Markdown(e.i18n.T(MsgReasoningGroupDefault))
	}
	choices := []CardSelectOption{{Text: e.i18n.T(MsgModelGroupResetButton), Value: "cmd:/reasoning group reset"}}
	selected := choices[0].Value
	if bound {
		selected = ""
	}
	if supportsGroupReasoning(agent) {
		for _, effort := range agent.(ReasoningEffortSwitcher).AvailableReasoningEfforts() {
			value := "cmd:/reasoning group " + effort
			choices = append(choices, CardSelectOption{Text: effort, Value: value})
			if bound && binding.Effort == effort {
				selected = value
			}
		}
		cb.Select(e.i18n.T(MsgReasoningGroupSelect), choices, selected)
	} else {
		cb.Markdown(e.i18n.T(MsgReasoningGroupUnsupported))
		cb.Buttons(CardButton{Text: e.i18n.T(MsgModelGroupResetButton), Value: choices[0].Value})
	}
	cb.Divider()
}
