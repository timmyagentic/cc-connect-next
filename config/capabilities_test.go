package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestCapabilityCatalog_CoversTypedConfigurationWithDescriptions(t *testing.T) {
	catalog := CapabilityCatalog("v-test")
	byPath := make(map[string]bool, len(catalog.Options))
	seen := make(map[string]bool, len(catalog.Options))
	for _, option := range catalog.Options {
		byPath[option.Path] = true
		identity := option.Path + "\x00" + option.Owner
		if seen[identity] {
			t.Errorf("duplicate catalog option %s owner=%s", option.Path, option.Owner)
		}
		seen[identity] = true
		if option.Description == "" || option.DescriptionZH == "" {
			t.Errorf("%s missing bilingual description", option.Path)
		}
		if strings.HasPrefix(option.Description, "Configure ") {
			t.Errorf("%s still has a generic description: %s", option.Path, option.Description)
		}
		if option.Type == "" || option.ApplyMode == "" {
			t.Errorf("%s missing type/apply metadata", option.Path)
		}
	}
	for _, path := range []string{
		"language", "display.reply_footer", "queue.busy_message_mode",
		"projects.agent.type", "projects.agent.answer_profiles.fast.service_tier",
		"projects.platforms.type", "projects.users.roles.<name>.user_ids",
		"speech.openai.api_key", "tts.minimax.config_file", "bridge.insecure",
	} {
		if !byPath[path] {
			t.Errorf("catalog missing typed config path %q", path)
		}
	}
	if len(catalog.Capabilities) < 10 {
		t.Fatalf("capability groups = %d, want a useful intent index", len(catalog.Capabilities))
	}
	for _, capability := range catalog.Capabilities {
		for _, path := range capability.Paths {
			if strings.Contains(path, ".options.") {
				// Adapter schemas are build-tag registrations and are validated
				// by the cmd package, where compiled plugins are imported.
				continue
			}
			if !byPath[path] {
				t.Errorf("capability %s references unknown path %s", capability.ID, path)
			}
		}
	}
}

func TestCapabilityCatalog_NaturalLanguageSearchFindsMessageDisplay(t *testing.T) {
	got := core.SearchConfigCatalog(CapabilityCatalog("v-test"), "隐藏思考")
	if len(got.Options) == 0 {
		t.Fatal("search returned no options")
	}
	var paths []string
	for _, option := range got.Options {
		paths = append(paths, option.Path)
	}
	if joined := strings.Join(paths, ","); !strings.Contains(joined, "display.thinking_messages") {
		t.Fatalf("search paths = %s", joined)
	}
}

func TestCapabilityCatalog_TokenBudgetIsNotSensitive(t *testing.T) {
	catalog := CapabilityCatalog("v-test")
	wantSensitive := map[string]bool{
		"projects.auto_compress.max_tokens": false,
		"bridge.token":                      true,
		"management.token":                  true,
		"speech.openai.api_key":             true,
		"webhook.token":                     true,
	}
	seen := make(map[string]bool, len(wantSensitive))
	for _, option := range catalog.Options {
		want, ok := wantSensitive[option.Path]
		if !ok || option.Owner != "" {
			continue
		}
		seen[option.Path] = true
		if option.Sensitive != want {
			t.Errorf("%s sensitive = %t, want %t", option.Path, option.Sensitive, want)
		}
	}
	for path := range wantSensitive {
		if !seen[path] {
			t.Errorf("catalog option %s not found", path)
		}
	}
}

func TestCapabilityCatalog_CoversOperationalEnvironmentOverrides(t *testing.T) {
	catalog := CapabilityCatalog("v-test")
	byPath := make(map[string]core.ConfigOption)
	for _, option := range catalog.Options {
		byPath[option.Path] = option
	}
	for _, path := range []string{
		"CC_LOG_FILE", "CC_LOG_MAX_SIZE", "CC_LOG_MAX_BACKUPS", "CC_MAX_ATTACHMENT_SIZE_MB",
		"CC_DAEMON_NO_CAPTURE_SECRETS", "CC_NEXT_ALLOW_OFFICIAL_CONFLICT", "CC_DATA_DIR",
		"CC_PROJECT", "CC_SESSION_KEY",
	} {
		option, ok := byPath[path]
		if !ok {
			t.Errorf("catalog missing environment override %s", path)
			continue
		}
		if option.Source != core.ConfigSourceEnvironment || option.Example == "" {
			t.Errorf("environment option %s contract = %#v", path, option)
		}
	}
	for _, path := range []string{"--config", "--log-max-size", "--log-max-backups", "daemon install --config", "daemon install --work-dir", "daemon install --log-max-size", "daemon install --log-file", "daemon install --no-capture-secrets"} {
		option, ok := byPath[path]
		if !ok || option.Source != core.ConfigSourceCLI || option.Placement != "command line" {
			t.Errorf("CLI contract %s = %#v", path, option)
		}
	}
}

func TestFilterUnavailableOwnedOptionsHonorsCompiledAdapters(t *testing.T) {
	options := []core.ConfigOption{
		{Path: "global", Scope: core.ConfigScopeGlobal},
		{Path: "codex", Scope: core.ConfigScopeAgent, Owner: "codex"},
		{Path: "pi", Scope: core.ConfigScopeAgent, Owner: "pi"},
		{Path: "feishu", Scope: core.ConfigScopePlatform, Owner: "feishu"},
		{Path: "matrix", Scope: core.ConfigScopePlatform, Owner: "matrix"},
	}
	got := filterUnavailableOwnedOptions(options, map[string][]core.ConfigOption{"codex": nil}, map[string][]core.ConfigOption{"feishu": nil})
	var paths []string
	for _, option := range got {
		paths = append(paths, option.Path)
	}
	if strings.Join(paths, ",") != "global,codex,feishu" {
		t.Fatalf("filtered paths = %v", paths)
	}
}

func TestCapabilityCatalog_PublicContractsAreActionable(t *testing.T) {
	catalog := CapabilityCatalog("v-test")
	for _, option := range catalog.Options {
		if option.Internal {
			continue
		}
		identity := option.Path
		if option.Owner != "" {
			identity += " owner=" + option.Owner
		}
		if option.Requirement == "" {
			t.Errorf("%s missing requirement metadata", identity)
		}
		if option.DefaultSource == "" {
			t.Errorf("%s missing default-source metadata", identity)
		}
		if option.Placement == "" {
			t.Errorf("%s missing placement metadata", identity)
		}
		if option.Example == "" {
			t.Errorf("%s missing a TOML assignment example", identity)
			continue
		}
		if option.Source == core.ConfigSourceTOML {
			var decoded map[string]any
			if _, err := toml.Decode(option.Example, &decoded); err != nil {
				t.Errorf("%s example is not valid TOML: %q: %v", identity, option.Example, err)
			}
		}
		if option.Default == "unset / runtime default" || option.Default == "unset / adapter default" {
			t.Errorf("%s still uses ambiguous default %q", identity, option.Default)
		}
	}
}

func TestCapabilityCatalog_EncodesTypedValidationContract(t *testing.T) {
	catalog := CapabilityCatalog("v-test")
	find := func(path string) core.ConfigOption {
		t.Helper()
		for _, option := range catalog.Options {
			if option.Owner == "" && option.Path == path {
				return option
			}
		}
		t.Fatalf("catalog option %q not found", path)
		return core.ConfigOption{}
	}

	for _, path := range []string{"projects.name", "projects.agent.type", "projects.platforms.type"} {
		if got := find(path).Requirement; got != core.ConfigRequirementRequired {
			t.Errorf("%s requirement = %q, want required", path, got)
		}
	}
	baseDir := find("projects.base_dir")
	if baseDir.Requirement != core.ConfigRequirementConditional || !strings.Contains(strings.Join(baseDir.RequiredWhen, " "), "multi-workspace") {
		t.Errorf("base_dir contract = %#v", baseDir)
	}
	if !strings.Contains(strings.Join(baseDir.ConflictsWith, " "), "projects.agent.options.work_dir") {
		t.Errorf("base_dir conflict contract = %#v", baseDir)
	}
	for path, want := range map[string][]string{
		"projects.references.display_path":                     {"absolute", "relative", "basename", "dirname_basename", "smart"},
		"projects.references.marker_style":                     {"none", "ascii", "emoji"},
		"projects.references.enclosure_style":                  {"none", "bracket", "angle", "fullwidth", "code"},
		"projects.agent.answer_profiles.fast.reasoning_effort": {"low", "medium", "high", "xhigh", "max"},
	} {
		option := find(path)
		for _, value := range want {
			if !containsTestString(option.Values, value) {
				t.Errorf("%s allowed values %v missing %q", path, option.Values, value)
			}
		}
	}
	if got := find("providers.base_url").ApplyMode; got != core.ConfigApplyReload {
		t.Errorf("global provider apply mode = %q, want reload", got)
	}
	language := find("language")
	if !language.OpenValues || language.ClosedValues {
		t.Errorf("language alias contract = open=%t closed=%t", language.OpenValues, language.ClosedValues)
	}
	for path, want := range map[string]string{
		"commands.name":          "[[commands]]",
		"providers.models.model": "[[providers.models]]",
		"projects.display.mode":  "[projects.display]",
	} {
		if got := find(path).Placement; !strings.Contains(got, want) {
			t.Errorf("%s placement = %q, want contains %q", path, got, want)
		}
	}
}

func containsTestString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
