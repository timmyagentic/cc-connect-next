package codex

import (
	"context"
	"os/exec"
	"slices"
	"testing"
	"time"
)

const testModelContextWindow int64 = 872000

func TestNew_ModelContextWindowAcceptsPositiveIntegers(t *testing.T) {
	for name, raw := range map[string]any{
		"toml int64": int64(testModelContextWindow),
		"native int": int(testModelContextWindow),
	} {
		t.Run(name, func(t *testing.T) {
			agent, err := New(map[string]any{
				"cmd":                  "go",
				"model_context_window": raw,
			})
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			got := agent.(*Agent).WorkspaceAgentOptions()["model_context_window"]
			if got != testModelContextWindow {
				t.Fatalf("WorkspaceAgentOptions()[model_context_window] = %#v, want %d", got, testModelContextWindow)
			}
		})
	}
}

func TestNew_ModelContextWindowRejectsInvalidValues(t *testing.T) {
	for name, raw := range map[string]any{
		"zero":              int64(0),
		"negative":          int64(-1),
		"floating point":    float64(testModelContextWindow),
		"numeric as string": "872000",
	} {
		t.Run(name, func(t *testing.T) {
			agent, err := New(map[string]any{
				"cmd":                  "go",
				"model_context_window": raw,
			})
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			if _, ok := agent.(*Agent).WorkspaceAgentOptions()["model_context_window"]; ok {
				t.Fatalf("invalid model_context_window %#v must be omitted", raw)
			}
		})
	}
}

func TestModelContextWindow_ExecAndAppServerParity(t *testing.T) {
	want := "model_context_window=872000"
	appServerArgs := (&appServerSession{
		url:           "stdio://",
		contextWindow: testModelContextWindow,
	}).launchArgs()
	execArgs := (&codexSession{
		mode:          "suggest",
		workDir:       t.TempDir(),
		contextWindow: testModelContextWindow,
	}).launchArgs("hello", nil)

	for name, args := range map[string][]string{"app_server": appServerArgs, "exec": execArgs} {
		idx := slices.Index(args, want)
		if idx < 1 || args[idx-1] != "-c" {
			t.Errorf("%s backend: expected -c %s in argv, got %q", name, want, args)
		}
	}
}

func TestModelContextWindow_StructuredOptionWinsOverCmdExtras(t *testing.T) {
	extras := []string{"-c", "model_context_window=272000"}
	appServerArgs := (&appServerSession{
		url:           "stdio://",
		cliExtraArgs:  extras,
		contextWindow: testModelContextWindow,
	}).launchArgs()
	execArgs := (&codexSession{
		mode:          "suggest",
		workDir:       t.TempDir(),
		cliExtraArgs:  extras,
		contextWindow: testModelContextWindow,
	}).launchArgs("hello", nil)

	for name, args := range map[string][]string{"app_server": appServerArgs, "exec": execArgs} {
		extraIdx := slices.Index(args, "model_context_window=272000")
		structuredIdx := slices.Index(args, "model_context_window=872000")
		if extraIdx == -1 || structuredIdx == -1 {
			t.Fatalf("%s backend: expected both context window overrides, got %q", name, args)
		}
		if structuredIdx < extraIdx {
			t.Errorf("%s backend: structured override must come last, got %q", name, args)
		}
	}
}

func TestModelContextWindow_ZeroIsOmittedOnBothBackends(t *testing.T) {
	appServerArgs := (&appServerSession{url: "stdio://"}).launchArgs()
	execArgs := (&codexSession{mode: "suggest", workDir: t.TempDir()}).launchArgs("hello", nil)

	for name, args := range map[string][]string{"app_server": appServerArgs, "exec": execArgs} {
		if slices.Contains(args, "model_context_window=0") {
			t.Errorf("%s backend: zero context window must be omitted, got %q", name, args)
		}
	}
}

func TestStartSession_ExecCarriesModelContextWindow(t *testing.T) {
	agent := &Agent{
		workDir:       t.TempDir(),
		mode:          "suggest",
		backend:       "exec",
		contextWindow: testModelContextWindow,
		activeIdx:     -1,
	}
	session, err := agent.StartSession(context.Background(), "")
	if err != nil {
		t.Fatalf("StartSession() error: %v", err)
	}
	defer session.Close()

	got := session.(*codexSession).contextWindow
	if got != testModelContextWindow {
		t.Fatalf("exec session modelContextWindow = %d, want %d", got, testModelContextWindow)
	}
}

func TestKnownOptionKeys_IncludesModelContextWindow(t *testing.T) {
	if !slices.Contains((&Agent{}).KnownOptionKeys(), "model_context_window") {
		t.Fatal("KnownOptionKeys() must include model_context_window")
	}
}

func TestIntegration_AppServerHonorsModelContextWindow(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex CLI not in PATH, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agent, err := New(map[string]any{
		"cmd":                  "codex",
		"backend":              "app_server",
		"work_dir":             t.TempDir(),
		"model_context_window": testModelContextWindow,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	session, err := agent.StartSession(ctx, "")
	if err != nil {
		t.Fatalf("StartSession() error: %v", err)
	}
	defer session.Close()
	appServer := session.(*appServerSession)

	var response struct {
		Config struct {
			ModelContextWindow int64 `json:"model_context_window"`
		} `json:"config"`
	}
	if err := appServer.request("config/read", map[string]any{
		"includeLayers": false,
	}, &response); err != nil {
		t.Fatalf("config/read error: %v", err)
	}
	if got := response.Config.ModelContextWindow; got != testModelContextWindow {
		t.Fatalf("effective model_context_window = %d, want %d", got, testModelContextWindow)
	}
}
