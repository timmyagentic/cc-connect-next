package core

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recoveringJourneySession struct {
	events  chan Event
	thread  atomic.Value
	sends   atomic.Int32
	release chan struct{}
	closed  chan struct{}
	once    sync.Once
	onReady func() error
}

func (s *recoveringJourneySession) Send(_ string, _ []ImageAttachment, _ []FileAttachment) error {
	if s.sends.Add(1) != 1 {
		s.events <- Event{Type: EventResult, Content: "Queued follow-up completed", Done: true}
		return nil
	}
	go func() {
		s.events <- Event{Type: EventSessionRecovery}
		select {
		case <-s.release:
		case <-s.closed:
			return
		}
		s.thread.Store("restored-thread")
		ready := make(chan error, 1)
		s.events <- Event{Type: EventSessionRecovery, SessionID: "restored-thread", RecoveryReady: ready}
		select {
		case err := <-ready:
			if err == nil && s.onReady != nil {
				err = s.onReady()
			}
			if err != nil {
				s.events <- Event{Type: EventError, Error: err}
				return
			}
		case <-s.closed:
			return
		}
		s.events <- Event{Type: EventResult, Content: "Recovered answer using the recorded result", Done: true}
	}()
	return nil
}

func (s *recoveringJourneySession) RespondPermission(string, PermissionResult) error { return nil }
func (s *recoveringJourneySession) Events() <-chan Event                             { return s.events }
func (s *recoveringJourneySession) CurrentSessionID() string {
	id, _ := s.thread.Load().(string)
	return id
}
func (s *recoveringJourneySession) Alive() bool {
	select {
	case <-s.closed:
		return false
	default:
		return true
	}
}
func (s *recoveringJourneySession) Close() error { s.once.Do(func() { close(s.closed) }); return nil }

func TestProcessInteractiveEvents_RecoveryKeepsRichCardAndPersistsBinding(t *testing.T) {
	p := &stubCompactProgressPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}, style: "card", supportPayload: true}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	e.SetDisplayConfig(DisplayCfg{CardMode: "rich", Mode: "full"})
	key := "test:recovery-card"
	session := e.sessions.GetOrCreateActive(key)
	session.SetAgentSessionID("original", "stub")
	backend := newControllableSession("replacement")
	state := &interactiveState{agentSession: backend, platform: p, replyCtx: "original-reply"}
	ready := make(chan error, 1)
	backend.events <- Event{Type: EventSessionRecovery}
	backend.events <- Event{Type: EventSessionRecovery, SessionID: "replacement", RecoveryReady: ready}
	backend.events <- Event{Type: EventResult, Content: "Finished", Done: true}
	e.processInteractiveEvents(state, session, e.sessions, key, "original-message", time.Now(), nil, nil, state.replyCtx)
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	if got := NewSessionManager(e.sessions.storePath).GetOrCreateActive(key).GetAgentSessionID(); got != "replacement" {
		t.Fatalf("binding=%s", got)
	}
	if len(p.getPreviewStarts()) != 1 {
		t.Fatal("recovery created a second card")
	}
	rendered := strings.Join(p.getPreviewEdits(), "\n")
	if !strings.Contains(rendered, "Restoring the conversation") || !strings.Contains(rendered, "Conversation restored") || !strings.Contains(rendered, "Finished") {
		t.Fatalf("card missed recovery or final state: %s", rendered)
	}
}

func TestSessionRecoveryAcknowledgesPersistenceFailure(t *testing.T) {
	e := newTestEngine()
	session := e.sessions.GetOrCreateActive("test:recovery")
	// A file cannot contain the target directory.
	e.sessions.storePath = filepath.Join(t.TempDir(), "blocked", "sessions.json")
	if err := AtomicWriteFile(filepath.Dir(e.sessions.storePath), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	processor := &turnProcessor{engine: e, session: session, sessions: e.sessions}
	if err := processor.persistAgentSessionID("replacement"); err == nil {
		t.Fatal("recovery would continue after its session binding failed to persist")
	}
}

func recoveryDiskBindingCheck(path, key string) func() error {
	return func() error {
		if NewSessionManager(path).GetOrCreateActive(key).GetAgentSessionID() != "restored-thread" {
			return errors.New("continued before binding was durable")
		}
		return nil
	}
}

func TestSessionRecoveryMatchingBindingStillRequiresDurableSave(t *testing.T) {
	e := newTestEngine()
	session := e.sessions.GetOrCreateActive("test:recovery-repeat")
	e.sessions.storePath = filepath.Join(t.TempDir(), "blocked", "sessions.json")
	if err := AtomicWriteFile(filepath.Dir(e.sessions.storePath), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	processor := &turnProcessor{engine: e, session: session, sessions: e.sessions}
	for attempt := 0; attempt < 2; attempt++ {
		if err := processor.persistAgentSessionID("replacement"); err == nil {
			t.Fatalf("attempt %d acknowledged an unpersisted binding", attempt)
		}
	}
	e.sessions.storePath = filepath.Join(t.TempDir(), "sessions.json")
	if err := processor.persistAgentSessionID("replacement"); err != nil {
		t.Fatal(err)
	}
	if got := NewSessionManager(e.sessions.storePath).GetOrCreateActive("test:recovery-repeat").GetAgentSessionID(); got != "replacement" {
		t.Fatalf("saved binding=%q", got)
	}
}
