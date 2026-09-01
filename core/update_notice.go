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
// sends one localized private notice to each project's explicit admin_from
// users. Recent chats, groups, and topics are never notification targets. A
// project/version is complete only when one pass reaches every administrator;
// after a partial failure the next pass deliberately retries the full list.

const (
	updateNoticeInitialDelay = 2 * time.Minute
	updateNoticeInterval     = 24 * time.Hour
	updateNoticeStateFile    = "update_notice.json"
)

// UpdateNotifier periodically checks for a newer stable release and retries
// each project's full explicit-administrator list until one pass succeeds.
type UpdateNotifier struct {
	mu       sync.Mutex
	engines  map[string]*Engine
	order    []string // registration order for deterministic iteration
	dataDir  string
	notified map[string]string

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

func (e *Engine) explicitAdminUserIDs() []string {
	e.userRolesMu.RLock()
	raw := e.adminFrom
	e.userRolesMu.RUnlock()
	seen := make(map[string]struct{})
	users := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		userID := strings.TrimSpace(part)
		if userID == "*" {
			return nil
		}
		if userID == "" {
			continue
		}
		key := strings.ToLower(userID)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		users = append(users, userID)
	}
	sort.Slice(users, func(i, j int) bool {
		return strings.ToLower(users[i]) < strings.ToLower(users[j])
	})
	return users
}

func (e *Engine) directUserNoticePlatform() (Platform, DirectUserSender, bool) {
	var platform Platform
	var sender DirectUserSender
	for _, candidate := range e.platforms {
		direct, ok := candidate.(DirectUserSender)
		if !ok {
			continue
		}
		if sender != nil {
			return nil, nil, false
		}
		platform = candidate
		sender = direct
	}
	return platform, sender, sender != nil
}

type updateNoticeTarget struct {
	userID   string
	platform Platform
	sender   DirectUserSender
}

func (e *Engine) updateNoticeTargets() []updateNoticeTarget {
	admins := e.explicitAdminUserIDs()
	if len(admins) == 0 {
		slog.Debug("update notice: no explicit admin_from private target", "project", e.name)
		return nil
	}
	platform, sender, ok := e.directUserNoticePlatform()
	if !ok {
		slog.Debug("update notice: direct-user platform is missing or ambiguous", "project", e.name)
		return nil
	}
	targets := make([]updateNoticeTarget, 0, len(admins))
	for _, userID := range admins {
		targets = append(targets, updateNoticeTarget{
			userID: userID, platform: platform, sender: sender,
		})
	}
	return targets
}

// NotifyUpdateAvailable sends one localized update notice to every explicit
// admin_from user through exactly one platform that supports private direct
// delivery. It never borrows a recent chat/session target. Returns true only
// when every configured admin received the notice. A partial failure leaves
// the project/version unfinished, so the next notifier pass retries every
// explicit administrator rather than maintaining per-recipient state.
//
// The notice is actionable: platforms with cards or inline buttons get an
// [review update] button, and the delivered session opens a natural-language
// discovery window so a plain "更新" reply prepares the exact Plan. A second
// action is still required before installation.
func (e *Engine) NotifyUpdateAvailable(release *ReleaseInfo) bool {
	if release == nil || release.TagName == "" {
		return false
	}
	targets := e.updateNoticeTargets()
	if len(targets) == 0 {
		return false
	}
	text := e.i18n.Tf(MsgUpdateNoticeAvailable, release.TagName, CurrentVersion)
	action := e.i18n.Tf(MsgUpdateNoticeAvailableAction, release.TagName, CurrentVersion)
	delivered := 0
	for _, target := range targets {
		if err := e.sendUpdateNoticeTarget(target, action, text); err != nil {
			slog.Debug("update notice: admin private send failed",
				"project", e.name, "platform", target.platform.Name(), "error", err)
			continue
		}
		delivered++
	}
	if delivered != len(targets) {
		slog.Debug("update notice: partial admin delivery",
			"project", e.name, "delivered", delivered, "expected", len(targets),
			"retry", "all explicit admins")
		return false
	}
	slog.Info("update notice: delivered to explicit admins",
		"project", e.name, "platform", targets[0].platform.Name(), "count", delivered)
	return true
}

func (e *Engine) sendUpdateNoticeTarget(target updateNoticeTarget, actionCopy, textCopy string) error {
	if err := e.sendDirectUpdateNotice(target.platform, target.sender, target.userID, actionCopy, textCopy); err != nil {
		return err
	}
	e.updateIntents.recordDirectNotice(target.platform.Name(), target.userID)
	return nil
}

func (e *Engine) sendDirectUpdateNotice(platform Platform, sender DirectUserSender, userID, actionCopy, textCopy string) error {
	if cards, ok := platform.(DirectUserCardSender); ok {
		btnNow := e.i18n.T(MsgUpdateBtnNow)
		btnLog := e.i18n.T(MsgUpdateBtnChangelog)
		hint := e.i18n.T(MsgUpdateHintReplyUpdate)
		card := e.renderCardForPlatform(platform, NewCard().
			Markdown(actionCopy).
			Buttons(
				CardButton{Text: btnNow, Type: "primary", Value: "cmd:/upgrade"},
				CardButton{Text: btnLog, Value: "nav:/upgrade"},
			).
			Note(hint).
			Build())
		if err := cards.SendDirectUserCard(e.ctx, userID, card); err == nil {
			return nil
		}
	}
	return sender.SendDirectUser(e.ctx, userID, textCopy)
}

// notifyMostRecentSessionFn invokes deliver for candidates ordered by recent
// user activity until one succeeds. It returns the receiving session key so
// callers can associate follow-up state with that conversation.
func (e *Engine) notifyMostRecentSessionFn(logTag string, deliver func(string, Platform, any) error) (string, bool) {
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
		replyCtx, err := e.reconstructReplyContext(target, c.key)
		if err != nil {
			slog.Debug(logTag+": reconstruct reply context failed",
				"session_key", c.key, "error", err)
			continue
		}
		if err := deliver(c.key, target, replyCtx); err != nil {
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
