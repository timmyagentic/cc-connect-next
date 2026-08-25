package codex

import (
	"slices"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

var (
	_ core.TurnOptionsSession  = (*appServerSession)(nil)
	_ core.TurnOptionsSession  = (*codexSession)(nil)
	_ core.ServiceTierProvider = (*Agent)(nil)
)

func TestAppServerTurnStartParamsUseOneShotOptionsAndClearBlankDefaults(t *testing.T) {
	session := &appServerSession{model: "balanced-model", effort: "medium", serviceTier: "default", mode: "suggest"}
	input := []map[string]any{{"type": "text", "text": "hello"}}

	quality := core.TurnOptions{
		AnswerProfile:   core.AnswerProfileQuality,
		Model:           "quality-model",
		ReasoningEffort: "max",
		ServiceTier:     "fast",
	}
	params := session.turnStartParams("thread-1", input, &quality)
	if params["model"] != "quality-model" || params["effort"] != "max" || params["serviceTier"] != "fast" {
		t.Fatalf("quality params = %#v", params)
	}

	defaultsWithoutExplicitValues := core.TurnOptions{}
	params = session.turnStartParams("thread-1", input, &defaultsWithoutExplicitValues)
	for _, key := range []string{"model", "effort", "serviceTier"} {
		value, ok := params[key]
		if !ok || value != nil {
			t.Fatalf("default reset %s = %#v (present=%v), want explicit null", key, value, ok)
		}
	}
}

func TestAppServerLegacySendParamsPreserveExistingBehavior(t *testing.T) {
	session := &appServerSession{model: "balanced-model", effort: "medium", serviceTier: "fast", mode: "suggest"}
	params := session.turnStartParams("thread-1", nil, nil)
	if params["model"] != "balanced-model" || params["effort"] != "medium" {
		t.Fatalf("legacy params = %#v", params)
	}
	if _, ok := params["serviceTier"]; ok {
		t.Fatalf("legacy params unexpectedly override per-turn service tier: %#v", params)
	}
}

func TestExecArgsUseOneShotOptionsThenDefaults(t *testing.T) {
	session := &codexSession{
		mode:        "suggest",
		workDir:     t.TempDir(),
		model:       "balanced-model",
		effort:      "medium",
		serviceTier: "default",
	}
	quality := core.TurnOptions{
		AnswerProfile:   core.AnswerProfileQuality,
		Model:           "quality-model",
		ReasoningEffort: "max",
		ServiceTier:     "fast",
	}
	qualityArgs := session.launchArgsWithTurnOptions("hello", nil, &quality)
	for _, want := range []string{"quality-model", `model_reasoning_effort="max"`, `service_tier="fast"`} {
		if !slices.Contains(qualityArgs, want) {
			t.Fatalf("quality args %q missing %q", qualityArgs, want)
		}
	}

	defaults := core.TurnOptions{Model: "balanced-model", ReasoningEffort: "medium", ServiceTier: "default"}
	defaultArgs := session.launchArgsWithTurnOptions("hello", nil, &defaults)
	for _, want := range []string{"balanced-model", `model_reasoning_effort="medium"`, `service_tier="default"`} {
		if !slices.Contains(defaultArgs, want) {
			t.Fatalf("default args %q missing %q", defaultArgs, want)
		}
	}
	for _, leaked := range []string{"quality-model", `model_reasoning_effort="max"`, `service_tier="fast"`} {
		if slices.Contains(defaultArgs, leaked) {
			t.Fatalf("default args %q leaked %q", defaultArgs, leaked)
		}
	}
}

func TestAgentGetServiceTier(t *testing.T) {
	agent := &Agent{serviceTier: "  fast  "}
	if got := agent.GetServiceTier(); got != "fast" {
		t.Fatalf("GetServiceTier() = %q, want fast", got)
	}
}
