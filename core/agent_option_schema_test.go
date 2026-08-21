package core

import (
	"strings"
	"testing"
)

func TestBuildCapabilityBrief_ListsEveryKeyAndTheFeedbackPath(t *testing.T) {
	brief := BuildCapabilityBrief("codex", []string{"work_dir", "model", "service_tier"})
	for _, want := range []string{"codex", "`model`", "`service_tier`", "`work_dir`", "/feedback"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q: %q", want, brief)
		}
	}
	if BuildCapabilityBrief("codex", nil) != "" {
		t.Error("no declared keys must mean no brief")
	}
}

// The brief is prepended exactly once per session state: the first prompt
// carries it, every later prompt is untouched.
func TestBuildCapabilityPrompt_OncePerSessionState(t *testing.T) {
	e := &Engine{}
	e.SetCapabilityBrief(BuildCapabilityBrief("codex", []string{"model"}))
	state := &interactiveState{}

	first := e.buildCapabilityPrompt(state, "hello")
	if !strings.HasPrefix(first, "[cc-connect-next capability brief]") || !strings.HasSuffix(first, "hello") {
		t.Fatalf("first prompt must carry the brief, got %q", first)
	}
	if second := e.buildCapabilityPrompt(state, "again"); second != "again" {
		t.Fatalf("second prompt must be untouched, got %q", second)
	}

	// A fresh session state gets the brief again.
	if fresh := e.buildCapabilityPrompt(&interactiveState{}, "hi"); !strings.Contains(fresh, "capability brief") {
		t.Fatalf("new state must get the brief, got %q", fresh)
	}
}

func TestBuildCapabilityPrompt_NoBriefIsPassthrough(t *testing.T) {
	e := &Engine{}
	state := &interactiveState{}
	if got := e.buildCapabilityPrompt(state, "hello"); got != "hello" {
		t.Fatalf("without a brief the prompt must be untouched, got %q", got)
	}
	state.mu.Lock()
	sent := state.capabilityBriefSent
	state.mu.Unlock()
	if sent {
		t.Error("passthrough must not consume the once-per-state budget")
	}
}
