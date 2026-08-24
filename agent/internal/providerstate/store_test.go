package providerstate

import (
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestStoreProviderSwitcherContract(t *testing.T) {
	var store Store
	providers := []core.ProviderConfig{
		{Name: "a", Model: "model-a"},
		{Name: "b", Model: "model-b", Models: []core.ModelOption{{Name: "b-1"}}},
	}
	store.SetProviders(providers)

	if store.GetActiveProvider() != nil {
		t.Fatal("zero-value store should not select a provider")
	}
	if store.SetActiveProvider("missing") {
		t.Fatal("unknown provider should be rejected")
	}
	if !store.SetActiveProvider("b") {
		t.Fatal("known provider should be selected")
	}
	if got := store.Model("fallback"); got != "model-b" {
		t.Fatalf("Model() = %q, want model-b", got)
	}
	if got := store.Models(); len(got) != 1 || got[0].Name != "b-1" {
		t.Fatalf("Models() = %v, want b-1", got)
	}

	listed := store.ListProviders()
	listed[1].Name = "mutated"
	if got := store.GetActiveProvider(); got == nil || got.Name != "b" {
		t.Fatalf("ListProviders exposed store slice: %v", got)
	}

	store.SetProviders([]core.ProviderConfig{{Name: "b", Model: "new-b"}})
	if got := store.Model("fallback"); got != "new-b" {
		t.Fatalf("active selection was not preserved by name: %q", got)
	}
	store.SetProviders([]core.ProviderConfig{{Name: "a"}})
	if store.GetActiveProvider() != nil {
		t.Fatal("removed active provider should clear the selection")
	}
	if !store.SetActiveProvider("") {
		t.Fatal("clearing provider should always succeed")
	}
}
