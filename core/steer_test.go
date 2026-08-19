package core

// Tests for busy-message steering (issue #27): the SteerableSession routing
// decision in cmdPs and trySteerBusyMessage. The Codex app-server RPC shape
// is covered in agent/codex/appserver_session_test.go; user-perspective CUJs
// live in cuj_test.go.

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// steerableAgentSession is a queuingAgentSession that also implements
// SteerableSession with a configurable outcome.
type steerableAgentSession struct {
	queuingAgentSession
	steerErr   error
	steerCalls []string
	steerMu    sync.Mutex
}

func newSteerableSession(id string) *steerableAgentSession {
	return &steerableAgentSession{
		queuingAgentSession: queuingAgentSession{
			controllableAgentSession: controllableAgentSession{
				sessionID: id,
				alive:     true,
				events:    make(chan Event, 16),
				closed:    make(chan struct{}),
			},
		},
	}
}

func (s *steerableAgentSession) Steer(prompt string, _ []ImageAttachment, _ []FileAttachment) error {
	s.steerMu.Lock()
	s.steerCalls = append(s.steerCalls, prompt)
	s.steerMu.Unlock()
	return s.steerErr
}

func (s *steerableAgentSession) getSteerCalls() []string {
	s.steerMu.Lock()
	defer s.steerMu.Unlock()
	return append([]string(nil), s.steerCalls...)
}

func (s *steerableAgentSession) getSendCalls() []string {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return append([]string(nil), s.sendCalls...)
}

// installBusySteerableState wires a steerable session into the engine and
// locks the session to simulate a turn in flight. Returns the unlock func.
func installBusySteerableState(t *testing.T, e *Engine, p Platform, key string, sess AgentSession) func() {
	t.Helper()
	state := &interactiveState{agentSession: sess, platform: p}
	e.interactiveMu.Lock()
	e.interactiveStates[key] = state
	e.interactiveMu.Unlock()

	session := e.sessions.GetOrCreateActive(key)
	if !session.TryLock() {
		t.Fatal("expected TryLock to succeed")
	}
	return session.Unlock
}

// --- /ps dispatch ---

func TestCmdPs_SteerableSession_UsesSteerNotSend(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("ps-steer")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	msg := &Message{SessionKey: key, Content: "/ps focus on tests", ReplyCtx: "ctx"}
	e.cmdPs(p, msg, []string{"focus", "on", "tests"})

	if calls := sess.getSteerCalls(); len(calls) != 1 || calls[0] != "focus on tests" {
		t.Fatalf("expected Steer(\"focus on tests\"), got %v", calls)
	}
	if calls := sess.getSendCalls(); len(calls) != 0 {
		t.Fatalf("steerable session must not receive Send from /ps, got %v", calls)
	}
	sent := p.getSent()
	found := false
	for _, s := range sent {
		if strings.Contains(s, e.i18n.T(MsgPsSent)) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MsgPsSent reply, got %v", sent)
	}
}

func TestCmdPs_SteerErrors_MapToLocalizedReplies(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantKey MsgKey
	}{
		{"unsupported backend", fmt.Errorf("exec: %w", ErrSteerUnsupported), MsgSteerUnsupportedBackend},
		{"no active turn", ErrSteerNoActiveTurn, MsgSteerTurnGone},
		{"turn mismatch", fmt.Errorf("rpc: %w", ErrSteerTurnMismatch), MsgSteerTurnGone},
		{"outcome unknown", fmt.Errorf("timeout: %w", ErrSteerOutcomeUnknown), MsgSteerOutcomeUnknown},
		{"generic rejection", fmt.Errorf("boom: %w", ErrSteerRejected), MsgPsSendFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &stubPlatformEngine{n: "test"}
			sess := newSteerableSession("ps-steer-err")
			sess.steerErr = tc.err
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

			key := "test:user1"
			unlock := installBusySteerableState(t, e, p, key, sess)
			defer unlock()

			msg := &Message{SessionKey: key, Content: "/ps x", ReplyCtx: "ctx"}
			e.cmdPs(p, msg, []string{"x"})

			// /ps must never fall back to Send on a steerable session — on
			// Codex exec that would launch a concurrent `codex exec resume`.
			if calls := sess.getSendCalls(); len(calls) != 0 {
				t.Fatalf("unexpected Send fallback: %v", calls)
			}
			sent := p.getSent()
			found := false
			for _, s := range sent {
				if strings.Contains(s, e.i18n.T(tc.wantKey)) {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected %s reply, got %v", tc.wantKey, sent)
			}
		})
	}
}

func TestCmdPs_NonSteerableSession_KeepsLegacySend(t *testing.T) {
	// Persistent-process agents without the capability keep the documented
	// mid-turn stdin injection behavior.
	p := &stubPlatformEngine{n: "test"}
	sess := newQueuingSession("ps-legacy")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	msg := &Message{SessionKey: key, Content: "/ps hello", ReplyCtx: "ctx"}
	e.cmdPs(p, msg, []string{"hello"})

	sess.sendMu.Lock()
	calls := append([]string(nil), sess.sendCalls...)
	sess.sendMu.Unlock()
	if len(calls) != 1 || calls[0] != "hello" {
		t.Fatalf("expected legacy Send(\"hello\"), got %v", calls)
	}
}

// --- ordinary busy-message routing ---

func TestTrySteerBusyMessage_DeliveredIntoActiveTurn(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("steer-ok")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetBusyMessageMode(BusyMessageModeSteer)

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	session := e.sessions.GetOrCreateActive(key)
	msg := &Message{SessionKey: key, MessageID: "m2", Content: "also check locking", ReplyCtx: "ctx2"}
	if !e.trySteerBusyMessage(p, msg, key, session, e.sessions) {
		t.Fatal("expected trySteerBusyMessage to handle the message")
	}

	if calls := sess.getSteerCalls(); len(calls) != 1 || !strings.Contains(calls[0], "also check locking") {
		t.Fatalf("expected steer call with content, got %v", calls)
	}
	// No queue entry may be created for a steered message.
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	state.mu.Lock()
	pending := len(state.pendingMessages)
	state.mu.Unlock()
	if pending != 0 {
		t.Fatalf("steered message must not enter the FIFO, found %d entries", pending)
	}
	// The steered content becomes part of the conversation history.
	hist := session.GetHistory(0)
	found := false
	for _, h := range hist {
		if h.Role == "user" && strings.Contains(h.Content, "also check locking") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected steered content in history, got %+v", hist)
	}
	sent := p.getSent()
	ack := false
	for _, s := range sent {
		if strings.Contains(s, e.i18n.T(MsgSteered)) {
			ack = true
		}
	}
	if !ack {
		t.Fatalf("expected MsgSteered ack, got %v", sent)
	}
}

func TestTrySteerBusyMessage_DefinitiveFailure_FallsBackToQueue(t *testing.T) {
	for _, steerErr := range []error{
		fmt.Errorf("exec: %w", ErrSteerUnsupported),
		ErrSteerNoActiveTurn,
		fmt.Errorf("rpc: %w", ErrSteerTurnMismatch),
		fmt.Errorf("rpc: %w", ErrSteerRejected),
	} {
		p := &stubPlatformEngine{n: "test"}
		sess := newSteerableSession("steer-fallback")
		sess.steerErr = steerErr
		e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
		e.SetBusyMessageMode(BusyMessageModeSteer)

		key := "test:user1"
		unlock := installBusySteerableState(t, e, p, key, sess)

		session := e.sessions.GetOrCreateActive(key)
		msg := &Message{SessionKey: key, MessageID: "m2", Content: "hi", ReplyCtx: "ctx2"}
		if e.trySteerBusyMessage(p, msg, key, session, e.sessions) {
			t.Fatalf("steerErr=%v: definitive failure must fall back to the queue", steerErr)
		}
		unlock()
	}
}

func TestTrySteerBusyMessage_OutcomeUnknown_NeverQueues(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("steer-unknown")
	sess.steerErr = fmt.Errorf("turn/steer timed out: %w", ErrSteerOutcomeUnknown)
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetBusyMessageMode(BusyMessageModeSteer)

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	session := e.sessions.GetOrCreateActive(key)
	msg := &Message{SessionKey: key, MessageID: "m2", Content: "risky", ReplyCtx: "ctx2"}
	if !e.trySteerBusyMessage(p, msg, key, session, e.sessions) {
		t.Fatal("unknown outcome must be final (handled=true) so the message is not re-queued")
	}

	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	state.mu.Lock()
	pending := len(state.pendingMessages)
	state.mu.Unlock()
	if pending != 0 {
		t.Fatalf("possibly-delivered message must not be queued, found %d entries", pending)
	}
	sent := p.getSent()
	warned := false
	for _, s := range sent {
		if strings.Contains(s, e.i18n.T(MsgSteerOutcomeUnknown)) {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected MsgSteerOutcomeUnknown reply, got %v", sent)
	}
}

func TestTrySteerBusyMessage_NonSteerableSession_FallsBackToQueue(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	sess := newQueuingSession("steer-nocap")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetBusyMessageMode(BusyMessageModeSteer)

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	session := e.sessions.GetOrCreateActive(key)
	msg := &Message{SessionKey: key, MessageID: "m2", Content: "hi", ReplyCtx: "ctx2"}
	if e.trySteerBusyMessage(p, msg, key, session, e.sessions) {
		t.Fatal("session without SteerableSession must fall back to the queue")
	}
	sess.sendMu.Lock()
	n := len(sess.sendCalls)
	sess.sendMu.Unlock()
	if n != 0 {
		t.Fatalf("ordinary busy message must not be force-Sent mid-turn, got %d call(s)", n)
	}
}

func TestHandleMessage_BusySteerMode_SteersInsteadOfQueueing(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("hm-steer")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetBusyMessageMode(BusyMessageModeSteer)

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	msg := &Message{SessionKey: key, MessageID: "m2", UserID: "u1", Content: "new evidence", ReplyCtx: "ctx2"}
	e.handleMessage(p, msg)

	if calls := sess.getSteerCalls(); len(calls) != 1 || !strings.Contains(calls[0], "new evidence") {
		t.Fatalf("expected message steered, got steer=%v", calls)
	}
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	state.mu.Lock()
	pending := len(state.pendingMessages)
	state.mu.Unlock()
	if pending != 0 {
		t.Fatalf("steer mode must not queue, found %d entries", pending)
	}
}

func TestHandleMessage_DefaultQueueMode_DoesNotSteer(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("hm-queue")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	// Default mode: queue. No SetBusyMessageMode call.

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	msg := &Message{SessionKey: key, MessageID: "m2", UserID: "u1", Content: "later please", ReplyCtx: "ctx2"}
	e.handleMessage(p, msg)

	if calls := sess.getSteerCalls(); len(calls) != 0 {
		t.Fatalf("default queue mode must not steer, got %v", calls)
	}
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	state.mu.Lock()
	pending := len(state.pendingMessages)
	state.mu.Unlock()
	if pending != 1 {
		t.Fatalf("expected exactly one queued message, found %d", pending)
	}
}

func TestSteerFailureAllowsQueueFallback(t *testing.T) {
	if !SteerFailureAllowsQueueFallback(fmt.Errorf("x: %w", ErrSteerUnsupported)) ||
		!SteerFailureAllowsQueueFallback(ErrSteerNoActiveTurn) ||
		!SteerFailureAllowsQueueFallback(fmt.Errorf("x: %w", ErrSteerTurnMismatch)) ||
		!SteerFailureAllowsQueueFallback(fmt.Errorf("x: %w", ErrSteerRejected)) {
		t.Fatal("definitive steer failures must allow queue fallback")
	}
	if SteerFailureAllowsQueueFallback(fmt.Errorf("x: %w", ErrSteerOutcomeUnknown)) {
		t.Fatal("unknown steer outcome must NOT allow queue fallback")
	}
	if SteerFailureAllowsQueueFallback(errors.New("unrelated")) {
		t.Fatal("unrelated errors must not allow queue fallback")
	}
}
