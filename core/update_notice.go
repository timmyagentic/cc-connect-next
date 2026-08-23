package core

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Update notice: a daemon-side reminder that a newer stable release exists.
//
// Before this, every update surface was pull-only (`/upgrade`, the
// `check-update` CLI command, and the CLI usage-page hint), so a user running
// an old daemon would never learn that a newer version had shipped. The
// UpdateNotifier closes that gap: it periodically compares CurrentVersion
// against the newest stable GitHub release and, when a newer one appears,
// sends one localized chat notice per project to that project's most recently
// active session. Each discovered version is announced at most once per
// project (persisted across restarts), so the reminder can never become spam.

const (
	updateNoticeInitialDelay = 2 * time.Minute
	updateNoticeInterval     = 24 * time.Hour
	updateNoticeStateFile    = "update_notice.json"
)

// UpdateNotifier periodically checks for a newer stable release and notifies
// each registered project's most recently active session once per version.
type UpdateNotifier struct {
	mu       sync.Mutex
	engines  map[string]*Engine
	order    []string // registration order for deterministic iteration
	dataDir  string
	notified map[string]string // project name → last announced version tag

	initialDelay time.Duration
	interval     time.Duration
	stopCh       chan struct{}
	stopOnce     sync.Once

	// checkFn is a test seam; production uses CheckForUpdate.
	checkFn func(currentVersion string) (*ReleaseInfo, error)
}

// NewUpdateNotifier creates a notifier whose once-per-version state persists
// under dataDir/run/update_notice.json.
func NewUpdateNotifier(dataDir string) *UpdateNotifier {
	n := &UpdateNotifier{
		engines:      make(map[string]*Engine),
		dataDir:      dataDir,
		notified:     make(map[string]string),
		initialDelay: updateNoticeInitialDelay,
		interval:     updateNoticeInterval,
		stopCh:       make(chan struct{}),
		checkFn: func(cur string) (*ReleaseInfo, error) {
			return CheckForUpdate(cur, false)
		},
	}
	n.loadState()
	return n
}

// RegisterEngine adds a project engine as a notification target.
func (n *UpdateNotifier) RegisterEngine(name string, e *Engine) {
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

// Start launches the periodic check loop. The first check runs after a short
// delay so platforms have connected and a fresh install is not greeted with
// network traffic during setup.
func (n *UpdateNotifier) Start() {
	go func() {
		initial := time.NewTimer(n.initialDelay)
		defer initial.Stop()
		select {
		case <-initial.C:
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

// Stop terminates the check loop. Safe to call multiple times.
func (n *UpdateNotifier) Stop() {
	n.stopOnce.Do(func() { close(n.stopCh) })
}

// CheckOnce performs a single check-and-notify pass. Exposed for tests and
// callable independently of the Start loop.
func (n *UpdateNotifier) CheckOnce() {
	cur := CurrentVersion
	if cur == "" || cur == "dev" {
		return
	}
	release, err := n.checkFn(cur)
	if err != nil {
		slog.Debug("update notice: version check failed", "error", err)
		return
	}
	if release == nil || release.TagName == "" {
		return
	}

	slog.Info("update notice: newer stable release available",
		"current", cur,
		"latest", release.TagName,
	)

	n.mu.Lock()
	names := append([]string(nil), n.order...)
	n.mu.Unlock()

	changed := false
	for _, name := range names {
		n.mu.Lock()
		e := n.engines[name]
		already := n.notified[name] == release.TagName
		n.mu.Unlock()
		if e == nil || already {
			continue
		}
		if !e.NotifyUpdateAvailable(release) {
			// No reachable session yet (fresh project, platform without
			// proactive messaging, or send failure). Leave the project
			// unmarked so the next cycle retries.
			continue
		}
		n.mu.Lock()
		n.notified[name] = release.TagName
		n.mu.Unlock()
		changed = true
	}
	if changed {
		n.saveState()
	}
}

func (n *UpdateNotifier) statePath() string {
	return filepath.Join(n.dataDir, "run", updateNoticeStateFile)
}

func (n *UpdateNotifier) loadState() {
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

func (n *UpdateNotifier) saveState() {
	if n.dataDir == "" {
		return
	}
	n.mu.Lock()
	data, err := json.Marshal(n.notified)
	n.mu.Unlock()
	if err != nil {
		return
	}
	dir := filepath.Dir(n.statePath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Debug("update notice: mkdir failed", "dir", dir, "error", err)
		return
	}
	if err := os.WriteFile(n.statePath(), data, 0o644); err != nil {
		slog.Debug("update notice: persist failed", "error", err)
	}
}

// NotifyUpdateAvailable sends a localized update notice to this engine's most
// recently active session. Returns true only when a message was actually
// delivered, so callers can retry later instead of losing the notice.
//
// The notice is actionable: platforms with cards or inline buttons get an
// [update now] button, and the delivered session opens a natural-language
// consent window so a plain "更新" reply installs the update — no command
// syntax required.
func (e *Engine) NotifyUpdateAvailable(release *ReleaseInfo) bool {
	if release == nil || release.TagName == "" {
		return false
	}
	content := e.i18n.Tf(MsgUpdateNoticeAvailable, release.TagName, CurrentVersion)
	key, ok := e.notifyMostRecentSessionFn("update notice", func(p Platform, replyCtx any) error {
		return e.sendUpdateNotice(p, replyCtx, content)
	})
	if ok {
		e.updateIntents.recordNotice(key)
	}
	return ok
}

// sendUpdateNotice delivers the notice with [update now] / [what's new]
// actions where the platform supports them, plain text elsewhere. Buttons
// use the cmd:/ scheme so a tap passes the same gates as a typed command.
func (e *Engine) sendUpdateNotice(p Platform, replyCtx any, content string) error {
	btnNow := e.i18n.T(MsgUpdateBtnNow)
	btnLog := e.i18n.T(MsgUpdateBtnChangelog)
	if cs, ok := p.(CardSender); ok {
		card := e.renderCardForPlatform(p, NewCard().
			Markdown(content).
			Buttons(
				CardButton{Text: btnNow, Type: "primary", Value: "cmd:/upgrade confirm"},
				CardButton{Text: btnLog, Value: "cmd:/upgrade"},
			).Build())
		if err := cs.SendCard(e.ctx, replyCtx, card); err == nil {
			return nil
		}
	}
	if bs, ok := p.(InlineButtonSender); ok {
		buttons := [][]ButtonOption{{
			{Text: btnNow, Data: "cmd:/upgrade confirm"},
			{Text: btnLog, Data: "cmd:/upgrade"},
		}}
		if err := bs.SendWithButtons(e.ctx, replyCtx, content, buttons); err == nil {
			return nil
		}
	}
	return e.sendWithError(p, replyCtx, content)
}

// notifyMostRecentSession delivers a proactive daemon-side message to this
// engine's most recently active session (ranked by real user activity;
// UpdatedAt is the fallback for stores predating LastUserActivity). Returns
// true only when a message was actually delivered, so callers can retry
// later instead of losing the notice. logTag labels the slog lines.
func (e *Engine) notifyMostRecentSession(content, logTag string) bool {
	_, ok := e.notifyMostRecentSessionFn(logTag, func(p Platform, replyCtx any) error {
		return e.sendWithError(p, replyCtx, content)
	})
	return ok
}

// notifyMostRecentSessionFn is the delivery-agnostic core of
// notifyMostRecentSession: deliver is invoked per candidate (newest first)
// until one send succeeds. Returns the session key that received the
// message so callers can associate follow-up state with it.
func (e *Engine) notifyMostRecentSessionFn(logTag string, deliver func(Platform, any) error) (string, bool) {
	sessions := e.sessions.AllSessions()
	idToKey, _ := e.sessions.SessionKeyMap()
	type candidate struct {
		key      string
		lastSeen time.Time
	}
	seen := make(map[string]time.Time)
	for _, s := range sessions {
		key := idToKey[s.ID]
		if key == "" {
			continue
		}
		last := s.GetLastUserActivity()
		if last.IsZero() {
			last = s.UpdatedAt
		}
		if last.After(seen[key]) {
			seen[key] = last
		}
	}
	candidates := make([]candidate, 0, len(seen))
	for key, last := range seen {
		candidates = append(candidates, candidate{key: key, lastSeen: last})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].lastSeen.After(candidates[j].lastSeen)
	})

	for _, c := range candidates {
		platformName := ""
		if idx := strings.Index(c.key, ":"); idx > 0 {
			platformName = c.key[:idx]
		}
		var target Platform
		for _, p := range e.platforms {
			if p.Name() == platformName {
				target = p
				break
			}
		}
		if target == nil {
			continue
		}
		rc, ok := target.(ReplyContextReconstructor)
		if !ok {
			continue
		}
		replyCtx, err := rc.ReconstructReplyCtx(c.key)
		if err != nil {
			slog.Debug(logTag+": reconstruct reply context failed",
				"session_key", c.key, "error", err)
			continue
		}
		if err := deliver(target, replyCtx); err != nil {
			slog.Debug(logTag+": send failed", "session_key", c.key, "error", err)
			continue
		}
		slog.Info(logTag+": delivered",
			"project", e.name,
			"session_key", c.key,
		)
		return c.key, true
	}
	return "", false
}
