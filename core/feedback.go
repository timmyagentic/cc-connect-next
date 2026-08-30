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
// CC Connect Next chooses context, renders every structured field, owns cards,
// localization and fallback, and only submits the exact pending Draft after a
// separate explicit user action.

const (
	DefaultFeedbackEndpoint = "https://cc-connect-feedback.qianbi3956001.workers.dev/v1/feedback"

	feedbackFallbackURL       = "https://github.com/timmyagentic/cc-connect-next/issues/new"
	feedbackSubmitTimeout     = 15 * time.Second
	feedbackPendingTTL        = 10 * time.Minute
	feedbackPendingMax        = 64
	feedbackErrorHintCooldown = 10 * time.Minute
	feedbackErrorAttachWindow = 30 * time.Minute
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
// they seed proactive previews and a bare /feedback draft.
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

// cmdFeedback is deliberately two-step:
//
//	/feedback <description>  -> render and retain the complete redacted draft
//	/feedback confirm        -> approve and submit that exact draft
//	/feedback cancel         -> discard it
//
// Card buttons provide the same confirm/cancel actions without requiring the
// user to learn command syntax.
func (e *Engine) cmdFeedback(platform Platform, message *Message, raw string) {
	if !e.feedbackActive() {
		e.reply(platform, message.ReplyCtx, e.i18n.T(MsgFeedbackDisabled))
		return
	}
	key := feedbackSessionKey(platform, message)
	argument := strings.TrimSpace(raw)
	control, token, controlled := feedbackControl(argument)
	switch control {
	case "confirm", "submit":
		e.submitPendingFeedback(platform, message, key, message.UserID, token)
		return
	case "cancel", "dismiss":
		e.clearPendingFeedback(key, message.UserID, token)
		e.reply(platform, message.ReplyCtx, e.i18n.T(MsgFeedbackDismissed))
		return
	}
	if controlled {
		argument = ""
	}

	draft, err := e.buildFeedbackDraft(key, argument, nil)
	if err != nil {
		e.reply(platform, message.ReplyCtx, e.i18n.T(MsgFeedbackUsage))
		return
	}
	token, err = e.rememberPendingFeedback(key, message.UserID, draft)
	if err != nil {
		e.reply(platform, message.ReplyCtx, e.i18n.Tf(MsgError, err))
		return
	}
	if err := e.deliverFeedbackPreview(platform, message.ReplyCtx, draft.Report(), token, false); err != nil {
		e.clearPendingFeedback(key, message.UserID, token)
		slog.Warn("feedback: preview delivery failed", "platform", platform.Name(), "error", err)
	}
}

func feedbackControl(argument string) (control, token string, ok bool) {
	fields := strings.Fields(argument)
	if len(fields) == 0 || len(fields) > 2 {
		return "", "", false
	}
	control = strings.ToLower(fields[0])
	switch control {
	case "confirm", "submit", "cancel", "dismiss", "error", "config":
		if len(fields) == 2 {
			token = fields[1]
		}
		return control, token, true
	default:
		return "", "", false
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

	input := appfeatures.FeedbackContext{
		Description:    description,
		CapabilityGaps: gaps,
		Version:        CurrentVersion,
		Agent:          e.agent.Name(),
	}
	if recent != nil {
		input.RecentError = recent.Text
		input.RecentErrorAt = recent.At
	}
	return appfeatures.BuildFeedbackDraft(input)
}

func (e *Engine) rememberPendingFeedback(sessionKey, userID string, draft appfeatures.FeedbackDraft) (string, error) {
	if sessionKey == "" {
		return "", fmt.Errorf("feedback session is required")
	}
	token, err := newPendingActionToken()
	if err != nil {
		return "", err
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
	e.feedbackPending[token] = pendingFeedback{Draft: draft, At: now, SessionKey: sessionKey, UserID: userID}
	return token, nil
}

func (e *Engine) takePendingFeedback(sessionKey, userID, token string) (appfeatures.FeedbackDraft, bool) {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	now := time.Now()
	e.prunePendingFeedbackLocked(now)
	if token == "" {
		token = e.uniqueFeedbackTokenLocked(sessionKey, userID)
	}
	pending, exists := e.feedbackPending[token]
	if !exists {
		return appfeatures.FeedbackDraft{}, false
	}
	if pending.SessionKey != sessionKey || (pending.UserID != "" && pending.UserID != userID) {
		return appfeatures.FeedbackDraft{}, false
	}
	e.deletePendingFeedbackLocked(token)
	return pending.Draft, true
}

func (e *Engine) clearPendingFeedback(sessionKey, userID, token string) {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	e.prunePendingFeedbackLocked(time.Now())
	if token == "" {
		token = e.uniqueFeedbackTokenLocked(sessionKey, userID)
	}
	if pending, exists := e.feedbackPending[token]; exists && pending.SessionKey == sessionKey && (pending.UserID == "" || pending.UserID == userID) {
		e.deletePendingFeedbackLocked(token)
	}
}

func (e *Engine) uniqueFeedbackTokenLocked(sessionKey, userID string) string {
	match := ""
	for token, pending := range e.feedbackPending {
		if pending.SessionKey != sessionKey || (pending.UserID != "" && pending.UserID != userID) {
			continue
		}
		if match != "" {
			return ""
		}
		match = token
	}
	return match
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
		e.reply(platform, message.ReplyCtx, e.i18n.T(MsgFeedbackPendingMissing))
		return
	}

	e.feedbackMu.Lock()
	endpoint := e.feedbackEndpoint
	submit := e.feedbackSubmitFn
	e.feedbackMu.Unlock()
	if submit == nil {
		relay := appfeatures.FeedbackRelay{Endpoint: endpoint}
		submit = relay.Submit
	}
	ctx, cancel := context.WithTimeout(e.ctx, feedbackSubmitTimeout)
	defer cancel()
	receipt, err := submit(ctx, draft, true)
	if err != nil {
		slog.Warn("feedback: submission failed", "error", err)
		e.reply(platform, message.ReplyCtx, e.i18n.Tf(MsgFeedbackSubmitFailed, feedbackFallbackURL))
		return
	}

	slog.Info("feedback: submitted", "reference_url", receipt.ReferenceURL, "deduplicated", receipt.Deduplicated)
	e.reply(platform, message.ReplyCtx, e.i18n.Tf(MsgFeedbackSubmitted, receipt.ReferenceURL))
}

func (e *Engine) feedbackPreview(report appfeatures.FeedbackReport) string {
	empty := e.i18n.T(MsgFeedbackPreviewEmpty)
	description := strings.TrimSpace(report.Description)
	if description == "" {
		description = empty
	}
	recentError := empty
	if report.RecentError != nil {
		recentError = fmt.Sprintf("%s\n%s", report.RecentError.At.UTC().Format(time.RFC3339), report.RecentError.Text)
	}
	gaps := empty
	if len(report.CapabilityGaps) > 0 {
		gaps = "- `" + strings.Join(report.CapabilityGaps, "`\n- `") + "`"
	}
	return e.i18n.Tf(
		MsgFeedbackPreview,
		description,
		recentError,
		gaps,
		report.Environment.Product,
		report.Environment.Version,
		report.Environment.OS,
		report.Environment.Arch,
		report.Environment.Agent,
	)
}

func (e *Engine) feedbackPreviewCard(report appfeatures.FeedbackReport, token string) *Card {
	return NewCard().
		Title(e.i18n.T(MsgFeedbackAskTitle), "orange").
		Markdown(e.feedbackPreview(report)).
		Buttons(
			PrimaryBtn(e.i18n.T(MsgFeedbackBtnSubmit), "cmd:/feedback confirm "+token),
			DefaultBtn(e.i18n.T(MsgFeedbackBtnDismiss), "cmd:/feedback cancel "+token),
		).
		Build()
}

func (e *Engine) deliverFeedbackPreview(platform Platform, replyCtx any, report appfeatures.FeedbackReport, token string, proactive bool) error {
	preview := e.feedbackPreview(report)
	if sender, ok := platform.(CardSender); ok {
		card := e.renderCardForPlatform(platform, e.feedbackPreviewCard(report, token))
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
			{Text: e.i18n.T(MsgFeedbackBtnSubmit), Data: "cmd:/feedback confirm " + token},
			{Text: e.i18n.T(MsgFeedbackBtnDismiss), Data: "cmd:/feedback cancel " + token},
		}}
		if err := sender.SendWithButtons(e.ctx, replyCtx, preview, buttons); err == nil {
			return nil
		}
	}
	confirmation := strings.Replace(e.i18n.T(MsgFeedbackPreviewConfirm), "/feedback confirm", "/feedback confirm "+token, 1)
	text := preview + "\n\n" + confirmation
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
	if err := e.deliverFeedbackPreview(platform, replyCtx, draft.Report(), token, true); err != nil {
		e.clearPendingFeedback(sessionKey, "", token)
		slog.Debug("feedback: proactive preview failed", "session_key", sessionKey, "error", err)
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
		if err := e.deliverFeedbackPreview(platform, replyCtx, draft.Report(), token, true); err != nil {
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
