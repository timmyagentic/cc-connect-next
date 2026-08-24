package core

import "testing"

func TestSetProviderModel(t *testing.T) {
	providers := []ProviderConfig{
		{Name: "openai", Model: "gpt-4.1"},
		{Name: "backup", Model: "gpt-4.1-mini"},
	}

	updated, ok := SetProviderModel(providers, "openai", "gpt-5.4")
	if !ok {
		t.Fatal("SetProviderModel() did not find existing provider")
	}
	if updated[0].Model != "gpt-5.4" {
		t.Fatalf("updated provider model = %q, want gpt-5.4", updated[0].Model)
	}
	if providers[0].Model != "gpt-4.1" {
		t.Fatalf("original providers mutated = %q, want gpt-4.1", providers[0].Model)
	}

	updated, ok = SetProviderModel(providers, "missing", "gpt-5.4")
	if ok {
		t.Fatal("SetProviderModel() unexpectedly found missing provider")
	}
	if updated[0].Model != providers[0].Model {
		t.Fatalf("missing provider should leave copy unchanged, got %q want %q", updated[0].Model, providers[0].Model)
	}
}
