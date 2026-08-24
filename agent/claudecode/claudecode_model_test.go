package claudecode

import (
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestConfiguredModels_BoundaryConditions(t *testing.T) {
	a := &Agent{}
	a.SetProviders([]core.ProviderConfig{
		{Name: "first", Models: []core.ModelOption{{Name: "first"}}},
		{Name: "second", Models: []core.ModelOption{{Name: "second"}}},
	})
	if got := a.configuredModels(); got != nil {
		t.Fatalf("configuredModels() without active provider = %v, want nil", got)
	}
	if !a.SetActiveProvider("second") {
		t.Fatal("SetActiveProvider(second) = false")
	}
	if got := a.configuredModels(); len(got) != 1 || got[0].Name != "second" {
		t.Fatalf("configuredModels() = %v, want second", got)
	}
}

func TestGetModel_PrefersActiveProviderModel(t *testing.T) {
	a := &Agent{model: "sonnet"}
	a.SetProviders([]core.ProviderConfig{{Name: "anthropic", Model: "opus"}})
	a.SetActiveProvider("anthropic")

	if got := a.GetModel(); got != "opus" {
		t.Fatalf("GetModel() = %q, want opus", got)
	}
}
