package core

import (
	"strings"
	"testing"
)

type manifestPerTurnModelAgent struct {
	stubAgent
	supported bool
}

func (a *manifestPerTurnModelAgent) SupportsPerTurnModel() bool { return a.supported }

func TestAgentCapabilityManifest_CronModelContract(t *testing.T) {
	for _, tool := range agentToolDefinitions {
		if tool.id != "cron" {
			continue
		}
		for _, parameter := range tool.parameters {
			if parameter.Name == "model" {
				if parameter.Required || parameter.Type != "string" || !strings.Contains(parameter.Description, "per_turn_model") || parameter.DescriptionZH == "" {
					t.Fatalf("model parameter does not describe optional capability requirement: %#v", parameter)
				}
				if tool.fallback.Mode != "reject" || !strings.Contains(tool.fallback.Description, "unsupported") {
					t.Fatalf("explicit model must not silently fall back: %#v", tool.fallback)
				}
				return
			}
		}
	}
	t.Fatal("cron CLI manifest is missing model parameter")
}

func TestAgentCapabilityManifest_PerTurnModelAvailabilityWithoutSession(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent Agent
		want  CapabilityAvailabilityState
	}{
		{"no-interface", &stubAgent{}, CapabilityUnavailable},
		{"explicitly-unsupported", &manifestPerTurnModelAgent{}, CapabilityUnavailable},
		{"declared-support", &manifestPerTurnModelAgent{supported: true}, CapabilityAvailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEngine("demo", tc.agent, nil, "", LangEnglish)
			feature := findRuntimeFeature(t, e.agentRuntimeCapabilities().Capabilities, "per_turn_model")
			if feature.Availability.State != tc.want || feature.Availability.Reason == "" || feature.Availability.ReasonZH == "" {
				t.Fatalf("unexpected availability: %#v", feature.Availability)
			}
			if feature.Fallback.Mode != "reject" || feature.ReadOnly == nil || *feature.ReadOnly || len(feature.SideEffects) == 0 {
				t.Fatalf("missing truthful action/fallback contract: %#v", feature)
			}
			for _, effect := range feature.SideEffects {
				if effect.Kind == "configuration" {
					t.Fatal("task model must not claim project configuration mutation")
				}
			}
		})
	}
}
