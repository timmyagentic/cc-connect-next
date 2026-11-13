package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	recoveryTestMeta    = `{"type":"session_meta","payload":{"id":"thread-a"}}`
	recoveryTestContext = `{"type":"turn_context","payload":{"turn_id":"turn-b"}}`
	recoveryTestDone    = `{"type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-b","error":{"message":"thinking_signature_invalid"}}}`
	recoveryTestInput   = `{"type":"message","role":"user","content":[{"type":"input_text","text":"Continue the investigation"},{"type":"input_image","image_url":"data:image/png;base64,AA=="}],"internal_chat_message_metadata_passthrough":{"turn_id":"turn-b"}}`
)

func recoveryTestItem(payload string) string {
	return `{"type":"response_item","payload":` + payload + `}`
}

func recoveryTestWrite(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSignatureRecoveryHistory(t *testing.T) {
	checkpoint := `{"type":"message","role":"user","content":[{"type":"input_text","text":"Earlier context retained in the checkpoint"}]}`
	assistant := `{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"I already ran the check."}]}`
	plainAgent := `{"type":"agent_message","author":"agent-a","recipient":"agent-b","content":[{"type":"input_text","text":"Read completed"}]}`
	function := `{"type":"function_call","name":"read_document","arguments":"{}","call_id":"read-a"}`
	custom := `{"type":"custom_tool_call","name":"exec","input":"read command","status":"completed","call_id":"read-b"}`
	functionOutput := `{"type":"function_call_output","call_id":"read-a","output":"` + strings.Repeat("x", 300*1024) + `"}`
	customOutput := `{"type":"custom_tool_call_output","call_id":"read-b","output":[{"type":"input_text","text":"Read result"},{"type":"input_image","image_url":"data:image/png;base64,AA=="}]}`

	path := recoveryTestWrite(t, strings.Join([]string{
		recoveryTestMeta,
		`{"type":"compacted","payload":{"replacement_history":[{"type":"compaction","encrypted_content":"superseded"}]}}`,
		recoveryTestItem(`{"type":"unavailable_item_type"}`),
		`{"type":"compacted","payload":{"replacement_history":[` + checkpoint + `,{"type":"reasoning","encrypted_content":"discarded-model-state"}]}}`,
		recoveryTestContext,
		recoveryTestItem(`{"type":"additional_tools","role":"developer","tools":[]}`),
		recoveryTestItem(`{"type":"message","role":"developer","content":[{"type":"input_text","text":"Reloaded instructions"}]}`),
		recoveryTestItem(recoveryTestInput),
		recoveryTestItem(assistant),
		recoveryTestItem(plainAgent),
		recoveryTestItem(`{"type":"reasoning","encrypted_content":"previous-model-state"}`),
		recoveryTestItem(function),
		recoveryTestItem(custom),
		recoveryTestItem(customOutput),
		recoveryTestItem(functionOutput),
		recoveryTestDone,
		"",
	}, "\n"))

	got, err := loadSignatureRecoveryHistory(context.Background(), path, "thread-a", "turn-b")
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, item := range got {
		actual = append(actual, string(item))
	}
	want := []string{checkpoint, recoveryTestInput, assistant, plainAgent, function, custom, customOutput, functionOutput}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("replay did not preserve the expected %d original items; got %d", len(want), len(actual))
	}
}

func TestSignatureRecoveryHistoryRejectsUnsafeReplay(t *testing.T) {
	call := recoveryTestItem(`{"type":"function_call","name":"read_document","arguments":"{}","call_id":"a"}`)
	for _, tt := range []struct {
		name    string
		add     []string
		wantErr string
	}{
		{
			name:    "unresolved call",
			add:     []string{call},
			wantErr: "without a recorded result",
		},
		{
			name: "wrong output type for call",
			add: []string{
				call,
				recoveryTestItem(`{"type":"custom_tool_call_output","call_id":"a","output":"wrong result type"}`),
			},
			wantErr: "unmatched tool output",
		},
		{
			name:    "duplicate tool call",
			add:     []string{call, call},
			wantErr: "duplicate tool call",
		},
		{
			name: "encrypted tool result alongside plain metadata",
			add: []string{
				call,
				recoveryTestItem(`{"type":"function_call_output","call_id":"a","output":[{"type":"input_text","text":"Tool finished"},{"type":"encrypted_content","encrypted_content":"only-copy"}]}`),
			},
			wantErr: "without a readable result",
		},
		{
			name:    "opaque agent message",
			add:     []string{recoveryTestItem(`{"type":"agent_message","author":"agent-a","recipient":"agent-b","content":[{"type":"encrypted_content","encrypted_content":"only-copy"}]}`)},
			wantErr: "unreadable agent message",
		},
		{
			name:    "unknown response item",
			add:     []string{recoveryTestItem(`{"type":"future_response_item"}`)},
			wantErr: "unsupported response item",
		},
		{
			name:    "unreadable compaction",
			add:     []string{`{"type":"compacted","payload":{"replacement_history":[` + recoveryTestInput + `,{"type":"compaction","encrypted_content":"only-copy"}]}}`},
			wantErr: "compaction contains unreadable context",
		},
		{
			name:    "additional context only exists in retained answers",
			add:     []string{`{"type":"compacted","payload":{"replacement_history":[` + recoveryTestInput + `],"retained_context":{"verified_answers":[{"text":"only copy"}]}}}`},
			wantErr: "retains context outside",
		},
		{
			name:    "unresolved native command",
			add:     []string{`{"type":"event_msg","payload":{"type":"item_started","turn_id":"turn-b","item":{"type":"CommandExecution","id":"command-1","status":"inProgress"}}}`},
			wantErr: "unfinished tool operations",
		},
		{
			name:    "unknown file update outcome",
			add:     []string{`{"type":"event_msg","payload":{"type":"item_completed","turn_id":"turn-b","item":{"type":"FileChange","id":"file-change-1","status":"cancelled"}}}`},
			wantErr: "unknown result",
		},
		{
			name:    "rollback",
			add:     []string{`{"type":"event_msg","payload":{"type":"thread_rolled_back","num_turns":1}}`},
			wantErr: "thread rollback",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lines := append([]string{recoveryTestMeta, recoveryTestContext, recoveryTestItem(recoveryTestInput)}, tt.add...)
			lines = append(lines, recoveryTestDone, "")
			path := recoveryTestWrite(t, strings.Join(lines, "\n"))
			got, err := loadSignatureRecoveryHistory(context.Background(), path, "thread-a", "turn-b")
			if got != nil || err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %d items and error %v, want no items and %q", len(got), err, tt.wantErr)
			}
		})
	}
}

func TestSignatureRecoveryHistoryRequiresExactCompleteTurn(t *testing.T) {
	prefix := recoveryTestMeta + "\n" + recoveryTestContext + "\n"
	withInput := prefix + recoveryTestItem(recoveryTestInput) + "\n"
	for _, tt := range []struct {
		name     string
		contents string
		threadID string
		wantErr  string
	}{
		{"wrong thread", withInput + recoveryTestDone + "\n", "other-thread", "thread identity"},
		{"input not persisted", prefix + recoveryTestDone + "\n", "thread-a", "user input"},
		{"synthetic user context is not the request", prefix + recoveryTestItem(`{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>example</environment_context>"}]}`) + "\n" + recoveryTestDone + "\n", "thread-a", "user input"},
		{"input belongs to another turn", prefix + recoveryTestItem(strings.ReplaceAll(recoveryTestInput, "turn-b", "other-turn")) + "\n" + recoveryTestDone + "\n", "thread-a", "user input"},
		{"terminal not persisted", withInput, "thread-a", "failed turn"},
		{"truncated final line", withInput + recoveryTestDone, "thread-a", "incomplete"},
		{"malformed complete line", withInput + "{\n" + recoveryTestDone + "\n", "thread-a", "invalid"},
		{"history advanced after failed turn", withInput + recoveryTestDone + "\n" + strings.ReplaceAll(recoveryTestContext, "turn-b", "turn-c") + "\n", "thread-a", "turn boundary"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := recoveryTestWrite(t, tt.contents)
			got, err := loadSignatureRecoveryHistory(context.Background(), path, tt.threadID, "turn-b")
			if got != nil || err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %d items and error %v, want no items and %q", len(got), err, tt.wantErr)
			}
		})
	}
}

func TestSignatureRecoveryHistoryRejectsOversizedRecordAndCancellation(t *testing.T) {
	oversized := `{"type":"message","role":"user","content":[{"type":"input_text","text":"` + strings.Repeat("x", signatureHistoryMaxLine) + `"}]}`
	path := recoveryTestWrite(t, strings.Join([]string{
		recoveryTestMeta, recoveryTestContext, recoveryTestItem(oversized), recoveryTestDone, "",
	}, "\n"))
	got, err := loadSignatureRecoveryHistory(context.Background(), path, "thread-a", "turn-b")
	if got != nil || err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("got %d items and error %v, want rejected oversized record", len(got), err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = loadSignatureRecoveryHistory(ctx, path, "thread-a", "turn-b")
	if got != nil || err != context.Canceled {
		t.Fatalf("got %d items and error %v, want context cancellation", len(got), err)
	}
}

func TestSignatureRecoveryHistoryPreservesInputInLatestCheckpoint(t *testing.T) {
	path := recoveryTestWrite(t, strings.Join([]string{
		recoveryTestMeta,
		recoveryTestContext,
		recoveryTestItem(recoveryTestInput),
		`{"type":"compacted","payload":{"replacement_history":[` + recoveryTestInput + `]}}`,
		recoveryTestDone,
		"",
	}, "\n"))
	got, err := loadSignatureRecoveryHistory(context.Background(), path, "thread-a", "turn-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !json.Valid(got[0]) || string(got[0]) != recoveryTestInput {
		t.Fatalf("got %d items, want the original request once", len(got))
	}
}
