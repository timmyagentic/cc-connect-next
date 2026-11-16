package core

import (
	"strings"
	"testing"
)

func TestCronBackendCapabilityContract(t *testing.T) {
	e := newTestEngine()
	found := false
	for _, tool := range e.agentToolCapabilities(capabilitySnapshot{}) {
		if tool.ID != "cron" {
			continue
		}
		for _, parameter := range tool.Parameters {
			if parameter.Name != "backend" {
				continue
			}
			found = true
			if parameter.Type != "string" || parameter.Required {
				t.Fatalf("backend must be optional: %#v", parameter)
			}
			for _, term := range []string{"--backend", "exec", "app_server", "new_per_run", "empty string", "inherit"} {
				if !strings.Contains(parameter.Description, term) {
					t.Errorf("missing backend contract %q", term)
				}
			}
			if parameter.DescriptionZH == "" {
				t.Fatal("missing Chinese contract")
			}
		}
	}
	if !found {
		t.Fatal("cron backend parameter missing")
	}
	for _, capability := range e.agentRuntimeCapabilities().Capabilities {
		if capability.ID == "cron_backend" {
			if capability.Availability.State != CapabilityUnavailable || capability.ReadOnly == nil || *capability.ReadOnly || capability.Fallback.Mode != "reject" {
				t.Fatalf("invalid unsupported-backend contract: %#v", capability)
			}
			return
		}
	}
	t.Fatal("cron backend runtime capability missing")
}
