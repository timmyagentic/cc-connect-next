package core

import (
	"reflect"
	"testing"
)

type schemaStubAgent struct {
	stubAgent
	known []string
}

func (a *schemaStubAgent) KnownOptionKeys() []string { return a.known }

func TestUnknownAgentOptionKeys_DiffsAgainstSchema(t *testing.T) {
	agent := &schemaStubAgent{known: []string{"work_dir", "model", "Backend"}}
	got := UnknownAgentOptionKeys([]string{"work_dir", "service_tier", "BACKEND", "sparkle_mode"}, agent)
	want := []string{"service_tier", "sparkle_mode"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnknownAgentOptionKeys = %v, want %v (case-insensitive match)", got, want)
	}
}

func TestUnknownAgentOptionKeys_BootstrapKeysAreConsumed(t *testing.T) {
	agent := &schemaStubAgent{known: []string{"work_dir"}}
	if got := UnknownAgentOptionKeys([]string{"provider", "work_dir"}, agent); got != nil {
		t.Errorf("provider is consumed by bootstrap, got %v", got)
	}
}

func TestUnknownAgentOptionKeys_NoSchemaMeansNoClaims(t *testing.T) {
	// An agent without a declared schema must not flag anything: no
	// declaration means no claim, not "everything is unknown".
	if got := UnknownAgentOptionKeys([]string{"whatever"}, &stubAgent{}); got != nil {
		t.Errorf("agent without schema must yield nil, got %v", got)
	}
}
