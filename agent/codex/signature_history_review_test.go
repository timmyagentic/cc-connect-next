package codex

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These are application data, not native encrypted model state. Recovery may
// discard native opaque context, but must not rewrite schemas or tool receipts.
func TestCleanSignatureHistoryPreservesBusinessPlaintext(t *testing.T) {
	for _, tc := range []struct{ name, record, keep string }{
		{"checkpoint-tool-schema", `{"type":"compacted","payload":{"replacement_history":[{"type":"additional_tools","role":"developer","tools":[{"parameters":{"properties":{"encrypted_content":{"type":"string","enum":["business-encrypted-field"]},"operation":{"const":{"type":"compaction","label":"business-operation"}}}}}]},{"type":"reasoning","encrypted_content":"native-cipher"}]}}`, "business-encrypted-field"},
		{"structured-tool-receipt", recoveryTestItem(`{"type":"function_call_output","call_id":"done","output":{"encrypted_content":"business-receipt","encrypted_function_args":"business-args","operation":{"type":"compaction","label":"business-result"}}}`), "business-receipt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Join([]string{recoveryTestMeta, tc.record, recoveryTestItem(`{"type":"reasoning","encrypted_content":"native-cipher"}`), ""}, "\n")
			path := recoveryTestWrite(t, source)
			_, err := cleanSignatureHistory(context.Background(), path, "thread-a")
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), tc.keep) {
				t.Errorf("lost application plaintext %q", tc.keep)
			}
			if strings.Contains(string(got), "native-cipher") {
				t.Error("native encrypted state survived")
			}
		})
	}
}

func TestCleanSignatureHistoryPreservesPrivateOriginalBackup(t *testing.T) {
	source := recoveryTestMeta + "\n" + recoveryTestItem(`{"type":"reasoning","encrypted_content":"native-cipher"}`) + "\n"
	path := recoveryTestWrite(t, source)
	for attempt := 0; attempt < 2; attempt++ {
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := cleanSignatureHistory(context.Background(), path, "thread-a"); err != nil {
			t.Fatal(err)
		}
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".signature-original-*.bak"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 {
		t.Fatalf("backups overwritten or absent: %v", backups)
	}
	for _, backup := range backups {
		got, err := os.ReadFile(backup)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != source {
			t.Fatal("original backup is not byte-exact")
		}
		info, err := os.Stat(backup)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("backup mode=%v", info.Mode().Perm())
		}
		if strings.HasSuffix(backup, ".jsonl") {
			t.Fatal("backup can be scanned as rollout")
		}
	}
}

func TestCleanSignatureHistoryBackupFailureLeavesOriginalUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires non-root Unix directory permissions")
	}
	source := recoveryTestMeta + "\n" + recoveryTestItem(`{"type":"reasoning","encrypted_content":"native-cipher"}`) + "\n"
	path := recoveryTestWrite(t, source)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Error(err)
		}
	}()
	if _, err := cleanSignatureHistory(context.Background(), path, "thread-a"); err == nil {
		t.Fatal("recovery continued despite backup creation failure")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != source {
		t.Fatal("backup failure modified original history")
	}
}
