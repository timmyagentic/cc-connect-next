package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAgentCapabilityManifest_CronReasoningEffortContract(t *testing.T) {
	for _, tool := range agentToolDefinitions {
		if tool.id != "cron" {
			continue
		}
		for _, parameter := range tool.parameters {
			if parameter.Name != "reasoning_effort" {
				continue
			}
			if parameter.Required || parameter.Type != "string" || parameter.DescriptionZH == "" || len(parameter.AllowedValues) != 0 {
				t.Fatalf("effort must be optional, bilingual, and server-validated: %#v", parameter)
			}
			for _, term := range []string{"prompt jobs only", "--reasoning-effort", "without model", "validated by the server", "empty string", "inheritance", "defaults"} {
				if !strings.Contains(parameter.Description, term) {
					t.Errorf("effort contract missing %q: %#v", term, parameter)
				}
			}
			if tool.fallback.Mode != "reject" || !strings.Contains(tool.fallback.Description, "reasoning efforts") || tool.fallback.DescriptionZH == "" {
				t.Fatalf("effort must not silently fall back: %#v", tool.fallback)
			}
			return
		}
	}
	t.Fatal("cron CLI manifest is missing reasoning_effort parameter")
}

type manifestPerTurnReasoningAgent struct {
	stubAgent
	supported bool
	values    []string
}

func (a *manifestPerTurnReasoningAgent) SupportsPerTurnReasoningEffort() bool { return a.supported }
func (a *manifestPerTurnReasoningAgent) SetReasoningEffort(string)            {}
func (a *manifestPerTurnReasoningAgent) GetReasoningEffort() string           { return "" }
func (a *manifestPerTurnReasoningAgent) AvailableReasoningEfforts() []string  { return a.values }

type manifestReasoningSupportOnlyAgent struct{ stubAgent }

func (*manifestReasoningSupportOnlyAgent) SupportsPerTurnReasoningEffort() bool { return true }

func TestAgentCapabilityManifest_PerTurnReasoningEffortWithoutSession(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent Agent
		want  CapabilityAvailabilityState
	}{
		{"no-interface", &stubAgent{}, CapabilityUnavailable},
		{"explicitly-unsupported", &manifestPerTurnReasoningAgent{}, CapabilityUnavailable},
		{"missing-catalog-interface", &manifestReasoningSupportOnlyAgent{}, CapabilityUnavailable},
		{"empty-catalog", &manifestPerTurnReasoningAgent{supported: true}, CapabilityUnavailable},
		{"declared-support", &manifestPerTurnReasoningAgent{supported: true, values: []string{"effort-a"}}, CapabilityAvailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEngine("demo", tc.agent, nil, "", LangEnglish)
			feature := findRuntimeFeature(t, e.agentRuntimeCapabilities().Capabilities, "per_turn_reasoning_effort")
			if feature.Availability.State != tc.want || feature.Availability.Reason == "" || feature.Availability.ReasonZH == "" {
				t.Fatalf("unexpected availability: %#v", feature.Availability)
			}
			if feature.Fallback.Mode != "reject" || feature.ReadOnly == nil || *feature.ReadOnly || feature.Permission != CapabilityPermissionLocalAgent || len(feature.SideEffects) == 0 {
				t.Fatalf("missing action/fallback contract: %#v", feature)
			}
			for _, effect := range feature.SideEffects {
				if effect.Kind == "configuration" {
					t.Fatal("task effort must not claim project configuration mutation")
				}
			}
		})
	}
}

func TestAgentCapabilityManifest_CronReasoningEffortAllowedValues(t *testing.T) {
	values := func(agent Agent) []string {
		t.Helper()
		e := NewEngine("demo", agent, nil, "", LangEnglish)
		for _, tool := range e.agentToolCapabilities(capabilitySnapshot{}) {
			if tool.ID == "cron" {
				for _, parameter := range tool.Parameters {
					if parameter.Name == "reasoning_effort" {
						return parameter.AllowedValues
					}
				}
			}
		}
		t.Fatal("missing reasoning_effort parameter")
		return nil
	}
	agent := &manifestPerTurnReasoningAgent{supported: true, values: []string{"effort-a", "effort-b"}}
	got := values(agent)
	if !reflect.DeepEqual(got, agent.values) {
		t.Fatalf("allowed values = %#v", got)
	}
	got[0] = "mutated"
	if agent.values[0] != "effort-a" {
		t.Fatal("manifest values must not alias Agent state")
	}
	for _, agent := range []Agent{&stubAgent{}, &manifestPerTurnReasoningAgent{values: []string{"effort-a"}}} {
		if got := values(agent); len(got) != 0 {
			t.Fatalf("unsupported Agent must not inherit another Agent's values: %#v", got)
		}
	}
}

func TestAgentCapabilityManifest_CronReasoningEffortDiscovery(t *testing.T) {
	brief := BuildAgentCapabilityBrief(ConfigCatalog{}, "stub", nil)
	if !strings.Contains(brief, "cc-connect-next capabilities --search") {
		t.Fatal("capability brief must direct agents to the searchable manifest")
	}
	e := NewEngine("demo", &stubAgent{}, nil, "", LangEnglish)
	manifest := AgentCapabilityManifest{Tools: e.agentToolCapabilities(capabilitySnapshot{})}
	for _, query := range []string{"cron reasoning effort", "reasoning_effort", "推理强度"} {
		result := SearchAgentCapabilityManifest(manifest, query)
		found := false
		for _, tool := range result.Tools {
			if tool.ID == "cron" {
				found = true
			}
		}
		if !found {
			t.Fatalf("brief-directed search %q must expose cron", query)
		}
		data, err := json.Marshal(result)
		if err != nil || !strings.Contains(string(data), `"name":"reasoning_effort"`) {
			t.Fatalf("JSON manifest missing effort: %s, %v", data, err)
		}
		for _, language := range []string{"en", "zh"} {
			markdown := RenderAgentCapabilityManifestMarkdown(result, language)
			for _, term := range []string{"`reasoning_effort`", "--reasoning-effort", "model"} {
				if !strings.Contains(markdown, term) {
					t.Errorf("%s manifest missing %q: %s", language, term, markdown)
				}
			}
		}
	}
}
