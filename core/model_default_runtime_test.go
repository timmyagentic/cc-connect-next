package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// This backend selects a model only when its process starts. It deliberately
// implements neither PerTurnModelSupporter nor TurnOptionsSession.
type modelDefaultProcessAgent struct {
	mu        sync.Mutex
	model     string
	startIDs  []string
	calls     []cronModelCall
	backends  []*modelDefaultProcessSession
	resumeErr error
	onSend    func(*modelDefaultProcessSession, cronModelCall)
}

func (a *modelDefaultProcessAgent) Name() string { return "model-default-process" }
func (a *modelDefaultProcessAgent) Stop() error  { return nil }
func (a *modelDefaultProcessAgent) ListSessions(context.Context) ([]AgentSessionInfo, error) {
	return nil, nil
}
func (a *modelDefaultProcessAgent) AvailableModels(context.Context) []ModelOption {
	return []ModelOption{{Name: "project-A"}, {Name: "project-C"}}
}
func (a *modelDefaultProcessAgent) GetModel() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.model
}
func (a *modelDefaultProcessAgent) SetModel(model string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.model = model
}
func (a *modelDefaultProcessAgent) StartSession(_ context.Context, id string) (AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.startIDs = append(a.startIDs, id)
	// Fresh starts intentionally work during resume failures: a silent fallback
	// would therefore deliver a reply and be observable by the regression test.
	if id != "" && a.resumeErr != nil {
		return nil, a.resumeErr
	}
	if id == "" {
		id = fmt.Sprintf("process-backend-%d", len(a.backends)+1)
	}
	s := &modelDefaultProcessSession{agent: a, model: a.model, id: id, events: make(chan Event, 16)}
	a.backends = append(a.backends, s)
	return s, nil
}
func (a *modelDefaultProcessAgent) snapshot() ([]string, []cronModelCall, []*modelDefaultProcessSession) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.startIDs...), append([]cronModelCall(nil), a.calls...), append([]*modelDefaultProcessSession(nil), a.backends...)
}

type modelDefaultProcessSession struct {
	agent  *modelDefaultProcessAgent
	model  string
	id     string
	events chan Event
	mu     sync.Mutex
	closed bool
}

func (s *modelDefaultProcessSession) Send(prompt string, _ []ImageAttachment, _ []FileAttachment) error {
	call := cronModelCall{prompt: prompt, model: s.model}
	s.agent.mu.Lock()
	s.agent.calls = append(s.agent.calls, call)
	onSend := s.agent.onSend
	s.agent.mu.Unlock()
	if onSend != nil {
		onSend(s, call)
	} else {
		s.events <- Event{Type: EventResult, Content: "model=" + s.model + " " + prompt, Done: true}
	}
	return nil
}
func (s *modelDefaultProcessSession) RespondPermission(string, PermissionResult) error { return nil }
func (s *modelDefaultProcessSession) Events() <-chan Event                             { return s.events }
func (s *modelDefaultProcessSession) CurrentSessionID() string                         { return s.id }
func (s *modelDefaultProcessSession) GetModel() string                                 { return s.model }
func (s *modelDefaultProcessSession) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed
}
func (s *modelDefaultProcessSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

type modelDefaultLivenessTransitionSession struct {
	*modelDefaultProcessSession
	probeMu sync.Mutex
	probes  int
}

func (s *modelDefaultLivenessTransitionSession) Alive() bool {
	s.probeMu.Lock()
	defer s.probeMu.Unlock()
	s.probes++
	// Still alive at the expected-state gate, exited by the live-session check.
	return s.probes == 1
}

func modelDefaultProcessTurn(t *testing.T, e *Engine, p *cronModelPlatform, msg *Message, model string) {
	t.Helper()
	e.ReceiveMessage(p, msg)
	want := "model=" + model + " " + msg.Content
	waitAnswerProfileTest(t, want, func() bool {
		return strings.Contains(strings.Join(p.getSent(), "\n"), want) && !e.sessions.GetOrCreateActive(msg.SessionKey).Busy()
	})
}

func TestModelDefaultRuntime_ProcessBoundChatsRecycleAtNextTurnWithoutLosingHistory(t *testing.T) {
	a := &modelDefaultProcessAgent{model: "project-A"}
	e, p := newCronModelEngine(t, a, "")
	e.SetAdminFrom("owner")
	e.SetBusyMessageMode(BusyMessageModeQueue)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a.onSend = func(s *modelDefaultProcessSession, call cronModelCall) {
		if call.prompt == "busy" {
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "model=" + call.model + " " + call.prompt, Done: true}
	}
	modelDefaultProcessTurn(t, e, p, chatModelMessage("A", "", "a-before", "remember-A"), "project-A")
	modelDefaultProcessTurn(t, e, p, chatModelMessage("B", "", "b-before", "remember-B"), "project-A")
	aSession, bSession := e.sessions.GetOrCreateActive("test:A"), e.sessions.GetOrCreateActive("test:B")
	aID, bID := aSession.GetAgentSessionID(), bSession.GetAgentSessionID()
	e.ReceiveMessage(p, chatModelMessage("B", "", "busy", "busy"))
	waitAnswerProfileTest(t, "B in flight", func() bool { _, calls, _ := a.snapshot(); return len(calls) == 3 })
	aHistory, bHistory := aSession.GetHistory(0), bSession.GetHistory(0)
	_, _, original := a.snapshot()
	e.ReceiveMessage(p, chatModelMessage("A", "", "default", "/model default project-C"))
	if !original[0].Alive() || !original[1].Alive() || !bSession.Busy() {
		t.Fatal("model command immediately closed an existing backend")
	}
	if aSession.GetAgentSessionID() != aID || bSession.GetAgentSessionID() != bID || !reflect.DeepEqual(aHistory, aSession.GetHistory(0)) || !reflect.DeepEqual(bHistory, bSession.GetHistory(0)) {
		t.Fatal("default change reset conversation IDs or history")
	}
	e.ReceiveMessage(p, chatModelMessage("B", "", "queued", "queued-B"))
	modelDefaultProcessTurn(t, e, p, chatModelMessage("A", "", "a-after", "after-A"), "project-C")
	if !original[1].Alive() {
		t.Fatal("another chat's next turn interrupted B")
	}
	once.Do(func() { close(release) })
	waitAnswerProfileTest(t, "B queue uses new process model", func() bool {
		return strings.Contains(strings.Join(p.getSent(), "\n"), "model=project-C queued-B") && !bSession.Busy()
	})
	starts, calls, backends := a.snapshot()
	if !reflect.DeepEqual(starts, []string{"", "", aID, bID}) || len(backends) != 4 {
		t.Fatalf("expected a single resume per existing chat, starts=%v backends=%d", starts, len(backends))
	}
	if len(calls) != 5 || calls[2].model != "project-A" || calls[3].model != "project-C" || calls[4].model != "project-C" {
		t.Fatalf("process model was not switched at turn boundaries: %+v", calls)
	}
	if aSession.GetAgentSessionID() != aID || bSession.GetAgentSessionID() != bID {
		t.Fatal("process recycle replaced saved session IDs")
	}
	for _, tc := range []struct {
		session *Session
		before  []HistoryEntry
	}{{aSession, aHistory}, {bSession, bHistory}} {
		after := tc.session.GetHistory(0)
		if len(after) < len(tc.before) || !reflect.DeepEqual(after[:len(tc.before)], tc.before) {
			t.Fatalf("process recycle lost history: before=%+v after=%+v", tc.before, after)
		}
	}
	if visible := strings.Join(p.getSent(), "\n"); !strings.Contains(visible, "model=project-A busy") {
		t.Fatalf("in-flight response changed models: %s", visible)
	}
}

func TestModelDefaultRuntime_TwoFailedResumesNeverFallBackToFreshSession(t *testing.T) {
	a := &modelDefaultProcessAgent{model: "project-A"}
	e, p := newCronModelEngine(t, a, "")
	e.SetAdminFrom("owner")
	modelDefaultProcessTurn(t, e, p, chatModelMessage("A", "", "before", "remember me"), "project-A")
	session := e.sessions.GetOrCreateActive("test:A")
	id, history := session.GetAgentSessionID(), session.GetHistory(0)
	e.ReceiveMessage(p, chatModelMessage("A", "", "default", "/model default project-C"))
	a.mu.Lock()
	a.resumeErr = errors.New("resume unavailable")
	a.mu.Unlock()
	for attempt := 1; attempt <= 2; attempt++ {
		p.clearSent()
		e.ReceiveMessage(p, chatModelMessage("A", "", fmt.Sprintf("retry-%d", attempt), fmt.Sprintf("retry-%d", attempt)))
		waitAnswerProfileTest(t, fmt.Sprintf("resume failure %d", attempt), func() bool {
			starts, _, _ := a.snapshot()
			return len(starts) >= attempt+1 && len(p.getSent()) > 0 && !session.Busy()
		})
		starts, calls, _ := a.snapshot()
		if len(starts) != attempt+1 {
			t.Fatalf("attempt %d silently tried a fallback: starts=%v", attempt, starts)
		}
		for _, resumeID := range starts[1:] {
			if resumeID != id {
				t.Fatalf("attempt %d retried with %q instead of saved ID %q", attempt, resumeID, id)
			}
		}
		after := session.GetHistory(0)
		// Failed input may be recorded as a user history entry; the original
		// conversation must stay intact, with no fresh backend or replay.
		if len(calls) != 1 || session.GetAgentSessionID() != id || len(after) < len(history) || !reflect.DeepEqual(after[:len(history)], history) {
			t.Fatalf("failed resume changed conversation: calls=%+v id=%s history=%+v", calls, session.GetAgentSessionID(), session.GetHistory(0))
		}
		if strings.Contains(strings.Join(p.getSent(), "\n"), "model=project-C") {
			t.Fatal("fresh-session fallback produced a misleading successful reply")
		}
	}
	a.mu.Lock()
	a.resumeErr = nil
	a.mu.Unlock()
	p.clearSent()
	modelDefaultProcessTurn(t, e, p, chatModelMessage("A", "", "recovered", "continue"), "project-C")
	starts, _, _ := a.snapshot()
	if len(starts) != 4 || starts[3] != id || session.GetAgentSessionID() != id {
		t.Fatalf("recovery failed to resume original conversation: starts=%v id=%s", starts, session.GetAgentSessionID())
	}
}

func TestModelDefaultRuntime_QueuedResumeFailureRetainsGuardForRetry(t *testing.T) {
	a := &modelDefaultProcessAgent{model: "project-A"}
	e, p := newCronModelEngine(t, a, "")
	e.SetAdminFrom("owner")
	e.SetBusyMessageMode(BusyMessageModeQueue)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a.onSend = func(s *modelDefaultProcessSession, call cronModelCall) {
		if call.prompt == "busy" {
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "model=" + call.model + " " + call.prompt, Done: true}
	}
	modelDefaultProcessTurn(t, e, p, chatModelMessage("A", "", "before", "remember"), "project-A")
	session := e.sessions.GetOrCreateActive("test:A")
	id, history := session.GetAgentSessionID(), session.GetHistory(0)
	e.ReceiveMessage(p, chatModelMessage("A", "", "busy", "busy"))
	waitAnswerProfileTest(t, "old-model turn running", func() bool { _, calls, _ := a.snapshot(); return len(calls) == 2 })
	e.ReceiveMessage(p, chatModelMessage("A", "", "queued", "queued"))
	e.ReceiveMessage(p, chatModelMessage("A", "", "default", "/model default project-C"))
	a.mu.Lock()
	a.resumeErr = errors.New("queued resume unavailable")
	a.mu.Unlock()
	once.Do(func() { close(release) })
	for attempt := 1; attempt <= 2; attempt++ {
		if attempt == 2 {
			p.clearSent()
			e.ReceiveMessage(p, chatModelMessage("A", "", "retry", "retry after queued failure"))
		}
		waitAnswerProfileTest(t, fmt.Sprintf("queued/resumed failure %d", attempt), func() bool {
			starts, _, _ := a.snapshot()
			return len(starts) >= attempt+1 && !session.Busy() && len(p.getSent()) > 0
		})
		starts, calls, _ := a.snapshot()
		if len(starts) != attempt+1 {
			t.Fatalf("queued failure silently retried a fresh session: starts=%v", starts)
		}
		for _, resumeID := range starts[1:] {
			if resumeID != id {
				t.Fatalf("queued failure lost retry guard: starts=%v want %s", starts, id)
			}
		}
		after := session.GetHistory(0)
		if len(calls) != 2 || session.GetAgentSessionID() != id || len(after) < len(history) || !reflect.DeepEqual(after[:len(history)], history) {
			t.Fatalf("queued resume failure lost conversation: starts=%v calls=%+v id=%s", starts, calls, session.GetAgentSessionID())
		}
		if strings.Contains(strings.Join(p.getSent(), "\n"), "model=project-C") {
			t.Fatal("failed queued resume delivered a fresh-session response")
		}
	}
}

func TestModelDefaultRuntime_StaleExpectedStateCannotStartBackend(t *testing.T) {
	for _, condition := range []string{"removed", "replaced", "stopped"} {
		t.Run(condition, func(t *testing.T) {
			a := &modelDefaultProcessAgent{model: "project-C"}
			e, p := newCronModelEngine(t, a, "")
			const key = "test:A"
			session := e.sessions.GetOrCreateActive(key)
			session.SetAgentSessionID("saved-conversation", a.Name())
			session.AddHistory("user", "keep history")
			state := &interactiveState{agent: a, modelResumeID: "saved-conversation"}
			e.interactiveStates[key] = state
			var remaining *interactiveState
			switch condition {
			case "removed":
				delete(e.interactiveStates, key)
			case "replaced":
				remaining = &interactiveState{agent: a}
				e.interactiveStates[key] = remaining
			case "stopped":
				state.markStopped()
				remaining = state
			}
			err := e.ensureBackendForTurn(state, key, session, e.sessions, queuedMessage{platform: p, replyCtx: key, msgSessionKey: key})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("stale state was not rejected: %v", err)
			}
			if starts, calls, _ := a.snapshot(); len(starts) != 0 || len(calls) != 0 {
				t.Fatalf("stale state started a backend: starts=%v calls=%v", starts, calls)
			}
			if e.interactiveStates[key] != remaining || session.GetAgentSessionID() != "saved-conversation" || len(session.GetHistory(0)) != 1 {
				t.Fatal("stale helper replaced live state or changed saved conversation")
			}
		})
	}
}

func TestModelDefaultRuntime_ExpectedUnusableBackendRetainsStateAndError(t *testing.T) {
	for _, condition := range []string{"ended", "nil", "conversation-changed", "liveness-changed"} {
		t.Run(condition, func(t *testing.T) {
			a := &modelDefaultProcessAgent{model: "project-C"}
			e, p := newCronModelEngine(t, a, "")
			const key = "test:A"
			session := e.sessions.GetOrCreateActive(key)
			id := "saved-conversation"
			if condition == "conversation-changed" {
				id = "different-conversation"
			}
			session.SetAgentSessionID(id, a.Name())
			session.AddHistory("user", "keep history")
			backend := &modelDefaultProcessSession{agent: a, model: "project-A", id: "saved-conversation", events: make(chan Event, 1)}
			state := &interactiveState{
				agent: a, agentSession: backend, sessionModel: "project-A",
				sessionModelProvider: modelProvider(a), modelResumeID: "saved-conversation",
				stopCh: make(chan struct{}), pendingMessages: []queuedMessage{{platform: p, replyCtx: key, msgSessionKey: key, messageID: "keep-queued", content: "pending"}},
			}
			if condition == "ended" {
				_ = backend.Close()
			} else if condition == "nil" {
				state.agentSession = nil
			} else if condition == "liveness-changed" {
				state.agentSession = &modelDefaultLivenessTransitionSession{modelDefaultProcessSession: backend}
			}
			stopCh := state.stopCh
			e.interactiveStates[key] = state
			got, fresh := e.getOrCreateInteractiveState(key, p, key, session, e.sessions, a, key, state)
			if got != state || fresh != nil || state.startError == nil {
				t.Fatalf("unusable expected state was replaced or error lost: same=%v fresh=%v error=%v", got == state, fresh, state.startError)
			}
			err := e.ensureBackendForTurn(state, key, session, e.sessions, queuedMessage{platform: p, replyCtx: key, msgSessionKey: key})
			if err == nil || err != state.startError || errors.Is(err, context.Canceled) {
				t.Fatalf("backend failure was not retained on the original state: returned=%v retained=%v", err, state.startError)
			}
			if starts, _, _ := a.snapshot(); len(starts) != 0 {
				t.Fatalf("unusable queued backend unexpectedly started another session: %v", starts)
			}
			if e.interactiveStates[key] != state || state.stopCh != stopCh || len(state.pendingMessages) != 1 || state.pendingMessages[0].messageID != "keep-queued" || state.modelResumeID != "saved-conversation" {
				t.Fatal("failure replaced state or orphaned its pending queue/stop/retry guard")
			}
			if session.GetAgentSessionID() != id || len(session.GetHistory(0)) != 1 {
				t.Fatal("queued backend failure changed saved conversation")
			}
		})
	}
}

func TestModelDefaultRuntime_BackendEndingBeforeQueueDoesNotStartReplacement(t *testing.T) {
	a := &modelDefaultProcessAgent{model: "project-A"}
	e, p := newCronModelEngine(t, a, "")
	e.SetAdminFrom("owner")
	e.SetBusyMessageMode(BusyMessageModeQueue)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a.onSend = func(s *modelDefaultProcessSession, call cronModelCall) {
		<-release
		// Terminal output remains deliverable after the process exits. Closing
		// before enqueueing the event makes this boundary deterministic.
		_ = s.Close()
		s.events <- Event{Type: EventResult, Content: "model=" + call.model + " " + call.prompt, Done: true}
	}
	e.ReceiveMessage(p, chatModelMessage("A", "", "first", "first"))
	waitAnswerProfileTest(t, "process turn started", func() bool { _, calls, _ := a.snapshot(); return len(calls) == 1 })
	session := e.sessions.GetOrCreateActive("test:A")
	id := session.GetAgentSessionID()
	e.interactiveMu.Lock()
	original := e.interactiveStates["test:A"]
	e.interactiveMu.Unlock()
	e.ReceiveMessage(p, chatModelMessage("A", "", "queued", "queued"))
	e.ReceiveMessage(p, chatModelMessage("A", "", "default", "/model default project-C"))
	once.Do(func() { close(release) })
	waitAnswerProfileTest(t, "ended backend reported", func() bool {
		return !session.Busy() && strings.Contains(strings.Join(p.getSent(), "\n"), e.i18n.T(MsgFailedToStartAgentSession))
	})
	starts, calls, _ := a.snapshot()
	if len(starts) != 1 || len(calls) != 1 || session.GetAgentSessionID() != id {
		t.Fatalf("ended process created an orphaned replacement: starts=%v calls=%v id=%s", starts, calls, session.GetAgentSessionID())
	}
	e.interactiveMu.Lock()
	current := e.interactiveStates["test:A"]
	e.interactiveMu.Unlock()
	original.mu.Lock()
	err, pending := original.startError, len(original.pendingMessages)
	original.mu.Unlock()
	if current != original || err == nil || pending != 0 {
		t.Fatalf("ended queue lost its state/error or left work orphaned: same=%v error=%v pending=%d", current == original, err, pending)
	}
}
