package codex

import (
	"context"
	"os/exec"
	"reflect"
	"slices"
	"testing"
	"time"
)

// Regression tests for issue #37: with backend = "app_server", the binary and
// extra args parsed from the cmd option were silently dropped — the spawned
// process was always `codex app-server ...` regardless of configuration —
// while backend = "exec" honored the same cmd option.

func TestAppServerLaunchArgs_PropagatesCmdExtraArgs(t *testing.T) {
	extras := []string{"-c", `service_tier="fast"`, "-c", "features.fast_mode=true"}
	args := (&appServerSession{cliExtraArgs: extras, url: "stdio://", model: "o3", effort: "max", modelProvider: "myprov", baseURL: "https://api.example.com"}).launchArgs()

	want := []string{
		"-c", `service_tier="fast"`,
		"-c", "features.fast_mode=true",
		"app-server",
		"--listen", "stdio://",
		"-c", `model="o3"`,
		"-c", `model_reasoning_effort="max"`,
		"-c", `model_provider="myprov"`,
		"-c", `openai_base_url="https://api.example.com"`,
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("launch args mismatch:\n got  %q\n want %q", args, want)
	}
}

func TestAppServerLaunchArgs_NoExtrasKeepsLegacyShape(t *testing.T) {
	args := (&appServerSession{url: "stdio://", model: "o3", effort: "high"}).launchArgs()

	want := []string{
		"app-server",
		"--listen", "stdio://",
		"-c", `model="o3"`,
		"-c", `model_reasoning_effort="high"`,
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("launch args mismatch:\n got  %q\n want %q", args, want)
	}
}

// Codex resolves duplicate -c keys last-wins, so structured options must come
// after cmd extras — the same precedence the exec backend has always had.
func TestAppServerLaunchArgs_StructuredOptionsWinOverCmdExtras(t *testing.T) {
	extras := []string{"-c", `model_reasoning_effort="low"`}
	args := (&appServerSession{cliExtraArgs: extras, url: "stdio://", effort: "high"}).launchArgs()

	lowIdx := slices.Index(args, `model_reasoning_effort="low"`)
	highIdx := slices.Index(args, `model_reasoning_effort="high"`)
	if lowIdx == -1 || highIdx == -1 {
		t.Fatalf("expected both effort overrides in args, got %q", args)
	}
	if highIdx < lowIdx {
		t.Errorf("structured -c must come after cmd extras (last-wins), got %q", args)
	}
}

// The same cmd option must position its extra args identically on both
// backends: before the subcommand, so structured options win on conflicts.
func TestCmdExtraArgs_ExecAndAppServerParity(t *testing.T) {
	extras := []string{"-c", `service_tier="fast"`}

	cs := &codexSession{mode: "full-auto", workDir: t.TempDir(), cliExtraArgs: extras}
	execArgs := cs.launchArgs("hello", nil)
	appServerArgs := (&appServerSession{cliExtraArgs: extras, url: "stdio://"}).launchArgs()

	for name, args := range map[string][]string{"exec": execArgs, "app_server": appServerArgs} {
		if len(args) < 3 || !reflect.DeepEqual(args[:2], extras) {
			t.Errorf("%s backend: cmd extras must lead the argv, got %q", name, args)
		}
	}
	if appServerArgs[2] != "app-server" {
		t.Errorf("app_server backend: expected subcommand right after extras, got %q", appServerArgs)
	}
	if execArgs[2] != "exec" {
		t.Errorf("exec backend: expected subcommand right after extras, got %q", execArgs)
	}
}

func TestNormalizeReasoningEffort_AcceptsMax(t *testing.T) {
	for raw, want := range map[string]string{
		"max":   "max",
		"MAX":   "max",
		" max ": "max",
		"xhigh": "xhigh",
		"med":   "medium",
		"":      "",
	} {
		if got := normalizeReasoningEffort(raw); got != want {
			t.Errorf("normalizeReasoningEffort(%q) = %q, want %q", raw, got, want)
		}
	}
}

// Unknown values fall back to the CLI default ("") but must no longer be
// swallowed silently — normalizeReasoningEffort logs a warning naming the
// supported values.
func TestNormalizeReasoningEffort_UnknownFallsBackToCLIDefault(t *testing.T) {
	if got := normalizeReasoningEffort("banana"); got != "" {
		t.Errorf("normalizeReasoningEffort(banana) = %q, want empty", got)
	}
}

// End-to-end check against the real CLI: cmd extra args must reach the
// spawned app-server process and change its effective configuration, as
// reported by config/read. This is exactly what issue #37 observed breaking.
func TestIntegration_RuntimeConfigHonorsCmdExtraArgs(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex CLI not in PATH, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extras := []string{"-c", `model="cc-connect-issue37-probe"`, "-c", `model_reasoning_effort="xhigh"`}
	model, effort, err := loadCodexRuntimeConfig(ctx, "codex", extras, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("loadCodexRuntimeConfig with extras: %v", err)
	}
	if model != "cc-connect-issue37-probe" {
		t.Errorf("effective model = %q, want the -c override from cmd extras", model)
	}
	if effort != "xhigh" {
		t.Errorf("effective reasoning effort = %q, want %q", effort, "xhigh")
	}
}

func TestAvailableReasoningEfforts_IncludesMax(t *testing.T) {
	a := &Agent{}
	if efforts := a.AvailableReasoningEfforts(); !slices.Contains(efforts, "max") {
		t.Errorf("AvailableReasoningEfforts() = %q, want it to include \"max\"", efforts)
	}
}
