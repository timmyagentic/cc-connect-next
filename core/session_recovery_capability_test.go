package core

import (
	"encoding/json"
	"strings"
	"testing"
)

type recoveryCapabilitySession struct {
	AgentSession
	local bool
}

func (s recoveryCapabilitySession) SupportsAutomaticSessionRecovery() bool { return s.local }

func TestAutomaticSessionRecoveryManifestContract(t *testing.T) {
	e := newTestEngine()
	for _, tc := range []struct {
		name    string
		session AgentSession
		want    CapabilityAvailabilityState
	}{
		{"unsupported", newControllableSession("test"), CapabilityUnavailable},
		{"remote", recoveryCapabilitySession{local: false}, CapabilityUnavailable},
		{"local", recoveryCapabilitySession{local: true}, CapabilityConditional},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := e.sessionRuntimeCapabilities(tc.session)
			var found bool
			for _, c := range runtime.Capabilities {
				if c.ID != "automatic_session_recovery" {
					continue
				}
				found = true
				if c.Availability.State != tc.want || c.Permission != CapabilityPermissionConditional || c.ReadOnly == nil || *c.ReadOnly || len(c.SideEffects) != 4 || c.Fallback.Mode != "fail-closed" {
					t.Fatalf("incomplete recovery contract: %#v", c)
				}
				if c.DescriptionZH == "" || c.Fallback.DescriptionZH == "" || !strings.Contains(c.Description, "once per user Send") || !strings.Contains(c.Fallback.Description, "persistence") {
					t.Fatalf("missing recovery boundaries: %#v", c)
				}
				encoded, _ := json.Marshal(c)
				if strings.Contains(string(encoded), "/sessions/") {
					t.Fatal("manifest leaked history path")
				}
			}
			if !found {
				t.Fatal("missing automatic_session_recovery")
			}
		})
	}
	unbound := e.unboundSessionRuntimeCapabilities()
	for _, c := range unbound.Capabilities {
		if c.ID == "automatic_session_recovery" {
			if c.Availability.State != CapabilityConditional {
				t.Fatal(c)
			}
			return
		}
	}
	t.Fatal("unbound recovery contract missing")
}
