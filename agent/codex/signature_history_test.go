package codex

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const recoveryTestMeta = `{"type":"session_meta","payload":{"id":"thread-a"}}`

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

func TestCleanSignatureHistoryPreservesPlainConversationAndToolResults(t *testing.T) {
	plain := []string{
		recoveryTestMeta,
		`{"type":"turn_context","payload":{"turn_id":"turn-a"}}`,
		recoveryTestItem(`{"type":"additional_tools","tools":[{"parameters":{"properties":{"encrypted_content":{"type":"string"}}}}]}`),
		recoveryTestItem(`{"type":"message","role":"user","content":[{"type":"input_text","text":"Continue the investigation"}]}`),
		recoveryTestItem(`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Already completed the write"}]}`),
		recoveryTestItem(`{"type":"function_call","call_id":"write-1","name":"write","arguments":"{}"}`),
		recoveryTestItem(`{"type":"function_call_output","call_id":"write-1","output":"Committed receipt 123"}`),
		recoveryTestItem(`{"type":"custom_tool_call","call_id":"read-1","name":"read","input":"document"}`),
		recoveryTestItem(`{"type":"custom_tool_call_output","call_id":"read-1","output":"Plain result"}`),
	}
	source := strings.Join(plain, "\n") + "\n" + recoveryTestItem(`{"type":"reasoning","id":"rs_a","sequence":9007199254740993,"summary":[{"type":"summary_text","text":"Visible summary"}],"encrypted_content":"bad-cipher"}`) + "\n"
	path := recoveryTestWrite(t, source)
	count, err := cleanSignatureHistory(context.Background(), path, "thread-a")
	if err != nil || count != 1 {
		t.Fatalf("removed=%d, err=%v", count, err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.HasPrefix(got, []byte(strings.Join(plain, "\n")+"\n")) {
		t.Fatal("plain history or tool data changed")
	}
	if !bytes.Contains(got, []byte("9007199254740993")) {
		t.Fatal("cleanup rounded an integer")
	}
	if bytes.Contains(got, []byte("bad-cipher")) || !bytes.Contains(got, []byte("Visible summary")) {
		t.Fatal("encrypted state or visible reasoning was not handled correctly")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatal("history permissions changed")
	}
}

func TestCleanSignatureHistoryClearsEncryptedToolAndCheckpointBlocks(t *testing.T) {
	source := strings.Join([]string{
		recoveryTestMeta,
		recoveryTestItem(`{"type":"function_call","call_id":"a","arguments":"plain arguments","encrypted_function_args":"secret"}`),
		recoveryTestItem(`{"type":"function_call_output","call_id":"a","output":[{"type":"input_text","text":"Plain receipt"},{"type":"encrypted_content","encrypted_content":"secret"}]}`),
		`{"type":"compacted","payload":{"replacement_history":[{"type":"compaction","encrypted_content":"secret"}]}}`,
		`{"type":"compacted","payload":{"replacement_history":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Plain checkpoint"}]},{"type":"reasoning","encrypted_content":"secret"}]}}`,
	}, "\n") + "\n"
	path := recoveryTestWrite(t, source)
	count, err := cleanSignatureHistory(context.Background(), path, "thread-a")
	if err != nil || count != 4 {
		t.Fatalf("removed=%d, err=%v", count, err)
	}
	got, _ := os.ReadFile(path)
	for _, text := range []string{"Plain receipt", "plain arguments", "Plain checkpoint"} {
		if !bytes.Contains(got, []byte(text)) {
			t.Fatalf("lost %q", text)
		}
	}
	if bytes.Contains(got, []byte("secret")) {
		t.Fatal("encrypted content survived cleanup")
	}
	if bytes.Count(got, []byte(`"compacted"`)) != 1 {
		t.Fatal("empty encrypted checkpoint was retained")
	}
}

func TestCleanSignatureHistoryRejectsInvalidOrCancelledFile(t *testing.T) {
	for _, input := range []string{
		recoveryTestMeta + "\n{\n",
		recoveryTestMeta + "\n" + recoveryTestItem(`{"type":"reasoning","encrypted_content":"secret"}`),
		strings.ReplaceAll(recoveryTestMeta, "thread-a", "other-thread") + "\n",
		recoveryTestMeta + "\n",
	} {
		path := recoveryTestWrite(t, input)
		if _, err := cleanSignatureHistory(context.Background(), path, "thread-a"); err == nil {
			t.Fatal("expected cleanup refusal")
		}
		got, _ := os.ReadFile(path)
		if string(got) != input {
			t.Fatal("failed cleanup changed history")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input := recoveryTestMeta + "\n" + recoveryTestItem(`{"type":"reasoning","encrypted_content":"secret"}`) + "\n"
	path := recoveryTestWrite(t, input)
	if _, err := cleanSignatureHistory(ctx, path, "thread-a"); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != input {
		t.Fatal("cancelled cleanup changed history")
	}
}
