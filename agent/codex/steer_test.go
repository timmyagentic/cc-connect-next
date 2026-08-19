package codex

// Tests for native Codex steering (issue #27): the app-server turn/steer RPC
// and the exec backend's explicit refusal.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

// steerFakeStdin captures JSON-RPC requests written by the session and lets
// the test synthesize a server response through session.handleResponse — the
// same delivery path the real read loop uses.
type steerFakeStdin struct {
	mu       sync.Mutex
	requests []map[string]any
	respond  func(id int64, req map[string]any)
}

func (f *steerFakeStdin) Write(p []byte) (int, error) {
	var req map[string]any
	if err := json.Unmarshal(p, &req); err != nil {
		return 0, err
	}
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	if f.respond != nil {
		if idf, ok := req["id"].(float64); ok {
			f.respond(int64(idf), req)
		}
	}
	return len(p), nil
}

func (f *steerFakeStdin) Close() error { return nil }

func (f *steerFakeStdin) lastRequest() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		return nil
	}
	return f.requests[len(f.requests)-1]
}

func newSteerTestSession(t *testing.T, stdin *steerFakeStdin, threadID, currentTurn string) *appServerSession {
	t.Helper()
	s := &appServerSession{
		workDir: t.TempDir(),
		events:  make(chan core.Event, 16),
		ctx:     context.Background(),
	}
	s.alive.Store(true)
	s.stdin = stdin
	if threadID != "" {
		s.threadID.Store(threadID)
	}
	s.currentTurn = currentTurn
	s.pendingMsgs = []string{"partial-thinking"}
	return s
}

func respondSuccess(s *appServerSession, result string) func(int64, map[string]any) {
	return func(id int64, _ map[string]any) {
		s.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(result)})
	}
}

func respondError(s *appServerSession, msg string) func(int64, map[string]any) {
	return func(id int64, _ map[string]any) {
		s.handleResponse(rpcResponseEnvelope{ID: id, Error: &rpcError{Message: msg}})
	}
}

func TestAppServerSteer_SendsTurnSteerWithExpectedTurnID(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "turn-1")
	stdin.respond = respondSuccess(s, `{"turnId":"turn-1"}`)

	if err := s.Steer("focus on the failing tests first", nil, nil); err != nil {
		t.Fatalf("Steer() = %v, want nil", err)
	}

	req := stdin.lastRequest()
	if req == nil {
		t.Fatal("no request captured")
	}
	if req["method"] != "turn/steer" {
		t.Fatalf("method = %v, want turn/steer", req["method"])
	}
	params, _ := req["params"].(map[string]any)
	if params == nil {
		t.Fatal("params missing")
	}
	if params["threadId"] != "thread-1" {
		t.Fatalf("threadId = %v, want thread-1", params["threadId"])
	}
	if params["expectedTurnId"] != "turn-1" {
		t.Fatalf("expectedTurnId = %v, want turn-1", params["expectedTurnId"])
	}
	input, _ := params["input"].([]any)
	if len(input) != 1 {
		t.Fatalf("input length = %d, want 1", len(input))
	}
	item, _ := input[0].(map[string]any)
	if item["type"] != "text" || item["text"] != "focus on the failing tests first" {
		t.Fatalf("input[0] = %v", item)
	}

	// The original turn's lifecycle stays authoritative: no state mutation.
	s.stateMu.Lock()
	turn, pending := s.currentTurn, len(s.pendingMsgs)
	s.stateMu.Unlock()
	if turn != "turn-1" {
		t.Fatalf("currentTurn = %q, want unchanged turn-1", turn)
	}
	if pending != 1 {
		t.Fatalf("pendingMsgs = %d entries, want untouched 1", pending)
	}
}

func TestAppServerSteer_NoActiveTurn(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "")

	err := s.Steer("x", nil, nil)
	if !errors.Is(err, core.ErrSteerNoActiveTurn) {
		t.Fatalf("Steer() = %v, want ErrSteerNoActiveTurn", err)
	}
	if stdin.lastRequest() != nil {
		t.Fatal("no RPC may be sent without an active turn")
	}
}

func TestAppServerSteer_ServerRejection_IsDefinitive(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "turn-1")
	stdin.respond = respondError(s, "invalid request")

	err := s.Steer("x", nil, nil)
	if !errors.Is(err, core.ErrSteerRejected) {
		t.Fatalf("Steer() = %v, want ErrSteerRejected", err)
	}
	if !core.SteerFailureAllowsQueueFallback(err) {
		t.Fatal("server rejection must allow queue fallback")
	}
}

func TestAppServerSteer_TurnChangedDuringRPC_IsMismatch(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "turn-1")
	stdin.respond = func(id int64, _ map[string]any) {
		// The turn completes while the steer request is in flight; the server
		// rejects due to the expectedTurnId precondition.
		s.stateMu.Lock()
		s.currentTurn = ""
		s.stateMu.Unlock()
		s.handleResponse(rpcResponseEnvelope{ID: id, Error: &rpcError{Message: "expected turn mismatch"}})
	}

	err := s.Steer("x", nil, nil)
	if !errors.Is(err, core.ErrSteerTurnMismatch) {
		t.Fatalf("Steer() = %v, want ErrSteerTurnMismatch", err)
	}
	if !core.SteerFailureAllowsQueueFallback(err) {
		t.Fatal("expected-turn mismatch must allow queue fallback")
	}
}

func TestAppServerSteer_TransportDeath_IsOutcomeUnknown(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "turn-1")
	stdin.respond = func(id int64, _ map[string]any) {
		// rejectPending synthesizes local failure envelopes when the process
		// dies — that is NOT a server answer, so the outcome is unknown.
		s.handleResponse(rpcResponseEnvelope{ID: id, Error: &rpcError{Message: "process exited"}, localFailure: true})
	}

	err := s.Steer("x", nil, nil)
	if !errors.Is(err, core.ErrSteerOutcomeUnknown) {
		t.Fatalf("Steer() = %v, want ErrSteerOutcomeUnknown", err)
	}
	if core.SteerFailureAllowsQueueFallback(err) {
		t.Fatal("unknown outcome must NOT allow queue fallback")
	}
}

func TestAppServerSteer_AcceptedByDifferentTurn_IsOutcomeUnknown(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "turn-1")
	stdin.respond = respondSuccess(s, `{"turnId":"turn-2"}`)

	err := s.Steer("x", nil, nil)
	if !errors.Is(err, core.ErrSteerOutcomeUnknown) {
		t.Fatalf("Steer() = %v, want ErrSteerOutcomeUnknown (input was delivered somewhere)", err)
	}
	if core.SteerFailureAllowsQueueFallback(err) {
		t.Fatal("input accepted by another turn must never be re-queued")
	}
}

func TestAppServerSteer_UnparseableSuccessResponse_IsAccepted(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "turn-1")
	stdin.respond = respondSuccess(s, `"unexpected-shape"`)

	if err := s.Steer("x", nil, nil); err != nil {
		t.Fatalf("Steer() = %v, want nil (server accepted; only decode failed)", err)
	}
}

func TestExecSteer_IsUnsupported(t *testing.T) {
	cs := &codexSession{}
	err := cs.Steer("x", nil, nil)
	if !errors.Is(err, core.ErrSteerUnsupported) {
		t.Fatalf("exec Steer() = %v, want ErrSteerUnsupported", err)
	}
	if !core.SteerFailureAllowsQueueFallback(err) {
		t.Fatal("unsupported backend must allow queue fallback")
	}
}

func TestSteerInterfaceCompliance(t *testing.T) {
	// Both backends expose the capability; exec reports unsupported at call
	// time so core never needs a codex-specific type check.
	var _ core.SteerableSession = (*appServerSession)(nil)
	var _ core.SteerableSession = (*codexSession)(nil)
	if !strings.Contains(core.ErrSteerUnsupported.Error(), "not supported") {
		t.Fatal("sanity")
	}
}
