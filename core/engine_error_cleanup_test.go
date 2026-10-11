package core

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// Only the external session is faked; cleanup and error handling are the real
// production paths. Atomic liveness keeps the fixture from introducing races.
type errorCleanupSession struct {
	stubAgentSession
	dead       atomic.Bool
	closeCount atomic.Int32
	events     chan Event
}

func (s *errorCleanupSession) CurrentSessionID() string { return "" }
func (s *errorCleanupSession) Events() <-chan Event     { return s.events }
func (s *errorCleanupSession) Alive() bool              { return !s.dead.Load() }
func (s *errorCleanupSession) Close() error {
	s.dead.Store(true)
	s.closeCount.Add(1)
	return nil
}

func cleanupTestFailure(e *Engine, state *interactiveState, key string) *turnFailure {
	return &turnFailure{
		processor:                 &turnProcessor{engine: e, state: state, sessionKey: key},
		event:                     Event{Type: EventError, Error: errors.New("turn failed")},
		platform:                  &stubPlatformEngine{n: "test"},
		progress:                  &compactProgressWriter{},
		streamPreview:             &streamPreview{},
		discardStreamingCard:      func() {},
		persistVisibleRichPartial: func(Platform) string { return "" },
		usesRichCard:              func(Platform) bool { return true },
		sendRichFailure:           func(Platform, any, any, string, ...string) {},
	}
}

func TestTurnFailure_ConcurrentCleanup(t *testing.T) {
	for i := 0; i < 32; i++ {
		e := newTestEngine()
		key := "test:error-cleanup"
		s := &errorCleanupSession{}
		state := &interactiveState{agentSession: s}
		e.interactiveStates[key] = state
		start := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			<-start
			e.cleanupInteractiveState(key, state)
		}()
		failure := cleanupTestFailure(e, state, key)
		// Error delivery occurs after handle's initial locked snapshot. Release
		// cleanup here so the final liveness check races with session teardown.
		failure.sendRichFailure = func(Platform, any, any, string, ...string) { close(start) }
		failure.handle()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("cleanup blocked during error handling")
		}
		if got := s.closeCount.Load(); got != 1 {
			t.Fatalf("Close count = %d, want 1", got)
		}
	}
}

func TestTurnFailure_QueuedMessagesFollowSessionLiveness(t *testing.T) {
	for _, status := range []string{"alive", "dead", "nil"} {
		t.Run(status, func(t *testing.T) {
			e := newTestEngine()
			p := &stubPlatformEngine{n: "test"}
			s := &errorCleanupSession{}
			s.dead.Store(status == "dead")
			state := &interactiveState{
				agentSession:    s,
				pendingMessages: []queuedMessage{{platform: p, content: "next request"}},
			}
			if status == "nil" {
				state.agentSession = nil
			}
			cleanupTestFailure(e, state, "test:queue").handle()
			wantQueued, wantNotices := 0, 1
			if status == "alive" {
				wantQueued, wantNotices = 1, 0
			}
			if got := len(state.pendingMessages); got != wantQueued {
				t.Fatalf("queued messages = %d, want %d", got, wantQueued)
			}
			if got := len(p.getSent()); got != wantNotices {
				t.Fatalf("dropped-message notices = %d, want %d", got, wantNotices)
			}
		})
	}
}

type sendErrorCleanupPlatform struct {
	stubPlatformEngine
	onQueuedNotice func()
}

func (p *sendErrorCleanupPlatform) Send(ctx context.Context, replyCtx any, content string) error {
	err := p.stubPlatformEngine.Send(ctx, replyCtx, content)
	if replyCtx == "queued" && p.onQueuedNotice != nil {
		p.onQueuedNotice()
	}
	return err
}

func TestOnSendError_ConcurrentCleanup(t *testing.T) {
	for i := 0; i < 32; i++ {
		p := &sendErrorCleanupPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
		e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
		key := "test:send-error-cleanup"
		s := &errorCleanupSession{events: make(chan Event)}
		state := &interactiveState{
			agentSession: s, platform: p, replyCtx: "current",
			pendingMessages: []queuedMessage{{platform: p, replyCtx: "queued", content: "next request"}},
		}
		e.interactiveStates[key] = state
		start := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			<-start
			e.cleanupInteractiveState(key, state)
		}()
		// onSendError first drops the queued messages. Release teardown from
		// that notification, immediately before its session liveness check.
		p.onQueuedNotice = func() {
			close(start)
			// Let the released teardown run before the event loop's deferred
			// state locks can accidentally serialize both accesses. Yielding
			// adds no happens-before edge between the conflicting accesses.
			runtime.Gosched()
		}
		sendDone := make(chan error, 1)
		sendDone <- errors.New("prompt send failed")
		e.processInteractiveEvents(state, e.sessions.GetOrCreateActive(key), e.sessions,
			key, "message", time.Now(), nil, sendDone, "current")
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("cleanup blocked during send failure")
		}
		if got := s.closeCount.Load(); got != 1 {
			t.Fatalf("Close count = %d, want 1", got)
		}
		if got := len(p.getSent()); got != 2 {
			t.Fatalf("error notices = %d, want one queued and one current", got)
		}
	}
}

func TestOnSendError_PreservesLiveSessionAndCleansDeadSession(t *testing.T) {
	for _, dead := range []bool{false, true} {
		t.Run(map[bool]string{false: "alive", true: "dead"}[dead], func(t *testing.T) {
			p := &stubPlatformEngine{n: "test"}
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			key := "test:send-error-liveness"
			s := &errorCleanupSession{events: make(chan Event)}
			s.dead.Store(dead)
			state := &interactiveState{
				agentSession: s, platform: p, replyCtx: "current",
				pendingMessages: []queuedMessage{{platform: p, replyCtx: "queued", content: "next request"}},
			}
			e.interactiveStates[key] = state
			sendDone := make(chan error, 1)
			sendDone <- errors.New("prompt send failed")
			e.processInteractiveEvents(state, e.sessions.GetOrCreateActive(key), e.sessions,
				key, "message", time.Now(), nil, sendDone, "current")
			_, retained := e.interactiveStates[key]
			if retained == dead {
				t.Fatalf("session retained = %v, want %v", retained, !dead)
			}
			if got := len(p.getSent()); got != 2 {
				t.Fatalf("error notices = %d, want one queued and one current", got)
			}
			if len(state.pendingMessages) != 0 {
				t.Fatal("send failure left queued messages behind")
			}
		})
	}
}
