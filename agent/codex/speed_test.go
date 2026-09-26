package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestSpeedLiveModelCatalogPaginationAndErrors(t *testing.T) {
	pages := []string{
		`{"data":[{"model":"other","serviceTiers":[]}],"nextCursor":"page-2"}`,
		`{"data":[{"model":"native-model","defaultServiceTier":"priority","serviceTiers":[{"id":"priority","name":"Fast","description":"faster"}]}],"nextCursor":null}`,
	}
	index := 0
	caps, err := readAppServerServiceTiers("native-model", "", func(params map[string]any, out any) error {
		if params["includeHidden"] != true {
			t.Fatal("current hidden models must remain queryable")
		}
		if index == 1 && params["cursor"] != "page-2" {
			t.Fatalf("cursor = %#v", params)
		}
		data := pages[index]
		index++
		return json.Unmarshal([]byte(data), out)
	})
	if err != nil || caps.Default != "priority" || len(caps.Options) != 2 || caps.Options[1].Name != "fast" || caps.Options[1].Value != "priority" {
		t.Fatalf("live catalog = %+v, %v", caps, err)
	}
	_, err = readAppServerServiceTiers("native-model", "", func(map[string]any, any) error { return errors.New("permission denied") })
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("catalog permission failure = %v", err)
	}
	index = 0
	_, err = readAppServerServiceTiers("missing", "", func(_ map[string]any, out any) error {
		index++
		return json.Unmarshal([]byte(`{"data":[],"nextCursor":"same-page"}`), out)
	})
	if err == nil || index != 2 {
		t.Fatalf("repeated cursor must stop without inventing support: %d, %v", index, err)
	}
}

func TestServiceTierCatalogCapabilities(t *testing.T) {
	for _, test := range []struct {
		name, metadata string
		want           []core.ServiceTierOption
	}{
		{"modern", `"service_tiers":[{"id":"priority"},{"id":"flex"},{"id":"priority"}]`, []core.ServiceTierOption{{Name: "default", Value: "default"}, {Name: "fast", Value: "priority"}, {Name: "flex", Value: "flex"}}},
		{"legacy", `"additional_speed_tiers":["fast"]`, []core.ServiceTierOption{{Name: "default", Value: "default"}, {Name: "fast", Value: "fast"}}},
		{"modern takes precedence", `"service_tiers":[],"additional_speed_tiers":["fast"]`, []core.ServiceTierOption{{Name: "default", Value: "default"}}},
		{"no fast support", `"service_tiers":[]`, []core.ServiceTierOption{{Name: "default", Value: "default"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{"models":[{"slug":"model-a",%s}]}`, test.metadata))
			got, err := parseServiceTierCapabilities(data, "model-a", "default")
			if err != nil || !reflect.DeepEqual(got.Options, test.want) {
				t.Fatalf("capabilities = %+v, %v", got, err)
			}
			if _, err := parseServiceTierCapabilities(data, "missing", "default"); err == nil {
				t.Fatal("unknown model must not inherit another model's tiers")
			}
		})
	}
}

func TestServiceTierCatalogUsesEffectiveHomeAndNativeDefaults(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("model = 'model-a'\nservice_tier = 'priority'\nmodel_catalog_json = 'catalog.json'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "catalog.json"), []byte(`{"models":[{"slug":"model-a","service_tiers":[{"id":"priority"}]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// The process-level home intentionally has no catalog. Never fall back to it.
	t.Setenv("CODEX_HOME", t.TempDir())
	for _, a := range []*Agent{
		{codexHome: home, appServerURL: "stdio://"},
		{configEnv: []string{"CODEX_HOME=" + home}},
		{configEnv: []string{"CODEX_HOME=missing"}, sessionEnv: []string{"CODEX_HOME=" + home}},
	} {
		got, err := a.ServiceTierCapabilities("")
		if err != nil || got.Model != "model-a" || got.Default != "priority" || len(got.Options) != 2 {
			t.Fatalf("capabilities = %+v, %v", got, err)
		}
	}
	a := &Agent{codexHome: home, serviceTier: "default"}
	got, err := a.ServiceTierCapabilities("model-a")
	if err != nil || got.Default != "default" {
		t.Fatalf("explicit default = %+v, %v", got, err)
	}
	if err := os.Remove(filepath.Join(home, "catalog.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ServiceTierCapabilities("model-a"); err == nil {
		t.Fatal("missing authoritative catalog must be rejected")
	}
}

func TestServiceTierCatalogRejectsUnverifiableConfiguration(t *testing.T) {
	for _, a := range []*Agent{
		{appServerURL: "ws://127.0.0.1:9999"},
		{cliExtraArgs: []string{"--profile", "another-account"}},
		{codexHome: t.TempDir(), model: "unknown"},
	} {
		if _, err := a.ServiceTierCapabilities(""); err == nil {
			t.Fatal("unverifiable capability reported success")
		}
	}
}

func TestSpeedTurnDefaultsPreserveNativeModelEffortAndTier(t *testing.T) {
	s := &appServerSession{model: "native-model", effort: "high", serviceTier: "priority"}
	s.storeActiveTurnOptions(&core.TurnOptions{Model: "one-shot", ReasoningEffort: "low", ServiceTier: "default"})
	got := s.DefaultTurnOptions()
	if got.Model != "native-model" || got.ReasoningEffort != "high" || got.ServiceTier != "priority" {
		t.Fatalf("startup defaults polluted by profile: %+v", got)
	}
	for _, tier := range []string{"priority", "default"} {
		got.ServiceTier = tier
		params := s.turnStartParams("thread-1", nil, &got)
		if params["model"] != "native-model" || params["effort"] != "high" || params["serviceTier"] != tier {
			t.Fatalf("speed RPC params = %#v", params)
		}
		execSession := &codexSession{mode: "suggest", workDir: t.TempDir()}
		args := strings.Join(execSession.launchArgsWithTurnOptions("task", nil, &got), " ")
		for _, want := range []string{"native-model", `model_reasoning_effort="high"`, fmt.Sprintf(`service_tier=%q`, tier)} {
			if !strings.Contains(args, want) {
				t.Fatalf("exec args %q omit %q", args, want)
			}
		}
	}
}
