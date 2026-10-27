package codex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestAvailableModels_FallbackToModelsCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cache := `{
  "models": [
    {"slug":"gpt-5.4","description":"Latest frontier agentic coding model.","visibility":"list","supported_in_api":true},
    {"slug":"gpt-5.3-codex","description":"Great for coding","visibility":"list","supported_in_api":true},
    {"slug":"hidden-internal","visibility":"hidden","supported_in_api":true},
    {"slug":"tool-only","visibility":"list","supported_in_api":false}
  ]
}`
	if err := os.WriteFile(tmp+"/models_cache.json", []byte(cache), 0o600); err != nil {
		t.Fatalf("write models_cache.json: %v", err)
	}

	t.Setenv("CODEX_HOME", tmp)
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", srv.URL)

	a := &Agent{}
	models := a.AvailableModels(context.Background())
	if len(models) != 2 {
		t.Fatalf("models length = %d, want 2, models=%v", len(models), models)
	}
	if models[0].Name != "gpt-5.4" || models[1].Name != "gpt-5.3-codex" {
		t.Fatalf("models = %v, want [gpt-5.4 gpt-5.3-codex]", models)
	}
}

// TestAvailableModels_UsesModelCatalog tests that when CODEX_HOME/config.toml
// contains model_catalog_json, readCodexModelCatalog reads and parses that file
// when no explicit provider model list is configured.
func TestAvailableModels_UsesModelCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	tmp := t.TempDir()

	config := `model_catalog_json = "model_catalog.json"
`
	if err := os.WriteFile(tmp+"/config.toml", []byte(config), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// Mix of models: list-visible API-supported, hidden, non-API, and
	// one with an empty slug that falls back to display_name.
	catalog := `{
  "models": [
    {"slug":"gpt-5.4","display_name":"GPT-5.4","description":"Latest frontier agentic coding model.","visibility":"list","supported_in_api":true},
    {"slug":"gpt-5.3-codex","display_name":"GPT-5.3 Codex","description":"Great for coding","visibility":"list","supported_in_api":true},
    {"slug":"hidden-internal","visibility":"hidden","supported_in_api":true},
    {"slug":"tool-only","visibility":"list","supported_in_api":false},
    {"slug":"","display_name":"MissingSlug","visibility":"list","supported_in_api":true}
  ]
}`
	if err := os.WriteFile(tmp+"/model_catalog.json", []byte(catalog), 0o644); err != nil {
		t.Fatalf("write model_catalog.json: %v", err)
	}

	t.Setenv("CODEX_HOME", tmp)
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", srv.URL)

	a := &Agent{}
	models := a.AvailableModels(context.Background())

	// 3 visible+supported: gpt-5.4, gpt-5.3-codex, MissingSlug (slug empty → display_name fallback)
	if len(models) != 3 {
		t.Fatalf("models length = %d, want 3, models=%v", len(models), models)
	}
	if models[0].Name != "gpt-5.4" || models[0].Desc != "Latest frontier agentic coding model." {
		t.Errorf("models[0] = %+v, want gpt-5.4 with description", models[0])
	}
	if models[1].Name != "gpt-5.3-codex" || models[1].Desc != "Great for coding" {
		t.Errorf("models[1] = %+v, want gpt-5.3-codex with description", models[1])
	}
	if models[2].Name != "MissingSlug" || models[2].Desc != "" {
		t.Errorf("models[2] = %+v, want MissingSlug (display_name fallback)", models[2])
	}
}

// TestReadCodexModelCatalog_NoConfigFile tests graceful fallback when
// CODEX_HOME/config.toml does not exist.
func TestReadCodexModelCatalog_NoConfigFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", tmp)

	a := &Agent{}
	models := a.AvailableModels(context.Background())

	// No config.toml → no model_catalog.json → no models_cache.json
	// → no OPENAI_API_KEY → all the way to hardcoded fallback (6 models)
	if len(models) != 6 {
		t.Fatalf("expected 6 hardcoded fallback models, got %d: %v", len(models), models)
	}
}

func TestAvailableModels_ConfiguredListOverridesDesktopCatalog(t *testing.T) {
	home := t.TempDir()
	writeModelDiscoveryFixture(t, home, true, "desktop-model")
	t.Setenv("CODEX_HOME", home)
	want := []core.ModelOption{
		{Name: "custom-z", Alias: "First", Desc: "Custom provider model"},
		{Name: "custom-a", Alias: "Second"},
	}
	a := &Agent{}
	a.SetProviders([]core.ProviderConfig{{Name: "custom", Models: want}})
	a.SetActiveProvider("custom")
	if got := a.AvailableModels(context.Background()); !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %#v, want configured models in order %#v", got, want)
	}
}

func TestAvailableModels_DiscoveryUsesSessionHomePrecedence(t *testing.T) {
	for _, catalog := range []bool{true, false} {
		for _, source := range []string{"config", "provider", "session", "codex_home"} {
			name := source + "/cache"
			if catalog {
				name = source + "/catalog"
			}
			t.Run(name, func(t *testing.T) {
				desktop, bot := t.TempDir(), t.TempDir()
				writeModelDiscoveryFixture(t, desktop, true, "desktop-model")
				writeModelDiscoveryFixture(t, bot, catalog, "bot-model")
				t.Setenv("CODEX_HOME", desktop)
				t.Setenv("OPENAI_API_KEY", "")
				a := &Agent{configEnv: []string{"CODEX_HOME=" + bot}}
				if source != "config" {
					a.configEnv = []string{"CODEX_HOME=" + desktop}
					a.SetProviders([]core.ProviderConfig{{Name: "custom", Env: map[string]string{"CODEX_HOME": bot}}})
					a.SetActiveProvider("custom")
				}
				if source == "session" || source == "codex_home" {
					a.SetProviders([]core.ProviderConfig{{Name: "custom", Env: map[string]string{"CODEX_HOME": desktop}}})
					a.sessionEnv = []string{"CODEX_HOME=" + bot}
				}
				if source == "codex_home" {
					a.sessionEnv = []string{"CODEX_HOME=" + desktop}
					a.codexHome = bot
				}
				got := a.AvailableModels(context.Background())
				if len(got) != 1 || got[0].Name != "bot-model" {
					t.Fatalf("models = %#v, want only bot-model", got)
				}
			})
		}
	}
}

func writeModelDiscoveryFixture(t *testing.T, home string, catalog bool, model string) {
	t.Helper()
	name := "models_cache.json"
	if catalog {
		name = "catalog.json"
		if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("model_catalog_json = \"catalog.json\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data := `{"models":[{"slug":"` + model + `","visibility":"list","supported_in_api":true}]}`
	if err := os.WriteFile(filepath.Join(home, name), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
