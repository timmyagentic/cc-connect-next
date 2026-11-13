package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

// This fake owns native history independently from each appServerSession. A
// replacement session has no in-memory knowledge of prior requests, just as
// after the bridge and its stdio child have both been restarted.
type recoveryRPCFixture struct {
	t             *testing.T
	workDir       string
	threadID      string
	status        string
	turns         []recoveryTurn
	starts        []map[string]any
	dropNextStart bool
}

func newRecoveryFixture(t *testing.T) *recoveryRPCFixture {
	t.Helper()
	return &recoveryRPCFixture{t: t, workDir: t.TempDir(), threadID: "thread-1", status: "idle", turns: []recoveryTurn{}}
}

func (f *recoveryRPCFixture) session() (*appServerSession, *steerFakeStdin) {
	f.t.Helper()
	s, stdin := cronModelAppServer(f.t, "fixture-model")
	s.workDir, s.mode, s.providerName = f.workDir, "suggest", "fixture-provider"
	s.preambleSent, s.pendingMsgs = true, nil
	stdin.respond = func(id int64, request map[string]any) {
		params := request["params"].(map[string]any)
		if params["threadId"] != f.threadID {
			f.t.Errorf("RPC changed native thread: %v", request)
		}
		switch request["method"] {
		case "thread/read":
			if params["includeTurns"] != true {
				f.t.Error("recovery read omitted native turns")
			}
			body, _ := json.Marshal(map[string]any{"thread": map[string]any{
				"id": f.threadID, "status": map[string]string{"type": f.status}, "turns": f.turns,
			}})
			s.handleResponse(rpcResponseEnvelope{ID: id, Result: body})
		case "turn/start":
			f.starts = append(f.starts, params)
			turnID := fmt.Sprintf("native-turn-%d", len(f.starts))
			f.turns = append(f.turns, recoveryTurn{ID: turnID, Status: "inProgress", Items: []map[string]any{{
				"type": "userMessage", "id": "native-user", "clientId": params["clientUserMessageId"], "content": params["input"],
			}}})
			if f.dropNextStart {
				f.dropNextStart = false
				s.finishTransport(io.EOF)
				return
			}
			s.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(fmt.Sprintf(`{"turn":{"id":%q}}`, turnID))})
		default:
			f.t.Errorf("unexpected recovery RPC: %v", request)
			s.handleResponse(rpcResponseEnvelope{ID: id, Error: &rpcError{Message: "unexpected RPC"}})
		}
	}
	return s, stdin
}

func restartRecoveryTestInput() core.TurnRecoveryInput {
	return core.TurnRecoveryInput{Prompt: "Finish the authorized task", UseOptions: true, Options: core.TurnOptions{Model: "fixture-model", ReasoningEffort: "high", ServiceTier: "fast"}}
}

func beginRecoveryTurn(t *testing.T, f *recoveryRPCFixture, input core.TurnRecoveryInput) (json.RawMessage, *appServerSession) {
	t.Helper()
	s, _ := f.session()
	raw, err := s.PrepareTurnRecovery(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SendWithTurnOptions(input.Prompt, input.Images, input.Files, input.Options); err != nil {
		t.Fatal(err)
	}
	return raw, s
}

func TestRestartRecovery_CompletedNativeTurnReplaysOnlyFinalWithoutExecuting(t *testing.T) {
	f := newRecoveryFixture(t)
	input := restartRecoveryTestInput()
	raw, _ := beginRecoveryTurn(t, f, input)
	turn := &f.turns[0]
	turn.Status = "completed"
	turn.Items = append(turn.Items,
		map[string]any{"type": "agentMessage", "phase": "commentary", "text": "I am starting"},
		map[string]any{"type": "commandExecution", "status": "completed", "command": "one external action"},
		map[string]any{"type": "agentMessage", "phase": "final_answer", "text": "Already completed and verified."},
	)
	restored, stdin := f.session()
	if err := restored.ResumeInterruptedTurn(raw, input, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.starts) != 1 || len(stdin.requests) != 1 || stdin.requests[0]["method"] != "thread/read" {
		t.Fatalf("completed task was executed again: %v", stdin.requests)
	}
	event := <-restored.events
	if event.Type != core.EventResult || event.Content != "Already completed and verified." || !event.Done || event.SessionID != "thread-1" {
		t.Fatalf("restored answer = %#v", event)
	}
	assertNoAppServerEvent(t, restored.events, "replayed final answer")
}

func TestRestartRecovery_AcceptedSubmissionWithLostResponseIsNotRepeated(t *testing.T) {
	f := newRecoveryFixture(t)
	input := restartRecoveryTestInput()
	s, _ := f.session()
	raw, err := s.PrepareTurnRecovery(input)
	if err != nil {
		t.Fatal(err)
	}
	f.dropNextStart = true
	if err := s.SendWithTurnOptions(input.Prompt, nil, nil, input.Options); err == nil {
		t.Fatal("expected the old transport to lose the accepted response")
	}
	f.turns[0].Status = "completed"
	f.turns[0].Items = append(f.turns[0].Items, map[string]any{"type": "agentMessage", "phase": "final_answer", "text": "Done once"})
	restored, _ := f.session()
	if err := restored.ResumeInterruptedTurn(raw, input, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.starts) != 1 || (<-restored.events).Content != "Done once" {
		t.Fatal("lost response caused duplicate execution")
	}
}

func TestRestartRecovery_InterruptedTurnPersistsContinuationBeforeStarting(t *testing.T) {
	for _, status := range []string{"interrupted", "inProgress"} {
		t.Run(status, func(t *testing.T) {
			f := newRecoveryFixture(t)
			input := restartRecoveryTestInput()
			raw, _ := beginRecoveryTurn(t, f, input)
			f.turns[0].Status = status
			// Accepted steering shares the original turn and must not erase
			// the original client identity or be resubmitted separately.
			f.turns[0].Items = append(f.turns[0].Items, map[string]any{"type": "userMessage", "clientId": "steer", "content": []any{}})
			restored, _ := f.session()
			var next codexTurnCheckpoint
			err := restored.ResumeInterruptedTurn(raw, input, func(saved json.RawMessage) error {
				if len(f.starts) != 1 {
					t.Fatal("continuation started before its checkpoint was saved")
				}
				return json.Unmarshal(saved, &next)
			})
			if err != nil {
				t.Fatal(err)
			}
			if next.BoundaryID != f.turns[0].ID || next.ClientID == f.starts[0]["clientUserMessageId"] || len(f.starts) != 2 {
				t.Fatalf("invalid continuation identity: %+v", next)
			}
			params := f.starts[1]
			if params["clientUserMessageId"] != next.ClientID || params["approvalPolicy"] != "on-request" || params["model"] != input.Options.Model || params["effort"] != "high" {
				t.Fatalf("continuation changed options or permission: %v", params)
			}
			if got := params["input"].([]any)[0].(map[string]any)["text"]; got != interruptedTurnContinuation {
				t.Fatalf("continuation replayed original task: %v", got)
			}
			assertNoAppServerEvent(t, restored.events, "continuation before new output")
		})
	}
}

func TestRestartRecovery_SaveFailureNeverStartsContinuation(t *testing.T) {
	f := newRecoveryFixture(t)
	input := restartRecoveryTestInput()
	raw, _ := beginRecoveryTurn(t, f, input)
	f.turns[0].Status = "interrupted"
	restored, _ := f.session()
	diskErr := errors.New("disk unavailable")
	err := restored.ResumeInterruptedTurn(raw, input, func(json.RawMessage) error { return diskErr })
	if !errors.Is(err, diskErr) || len(f.starts) != 1 {
		t.Fatalf("unsafe continuation after failed persistence: starts=%d error=%v", len(f.starts), err)
	}
}

func TestRestartRecovery_RepeatedRestartReconcilesContinuationSubmission(t *testing.T) {
	for _, continuationStarted := range []bool{false, true} {
		t.Run(fmt.Sprintf("accepted=%t", continuationStarted), func(t *testing.T) {
			f := newRecoveryFixture(t)
			input := restartRecoveryTestInput()
			raw, _ := beginRecoveryTurn(t, f, input)
			f.turns[0].Status = "interrupted"
			restored, _ := f.session()
			var next json.RawMessage
			err := restored.ResumeInterruptedTurn(raw, input, func(saved json.RawMessage) error {
				next = append(json.RawMessage(nil), saved...)
				if !continuationStarted {
					// Crash immediately after durable storage but before RPC.
					return io.EOF
				}
				return nil
			})
			if continuationStarted && err != nil || !continuationStarted && !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			if continuationStarted {
				f.turns[1].Status = "completed"
				f.turns[1].Items = append(f.turns[1].Items, map[string]any{"type": "agentMessage", "phase": "final_answer", "text": "Recovered once"})
			}
			again, _ := f.session()
			if err := again.ResumeInterruptedTurn(next, input, nil); err != nil {
				t.Fatal(err)
			}
			if len(f.starts) != 2 {
				t.Fatalf("continuation duplicated across restart: starts=%d", len(f.starts))
			}
			if continuationStarted && (<-again.events).Content != "Recovered once" {
				t.Fatal("lost completed continuation result")
			}
		})
	}
}

func TestRestartRecovery_ForeignAndFailedHistoryNeverStartsWork(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*recoveryRPCFixture, *appServerSession)
	}{
		{"foreign_same_prompt", func(f *recoveryRPCFixture, _ *appServerSession) { f.turns[0].Items[0]["clientId"] = "another-client" }},
		{"no_client_identity", func(f *recoveryRPCFixture, _ *appServerSession) { delete(f.turns[0].Items[0], "clientId") }},
		{"newer_turn", func(f *recoveryRPCFixture, _ *appServerSession) {
			f.turns = append(f.turns, recoveryTurn{ID: "foreign-turn", Status: "completed"})
		}},
		{"active_thread", func(f *recoveryRPCFixture, _ *appServerSession) { f.status = "active" }},
		{"waiting_approval", func(f *recoveryRPCFixture, _ *appServerSession) { f.status = "active" }},
		{"failed_turn", func(f *recoveryRPCFixture, _ *appServerSession) { f.turns[0].Status = "failed" }},
		{"partial_items", func(f *recoveryRPCFixture, _ *appServerSession) { f.turns[0].ItemsView = "summary" }},
		{"different_thread", func(_ *recoveryRPCFixture, s *appServerSession) { s.threadID.Store("different") }},
		{"different_permissions", func(_ *recoveryRPCFixture, s *appServerSession) { s.mode = "yolo" }},
		{"different_provider", func(_ *recoveryRPCFixture, s *appServerSession) { s.providerName = "different" }},
		{"different_workspace", func(_ *recoveryRPCFixture, s *appServerSession) { s.workDir += "/different" }},
		{"no_final", func(f *recoveryRPCFixture, _ *appServerSession) { f.turns[0].Status = "completed" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecoveryFixture(t)
			input := restartRecoveryTestInput()
			raw, _ := beginRecoveryTurn(t, f, input)
			restored, _ := f.session()
			tc.mutate(f, restored)
			if err := restored.ResumeInterruptedTurn(raw, input, func(json.RawMessage) error { return nil }); err == nil {
				t.Fatal("unsafe recovery was accepted")
			}
			if len(f.starts) != 1 {
				t.Fatal("unsafe recovery started a model request")
			}
			assertNoAppServerEvent(t, restored.events, "rejected recovery")
		})
	}
}

func TestRestartRecovery_PreparedSubmissionKeepsOriginalWireInputAndClientID(t *testing.T) {
	f := newRecoveryFixture(t)
	s, stdin := f.session()
	s.recoveryFreshThread, s.preambleSent = true, false
	s.promptPreamble = "Keep project instructions"
	input := restartRecoveryTestInput()
	input.Files = []core.FileAttachment{{FileName: "evidence.txt", Data: []byte("fixture evidence")}}
	raw, err := s.PrepareTurnRecovery(input)
	if err != nil {
		t.Fatal(err)
	}
	// Fresh native threads have no materialized history yet. The real 0.167
	// CLI rejects includeTurns at that boundary; no history RPC is needed.
	if len(stdin.requests) != 0 {
		t.Fatal("fresh thread attempted unsupported full history read")
	}
	var cp codexTurnCheckpoint
	if err := json.Unmarshal(raw, &cp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cp.Input[0]["text"].(string), "Keep project instructions") || !strings.Contains(cp.Input[0]["text"].(string), "evidence.txt") {
		t.Fatalf("prepared preamble or attachment lost: %v", cp.Input)
	}
	if err := s.SendWithTurnOptions(input.Prompt, input.Images, input.Files, input.Options); err != nil {
		t.Fatal(err)
	}
	preparedJSON, _ := json.Marshal(cp.Input)
	sentJSON, _ := json.Marshal(f.starts[0]["input"])
	if string(preparedJSON) != string(sentJSON) || f.starts[0]["clientUserMessageId"] != cp.ClientID {
		t.Fatalf("Send did not use durable prepared input: %v", f.starts[0])
	}
}

func TestRestartRecovery_UnsubmittedInputUsesSavedAttachmentPath(t *testing.T) {
	f := newRecoveryFixture(t)
	s, _ := f.session()
	input := restartRecoveryTestInput()
	input.Files = []core.FileAttachment{{FileName: "evidence.txt", Data: []byte("fixture evidence")}}
	raw, err := s.PrepareTurnRecovery(input)
	if err != nil {
		t.Fatal(err)
	}
	var cp codexTurnCheckpoint
	_ = json.Unmarshal(raw, &cp)
	restored, _ := f.session()
	if err := restored.ResumeInterruptedTurn(raw, input, nil); err != nil {
		t.Fatal(err)
	}
	sentJSON, _ := json.Marshal(f.starts[0]["input"])
	preparedJSON, _ := json.Marshal(cp.Input)
	if string(sentJSON) != string(preparedJSON) || f.starts[0]["clientUserMessageId"] != cp.ClientID {
		t.Fatal("unsubmitted recovery replaced its attachment or submission identity")
	}
}

func TestRestartRecovery_InputChangedAfterCheckpointDoesNotSubmit(t *testing.T) {
	f := newRecoveryFixture(t)
	s, _ := f.session()
	input := restartRecoveryTestInput()
	if _, err := s.PrepareTurnRecovery(input); err != nil {
		t.Fatal(err)
	}
	if err := s.SendWithTurnOptions("a different task", nil, nil, input.Options); err == nil || len(f.starts) != 0 {
		t.Fatalf("changed input submitted under saved checkpoint: %v", err)
	}
}

func TestRestartRecovery_PreparationFailureDoesNotSubmit(t *testing.T) {
	f := newRecoveryFixture(t)
	s, _ := f.session()
	if err := os.Remove(f.workDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.workDir, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	input := restartRecoveryTestInput()
	input.Files = []core.FileAttachment{{FileName: "evidence.txt", Data: []byte("fixture")}}
	if _, err := s.PrepareTurnRecovery(input); err == nil || len(f.starts) != 0 {
		t.Fatalf("attachment preparation failure was ignored: %v", err)
	}
}

func TestRestartRecovery_UnmaterializedOrUnreadableHistoryDoesNotRebuildThread(t *testing.T) {
	f := newRecoveryFixture(t)
	input := restartRecoveryTestInput()
	s, _ := f.session()
	s.recoveryFreshThread = true
	raw, err := s.PrepareTurnRecovery(input)
	if err != nil {
		t.Fatal(err)
	}
	restored, stdin := f.session()
	stdin.respond = respondError(restored, "list_turns is not supported yet")
	if err := restored.ResumeInterruptedTurn(raw, input, nil); err == nil || len(f.starts) != 0 {
		t.Fatalf("unknown native history silently created new work: %v", err)
	}
	if len(stdin.requests) != 1 || stdin.requests[0]["method"] != "thread/read" {
		t.Fatalf("unsupported history caused fallback: %v", stdin.requests)
	}
}

func TestRestartRecovery_DurableInputRoundTripRetainsIdentity(t *testing.T) {
	f := newRecoveryFixture(t)
	input := restartRecoveryTestInput()
	input.Images, input.Files = []core.ImageAttachment{}, []core.FileAttachment{}
	raw, _ := beginRecoveryTurn(t, f, input)
	f.turns[0].Status = "interrupted"
	encoded, _ := json.Marshal(input)
	var restoredInput core.TurnRecoveryInput
	if err := json.Unmarshal(encoded, &restoredInput); err != nil {
		t.Fatal(err)
	}
	restored, _ := f.session()
	if err := restored.ResumeInterruptedTurn(raw, restoredInput, func(json.RawMessage) error { return nil }); err != nil {
		t.Fatalf("empty attachment serialization changed identity: %v", err)
	}
}

func TestRestartRecovery_LegacySendPreservesOriginalOptionsBehavior(t *testing.T) {
	f := newRecoveryFixture(t)
	input := restartRecoveryTestInput()
	input.UseOptions = false
	s, _ := f.session()
	raw, err := s.PrepareTurnRecovery(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Send(input.Prompt, nil, nil); err != nil {
		t.Fatal(err)
	}
	f.turns[0].Status = "interrupted"
	restored, _ := f.session()
	restored.defaultModelErr = errors.New("metadata unavailable")
	if err := restored.ResumeInterruptedTurn(raw, input, func(json.RawMessage) error { return nil }); err != nil {
		t.Fatalf("legacy recovery introduced a new per-turn default requirement: %v", err)
	}
	if f.starts[1]["effort"] != "medium" {
		t.Fatalf("legacy recovery silently adopted unused override: %v", f.starts[1])
	}
}

func TestRestartRecovery_MissingPriorBoundaryDoesNotReplayInput(t *testing.T) {
	f := newRecoveryFixture(t)
	f.turns = []recoveryTurn{{ID: "prior-turn", Status: "completed"}}
	input := restartRecoveryTestInput()
	raw, _ := beginRecoveryTurn(t, f, input)
	f.turns = f.turns[1:]
	restored, _ := f.session()
	if err := restored.ResumeInterruptedTurn(raw, input, nil); err == nil || len(f.starts) != 1 {
		t.Fatalf("history rewrite was accepted: %v", err)
	}
}

func TestRestartRecovery_MissingTurnsNeverMeansUnsubmitted(t *testing.T) {
	for _, turns := range []string{"", `,"turns":null`} {
		t.Run(fmt.Sprintf("turns=%s", turns), func(t *testing.T) {
			f := newRecoveryFixture(t)
			s, _ := f.session()
			input := restartRecoveryTestInput()
			raw, err := s.PrepareTurnRecovery(input)
			if err != nil {
				t.Fatal(err)
			}
			restored, stdin := f.session()
			stdin.respond = func(id int64, request map[string]any) {
				if request["method"] != "thread/read" {
					t.Errorf("unknown history started work: %v", request["method"])
				}
				restored.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(`{"thread":{"id":"thread-1","status":{"type":"idle"}` + turns + `}}`)})
			}
			if err := restored.ResumeInterruptedTurn(raw, input, nil); err == nil || !strings.Contains(err.Error(), "explicit complete turn list") {
				t.Fatalf("unknown history was accepted: %v", err)
			}
			if len(f.starts) != 0 {
				t.Fatal("unknown history submitted new model work")
			}
		})
	}
}

func TestRestartRecovery_PreparedSendPreservesSignatureAttemptScope(t *testing.T) {
	f := newRecoveryFixture(t)
	s, _ := f.session()
	input := restartRecoveryTestInput()
	s.recoveryUsed = true
	if _, err := s.PrepareTurnRecovery(input); err != nil {
		t.Fatal(err)
	}
	if !s.recoveryUsed {
		t.Fatal("preparing input reset the active signature recovery budget")
	}
	if err := s.SendWithTurnOptions(input.Prompt, nil, nil, input.Options); err != nil {
		t.Fatal(err)
	}
	if s.recoveryUsed {
		t.Fatal("new user Send did not reset signature recovery budget")
	}
	if f.starts[0]["clientUserMessageId"] == "" || f.starts[0]["clientUserMessageId"] == nil {
		t.Fatal("merged turn-start lost persisted client identity")
	}
	s.recoveryUsed = true
	if err := s.startTurn(s.CurrentSessionID(), []map[string]any{{"type": "text", "text": "internal continuation"}}, &input.Options); err != nil {
		t.Fatal(err)
	}
	if !s.recoveryUsed {
		t.Fatal("internal signature continuation allowed a second recovery")
	}
}
