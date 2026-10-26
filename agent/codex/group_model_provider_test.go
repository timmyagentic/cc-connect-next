package codex

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestGroupModelProvider_StartSessionSnapshotsBothBackends(t *testing.T) {
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []string{"exec", "app_server"} {
		t.Run(backend, func(t *testing.T) {
			root := t.TempDir()
			created, err := New(map[string]any{
				"cmd":        []string{bin, "-test.run=^TestDesktopCLIProcess$", "--", "desktop-fixture"},
				"backend":    backend,
				"model":      "fixture-model",
				"work_dir":   root,
				"codex_home": filepath.Join(root, "codex-home"),
				"env": map[string]string{
					"CC_DESKTOP_CLI_HELPER":    "1",
					"CC_GROUP_PROVIDER_MARKER": "",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			agent := created.(*Agent)
			agent.SetProviders([]core.ProviderConfig{
				{Name: "env-only", Env: map[string]string{"CC_GROUP_PROVIDER_MARKER": "env-only"}},
				{Name: "configured", CodexWireAPI: "responses", Env: map[string]string{"CC_GROUP_PROVIDER_MARKER": "configured"}},
			})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)
			var sessions []core.AgentSession
			selections := []string{"", "env-only", "configured", ""}
			for i, name := range selections {
				if !agent.SetActiveProvider(name) {
					t.Fatalf("cannot select provider %q", name)
				}
				session, err := agent.StartSession(ctx, "")
				if err != nil {
					t.Fatalf("StartSession(%q): %v", name, err)
				}
				cleanupCodexTestSession(t, session)
				sessions = append(sessions, session)
				// Check every existing session after each switch/clear, as well as
				// the new session. Identity must stay tied to its launch settings.
				for j := 0; j <= i; j++ {
					assertGroupModelProviderSnapshot(t, sessions[j], backend, selections[j])
				}
			}
		})
	}
}

func assertGroupModelProviderSnapshot(t *testing.T, session core.AgentSession, backend, want string) {
	t.Helper()
	identity, ok := session.(core.ModelProviderSession)
	if !ok {
		t.Fatalf("%s session does not expose its configured provider identity", backend)
	}
	if got := identity.ModelProviderName(); got != want {
		t.Fatalf("%s session provider = %q, want original %q", backend, got, want)
	}
	var env, args []string
	var cliProvider string
	switch s := session.(type) {
	case *codexSession:
		if backend != "exec" {
			t.Fatal("StartSession returned exec for app_server")
		}
		env, args, cliProvider = s.extraEnv, s.launchArgs("hello", nil), s.modelProvider
	case *appServerSession:
		if backend != "app_server" {
			t.Fatal("StartSession returned app_server for exec")
		}
		// Inspect the spawned fake CLI's environment, not only the session's
		// copied fields, to tie the reported identity to the actual launch.
		env, args, cliProvider = s.cmd.Env, s.cmd.Args[1:], s.modelProvider
	default:
		t.Fatalf("unexpected backend type %T", session)
	}
	if got := envSliceToMap(env)["CC_GROUP_PROVIDER_MARKER"]; got != want {
		t.Fatalf("%s launch provider marker = %q, want original %q", backend, got, want)
	}
	wantCLIProvider := ""
	if want == "configured" {
		wantCLIProvider = want
	}
	if cliProvider != wantCLIProvider {
		t.Fatalf("%s CLI provider = %q, want %q", backend, cliProvider, wantCLIProvider)
	}
	if got := slices.Contains(args, `model_provider="configured"`); got != (wantCLIProvider != "") {
		t.Fatalf("%s launch model_provider presence = %v, want %v", backend, got, wantCLIProvider != "")
	}
}
