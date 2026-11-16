package codex

import (
	"context"
	"slices"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestCronBackendExecRetainsConfigurationWithoutChangingAgent(t *testing.T) {
	agent := &Agent{backend: "app_server", workDir: t.TempDir(), codexHome: t.TempDir(),
		model: "chat-model", reasoningEffort: "high", mode: "yolo", cmd: "codex",
		configEnv: []string{"STATIC=value"}, sessionEnv: []string{"CC_PROJECT=project"}}
	scoped, err := agent.WithSessionBackend("exec")
	if err != nil {
		t.Fatal(err)
	}
	session, err := scoped.StartSession(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	exec, ok := session.(*codexSession)
	if !ok {
		t.Fatalf("expected exec session, got %T", session)
	}
	if agent.backend != "app_server" || exec.workDir != agent.workDir || exec.model != "chat-model" ||
		!slices.Contains(exec.extraEnv, "CODEX_HOME="+agent.codexHome) ||
		!slices.Contains(exec.extraEnv, "STATIC=value") ||
		!slices.Contains(exec.extraEnv, "CC_PROJECT=project") {
		t.Fatal("backend override lost configuration or mutated project")
	}
	for _, capability := range []any{core.SessionEnvInjector(scoped.(core.SessionEnvInjector)),
		core.ProviderSwitcher(scoped.(core.ProviderSwitcher)), core.PerTurnModelSupporter(scoped.(core.PerTurnModelSupporter))} {
		if capability == nil {
			t.Fatal("lost optional capability")
		}
	}
}

func TestCronBackendRejectsUnknownValueWithoutFallback(t *testing.T) {
	agent := &Agent{backend: "app_server"}
	for _, backend := range []string{"typo", "", "EXEC"} {
		if _, err := agent.WithSessionBackend(backend); err == nil {
			t.Fatalf("accepted %q", backend)
		}
	}
	if agent.backend != "app_server" {
		t.Fatal("mutated project backend")
	}
}
