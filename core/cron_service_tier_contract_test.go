package core

import (
	"strings"
	"testing"
)

func TestCronServiceTierCapabilityContract(t *testing.T) {
	e := newTestEngine()
	found := false
	for _, tool := range e.agentToolCapabilities(capabilitySnapshot{}) {
		if tool.ID != "cron" {
			continue
		}
		for _, p := range tool.Parameters {
			if p.Name != "service_tier" {
				continue
			}
			found = true
			if p.Type != "string" || p.Required || len(p.AllowedValues) != 0 {
				t.Fatalf("service tier must be optional and model-catalog scoped: %#v", p)
			}
			for _, word := range []string{"prompt-only", "catalog", "priority", "empty string", "inherit", "TurnOptionsSession"} {
				if !strings.Contains(p.Description, word) {
					t.Errorf("missing parameter boundary %q", word)
				}
			}
			if p.DescriptionZH == "" {
				t.Fatal("missing Chinese contract")
			}
		}
	}
	if !found {
		t.Fatal("cron service_tier parameter missing")
	}
	for _, c := range e.agentRuntimeCapabilities().Capabilities {
		if c.ID != "per_turn_service_tier" {
			continue
		}
		if c.Availability.State != CapabilityConditional || c.ReadOnly == nil || *c.ReadOnly || c.Permission != CapabilityPermissionLocalAgent || c.Fallback.Mode != "reject" {
			t.Fatalf("invalid runtime contract: %#v", c)
		}
		for _, word := range []string{"inherit", "Shell", "unsupported", "invalid"} {
			if !strings.Contains(c.Fallback.Description, word) {
				t.Errorf("missing fallback %q", word)
			}
		}
		return
	}
	t.Fatal("per_turn_service_tier runtime feature missing")
}
