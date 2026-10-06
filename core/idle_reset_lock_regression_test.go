package core

import (
	"testing"
	"time"
)

type idleResetCloseProbe struct {
	AgentSession
	check func()
}

func (s *idleResetCloseProbe) Close() error {
	s.check()
	return s.AgentSession.Close()
}

func TestIdleAutoResetLocksReplacementBeforeClosingOldProcess(t *testing.T) {
	e := newTestEngine()
	e.resetOnIdle = time.Minute
	key := "test:idle-lock"
	old := e.sessions.GetOrCreateActive(key)
	old.AddHistory("user", "old conversation")
	old.mu.Lock()
	old.LastUserActivity = time.Now().Add(-time.Hour)
	old.mu.Unlock()
	if !old.TryLock() {
		t.Fatal("old session must lock")
	}
	backend := &failOnceCodexLikeSession{threadID: "old-thread", events: make(chan Event, 4)}
	backend.alive.Store(true)
	closed := make(chan struct{})
	probe := &idleResetCloseProbe{AgentSession: backend, check: func() {
		active := e.sessions.GetOrCreateActive(key)
		if active.ID == old.ID || !active.Busy() || !old.Busy() {
			t.Errorf("closing old process without a locked replacement: active=%s busy=%v old=%s busy=%v", active.ID, active.Busy(), old.ID, old.Busy())
		}
		close(closed)
	}}
	e.interactiveStates[key] = &interactiveState{agentSession: probe, stopCh: make(chan struct{})}
	p := &stubPlatformEngine{n: "plain"}
	replacement := e.maybeAutoResetSessionOnIdle(p, &Message{SessionKey: key}, e.sessions, key, old)
	if replacement == nil || !replacement.Busy() || old.Busy() {
		t.Fatal("idle reset must transfer the busy lock")
	}
	replacement.UnlockWithoutUpdate()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("old process was not closed")
	}
}
