package core

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

func (e *Engine) observeUnreadReminderMessage(p Platform, msg *Message) {
	notifier, ok := p.(TurnCompletionNotifier)
	if !ok || !notifier.UnreadRemindersEnabled() {
		return
	}
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()
	if err := notifier.ObserveReminderMessage(ctx, msg); err != nil {
		slog.Warn("unread reminder: could not cancel pending notification", "platform", p.Name(), "error", err)
	}
}

func (completion *turnCompletion) notifyUnreadReminder(response preparedTurnResponse) {
	if response.isSilent || completion.event.Error != nil {
		return
	}
	p := completion.platform
	notifier, ok := p.(TurnCompletionNotifier)
	if !ok || !notifier.UnreadRemindersEnabled() {
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
	summary := unreadReminderSummary(label, response.base)
	notice := TurnCompletionNotice{
		UserID: userID, SessionKey: sessionKey, TriggerMessageID: completion.msgID,
		Summary: summary, Text: t.engine.i18n.Tf(MsgUnreadCompletion, summary),
		ReminderText: t.engine.i18n.T(MsgUnreadReminder),
	}
	// A reminder is auxiliary: delivery/storage failure cannot turn a successfully
	// delivered answer into a failed turn, or cause the agent to run again.
	ctx, cancel := context.WithTimeout(completion.terminalDeliveryCtx, 15*time.Second)
	defer cancel()
	if err := notifier.NotifyTurnCompleted(ctx, completion.replyCtx, notice); err != nil {
		slog.Warn("unread reminder: completion notice unavailable", "platform", p.Name(), "error", err)
	}
}

func unreadReminderSummary(project, answer string) string {
	// The notification is plain text, deliberately bounded, and never includes
	// tool calls, hidden reasoning, or the full conversation transcript.
	text := strings.Join(strings.Fields(project+" · "+answer), " ")
	runes := []rune(text)
	if len(runes) > 160 {
		return string(runes[:159]) + "…"
	}
	return text
}
