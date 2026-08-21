package codex

import (
	"slices"
	"testing"
)

// service_tier is a first-class structured option (like reasoning_effort):
// declared in config.toml and emitted as -c service_tier=... on both
// backends, instead of being injected through cmd extra args. Values are
// model-catalog-driven, so they pass through verbatim without validation.

func TestAppServerLaunchArgs_IncludesServiceTier(t *testing.T) {
	s := &appServerSession{url: "stdio://", serviceTier: "fast"}
	args := s.launchArgs()

	idx := slices.Index(args, `service_tier="fast"`)
	if idx < 1 || args[idx-1] != "-c" {
		t.Errorf("expected -c service_tier=\"fast\" in launch args, got %q", args)
	}
}

func TestAppServerLaunchArgs_OmitsEmptyServiceTier(t *testing.T) {
	s := &appServerSession{url: "stdio://", serviceTier: "  "}
	for _, arg := range s.launchArgs() {
		if arg == `service_tier=""` {
			t.Errorf("blank service_tier must be omitted, got %q", s.launchArgs())
		}
	}
}

func TestExecArgs_IncludesServiceTier(t *testing.T) {
	cs := &codexSession{mode: "full-auto", workDir: t.TempDir(), serviceTier: "fast"}
	args := cs.launchArgs("hello", nil)

	idx := slices.Index(args, `service_tier="fast"`)
	if idx < 1 || args[idx-1] != "-c" {
		t.Errorf("expected -c service_tier=\"fast\" in exec args, got %q", args)
	}
}

// The same structured option must reach the spawned process on both backends.
func TestServiceTier_ExecAndAppServerParity(t *testing.T) {
	appServer := (&appServerSession{url: "stdio://", serviceTier: "fast"}).launchArgs()
	execArgs := (&codexSession{mode: "suggest", workDir: t.TempDir(), serviceTier: "fast"}).launchArgs("hi", nil)

	for name, args := range map[string][]string{"app_server": appServer, "exec": execArgs} {
		if !slices.Contains(args, `service_tier="fast"`) {
			t.Errorf("%s backend: service_tier missing from argv %q", name, args)
		}
	}
}

func TestWorkspaceAgentOptions_IncludesServiceTier(t *testing.T) {
	a := &Agent{serviceTier: "fast"}
	if got := a.WorkspaceAgentOptions()["service_tier"]; got != "fast" {
		t.Errorf("WorkspaceAgentOptions()[service_tier] = %v, want \"fast\"", got)
	}

	b := &Agent{}
	if _, ok := b.WorkspaceAgentOptions()["service_tier"]; ok {
		t.Error("empty service_tier must not appear in WorkspaceAgentOptions")
	}
}
