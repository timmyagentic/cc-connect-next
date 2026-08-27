package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSwitchSessionMarksExplicitActivation(t *testing.T) {
	sm := NewSessionManager("")
	first := sm.NewSession("user", "first")
	sm.NewSession("user", "second")

	before := time.Now()
	switched, err := sm.SwitchSession("user", first.ID)
	after := time.Now()
	if err != nil {
		t.Fatalf("SwitchSession() error = %v", err)
	}
	activatedAt := switched.GetExplicitActivatedAt()
	if activatedAt.IsZero() || activatedAt.Before(before) || activatedAt.After(after) {
		t.Fatalf("ExplicitActivatedAt = %v, want between %v and %v", activatedAt, before, after)
	}
}

func TestExplicitActivationPreventsImmediateIdleReset(t *testing.T) {
	e := newTestEngine()
	e.SetResetOnIdle(30 * time.Minute)
	sm := NewSessionManager("")

	stale := sm.NewSession("user", "stale")
	stale.AddHistory("user", "old context")
	stale.SetAgentSessionID("agent-stale", "codex")
	stale.mu.Lock()
	stale.LastUserActivity = time.Now().Add(-2 * time.Hour)
	stale.UpdatedAt = stale.LastUserActivity
	stale.mu.Unlock()
	sm.NewSession("user", "current")

	switched, err := sm.SwitchSession("user", stale.ID)
	if err != nil {
		t.Fatalf("SwitchSession() error = %v", err)
	}
	if !switched.TryLock() {
		t.Fatal("switched session should be lockable")
	}
	defer switched.UnlockWithoutUpdate()

	p := &stubPlatformEngine{n: "test"}
	msg := &Message{SessionKey: "user", ReplyCtx: "ctx"}
	if rotated := e.maybeAutoResetSessionOnIdle(p, msg, sm, "test:user", switched); rotated != nil {
		t.Fatal("the first message after an explicit switch must stay in the selected session")
	}
}

func TestExplicitActivationExpiresAtNormalIdleThreshold(t *testing.T) {
	e := newTestEngine()
	e.SetResetOnIdle(30 * time.Minute)
	sm := NewSessionManager("")

	session := sm.GetOrCreateActive("user")
	session.AddHistory("user", "old context")
	session.SetAgentSessionID("agent-stale", "codex")
	old := time.Now().Add(-2 * time.Hour)
	session.mu.Lock()
	session.LastUserActivity = old
	session.UpdatedAt = old
	session.ExplicitActivatedAt = old
	session.mu.Unlock()
	if !session.TryLock() {
		t.Fatal("session should be lockable")
	}

	p := &stubPlatformEngine{n: "test"}
	msg := &Message{SessionKey: "user", ReplyCtx: "ctx"}
	if rotated := e.maybeAutoResetSessionOnIdle(p, msg, sm, "test:user", session); rotated == nil {
		t.Fatal("an explicit activation older than reset_on_idle_mins must not block normal rotation")
	}
}

func TestExplicitActivationPersistsAcrossRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	sm := NewSessionManager(path)
	first := sm.NewSession("user", "first")
	sm.NewSession("user", "second")
	if _, err := sm.SwitchSession("user", first.ID); err != nil {
		t.Fatalf("SwitchSession() error = %v", err)
	}

	restored := NewSessionManager(path).GetOrCreateActive("user")
	if restored.ID != first.ID {
		t.Fatalf("restored active session = %q, want %q", restored.ID, first.ID)
	}
	if restored.GetExplicitActivatedAt().IsZero() {
		t.Fatal("explicit activation must survive session persistence")
	}
}
