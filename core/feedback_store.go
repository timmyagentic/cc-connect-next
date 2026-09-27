package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

const feedbackStoreMaxBytes = 10 * 1024 * 1024

type feedbackSubmission struct {
	Draft       appfeatures.FeedbackDraftSnapshot
	Owner       string
	ContentKey  string
	PayloadHash string
	At          time.Time
	State       string
	Receipt     *appfeatures.FeedbackReceipt `json:",omitempty"`
	inFlight    chan struct{}
}

type storedFeedbackTurn struct {
	At         time.Time
	Active     bool
	Shared     bool
	Version    string
	Agent      string
	Diagnostic appfeatures.FeedbackDiagnostic
}

type storedPendingFeedback struct {
	Draft      appfeatures.FeedbackDraftSnapshot
	At         time.Time
	SessionKey string // hashes only, never transport identities
	UserID     string
	AgentOnly  bool
}

type feedbackDiskState struct {
	Schema      int
	Turns       map[string]storedFeedbackTurn
	Pending     map[string]storedPendingFeedback
	Submissions map[string]*feedbackSubmission
}

func (e *Engine) feedbackStatePath() string {
	if e.dataDir == "" {
		return ""
	}
	return filepath.Join(e.dataDir, "run", "feedback", e.feedbackBinding("", "")+".json")
}

// Configuration invokes loading before message handling starts. Loading never
// resumes network submissions; only a fresh authorized action may do that.
func (e *Engine) loadFeedbackStateLocked() {
	path := e.feedbackStatePath()
	if e.feedbackStoreLoaded || path == "" {
		return
	}
	e.feedbackStoreLoaded = true
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > feedbackStoreMaxBytes {
		slog.Warn("feedback: local state unavailable")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		slog.Warn("feedback: local state unavailable")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, feedbackStoreMaxBytes+1))
	if err != nil || len(data) > feedbackStoreMaxBytes {
		return
	}
	var saved feedbackDiskState
	if json.Unmarshal(data, &saved) != nil || saved.Schema != 1 || len(saved.Turns) > feedbackPendingMax || len(saved.Pending) > feedbackPendingMax || len(saved.Submissions) > feedbackPendingMax {
		slog.Warn("feedback: invalid local state")
		return
	}
	now := time.Now()
	e.feedbackTurns = make(map[string]*feedbackTurn)
	for key, item := range saved.Turns {
		if item.At.After(now) || (!item.Active && now.Sub(item.At) > feedbackRetain) || !validFeedbackHash(key) {
			continue
		}
		if item.Active {
			item.At = now // retention starts when the interrupted turn is frozen
			item.Diagnostic.Phase = "interrupted"
			item.Diagnostic.ErrorCode = "host_restart"
			item.Diagnostic.Error = "The host restarted before this turn's outcome was recorded. The backend outcome is unknown."
			item.Diagnostic.OccurredAt = now
			item.Diagnostic.Missing = append(item.Diagnostic.Missing, "interruption time and backend outcome: unknown after restart")
		}
		e.feedbackTurns[key] = &feedbackTurn{key: key, at: item.At, shared: item.Shared, version: item.Version, agent: item.Agent, diagnostic: appfeatures.CaptureFeedbackDiagnostic(item.Diagnostic)}
	}
	e.feedbackPending = make(map[string]pendingFeedback)
	for token, item := range saved.Pending {
		if !validPendingActionToken(token) || item.At.After(now) || now.Sub(item.At) > feedbackPendingTTL || !validFeedbackHash(item.SessionKey) || (item.UserID != "" && !validFeedbackHash(item.UserID)) {
			continue
		}
		draft, err := appfeatures.RestoreFeedbackDraft(item.Draft)
		if err != nil {
			continue
		}
		e.feedbackPending[token] = pendingFeedback{Draft: draft, At: item.At, SessionKey: item.SessionKey, UserID: item.UserID, AgentOnly: item.AgentOnly}
	}
	e.feedbackSubmissions = make(map[string]*feedbackSubmission)
	for id, item := range saved.Submissions {
		if item == nil || !validPendingActionToken(id) || !validFeedbackHash(item.Owner) || item.At.After(now) || now.Sub(item.At) > feedbackRetain {
			continue
		}
		draft, err := appfeatures.RestoreFeedbackDraft(item.Draft)
		if err != nil || draft.Report().ReportID != id {
			continue
		}
		hash, err := feedbackApprovedHash(draft)
		if err != nil || hash != item.PayloadHash || feedbackContentKey(item.Owner, draft) != item.ContentKey {
			continue
		}
		// A crash during dispatch is an unknown outcome, not a rejected POST.
		if item.State == "dispatching" {
			item.State = "outcome_unknown"
		}
		e.feedbackSubmissions[id] = item
	}
	e.pruneFeedbackTurnsLocked(now)
}

func validFeedbackHash(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == 32
}
func validPendingActionToken(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == 16
}

func feedbackContentKey(owner string, draft appfeatures.FeedbackDraft) string {
	input := appfeatures.SnapshotFeedbackDraft(draft).Input
	input.ReportID = ""
	if input.Diagnostic != nil {
		input.RecentError = nil
	} // duplicated incident error; freshness must not change retry identity
	data, _ := json.Marshal(input)
	hash := sha256.Sum256(append([]byte(owner+"\x00"), data...))
	return hex.EncodeToString(hash[:])
}

// This function is called only when consuming an explicitly approved action
// or verifying a previously approved local record. It performs no network I/O.
func feedbackApprovedHash(draft appfeatures.FeedbackDraft) (string, error) {
	approved, err := draft.Approve(true)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(approved)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// rememberApprovedFeedbackLocked atomically exchanges the one-use grant for
// an immutable approved intent. A consumed token is never made reusable.
func (e *Engine) rememberApprovedFeedbackLocked(sessionKey, userID string, draft appfeatures.FeedbackDraft) (appfeatures.FeedbackDraft, error) {
	owner := e.feedbackBinding(sessionKey, userID)
	id := draft.Report().ReportID
	hash, err := feedbackApprovedHash(draft)
	if err != nil {
		return appfeatures.FeedbackDraft{}, err
	}
	if e.feedbackSubmissions == nil {
		e.feedbackSubmissions = make(map[string]*feedbackSubmission)
	}
	if old := e.feedbackSubmissions[id]; old != nil {
		if old.Owner != owner || old.PayloadHash != hash {
			return appfeatures.FeedbackDraft{}, fmt.Errorf("feedback approval does not match the stored report")
		}
		return draft, nil
	}
	e.pruneFeedbackSubmissionsLocked(time.Now())
	content := feedbackContentKey(owner, draft)
	// Independent previews may have been built before either was approved.
	// Resolve their content under the same lock as insertion, then dispatch
	// the original approved bytes and identity for every equivalent intent.
	for _, item := range e.feedbackSubmissions {
		if item.Owner == owner && item.ContentKey == content {
			return appfeatures.RestoreFeedbackDraft(item.Draft)
		}
	}
	if len(e.feedbackSubmissions) >= feedbackPendingMax {
		return appfeatures.FeedbackDraft{}, fmt.Errorf("feedback submission storage is full")
	}
	e.feedbackSubmissions[id] = &feedbackSubmission{Draft: appfeatures.SnapshotFeedbackDraft(draft), Owner: owner, PayloadHash: hash, ContentKey: content, At: time.Now(), State: "approved"}
	e.scheduleFeedbackSaveLocked()
	return draft, nil
}

func (e *Engine) reuseFeedbackSubmission(sessionKey, userID string, draft appfeatures.FeedbackDraft) appfeatures.FeedbackDraft {
	owner := e.feedbackBinding(sessionKey, userID)
	content := feedbackContentKey(owner, draft)
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	e.pruneFeedbackSubmissionsLocked(time.Now())
	for _, item := range e.feedbackSubmissions {
		if item.Owner == owner && item.ContentKey == content {
			if restored, err := appfeatures.RestoreFeedbackDraft(item.Draft); err == nil {
				return restored
			}
		}
	}
	return draft
}

func (e *Engine) pruneFeedbackSubmissionsLocked(now time.Time) {
	for id, item := range e.feedbackSubmissions {
		if item.inFlight == nil && now.Sub(item.At) > feedbackRetain {
			delete(e.feedbackSubmissions, id)
		}
	}
	for len(e.feedbackSubmissions) >= feedbackPendingMax {
		oldest := ""
		var at time.Time
		for id, item := range e.feedbackSubmissions {
			if item.inFlight == nil && item.State == "submitted" && (oldest == "" || item.At.Before(at)) {
				oldest, at = id, item.At
			}
		}
		if oldest == "" {
			return
		}
		delete(e.feedbackSubmissions, oldest)
	}
}

func (e *Engine) scheduleFeedbackSaveLocked() {
	if e.feedbackStatePath() == "" || e.ctx.Err() != nil {
		return
	}
	if e.feedbackSaveCh == nil {
		e.feedbackSaveCh = make(chan struct{}, 1)
		e.feedbackSaveDone = make(chan struct{})
		go func() {
			defer close(e.feedbackSaveDone)
			for {
				select {
				case <-e.feedbackSaveCh:
					if err := e.saveFeedbackState(); err != nil {
						slog.Warn("feedback: could not save local diagnostics")
					}
					if e.ctx.Err() != nil {
						return
					}
				case <-e.ctx.Done():
					_ = e.saveFeedbackState()
					return
				}
			}
		}()
	}
	select {
	case e.feedbackSaveCh <- struct{}{}:
	default:
	}
}

func (e *Engine) stopFeedbackStore() {
	e.feedbackMu.Lock()
	done := e.feedbackSaveDone
	e.feedbackMu.Unlock()
	if done != nil {
		<-done
	}
	if err := e.saveFeedbackState(); err != nil {
		slog.Warn("feedback: could not save local diagnostics during shutdown")
	}
}

func (e *Engine) saveFeedbackState() error {
	e.feedbackPersistMu.Lock()
	defer e.feedbackPersistMu.Unlock()
	e.feedbackMu.Lock()
	path := e.feedbackStatePath()
	if path == "" || !e.feedbackStoreLoaded {
		e.feedbackMu.Unlock()
		return nil
	}
	now := time.Now()
	e.prunePendingFeedbackLocked(now)
	e.pruneFeedbackTurnsLocked(now)
	e.pruneFeedbackSubmissionsLocked(now)
	saved := feedbackDiskState{Schema: 1, Turns: make(map[string]storedFeedbackTurn), Pending: make(map[string]storedPendingFeedback), Submissions: e.feedbackSubmissions}
	for key, turn := range e.feedbackTurns {
		turn.mu.Lock()
		saved.Turns[key] = storedFeedbackTurn{At: turn.at, Active: turn.active, Shared: turn.shared, Version: turn.version, Agent: turn.agent, Diagnostic: appfeatures.CaptureFeedbackDiagnostic(turn.diagnostic)}
		turn.mu.Unlock()
	}
	for token, pending := range e.feedbackPending {
		saved.Pending[token] = storedPendingFeedback{Draft: appfeatures.SnapshotFeedbackDraft(pending.Draft), At: pending.At, SessionKey: pending.SessionKey, UserID: pending.UserID, AgentOnly: pending.AgentOnly}
	}
	data, err := json.Marshal(saved)
	e.feedbackMu.Unlock()
	if err != nil {
		return err
	}
	if len(data) > feedbackStoreMaxBytes {
		return fmt.Errorf("feedback local state exceeds limit")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".feedback-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	// Persist the renamed intent before an external POST. Windows does not
	// support syncing directory handles; the atomic file replacement remains.
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr = directory.Close()
	if err != nil {
		return err
	}
	return closeErr
}
