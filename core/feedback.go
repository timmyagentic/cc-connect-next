package core

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Feedback channel: when a user hits something this build cannot do — an
// unsupported config key, an unknown command, or anything they describe via
// /feedback — the daemon offers to report it to the project author as a
// GitHub issue.
//
// Two invariants are load-bearing:
//   - Consent is per submission. The user sees the exact issue title and body
//     (what you preview is what is sent) and must reply `/feedback confirm`.
//     Nothing is ever posted in the background.
//   - The payload must not carry credentials, chat/user ids, or filesystem
//     paths. The description passes redactFeedbackText; the environment
//     section is built from a fixed allowlist (version, OS, agent type).
//
// Submission goes to an author-operated relay (default endpoint below) that
// holds the GitHub token server-side, so reporters need no GitHub account.

const (
	// DefaultFeedbackEndpoint is the author-operated relay. Overridable via
	// [feedback] endpoint; an unreachable relay degrades to a message that
	// points at the public issue tracker.
	DefaultFeedbackEndpoint = "https://cc-connect-feedback.timmyagentic.workers.dev/v1/feedback"

	feedbackFallbackURL    = "https://github.com/timmyagentic/cc-connect-next/issues/new"
	feedbackDraftTTL       = 30 * time.Minute
	feedbackMaxDescription = 4000
	feedbackPostTimeout    = 10 * time.Second
)

// feedbackDraft is a staged, previewed-but-not-yet-confirmed submission.
type feedbackDraft struct {
	Title     string
	Body      string
	Trigger   string // "user" | "config_keys" | "error"
	CreatedAt time.Time
}

// feedbackError is the most recent user-visible failure in a session,
// reportable via `/feedback error`.
type feedbackError struct {
	Text string
	At   time.Time
}

const feedbackErrorHintCooldown = 10 * time.Minute

// FeedbackSubmission is the relay wire format (schema 1).
type FeedbackSubmission struct {
	Schema    int    `json:"schema"`
	InstallID string `json:"install_id"`
	Version   string `json:"version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Agent     string `json:"agent"`
	Trigger   string `json:"trigger"`
	Title     string `json:"title"`
	Body      string `json:"body"`
}

// SetFeedbackConfig wires the [feedback] config into the engine.
func (e *Engine) SetFeedbackConfig(enabled bool, endpoint, installID string) {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	e.feedbackEnabled = enabled
	e.feedbackEndpoint = strings.TrimSpace(endpoint)
	e.feedbackInstallID = installID
}

// SetFeedbackCapabilityGaps records config keys this build does not consume;
// they seed the proactive prompt and the `/feedback config` draft.
func (e *Engine) SetFeedbackCapabilityGaps(keys []string) {
	cleaned := make([]string, 0, len(keys))
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			cleaned = append(cleaned, k)
		}
	}
	sort.Strings(cleaned)
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	e.feedbackGapKeys = cleaned
}

// FeedbackCapabilityGaps returns the recorded unsupported config keys.
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

// cmdFeedback implements /feedback:
//
//	/feedback <description>  stage a draft and show the exact issue preview
//	/feedback config         stage a draft for the detected unsupported keys
//	/feedback confirm        submit the previewed draft
//	/feedback cancel         discard the previewed draft
func (e *Engine) cmdFeedback(p Platform, msg *Message, raw string) {
	if !e.feedbackActive() {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgFeedbackDisabled))
		return
	}
	key := feedbackDraftKey(p, msg)

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgFeedbackUsage))
	case "confirm":
		e.submitFeedbackDraft(p, msg, key)
	case "cancel":
		e.feedbackMu.Lock()
		_, had := e.feedbackDrafts[key]
		delete(e.feedbackDrafts, key)
		e.feedbackMu.Unlock()
		if had {
			e.reply(p, msg.ReplyCtx, e.i18n.T(MsgFeedbackCancelled))
		} else {
			e.reply(p, msg.ReplyCtx, e.i18n.T(MsgFeedbackNoDraft))
		}
	case "error":
		e.feedbackMu.Lock()
		fe := e.feedbackErrors[key]
		e.feedbackMu.Unlock()
		if fe == nil {
			e.reply(p, msg.ReplyCtx, e.i18n.T(MsgFeedbackNoError))
			return
		}
		errLine := fe.Text
		if i := strings.IndexByte(errLine, '\n'); i >= 0 {
			errLine = errLine[:i]
		}
		title := feedbackTitleFromDescription("error: " + errLine)
		desc := "A turn failed with the following error (" + fe.At.UTC().Format(time.RFC3339) + "):\n\n```\n" + fe.Text + "\n```"
		e.stageFeedbackDraft(p, msg, key, title, desc, "error")
	case "config":
		keys := e.FeedbackCapabilityGaps()
		if len(keys) == 0 {
			e.reply(p, msg.ReplyCtx, e.i18n.T(MsgFeedbackNoDraft))
			return
		}
		title := "Unsupported config key(s): " + strings.Join(keys, ", ")
		desc := "The configuration below is not consumed by this build:\n\n- `" +
			strings.Join(keys, "`\n- `") + "`\n\nReported so the author knows this capability is wanted."
		e.stageFeedbackDraft(p, msg, key, title, desc, "config_keys")
	default:
		desc := strings.TrimSpace(raw)
		title := feedbackTitleFromDescription(desc)
		e.stageFeedbackDraft(p, msg, key, title, desc, "user")
	}
}

func (e *Engine) stageFeedbackDraft(p Platform, msg *Message, key, title, description, trigger string) {
	title = redactFeedbackText(title)
	body := e.buildFeedbackBody(description, trigger)
	draft := &feedbackDraft{Title: title, Body: body, Trigger: trigger, CreatedAt: time.Now()}

	e.feedbackMu.Lock()
	if e.feedbackDrafts == nil {
		e.feedbackDrafts = make(map[string]*feedbackDraft)
	}
	e.feedbackDrafts[key] = draft
	e.feedbackMu.Unlock()

	e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgFeedbackPreview, draft.Title, draft.Body))
}

func (e *Engine) submitFeedbackDraft(p Platform, msg *Message, key string) {
	e.feedbackMu.Lock()
	draft := e.feedbackDrafts[key]
	if draft != nil && time.Since(draft.CreatedAt) > feedbackDraftTTL {
		delete(e.feedbackDrafts, key)
		draft = nil
	}
	endpoint := e.feedbackEndpoint
	installID := e.feedbackInstallID
	e.feedbackMu.Unlock()

	if draft == nil {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgFeedbackNoDraft))
		return
	}

	submission := FeedbackSubmission{
		Schema:    1,
		InstallID: installID,
		Version:   CurrentVersion,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Agent:     e.agent.Name(),
		Trigger:   draft.Trigger,
		Title:     draft.Title,
		Body:      draft.Body,
	}

	postFn := e.feedbackPostFn
	if postFn == nil {
		postFn = postFeedback
	}
	issueURL, err := postFn(endpoint, submission)
	if err != nil {
		slog.Warn("feedback: submission failed", "error", err)
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgFeedbackSubmitFailed, feedbackFallbackURL))
		return
	}

	e.feedbackMu.Lock()
	delete(e.feedbackDrafts, key)
	e.feedbackMu.Unlock()

	slog.Info("feedback: submitted", "trigger", draft.Trigger, "issue_url", issueURL)
	e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgFeedbackSubmitted, issueURL))
}

// recordFeedbackError remembers the most recent turn error per session so a
// later `/feedback error` can report it. Any user-visible failure — agent
// errors, idle timeouts, platform failures — may be recorded here; the
// feedback channel is for every problem users hit, not just config gaps.
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

// maybeSendFeedbackErrorHint follows a delivered error with a one-line "you
// can report this" pointer, at most once per session per cooldown window so
// repeated failures do not double the noise.
func (e *Engine) maybeSendFeedbackErrorHint(p Platform, replyCtx any, sessionKey string) {
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
	e.send(p, replyCtx, e.i18n.T(MsgFeedbackErrorHint))
}

// NotifyCapabilityGap proactively tells the project's most recently active
// session that some configured keys are not supported by this build, and how
// to report that. Returns true when a notice was delivered.
func (e *Engine) NotifyCapabilityGap(keys []string) bool {
	if !e.feedbackActive() || len(keys) == 0 {
		return false
	}
	content := e.i18n.Tf(MsgFeedbackCapabilityGap, "`"+strings.Join(keys, "`, `")+"`")
	return e.notifyMostRecentSession(content, "feedback notice")
}

// feedbackHint returns the one-line /feedback pointer appended to
// unknown-command replies, or "" when the channel is off.
func (e *Engine) feedbackHint() string {
	if !e.feedbackActive() {
		return ""
	}
	return "\n" + e.i18n.T(MsgFeedbackHint)
}

func feedbackDraftKey(p Platform, msg *Message) string {
	if msg.SessionKey != "" {
		return msg.SessionKey
	}
	return p.Name() + ":" + msg.UserID
}

func feedbackTitleFromDescription(desc string) string {
	title := desc
	if i := strings.IndexByte(title, '\n'); i >= 0 {
		title = title[:i]
	}
	title = strings.TrimSpace(title)
	if len(title) > 90 {
		title = title[:90]
		// avoid splitting a UTF-8 rune at the cut point
		for len(title) > 0 && !isUTF8Start(title[len(title)-1]) {
			title = title[:len(title)-1]
		}
		if len(title) > 0 {
			title = title[:len(title)-1]
		}
		title += "…"
	}
	return "[feedback] " + title
}

func isUTF8Start(b byte) bool { return b&0xC0 != 0x80 }

func (e *Engine) buildFeedbackBody(description, trigger string) string {
	description = strings.TrimSpace(redactFeedbackText(description))
	if len(description) > feedbackMaxDescription {
		description = description[:feedbackMaxDescription] + "\n\n_[truncated]_"
	}
	var b strings.Builder
	b.WriteString(description)
	b.WriteString("\n\n---\n**Environment (auto-generated)**\n")
	fmt.Fprintf(&b, "- cc-connect-next: %s\n", CurrentVersion)
	fmt.Fprintf(&b, "- OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "- Agent: %s\n", e.agent.Name())
	fmt.Fprintf(&b, "- Trigger: %s\n", trigger)
	b.WriteString("\n_Reported via in-app feedback; the reporter did not use a GitHub account._")
	return b.String()
}

// redactFeedbackText scrubs credential-shaped content from user-provided
// feedback text. It is deliberately aggressive: a mangled description is
// recoverable in review, a leaked secret is not.
var (
	feedbackKVSecretRe = regexp.MustCompile(`(?i)\b(app_secret|api_key|apikey|secret|token|password|authorization|access[_-]?key)\b(\s*[=:]\s*)\S+`)
	feedbackLongBlobRe = regexp.MustCompile(`\b[A-Za-z0-9+/_-]{28,}\b`)
	feedbackLarkIDRe   = regexp.MustCompile(`\b(ou|oc|om|on|cli)_[0-9a-f]{8,}\b`)
	feedbackHomePathRe = regexp.MustCompile(`(?:/Users/|/home/|[A-Za-z]:\\Users\\)\S+`)
)

func redactFeedbackText(s string) string {
	s = feedbackKVSecretRe.ReplaceAllString(s, "$1$2[REDACTED]")
	s = feedbackLarkIDRe.ReplaceAllString(s, "[REDACTED-ID]")
	s = feedbackHomePathRe.ReplaceAllString(s, "[REDACTED-PATH]")
	s = feedbackLongBlobRe.ReplaceAllString(s, "[REDACTED]")
	return s
}

func postFeedback(endpoint string, sub FeedbackSubmission) (string, error) {
	payload, err := json.Marshal(sub)
	if err != nil {
		return "", fmt.Errorf("feedback: marshal: %w", err)
	}
	client := &http.Client{Timeout: feedbackPostTimeout}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("feedback: post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("feedback: relay returned %d", resp.StatusCode)
	}
	var out struct {
		IssueURL string `json:"issue_url"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.IssueURL == "" {
		return "", fmt.Errorf("feedback: bad relay response")
	}
	return out.IssueURL, nil
}

// EnsureInstallID returns a stable random identifier persisted under
// dataDir/run. It exists solely so the relay can rate-limit; it is not
// derived from anything about the user or machine.
func EnsureInstallID(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	path := filepath.Join(dataDir, "run", "install_id")
	if b, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	id := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		_ = os.WriteFile(path, []byte(id+"\n"), 0o600)
	}
	return id
}

// ── FeedbackNotifier ─────────────────────────────────────────────
//
// Mirrors UpdateNotifier: delivers the capability-gap notice to each
// project's most recently active session, at most once per distinct key set
// (persisted across restarts), retrying while no session is reachable.

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
	notified map[string]string // project name → delivered key-set fingerprint

	initialDelay time.Duration
	interval     time.Duration
	stopCh       chan struct{}
	stopOnce     sync.Once
}

func NewFeedbackNotifier(dataDir string) *FeedbackNotifier {
	n := &FeedbackNotifier{
		engines:      make(map[string]*Engine),
		dataDir:      dataDir,
		notified:     make(map[string]string),
		initialDelay: feedbackNoticeInitialDelay,
		interval:     feedbackNoticeInterval,
		stopCh:       make(chan struct{}),
	}
	n.loadState()
	return n
}

func (n *FeedbackNotifier) RegisterEngine(name string, e *Engine) {
	if name == "" || e == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.engines[name]; !exists {
		n.order = append(n.order, name)
	}
	n.engines[name] = e
}

func (n *FeedbackNotifier) Start() {
	go func() {
		select {
		case <-time.After(n.initialDelay):
		case <-n.stopCh:
			return
		}
		n.CheckOnce()
		ticker := time.NewTicker(n.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n.CheckOnce()
			case <-n.stopCh:
				return
			}
		}
	}()
}

func (n *FeedbackNotifier) Stop() {
	n.stopOnce.Do(func() { close(n.stopCh) })
}

// CheckOnce delivers the gap notice for every registered engine whose key-set
// fingerprint has not been announced yet.
func (n *FeedbackNotifier) CheckOnce() {
	n.mu.Lock()
	names := append([]string(nil), n.order...)
	n.mu.Unlock()

	changed := false
	for _, name := range names {
		n.mu.Lock()
		e := n.engines[name]
		n.mu.Unlock()
		if e == nil {
			continue
		}
		keys := e.FeedbackCapabilityGaps()
		if len(keys) == 0 {
			continue
		}
		fp := feedbackKeysFingerprint(keys)
		n.mu.Lock()
		already := n.notified[name] == fp
		n.mu.Unlock()
		if already {
			continue
		}
		if !e.NotifyCapabilityGap(keys) {
			// No reachable session yet; retry next cycle.
			continue
		}
		n.mu.Lock()
		n.notified[name] = fp
		n.mu.Unlock()
		changed = true
	}
	if changed {
		n.saveState()
	}
}

func feedbackKeysFingerprint(keys []string) string {
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:8])
}

func (n *FeedbackNotifier) statePath() string {
	return filepath.Join(n.dataDir, "run", feedbackNoticeStateFile)
}

func (n *FeedbackNotifier) loadState() {
	if n.dataDir == "" {
		return
	}
	data, err := os.ReadFile(n.statePath())
	if err != nil {
		return
	}
	state := make(map[string]string)
	if json.Unmarshal(data, &state) != nil {
		return
	}
	n.mu.Lock()
	n.notified = state
	n.mu.Unlock()
}

func (n *FeedbackNotifier) saveState() {
	if n.dataDir == "" {
		return
	}
	n.mu.Lock()
	data, err := json.Marshal(n.notified)
	n.mu.Unlock()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(n.statePath()), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(n.statePath(), data, 0o644)
}
