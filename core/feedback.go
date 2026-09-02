package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

// Feedback remains a host-owned product flow over the reusable Foundation:
// CC Connect Next chooses and redacts the structured context, owns cards,
// localization and fallback, and treats an explicit chat command or card action
// as approval to submit immediately. Proactive offers remain zero-network until
// the user acts. The local-Agent CLI keeps its separate preview/token contract.

const (
	DefaultFeedbackEndpoint = "https://cc-connect-feedback.qianbi3956001.workers.dev/v1/feedback"

	feedbackSubmitTimeout     = 15 * time.Second
	feedbackPendingTTL        = 10 * time.Minute
	feedbackPendingMax        = 64
	feedbackErrorHintCooldown = 10 * time.Minute
	feedbackErrorAttachWindow = 30 * time.Minute
	feedbackContextWindow     = 15 * time.Minute
)

type feedbackError struct {
	Text string
	At   time.Time
}

type pendingFeedback struct {
	Draft      appfeatures.FeedbackDraft
	At         time.Time
	SessionKey string
	UserID     string
	AgentOnly  bool
}

type feedbackSubmitFunc func(context.Context, appfeatures.FeedbackDraft, bool) (appfeatures.FeedbackReceipt, error)

// SetFeedbackConfig wires the [feedback] config into the engine.
func (e *Engine) SetFeedbackConfig(enabled bool, endpoint string) {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	e.feedbackEnabled = enabled
	e.feedbackEndpoint = strings.TrimSpace(endpoint)
}

// SetFeedbackCapabilityGaps records config keys this build does not consume;
// they seed proactive offers and a bare /feedback draft.
func (e *Engine) SetFeedbackCapabilityGaps(keys []string) {
	cleaned := make([]string, 0, len(keys))
	for _, key := range keys {
		if key = strings.TrimSpace(key); key != "" {
			cleaned = append(cleaned, key)
		}
	}
	sort.Strings(cleaned)
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	e.feedbackGapKeys = cleaned
}

func (e *Engine) FeedbackCapabilityGaps() []string {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	return append([]string(nil), e.feedbackGapKeys...)
}

func (e *Engine) feedbackActive() bool {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	return e.feedbackEnabled && e.feedbackEndpoint != ""
}

// cmdFeedback treats the inbound command or card action as the explicit user
// approval. Ordinary /feedback invocations build and submit immediately. The
// internal submit-token form is used only by proactive zero-network offers so
// the user's later click submits the exact prepared Draft in one action.
func (e *Engine) cmdFeedback(platform Platform, message *Message, raw string) {
	if !e.feedbackActive() {
		if message.IsCardAction {
			e.deliverFeedbackSubmitResult(platform, message, false)
			return
		}
		e.reply(platform, message.ReplyCtx, e.i18n.T(MsgFeedbackDisabled))
		return
	}
	key := feedbackSessionKey(platform, message)
	argument := strings.TrimSpace(raw)
	if token, handled := feedbackOfferToken(argument); handled {
		e.submitPendingFeedback(platform, message, key, message.UserID, token)
		return
	}
	if isLegacyFeedbackControl(argument) {
		e.reply(platform, message.ReplyCtx, e.i18n.T(MsgFeedbackUsage))
		return
	}

	draft, err := e.buildFeedbackDraft(key, argument, nil)
	if err != nil {
		e.reply(platform, message.ReplyCtx, e.i18n.T(MsgFeedbackUsage))
		return
	}
	_, err = e.submitFeedbackDraft(e.ctx, draft)
	e.deliverFeedbackSubmitResult(platform, message, err == nil)
}

func feedbackOfferToken(argument string) (token string, handled bool) {
	fields := strings.Fields(argument)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "submit-token") {
		return "", false
	}
	if len(fields) != 2 {
		return "", true
	}
	return fields[1], true
}

func isLegacyFeedbackControl(argument string) bool {
	fields := strings.Fields(argument)
	if len(fields) == 0 || len(fields) > 2 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "confirm", "submit", "cancel", "dismiss":
		if len(fields) == 1 {
			return true
		}
		decoded, err := hex.DecodeString(fields[1])
		return err == nil && len(decoded) == 16
	default:
		return false
	}
}

func (e *Engine) buildFeedbackDraft(sessionKey, description string, overrideGaps []string) (appfeatures.FeedbackDraft, error) {
	e.feedbackMu.Lock()
	var recent *feedbackError
	if value := e.feedbackErrors[sessionKey]; value != nil {
		copy := *value
		recent = &copy
	}
	gaps := append([]string(nil), e.feedbackGapKeys...)
	e.feedbackMu.Unlock()
	if overrideGaps != nil {
		gaps = append([]string(nil), overrideGaps...)
	}
	if recent != nil && time.Since(recent.At) > feedbackErrorAttachWindow {
		recent = nil
	}

	previousUser, previousAssistant := e.feedbackRelatedContext(sessionKey)
	input := appfeatures.FeedbackContext{
		Description:               description,
		PreviousUserMessage:       previousUser,
		PreviousAssistantResponse: previousAssistant,
		CapabilityGaps:            gaps,
		Version:                   CurrentVersion,
		Agent:                     e.agent.Name(),
	}
	if recent != nil {
		input.RecentError = recent.Text
		input.RecentErrorAt = recent.At
	}
	return appfeatures.BuildFeedbackDraft(input)
}

func (e *Engine) feedbackRelatedContext(sessionKey string) (previousUser, previousAssistant string) {
	if strings.TrimSpace(sessionKey) == "" {
		return "", ""
	}
	_, sessions := e.sessionContextForKey(sessionKey)
	if sessions == nil {
		return "", ""
	}
	activeID := sessions.ActiveSessionID(sessionKey)
	if activeID == "" {
		return "", ""
	}
	session := sessions.FindByID(activeID)
	if session == nil {
		return "", ""
	}
	entries := session.GetHistory(8)
	now := time.Now()
	latestIndex := -1
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		if !entry.Timestamp.IsZero() && now.Sub(entry.Timestamp) > feedbackContextWindow {
			continue
		}
		content := strings.TrimSpace(entry.Content)
		if content == "" || strings.HasPrefix(strings.ToLower(content), "/feedback") {
			continue
		}
		if entry.Role == "assistant" || entry.Role == "user" {
			latestIndex = index
			break
		}
	}
	if latestIndex < 0 {
		return "", ""
	}
	latest := entries[latestIndex]
	if latest.Role == "user" {
		return strings.TrimSpace(latest.Content), ""
	}
	previousAssistant = strings.TrimSpace(latest.Content)
	for index := latestIndex - 1; index >= 0; index-- {
		entry := entries[index]
		if !entry.Timestamp.IsZero() && now.Sub(entry.Timestamp) > feedbackContextWindow {
			continue
		}
		content := strings.TrimSpace(entry.Content)
		if entry.Role == "user" && content != "" && !strings.HasPrefix(strings.ToLower(content), "/feedback") {
			previousUser = content
			break
		}
	}
	return previousUser, previousAssistant
}

func (e *Engine) rememberPendingFeedback(sessionKey, userID string, draft appfeatures.FeedbackDraft) (string, error) {
	token, _, err := e.rememberPendingFeedbackForCaller(sessionKey, userID, false, draft)
	return token, err
}

func (e *Engine) rememberPendingFeedbackForCaller(sessionKey, userID string, agentOnly bool, draft appfeatures.FeedbackDraft) (string, time.Time, error) {
	if sessionKey == "" {
		return "", time.Time{}, fmt.Errorf("feedback session is required")
	}
	token, err := newPendingActionToken()
	if err != nil {
		return "", time.Time{}, err
	}
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	if e.feedbackPending == nil {
		e.feedbackPending = make(map[string]pendingFeedback)
	}
	now := time.Now()
	e.prunePendingFeedbackLocked(now)
	for len(e.feedbackPending) >= feedbackPendingMax {
		e.evictOldestFeedbackLocked()
	}
	e.feedbackPending[token] = pendingFeedback{
		Draft: draft, At: now, SessionKey: sessionKey, UserID: userID, AgentOnly: agentOnly,
	}
	return token, now.Add(feedbackPendingTTL), nil
}

func (e *Engine) takePendingFeedback(sessionKey, userID, token string) (appfeatures.FeedbackDraft, bool) {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	now := time.Now()
	e.prunePendingFeedbackLocked(now)
	if token == "" {
		return appfeatures.FeedbackDraft{}, false
	}
	pending, exists := e.feedbackPending[token]
	if !exists {
		return appfeatures.FeedbackDraft{}, false
	}
	if pending.AgentOnly || pending.SessionKey != sessionKey || (pending.UserID != "" && pending.UserID != userID) {
		return appfeatures.FeedbackDraft{}, false
	}
	e.deletePendingFeedbackLocked(token)
	return pending.Draft, true
}

func (e *Engine) clearPendingFeedback(sessionKey, userID, token string) {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	e.prunePendingFeedbackLocked(time.Now())
	if pending, exists := e.feedbackPending[token]; exists && !pending.AgentOnly && pending.SessionKey == sessionKey && (pending.UserID == "" || pending.UserID == userID) {
		e.deletePendingFeedbackLocked(token)
	}
}

func (e *Engine) deletePendingFeedbackLocked(token string) {
	delete(e.feedbackPending, token)
}

func (e *Engine) prunePendingFeedbackLocked(now time.Time) {
	for token, pending := range e.feedbackPending {
		if now.Sub(pending.At) > feedbackPendingTTL {
			e.deletePendingFeedbackLocked(token)
		}
	}
}

func (e *Engine) evictOldestFeedbackLocked() {
	oldestToken := ""
	var oldest pendingFeedback
	for token, pending := range e.feedbackPending {
		if oldestToken == "" || pending.At.Before(oldest.At) {
			oldestToken, oldest = token, pending
		}
	}
	if oldestToken != "" {
		e.deletePendingFeedbackLocked(oldestToken)
	}
}

func (e *Engine) submitPendingFeedback(platform Platform, message *Message, sessionKey, userID, token string) {
	draft, exists := e.takePendingFeedback(sessionKey, userID, token)
	if !exists {
		if message.IsCardAction {
			e.deliverFeedbackSubmitResult(platform, message, false)
			return
		}
		e.reply(platform, message.ReplyCtx, e.i18n.T(MsgFeedbackPendingMissing))
		return
	}

	_, err := e.submitFeedbackDraft(e.ctx, draft)
	e.deliverFeedbackSubmitResult(platform, message, err == nil)
}

func (e *Engine) deliverFeedbackSubmitResult(platform Platform, message *Message, succeeded bool) {
	messageKey := MsgFeedbackSubmitted
	color := "green"
	if !succeeded {
		messageKey = MsgFeedbackSubmitFailed
		color = "red"
	}
	status := e.i18n.T(messageKey)
	if !message.IsCardAction {
		e.reply(platform, message.ReplyCtx, status)
		return
	}

	updater, ok := platform.(CardActionUpdater)
	if !ok {
		slog.Warn("feedback: platform cannot update clicked card", "platform", platform.Name(), "succeeded", succeeded)
		return
	}
	card := e.renderCardForPlatform(platform, NewCard().Title(status, color).Build())
	if err := updater.UpdateCard(e.ctx, message.ReplyCtx, card); err != nil {
		slog.Warn("feedback: result card update failed", "platform", platform.Name(), "succeeded", succeeded, "error", err)
	}
}

func (e *Engine) feedbackOfferCard(token string) *Card {
	return NewCard().
		Title(e.i18n.T(MsgFeedbackAskTitle), "orange").
		Markdown(e.i18n.T(MsgFeedbackHint)).
		Buttons(
			PrimaryBtn(e.i18n.T(MsgFeedbackBtnSubmit), "cmd:/feedback submit-token "+token),
		).
		Build()
}

func (e *Engine) deliverFeedbackOffer(platform Platform, replyCtx any, token string, proactive bool) error {
	if sender, ok := platform.(CardSender); ok {
		card := e.renderCardForPlatform(platform, e.feedbackOfferCard(token))
		ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
		defer cancel()
		var err error
		if proactive {
			err = sender.SendCard(ctx, replyCtx, card)
		} else {
			err = sender.ReplyCard(ctx, replyCtx, card)
		}
		if err == nil {
			return nil
		}
	}
	if sender, ok := platform.(InlineButtonSender); ok {
		buttons := [][]ButtonOption{{
			{Text: e.i18n.T(MsgFeedbackBtnSubmit), Data: "cmd:/feedback submit-token " + token},
		}}
		if err := sender.SendWithButtons(e.ctx, replyCtx, e.i18n.T(MsgFeedbackHint), buttons); err == nil {
			return nil
		}
	}
	text := e.i18n.T(MsgFeedbackHint) + "\n\n" + e.i18n.Tf(MsgFeedbackOfferSubmit, token)
	if proactive {
		return e.sendWithError(platform, replyCtx, text)
	}
	return e.replyWithError(platform, replyCtx, text)
}

func (e *Engine) recordFeedbackError(sessionKey, errText string) {
	if !e.feedbackActive() || strings.TrimSpace(errText) == "" {
		return
	}
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	if e.feedbackErrors == nil {
		e.feedbackErrors = make(map[string]*feedbackError)
	}
	e.feedbackErrors[sessionKey] = &feedbackError{Text: errText, At: time.Now()}
}

func (e *Engine) maybeSendFeedbackErrorHint(platform Platform, replyCtx any, sessionKey string) {
	if !e.feedbackActive() {
		return
	}
	e.feedbackMu.Lock()
	if e.feedbackErrorHintAt == nil {
		e.feedbackErrorHintAt = make(map[string]time.Time)
	}
	last := e.feedbackErrorHintAt[sessionKey]
	due := time.Since(last) >= feedbackErrorHintCooldown
	if due {
		e.feedbackErrorHintAt[sessionKey] = time.Now()
	}
	e.feedbackMu.Unlock()
	if !due {
		return
	}

	draft, err := e.buildFeedbackDraft(sessionKey, "", nil)
	if err != nil {
		return
	}
	token, err := e.rememberPendingFeedback(sessionKey, "", draft)
	if err != nil {
		return
	}
	if err := e.deliverFeedbackOffer(platform, replyCtx, token, true); err != nil {
		e.clearPendingFeedback(sessionKey, "", token)
		slog.Debug("feedback: proactive offer failed", "session_key", sessionKey, "error", err)
	}
}

func (e *Engine) NotifyCapabilityGap(keys []string) bool {
	if !e.feedbackActive() || len(keys) == 0 {
		return false
	}
	draft, err := e.buildFeedbackDraft("", "", keys)
	if err != nil {
		return false
	}
	_, delivered := e.notifyMostRecentSessionFn("feedback notice", func(sessionKey string, platform Platform, replyCtx any) error {
		token, err := e.rememberPendingFeedback(sessionKey, "", draft)
		if err != nil {
			return err
		}
		if err := e.deliverFeedbackOffer(platform, replyCtx, token, true); err != nil {
			e.clearPendingFeedback(sessionKey, "", token)
			return err
		}
		return nil
	})
	return delivered
}

func (e *Engine) feedbackHint() string {
	if !e.feedbackActive() {
		return ""
	}
	return "\n" + e.i18n.T(MsgFeedbackHint)
}

func feedbackSessionKey(platform Platform, message *Message) string {
	if message.SessionKey != "" {
		return message.SessionKey
	}
	return platform.Name() + ":" + message.UserID
}

// redactFeedbackText remains the package-local redaction entry used by the
// Agent Capability Manifest and runtime status projections.
func redactFeedbackText(text string) string {
	return appfeatures.RedactFeedbackText(text)
}

// ── FeedbackNotifier ─────────────────────────────────────────────

const (
	feedbackNoticeInitialDelay = 30 * time.Second
	feedbackNoticeInterval     = 10 * time.Minute
	feedbackNoticeStateFile    = "feedback_notice.json"
)

type FeedbackNotifier struct {
	mu       sync.Mutex
	engines  map[string]*Engine
	order    []string
	dataDir  string
	notified map[string]string

	initialDelay time.Duration
	interval     time.Duration
	stopCh       chan struct{}
	stopOnce     sync.Once
}

func NewFeedbackNotifier(dataDir string) *FeedbackNotifier {
	notifier := &FeedbackNotifier{
		engines:      make(map[string]*Engine),
		dataDir:      dataDir,
		notified:     make(map[string]string),
		initialDelay: feedbackNoticeInitialDelay,
		interval:     feedbackNoticeInterval,
		stopCh:       make(chan struct{}),
	}
	notifier.loadState()
	return notifier
}

func (notifier *FeedbackNotifier) RegisterEngine(name string, engine *Engine) {
	if name == "" || engine == nil {
		return
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if _, exists := notifier.engines[name]; !exists {
		notifier.order = append(notifier.order, name)
	}
	notifier.engines[name] = engine
}

func (notifier *FeedbackNotifier) Start() {
	go func() {
		select {
		case <-time.After(notifier.initialDelay):
		case <-notifier.stopCh:
			return
		}
		notifier.CheckOnce()
		ticker := time.NewTicker(notifier.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				notifier.CheckOnce()
			case <-notifier.stopCh:
				return
			}
		}
	}()
}

func (notifier *FeedbackNotifier) Stop() {
	notifier.stopOnce.Do(func() { close(notifier.stopCh) })
}

func (notifier *FeedbackNotifier) CheckOnce() {
	notifier.mu.Lock()
	names := append([]string(nil), notifier.order...)
	notifier.mu.Unlock()

	changed := false
	for _, name := range names {
		notifier.mu.Lock()
		engine := notifier.engines[name]
		notifier.mu.Unlock()
		if engine == nil {
			continue
		}
		keys := engine.FeedbackCapabilityGaps()
		if len(keys) == 0 {
			continue
		}
		fingerprint := feedbackKeysFingerprint(keys)
		notifier.mu.Lock()
		already := notifier.notified[name] == fingerprint
		notifier.mu.Unlock()
		if already || !engine.NotifyCapabilityGap(keys) {
			continue
		}
		notifier.mu.Lock()
		notifier.notified[name] = fingerprint
		notifier.mu.Unlock()
		changed = true
	}
	if changed {
		notifier.saveState()
	}
}

func feedbackKeysFingerprint(keys []string) string {
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:8])
}

func (notifier *FeedbackNotifier) statePath() string {
	return filepath.Join(notifier.dataDir, "run", feedbackNoticeStateFile)
}

func (notifier *FeedbackNotifier) loadState() {
	if notifier.dataDir == "" {
		return
	}
	data, err := os.ReadFile(notifier.statePath())
	if err != nil {
		return
	}
	state := make(map[string]string)
	if json.Unmarshal(data, &state) != nil {
		return
	}
	notifier.mu.Lock()
	notifier.notified = state
	notifier.mu.Unlock()
}

func (notifier *FeedbackNotifier) saveState() {
	if notifier.dataDir == "" {
		return
	}
	notifier.mu.Lock()
	data, err := json.Marshal(notifier.notified)
	notifier.mu.Unlock()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(notifier.statePath()), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(notifier.statePath(), data, 0o644)
}
