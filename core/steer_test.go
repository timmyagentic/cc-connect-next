package core

// Tests for busy-message steering (issue #27): the SteerableSession routing
// decision in cmdPs and trySteerBusyMessage. The Codex app-server RPC shape
// is covered in agent/codex/appserver_session_test.go; user-perspective CUJs
// live in cuj_test.go.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
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
// locks the session to simulate a turn in flight (with the event loop's
// steer adoption window open). Returns the unlock func.
func installBusySteerableState(t *testing.T, e *Engine, p Platform, key string, sess AgentSession) func() {
	t.Helper()
	state := &interactiveState{agentSession: sess, platform: p, presentationOpen: true}
	e.interactiveMu.Lock()
	e.interactiveStates[key] = state
	e.interactiveMu.Unlock()

	session := e.sessions.GetOrCreateActive(key)
	if !session.TryLock() {
		t.Fatal("expected TryLock to succeed")
	}
	return session.Unlock
}

func pendingHandoffsFor(e *Engine, key string) []steerHandoff {
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return append([]steerHandoff(nil), state.pendingHandoffs...)
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
	// The successful steer registers a presentation handoff for the event loop.
	if handoffs := pendingHandoffsFor(e, key); len(handoffs) != 1 {
		t.Fatalf("expected one pending presentation handoff, got %d", len(handoffs))
	}
}

func TestCmdPs_SteerAfterTurnFinalized_ResolvesWithoutHandoff(t *testing.T) {
	// The steer RPC succeeds, but the turn's presentation already closed
	// (turn finalized while the RPC was in flight): the user is told the
	// input merged into the completed answer, and no handoff is registered.
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("ps-steer-closed")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	state.mu.Lock()
	state.presentationOpen = false
	state.mu.Unlock()

	msg := &Message{SessionKey: key, Content: "/ps x", ReplyCtx: "ctx"}
	e.cmdPs(p, msg, []string{"x"})

	sent := p.getSent()
	found := false
	for _, s := range sent {
		if strings.Contains(s, e.i18n.T(MsgSteerMergedCompleted)) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MsgSteerMergedCompleted reply, got %v", sent)
	}
	if handoffs := pendingHandoffsFor(e, key); len(handoffs) != 0 {
		t.Fatalf("closed presentation must not accept handoffs, got %d", len(handoffs))
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
	// The successful steer registers a presentation handoff for the event
	// loop, carrying the steered message's own reply context.
	handoffs := pendingHandoffsFor(e, key)
	if len(handoffs) != 1 {
		t.Fatalf("expected one pending presentation handoff, got %d", len(handoffs))
	}
	if handoffs[0].messageID != "m2" || handoffs[0].replyCtx != "ctx2" {
		t.Fatalf("handoff must carry the steered message context, got %+v", handoffs[0])
	}
}

func TestTrySteerBusyMessageTouchesLastUserActivity(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("steer-activity")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetBusyMessageMode(BusyMessageModeSteer)

	key := "test:steer-activity"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()
	session := e.sessions.GetOrCreateActive(key)
	stale := time.Now().Add(-time.Hour)
	session.mu.Lock()
	session.LastUserActivity = stale
	session.mu.Unlock()

	msg := &Message{SessionKey: key, MessageID: "m2", Content: "steered", ReplyCtx: "ctx2"}
	if !e.trySteerBusyMessage(p, msg, key, session, e.sessions) {
		t.Fatal("expected steer to be accepted")
	}
	if got := session.GetLastUserActivity(); !got.After(stale) {
		t.Fatalf("last user activity = %v, want after %v", got, stale)
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

func TestHandleMessage_DefaultMode_Steers(t *testing.T) {
	// Since v0.1.3 the default busy-message mode is steer.
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("hm-default")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	// No SetBusyMessageMode call: the default applies.

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	msg := &Message{SessionKey: key, MessageID: "m2", UserID: "u1", Content: "by default", ReplyCtx: "ctx2"}
	e.handleMessage(p, msg)

	if calls := sess.getSteerCalls(); len(calls) != 1 || !strings.Contains(calls[0], "by default") {
		t.Fatalf("default mode must steer, got %v", calls)
	}

	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	state.mu.Lock()
	pending := len(state.pendingMessages)
	state.mu.Unlock()
	if pending != 0 {
		t.Fatalf("default steer must not queue, found %d entries", pending)
	}
}

func TestHandleMessage_ExplicitQueueMode_DoesNotSteer(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	sess := newSteerableSession("hm-queue")
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetBusyMessageMode(BusyMessageModeQueue)

	key := "test:user1"
	unlock := installBusySteerableState(t, e, p, key, sess)
	defer unlock()

	msg := &Message{SessionKey: key, MessageID: "m2", UserID: "u1", Content: "later please", ReplyCtx: "ctx2"}
	e.handleMessage(p, msg)

	if calls := sess.getSteerCalls(); len(calls) != 0 {
		t.Fatalf("explicit queue mode must not steer, got %v", calls)
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

// ---------------------------------------------------------------------------
// Event-loop-level presentation handoff tests (issue #27 card handoff).
// ---------------------------------------------------------------------------

type steerCardCall struct {
	kind     string // "start" | "update" | "stream" | "delete"
	handle   string
	replyCtx any
	content  string
}

// steerRichCardPlatform is a handle-aware rich-card platform stub: every card
// operation records which card it targeted so handoff tests can assert that a
// frozen card never receives another update.
type steerRichCardPlatform struct {
	stubPlatformEngine
	cardMu sync.Mutex
	seq    int
	calls  []steerCardCall
}

func (p *steerRichCardPlatform) BuildRichCard(status CardStatus, phase string, steps []ToolStep, markdown string, _ bool, _ string) string {
	return fmt.Sprintf("status=%s phase=%s steps=%d body=%q", status, phase, len(steps), markdown)
}

func (p *steerRichCardPlatform) SendPreviewStart(_ context.Context, replyCtx any, content string) (any, error) {
	p.cardMu.Lock()
	defer p.cardMu.Unlock()
	p.seq++
	handle := fmt.Sprintf("card-%d", p.seq)
	p.calls = append(p.calls, steerCardCall{kind: "start", handle: handle, replyCtx: replyCtx, content: content})
	return handle, nil
}

func (p *steerRichCardPlatform) UpdateMessage(_ context.Context, handle any, content string) error {
	p.cardMu.Lock()
	defer p.cardMu.Unlock()
	p.calls = append(p.calls, steerCardCall{kind: "update", handle: fmt.Sprint(handle), content: content})
	return nil
}

func (p *steerRichCardPlatform) StreamRichCardText(_ context.Context, handle any, fullText string) error {
	p.cardMu.Lock()
	defer p.cardMu.Unlock()
	p.calls = append(p.calls, steerCardCall{kind: "stream", handle: fmt.Sprint(handle), content: fullText})
	return nil
}

func (p *steerRichCardPlatform) DeletePreviewMessage(_ context.Context, handle any) error {
	p.cardMu.Lock()
	defer p.cardMu.Unlock()
	p.calls = append(p.calls, steerCardCall{kind: "delete", handle: fmt.Sprint(handle)})
	return nil
}

func (p *steerRichCardPlatform) cardCalls() []steerCardCall {
	p.cardMu.Lock()
	defer p.cardMu.Unlock()
	return append([]steerCardCall(nil), p.calls...)
}

func (p *steerRichCardPlatform) callsFor(handle string) []steerCardCall {
	var out []steerCardCall
	for _, c := range p.cardCalls() {
		if c.handle == handle {
			out = append(out, c)
		}
	}
	return out
}

func waitForSteer(t *testing.T, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", desc)
}

// steerCUJHarness spins up a real event loop over a controllable steerable
// session with a rich-card platform.
type steerCUJHarness struct {
	e       *Engine
	p       *steerRichCardPlatform
	sess    *steerableAgentSession
	state   *interactiveState
	session *Session
	key     string
	done    chan struct{}
}

func newSteerCUJHarness(t *testing.T, key string) *steerCUJHarness {
	t.Helper()
	p := &steerRichCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich", ThinkingMessages: true, ToolMessages: true})
	e.SetBusyMessageMode(BusyMessageModeSteer)

	sess := newSteerableSession("cuj-" + key)
	state := &interactiveState{agentSession: sess, platform: p, replyCtx: "ctx-m1"}
	e.interactiveMu.Lock()
	e.interactiveStates[key] = state
	e.interactiveMu.Unlock()
	session := e.sessions.GetOrCreateActive(key)

	h := &steerCUJHarness{e: e, p: p, sess: sess, state: state, session: session, key: key, done: make(chan struct{})}
	go func() {
		defer close(h.done)
		e.processInteractiveEvents(state, session, e.sessions, key, "m1", time.Now(), nil, nil, "ctx-m1")
	}()
	// The initial lifecycle card (C1) appears before the first agent event.
	waitForSteer(t, "initial card", func() bool { return len(p.callsFor("card-1")) > 0 })
	return h
}

func (h *steerCUJHarness) steer(t *testing.T, msgID, content, replyCtx string) {
	t.Helper()
	msg := &Message{SessionKey: h.key, MessageID: msgID, Content: content, ReplyCtx: replyCtx, Platform: "feishu"}
	if !h.e.trySteerBusyMessage(h.p, msg, h.key, h.session, h.e.sessions) {
		t.Fatalf("steer of %s was not handled", msgID)
	}
}

func (h *steerCUJHarness) finish(t *testing.T, finalAnswer string) {
	t.Helper()
	h.sess.events <- Event{Type: EventText, Content: finalAnswer}
	h.sess.events <- Event{Type: EventResult, Content: finalAnswer, Done: true}
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("event loop did not finish")
	}
}

func lastRedirect(calls []steerCardCall) (steerCardCall, bool) {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].kind == "update" && strings.Contains(calls[i].content, "status=redirected") {
			return calls[i], true
		}
	}
	return steerCardCall{}, false
}

func TestCUJ_Steer1_PartialFrozenThenFinalAnswerOnlyInSuccessorCard(t *testing.T) {
	h := newSteerCUJHarness(t, "feishu:user-steer-cuj1")

	// C1 shows a visible partial answer.
	h.sess.events <- Event{Type: EventText, Content: "partial answer one"}
	waitForSteer(t, "partial visible in C1", func() bool {
		for _, c := range h.p.callsFor("card-1") {
			if (c.kind == "stream" || c.kind == "update") && strings.Contains(c.content, "partial answer one") {
				return true
			}
		}
		return false
	})

	// M2 steers the running turn.
	h.steer(t, "m2", "focus on tests", "ctx-m2")

	// C2 was created replying to M2 in the pending steering phase, and C1
	// freezes into the neutral redirected state retaining its partial.
	waitForSteer(t, "C1 frozen redirected", func() bool {
		_, ok := lastRedirect(h.p.callsFor("card-1"))
		return ok
	})
	c2calls := h.p.callsFor("card-2")
	if len(c2calls) == 0 || c2calls[0].kind != "start" || !strings.Contains(c2calls[0].content, "phase=steering") {
		t.Fatalf("expected C2 created in steering phase, got %+v", c2calls)
	}
	if c2calls[0].replyCtx != "ctx-m2" {
		t.Fatalf("C2 must reply to M2's context, got %v", c2calls[0].replyCtx)
	}
	redirect, _ := lastRedirect(h.p.callsFor("card-1"))
	if !strings.Contains(redirect.content, "partial answer one") {
		t.Fatalf("frozen C1 must retain the visible partial, got %q", redirect.content)
	}

	frozenAt := len(h.p.callsFor("card-1"))

	// The turn finishes: the final answer must render only in C2.
	h.finish(t, "final answer two")

	c1calls := h.p.callsFor("card-1")
	if len(c1calls) != frozenAt {
		t.Fatalf("C1 received updates after freeze: %+v", c1calls[frozenAt:])
	}
	doneFound := false
	for _, c := range h.p.callsFor("card-2") {
		if c.kind == "update" && strings.Contains(c.content, "status=done") {
			doneFound = true
			if !strings.Contains(c.content, "final answer two") {
				t.Fatalf("C2 done card missing final answer: %q", c.content)
			}
			if strings.Contains(c.content, "partial answer one") {
				t.Fatalf("C2 must not duplicate C1's partial: %q", c.content)
			}
		}
		if strings.Contains(c.content, "status=done") && c.handle != "card-2" {
			t.Fatalf("done state leaked to %s", c.handle)
		}
	}
	if !doneFound {
		t.Fatal("expected C2 to reach done with the final answer")
	}
	for _, c := range c1calls {
		if strings.Contains(c.content, "status=done") {
			t.Fatalf("C1 must never be marked done, got %q", c.content)
		}
	}

	// History: steered user message, frozen partial, and final answer.
	hist := h.session.GetHistory(0)
	var flat []string
	for _, entry := range hist {
		flat = append(flat, entry.Role+":"+entry.Content)
	}
	joined := strings.Join(flat, "\n")
	for _, want := range []string{"user:focus on tests", "partial answer one", "final answer two"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("history missing %q:\n%s", want, joined)
		}
	}
}

func TestCUJ_Steer2_RapidDoubleSteerLeavesOnlyNewestCardActive(t *testing.T) {
	h := newSteerCUJHarness(t, "feishu:user-steer-cuj2")

	h.sess.events <- Event{Type: EventText, Content: "partial answer one"}
	waitForSteer(t, "partial visible in C1", func() bool {
		for _, c := range h.p.callsFor("card-1") {
			if strings.Contains(c.content, "partial answer one") {
				return true
			}
		}
		return false
	})

	h.steer(t, "m2", "steer two", "ctx-m2")
	waitForSteer(t, "C1 frozen", func() bool { _, ok := lastRedirect(h.p.callsFor("card-1")); return ok })
	h.steer(t, "m3", "steer three", "ctx-m3")
	waitForSteer(t, "C2 frozen", func() bool { _, ok := lastRedirect(h.p.callsFor("card-2")); return ok })

	h.finish(t, "final answer")

	for _, handle := range []string{"card-1", "card-2"} {
		for _, c := range h.p.callsFor(handle) {
			if strings.Contains(c.content, "status=done") {
				t.Fatalf("%s must not be done, got %q", handle, c.content)
			}
		}
	}
	doneFound := false
	for _, c := range h.p.callsFor("card-3") {
		if c.kind == "update" && strings.Contains(c.content, "status=done") && strings.Contains(c.content, "final answer") {
			doneFound = true
		}
	}
	if !doneFound {
		t.Fatal("expected only C3 to reach done with the final answer")
	}
}

func TestCUJ_Steer3_ErrorAfterSteerLandsInSuccessorCard(t *testing.T) {
	h := newSteerCUJHarness(t, "feishu:user-steer-cuj3")

	h.sess.events <- Event{Type: EventText, Content: "partial answer one"}
	waitForSteer(t, "partial visible in C1", func() bool {
		for _, c := range h.p.callsFor("card-1") {
			if strings.Contains(c.content, "partial answer one") {
				return true
			}
		}
		return false
	})

	h.steer(t, "m2", "steer two", "ctx-m2")
	waitForSteer(t, "C1 frozen", func() bool { _, ok := lastRedirect(h.p.callsFor("card-1")); return ok })

	h.sess.events <- Event{Type: EventError, Error: errors.New("boom")}
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("event loop did not finish after error")
	}

	// The failure renders in the successor card; the frozen card stays
	// redirected and never flips to error.
	errFound := false
	for _, c := range h.p.callsFor("card-2") {
		if c.kind == "update" && strings.Contains(c.content, "status=error") {
			errFound = true
		}
	}
	if !errFound {
		t.Fatal("expected error state in C2")
	}
	for _, c := range h.p.callsFor("card-1") {
		if strings.Contains(c.content, "status=error") || strings.Contains(c.content, "status=done") {
			t.Fatalf("frozen C1 mutated after handoff: %q", c.content)
		}
	}
}
