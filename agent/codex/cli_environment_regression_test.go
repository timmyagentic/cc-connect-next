package codex

import "testing"

func TestNew_CodexCLIPathEnvironmentIsExplicit(t *testing.T) {
	t.Setenv("CODEX_CLI_PATH", "/missing/codex-override")
	if _, err := New(map[string]any{}); err == nil {
		t.Fatal("an invalid explicit CODEX_CLI_PATH must fail instead of selecting PATH")
	}
}
