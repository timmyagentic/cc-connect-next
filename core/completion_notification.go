package core

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

func (completion *turnCompletion) notifyCompletion(response preparedTurnResponse) {
	if response.isSilent || completion.event.Error != nil {
		return
	}
	p := completion.platform
	notifier, ok := p.(TurnCompletionNotifier)
	if !ok || !notifier.CompletionNotificationsEnabled() {
		return
	}
	t := completion.processor
	t.state.mu.Lock()
	userID, sessionKey := t.state.currentReplyUserID, t.state.currentSessionKey
	t.state.mu.Unlock()
	if userID == "" || completion.msgID == "" || t.state.isStopped() || t.state.shouldDiscardTurn(completion.msgID) {
		return
	}
	label := t.engine.name
	if meta := t.sessions.GetUserMeta(sessionKey); meta != nil && meta.ChatName != "" {
		label += " · " + meta.ChatName
	}
	summary := completionSummary(label, response.base)
	notice := TurnCompletionNotice{
		UserID: userID, SessionKey: sessionKey, TriggerMessageID: completion.msgID,
		Summary: summary, Text: t.engine.i18n.Tf(MsgCompletionNotification, summary),
		LinkLabel: t.engine.i18n.T(MsgCompletionOpenConversation),
	}
	// A completion notification is auxiliary: delivery/storage failure cannot turn a successfully
	// delivered answer into a failed turn, or cause the agent to run again.
	ctx, cancel := context.WithTimeout(completion.terminalDeliveryCtx, 15*time.Second)
	defer cancel()
	if err := notifier.NotifyTurnCompleted(ctx, completion.replyCtx, notice); err != nil {
		slog.Warn("private completion notification unavailable", "platform", p.Name(), "error", err)
	}
}

func completionSummary(project, answer string) string {
	// The notification is plain text, deliberately bounded, and never includes
	// tool calls, hidden reasoning, or the full conversation transcript.
	text := strings.Join(strings.Fields(project+" · "+answer), " ")
	runes := []rune(text)
	if len(runes) > 160 {
		return string(runes[:159]) + "…"
	}
	return text
}
