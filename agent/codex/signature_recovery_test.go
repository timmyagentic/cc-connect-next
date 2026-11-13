package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

const signatureTestError = `{"error":{"code":"thinking_signature_invalid","message":"Encrypted content could not be verified"}}`

func newSignatureTestSession(t *testing.T) (*appServerSession, string) {
	t.Helper()
	s := newTerminalTestSession(t)
	s.threadID.Store("thread-a")
	s.currentTurn = "turn-b"
	s.workDir = t.TempDir()
	s.codexHome = t.TempDir()
	s.model = "original-model"
	s.turnOptions = &core.TurnOptions{Model: "task-model", ReasoningEffort: "high", ServiceTier: "fast"}
	dir := filepath.Join(s.codexHome, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "thread-a.jsonl")
	data := strings.Join([]string{
		recoveryTestMeta, recoveryTestContext, recoveryTestItem(recoveryTestInput),
		recoveryTestItem(`{"type":"reasoning","encrypted_content":"rejected-state"}`),
		recoveryTestItem(`{"type":"function_call","name":"completed_write","arguments":"{}","call_id":"write-1"}`),
		recoveryTestItem(`{"type":"function_call_output","call_id":"write-1","output":"Committed receipt 123"}`),
		recoveryTestDone, "",
	}, "\n")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return s, path
}

func signatureTestNotify(s *appServerSession, method, thread, turn, status, message string) {
	params := map[string]any{"threadId": thread, "turnId": turn}
	if method == "error" {
		params["error"] = map[string]any{"message": message}
		params["willRetry"] = false
	} else {
		value := map[string]any{"id": turn, "status": status}
		if message != "" {
			value["error"] = map[string]any{"message": message}
		}
		params["turn"] = value
	}
	raw, _ := json.Marshal(params)
	s.handleNotification(method, raw)
}

func awaitSignatureEvent(t *testing.T, s *appServerSession) core.Event {
	t.Helper()
	select {
	case event := <-s.events:
		return event
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for recovery event")
		return core.Event{}
	}
}

func signatureTestRPC(s *appServerSession, path string, onRequest func(string, map[string]any) bool) *steerFakeStdin {
	stdin := &steerFakeStdin{}
	stdin.respond = func(id int64, request map[string]any) {
		method := request["method"].(string)
		params := request["params"].(map[string]any)
		if onRequest != nil && !onRequest(method, params) {
			s.handleResponse(rpcResponseEnvelope{ID: id, Error: &rpcError{Message: "synthetic RPC failure"}})
			return
		}
		var result any
		switch method {
		case "thread/read":
			result = map[string]any{"thread": map[string]any{"id": "thread-a", "path": path}}
		case "thread/backgroundTerminals/list":
			result = map[string]any{"data": []any{}, "nextCursor": nil}
		case "thread/start":
			result = map[string]any{"thread": map[string]any{"id": "thread-new"}}
		case "turn/start":
			result = map[string]any{"turn": map[string]any{"id": "turn-new"}}
		default:
			result = map[string]any{}
		}
		raw, _ := json.Marshal(result)
		s.handleResponse(rpcResponseEnvelope{ID: id, Result: raw})
	}
	s.stdin = stdin
	return stdin
}

func TestSignatureErrorRecognition(t *testing.T) {
	for _, value := range []string{signatureTestError, "invalid_encrypted_content: rejected", "stream disconnected before completion: invalid_encrypted_content: rejected", "unexpected status 400 Bad Request: " + signatureTestError} {
		if !isSignatureError(errors.New(value)) {
			t.Errorf("did not recognize %q", value)
		}
	}
	for _, value := range []string{"upstream timed out", "tool output mentioned thinking_signature_invalid", `{"error":{"code":"quota_exceeded","message":"thinking_signature_invalid"}}`} {
		if isSignatureError(errors.New(value)) {
			t.Errorf("incorrectly recognized %q", value)
		}
	}
}

func TestSignatureRecoveryWaitsForFailedTurnAndDurableBinding(t *testing.T) {
	s, path := newSignatureTestSession(t)
	original, _ := os.ReadFile(path)
	var imported atomic.Bool
	var starts atomic.Int32
	signatureTestRPC(s, path, func(method string, params map[string]any) bool {
		if method == "thread/inject_items" {
			items, _ := json.Marshal(params["items"])
			if strings.Contains(string(items), "encrypted_content") || !strings.Contains(string(items), "Committed receipt 123") || strings.Count(string(items), "Continue the investigation") != 1 {
				t.Error("import lost input/results or included rejected encryption")
			}
			imported.Store(true)
		}
		if method == "turn/start" {
			starts.Add(1)
			input, _ := json.Marshal(params["input"])
			if strings.Contains(string(input), "Continue the investigation") || params["model"] != "task-model" || params["effort"] != "high" || params["serviceTier"] != "fast" {
				t.Error("continuation repeated the user request or lost task settings")
			}
		}
		return true
	})
	signatureTestNotify(s, "error", "thread-a", "turn-b", "", signatureTestError)
	s.handleNotification("thread/status/changed", json.RawMessage(`{"threadId":"thread-a","status":{"type":"idle"}}`))
	select {
	case event := <-s.events:
		t.Fatalf("idle completed or recovered an unconfirmed failure: %#v", event)
	case <-time.After(25 * time.Millisecond):
	}
	signatureTestNotify(s, "turn/completed", "thread-a", "turn-b", "failed", signatureTestError)
	if event := awaitSignatureEvent(t, s); event.Type != core.EventSessionRecovery || event.SessionID != "" {
		t.Fatalf("unexpected starting event: %#v", event)
	}
	ready := awaitSignatureEvent(t, s)
	if ready.Type != core.EventSessionRecovery || ready.SessionID != "thread-new" || ready.RecoveryReady == nil || !imported.Load() || starts.Load() != 0 {
		t.Fatalf("replacement continued before binding acknowledgement: %#v", ready)
	}
	if err := s.Steer("another request", nil, nil); !errors.Is(err, core.ErrSteerNoActiveTurn) {
		t.Fatalf("input was steered during recovery: %v", err)
	}
	ready.RecoveryReady <- nil
	deadline := time.Now().Add(time.Second)
	for appServerCurrentTurn(s) != "turn-new" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if starts.Load() != 1 {
		t.Fatal("expected exactly one continuation request")
	}
	// A second signature failure in the same logical user turn is terminal.
	signatureTestNotify(s, "error", "thread-new", "turn-new", "", signatureTestError)
	if event := awaitSignatureEvent(t, s); event.Type != core.EventError || !strings.Contains(event.Error.Error(), "thinking_signature_invalid") {
		t.Fatalf("second signature failure was not terminal: %#v", event)
	}
	after, _ := os.ReadFile(path)
	if string(original) != string(after) {
		t.Fatal("recovery rewrote original history")
	}
}

func TestSignatureRecoverySuccessfulTerminalOverridesPendingError(t *testing.T) {
	s, _ := newSignatureTestSession(t)
	signatureTestNotify(s, "error", "thread-a", "turn-b", "", signatureTestError)
	signatureTestNotify(s, "turn/completed", "thread-a", "turn-b", "completed", "")
	if event := awaitSignatureEvent(t, s); event.Type != core.EventResult {
		t.Fatalf("successful terminal unexpectedly started recovery: %#v", event)
	}
}

func TestSignatureRecoveryCancelBeforeContinuation(t *testing.T) {
	for _, phase := range []string{"thread/start", "thread/inject_items", "binding"} {
		t.Run(phase, func(t *testing.T) {
			s, path := newSignatureTestSession(t)
			var starts atomic.Int32
			signatureTestRPC(s, path, func(method string, _ map[string]any) bool {
				if method == "turn/start" {
					starts.Add(1)
				}
				if method == phase {
					s.cancel()
				}
				return true
			})
			signatureTestNotify(s, "turn/completed", "thread-a", "turn-b", "failed", signatureTestError)
			if phase == "binding" {
				awaitSignatureEvent(t, s)
				ready := awaitSignatureEvent(t, s)
				if ready.RecoveryReady == nil {
					t.Fatal("missing binding acknowledgement")
				}
				s.cancel()
			}
			done := make(chan struct{})
			go func() { s.wg.Wait(); close(done) }()
			awaitTerminalTest(t, done)
			if starts.Load() != 0 {
				t.Fatal("cancelled recovery started a model turn")
			}
			if phase != "binding" && s.CurrentSessionID() != "thread-a" {
				t.Fatal("cancelled setup changed the current thread")
			}
		})
	}
}

func TestSignatureRecoveryStartFailureRetiresTransport(t *testing.T) {
	s, path := newSignatureTestSession(t)
	signatureTestRPC(s, path, func(method string, _ map[string]any) bool { return method != "turn/start" })
	signatureTestNotify(s, "turn/completed", "thread-a", "turn-b", "failed", signatureTestError)
	awaitSignatureEvent(t, s)
	ready := awaitSignatureEvent(t, s)
	ready.RecoveryReady <- nil
	if event := awaitSignatureEvent(t, s); event.Type != core.EventError || s.Alive() {
		t.Fatalf("failed continuation left a reusable transport: %#v", event)
	}
}

func TestSignatureRecoveryBindingFailureStopsBeforeModelRequest(t *testing.T) {
	s, path := newSignatureTestSession(t)
	var starts atomic.Int32
	signatureTestRPC(s, path, func(method string, _ map[string]any) bool {
		if method == "turn/start" {
			starts.Add(1)
		}
		return true
	})
	signatureTestNotify(s, "turn/completed", "thread-a", "turn-b", "failed", signatureTestError)
	awaitSignatureEvent(t, s)
	ready := awaitSignatureEvent(t, s)
	ready.RecoveryReady <- errors.New("disk unavailable")
	if event := awaitSignatureEvent(t, s); event.Type != core.EventError || starts.Load() != 0 {
		t.Fatalf("persistence failure continued work: %#v", event)
	}
}

func TestSignatureRecoverySteerGuardIsScopedToTurn(t *testing.T) {
	s, path := newSignatureTestSession(t)
	s.unsafeRecoveryTurn = "previous-turn"
	signatureTestRPC(s, path, nil)
	signatureTestNotify(s, "turn/completed", "thread-a", "turn-b", "failed", signatureTestError)
	if event := awaitSignatureEvent(t, s); event.Type != core.EventSessionRecovery {
		t.Fatal("an earlier uncertain steer blocked a later independent turn")
	}
	s.cancel()

	other, _ := newSignatureTestSession(t)
	other.unsafeRecoveryTurn = "turn-b"
	signatureTestNotify(other, "turn/completed", "thread-a", "turn-b", "failed", signatureTestError)
	if event := awaitSignatureEvent(t, other); event.Type != core.EventError {
		t.Fatal("recovery could lose accepted but unpersisted steer input")
	}
}

func TestSignatureRecoveryEventSurvivesFullQueue(t *testing.T) {
	s := newTerminalTestSession(t)
	for i := 0; i < cap(s.events); i++ {
		s.events <- core.Event{Type: core.EventThinking}
	}
	done := make(chan struct{})
	go func() { s.emit(core.Event{Type: core.EventSessionRecovery, SessionID: "replacement"}); close(done) }()
	select {
	case <-done:
		t.Fatal("recovery binding event was dropped")
	case <-time.After(20 * time.Millisecond):
	}
	for i := 0; i < cap(s.events); i++ {
		<-s.events
	}
	awaitTerminalTest(t, done)
	if event := awaitSignatureEvent(t, s); event.Type != core.EventSessionRecovery {
		t.Fatal("lost recovery event")
	}
}
