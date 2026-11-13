package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

const signatureTestError = `{"error":{"code":"thinking_signature_invalid","message":"Encrypted content could not be verified"}}`

func TestSignatureErrorRecognition(t *testing.T) {
	for _, value := range []string{signatureTestError, "invalid_encrypted_content: rejected", "stream disconnected before completion: invalid_encrypted_content: rejected", "unexpected status 400 Bad Request: " + signatureTestError} {
		if !isSignatureError(errors.New(value)) {
			t.Errorf("did not recognize %q", value)
		}
	}
	for _, value := range []string{"network timeout", "mention of thinking_signature_invalid in another error", `{"error":{"code":"rate_limit"}}`} {
		if isSignatureError(errors.New(value)) {
			t.Errorf("recognized unrelated error %q", value)
		}
	}
}

// This subprocess implements only the native protocol used by the adapter.
// The same on-disk fixture and launch log survive its transport reload.
func TestSignatureRecoveryHelperProcess(t *testing.T) {
	if os.Getenv("CC_SIGNATURE_HELPER") != "1" {
		return
	}
	dir := os.Getenv("CC_SIGNATURE_DIR")
	path := filepath.Join(dir, "sessions", "thread-a.jsonl")
	first := true
	if _, err := os.Stat(filepath.Join(dir, "failed")); err == nil {
		first = false
	}
	log, err := os.OpenFile(filepath.Join(dir, "launches"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(2)
	}
	if _, err := fmt.Fprintln(log, "launch"); err != nil {
		os.Exit(2)
	}
	if err := log.Close(); err != nil {
		os.Exit(2)
	}
	out := json.NewEncoder(os.Stdout)
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		var req struct {
			ID     any            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal(scan.Bytes(), &req) != nil {
			os.Exit(3)
		}
		if req.ID == nil {
			continue
		}
		turnID := "turn-a"
		if !first {
			turnID = "turn-b"
		}
		result := any(map[string]any{})
		switch req.Method {
		case "thread/resume", "thread/read":
			result = map[string]any{"thread": map[string]any{"id": "thread-a", "path": path}, "model": "original-model", "reasoningEffort": "high"}
			if req.Params["includeTurns"] == true {
				result = map[string]any{"thread": map[string]any{"id": "thread-a", "status": map[string]string{"type": "idle"}, "turns": []any{map[string]any{"id": "turn-a", "status": "failed", "items": []any{map[string]any{"type": "userMessage", "clientId": "original-client"}}}}}}
			}
		case "thread/backgroundTerminals/list":
			result = map[string]any{"data": []any{}, "nextCursor": nil}
		case "turn/start":
			result = map[string]any{"turn": map[string]any{"id": turnID}}
		}
		if err := out.Encode(map[string]any{"id": req.ID, "result": result}); err != nil {
			os.Exit(4)
		}
		if req.Method != "turn/start" {
			continue
		}
		data, err := json.Marshal(req.Params)
		if err != nil {
			os.Exit(5)
		}
		_ = os.WriteFile(filepath.Join(dir, "last-turn.json"), data, 0o600)
		if first {
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(filepath.Join(dir, "send-ready")); err == nil {
					break
				}
				if time.Now().After(deadline) {
					os.Exit(6)
				}
				time.Sleep(time.Millisecond)
			}
			_ = os.WriteFile(filepath.Join(dir, "failed"), []byte("yes"), 0o600)
		}
		if err := out.Encode(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": "thread-a", "turn": map[string]any{"id": turnID}}}); err != nil {
			os.Exit(4)
		}
		failed := first || os.Getenv("CC_SIGNATURE_REFAIL") == "1"
		if failed {
			if err := out.Encode(map[string]any{"method": "error", "params": map[string]any{"threadId": "thread-a", "turnId": turnID, "willRetry": false, "error": map[string]any{"message": signatureTestError}}}); err != nil {
				os.Exit(4)
			}
			if err := out.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-a", "turn": map[string]any{"id": turnID, "status": "failed", "error": map[string]any{"message": signatureTestError}}}}); err != nil {
				os.Exit(4)
			}
		} else {
			data, _ := os.ReadFile(path)
			if strings.Contains(string(data), "bad-cipher") || !strings.Contains(string(data), "Committed receipt 123") {
				os.Exit(7)
			}
			if err := out.Encode(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": "thread-a", "turnId": turnID, "item": map[string]any{"type": "agentMessage", "id": "answer", "text": "Recovered using receipt 123"}}}); err != nil {
				os.Exit(4)
			}
			if err := out.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-a", "turn": map[string]any{"id": turnID, "status": "completed"}}}); err != nil {
				os.Exit(4)
			}
		}
	}
	if first && os.Getenv("CC_SIGNATURE_HOLD_CLOSE") == "1" {
		_ = os.WriteFile(filepath.Join(dir, "closing"), []byte("yes"), 0o600)
		for {
			time.Sleep(time.Second)
		}
	}
	os.Exit(0)
}

func signatureProcessSession(t *testing.T, extra ...string) (*appServerSession, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	source := recoveryTestMeta + "\n" + recoveryTestItem(`{"type":"message","role":"user","content":[{"type":"input_text","text":"Continue the investigation"}]}`) + "\n" + recoveryTestItem(`{"type":"reasoning","encrypted_content":"bad-cipher"}`) + "\n" + recoveryTestItem(`{"type":"function_call","call_id":"a","name":"write","arguments":"{}"}`) + "\n" + recoveryTestItem(`{"type":"function_call_output","call_id":"a","output":"Committed receipt 123"}`) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sessions", "thread-a.jsonl"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	env := append([]string{"CC_SIGNATURE_HELPER=1", "CC_SIGNATURE_DIR=" + dir}, extra...)
	s, err := newAppServerSession(context.Background(), appServerSessionParams{cliBin: executable, cliExtraArgs: []string{"-test.run=^TestSignatureRecoveryHelperProcess$", "--"}, codexHome: dir, workDir: dir, model: "original-model", effort: "high", resumeID: "thread-a", extraEnv: env})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close test session: %v", err)
		}
	})
	for _, option := range extra {
		if option == "CC_SIGNATURE_CHECKPOINT=1" {
			s.preparedRecovery = &codexTurnCheckpoint{Version: 1, ThreadID: "thread-a", ClientID: "original-client", InputHash: recoveryInputHash("Continue the investigation", nil, nil), Input: []map[string]any{{"type": "text", "text": "Continue the investigation"}}, WorkDir: s.workDir, Mode: normalizeMode(s.mode), Provider: s.providerName}
		}
	}
	if err := s.SendWithTurnOptions("Continue the investigation", nil, nil, core.TurnOptions{Model: "task-model", ReasoningEffort: "high", ServiceTier: "fast"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "send-ready"), []byte("yes"), 0o600); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func TestSignatureRecoveryRetriesSameThreadWithoutIntermediateError(t *testing.T) {
	s, dir := signatureProcessSession(t)
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	text := ""
	for {
		select {
		case event := <-s.Events():
			switch event.Type {
			case core.EventError:
				t.Fatalf("intermediate error surfaced: %v", event.Error)
			case core.EventText:
				text += event.Content
			case core.EventResult:
				if s.CurrentSessionID() != "thread-a" || !strings.Contains(text+event.Content, "Recovered using receipt 123") {
					t.Fatalf("thread=%s,text=%q,event=%#v", s.CurrentSessionID(), text, event)
				}
				launches, _ := os.ReadFile(filepath.Join(dir, "launches"))
				if strings.Count(string(launches), "launch") != 2 {
					t.Fatalf("unexpected reloads %q", launches)
				}
				data, _ := os.ReadFile(filepath.Join(dir, "last-turn.json"))
				var turn map[string]any
				if err := json.Unmarshal(data, &turn); err != nil {
					t.Fatal(err)
				}
				if turn["threadId"] != "thread-a" || turn["model"] != "task-model" || turn["serviceTier"] != "fast" {
					t.Fatalf("retry lost options: %s", data)
				}
				return
			}
		case <-timer.C:
			t.Fatal("timed out waiting for same-thread recovery")
		}
	}
}

func TestSignatureRecoverySecondFailureIsReportedOnce(t *testing.T) {
	s, dir := signatureProcessSession(t, "CC_SIGNATURE_REFAIL=1")
	select {
	case event := <-s.Events():
		if event.Type != core.EventError || !isSignatureError(event.Error) {
			t.Fatalf("unexpected final event %#v", event)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("no final error")
	}
	launches, _ := os.ReadFile(filepath.Join(dir, "launches"))
	if strings.Count(string(launches), "launch") != 2 {
		t.Fatalf("retry loop: %s", launches)
	}
	select {
	case event := <-s.Events():
		t.Fatalf("duplicate final event %#v", event)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestSignatureRecoveryStopDuringReloadPreventsRetry(t *testing.T) {
	s, dir := signatureProcessSession(t, "CC_SIGNATURE_HOLD_CLOSE=1")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "closing")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reload did not start")
		}
		time.Sleep(time.Millisecond)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	launches, _ := os.ReadFile(filepath.Join(dir, "launches"))
	if strings.Count(string(launches), "launch") != 1 {
		t.Fatalf("cancelled recovery restarted: %s", launches)
	}
}

func TestSignatureRecoverySilentRetryWaitsForDurableCheckpoint(t *testing.T) {
	s, dir := signatureProcessSession(t, "CC_SIGNATURE_CHECKPOINT=1")
	deadline := time.After(8 * time.Second)
	acknowledged := false
	for {
		select {
		case event := <-s.Events():
			switch event.Type {
			case core.EventError:
				t.Fatal(event.Error)
			case core.EventSessionRecovery:
				if acknowledged || !event.RecoverySilent || event.SessionID != "thread-a" || event.RecoveryReady == nil {
					t.Fatalf("unexpected retry event: %#v", event)
				}
				var cp codexTurnCheckpoint
				if err := json.Unmarshal(event.RecoveryCheckpoint, &cp); err != nil || !cp.SignatureRecovered || cp.BoundaryID != "turn-a" || cp.ClientID == "original-client" {
					t.Fatalf("invalid retry checkpoint: %+v %v", cp, err)
				}
				data, err := os.ReadFile(filepath.Join(dir, "last-turn.json"))
				if err != nil {
					t.Fatal(err)
				}
				var params map[string]any
				if err := json.Unmarshal(data, &params); err != nil {
					t.Fatal(err)
				}
				if params["clientUserMessageId"] != "original-client" {
					t.Fatal("retry started before persistence acknowledgement")
				}
				event.RecoveryReady <- nil
				acknowledged = true
			case core.EventResult:
				if !acknowledged {
					t.Fatal("finished retry without durable checkpoint")
				}
				return
			}
		case <-deadline:
			t.Fatal("retry did not finish")
		}
	}
}
