package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func loadOwnerEscalationFixture(t *testing.T, block string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	text := `[[projects]]
name = "demo"
[projects.agent]
type = "codex"
` + block
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return LoadPermissive(path)
}

func TestOwnerEscalationConfigOptInAndEnvironment(t *testing.T) {
	cfg, err := loadOwnerEscalationFixture(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Projects[0].OwnerEscalation.Enabled {
		t.Fatal("existing projects must not opt into owner mentions")
	}
	t.Setenv("OWNER_ESCALATION_TEST_KEY", "fixture-secret")
	cfg, err = loadOwnerEscalationFixture(t, `[projects.owner_escalation]
enabled = true
owner_id = "ou_owner"
owner_name = "Owner"
api_key = "${OWNER_ESCALATION_TEST_KEY}"
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.UnknownConfigKeys) != 0 {
		t.Fatalf("supported options reported as unknown: %v", cfg.UnknownConfigKeys)
	}
	resolved, err := core.NormalizeOwnerEscalationConfig(cfg.Projects[0].OwnerEscalation)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.APIKey != "fixture-secret" || resolved.BaseURL != "https://api.openai.com/v1" || resolved.Model != "gpt-6-luna" || resolved.TimeoutSeconds != 20 {
		t.Fatal("environment resolution or runtime defaults drifted")
	}
}

func TestOwnerEscalationConfigRejectsInvalidTargetsAndCredentials(t *testing.T) {
	valid := `[projects.owner_escalation]
enabled = true
owner_id = "ou_owner"
owner_name = "Owner"
base_url = "https://api.openai.com/v1"
api_key = "fixture-secret"
model = "gpt-6-luna"
timeout_seconds = 20
`
	for _, tc := range []struct{ name, old, replacement string }{
		{"missing target", `owner_id = "ou_owner"`, `owner_id = ""`},
		{"wildcard target", `owner_id = "ou_owner"`, `owner_id = "*"`},
		{"recipient list", `owner_id = "ou_owner"`, `owner_id = "ou_a,ou_b"`},
		{"spaced target", `owner_id = "ou_owner"`, `owner_id = "ou_a ou_b"`},
		{"missing owner name", `owner_name = "Owner"`, `owner_name = ""`},
		{"missing credential", `api_key = "fixture-secret"`, `api_key = ""`},
		{"remote plaintext", `https://api.openai.com/v1`, `http://example.com/v1`},
		{"credential in url", `https://api.openai.com/v1`, `https://user:fixture-secret@example.com/v1`},
		{"url query", `https://api.openai.com/v1`, `https://example.com/v1?key=fixture-secret`},
		{"negative timeout", `timeout_seconds = 20`, `timeout_seconds = -1`},
		{"excessive timeout", `timeout_seconds = 20`, `timeout_seconds = 121`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadOwnerEscalationFixture(t, strings.Replace(valid, tc.old, tc.replacement, 1))
			if err == nil || !strings.Contains(err.Error(), "owner_escalation") {
				t.Fatalf("want contextual validation error, got %v", err)
			}
			if strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal("validation error leaked credentials")
			}
		})
	}
}

func TestOwnerEscalationConfigRejectsWrongTOMLTypes(t *testing.T) {
	for _, field := range []string{
		`enabled = "true"`, `owner_id = ["ou_owner"]`, `owner_name = 42`,
		`base_url = true`, `api_key = 42`, `model = ["gpt-6-luna"]`, `timeout_seconds = "20"`,
	} {
		t.Run(field, func(t *testing.T) {
			if _, err := loadOwnerEscalationFixture(t, "[projects.owner_escalation]\n"+field); err == nil {
				t.Fatal("wrong typed option was silently accepted")
			}
		})
	}
}

func TestOwnerEscalationConfigCatalog(t *testing.T) {
	catalog := CapabilityCatalog("test")
	var count int
	for _, option := range catalog.Options {
		if !strings.HasPrefix(option.Path, "projects.owner_escalation.") {
			continue
		}
		count++
		if option.ApplyMode != core.ConfigApplyRestart || option.Description == "" || option.DescriptionZH == "" {
			t.Fatalf("incomplete restart-only metadata for %s", option.Path)
		}
		if option.Sensitive != (option.Key == "api_key") {
			t.Fatalf("incorrect sensitivity metadata for %s", option.Path)
		}
	}
	if count != 7 {
		t.Fatalf("owner escalation options = %d, want 7", count)
	}
	for _, query := range []string{"无法解决", "转人工", "原话题@负责人", "Luna"} {
		result := core.SearchConfigCatalog(catalog, query)
		found := false
		for _, option := range result.Options {
			found = found || option.Path == "projects.owner_escalation.enabled"
		}
		if !found {
			t.Errorf("natural-language query %q did not find owner escalation", query)
		}
	}
}
