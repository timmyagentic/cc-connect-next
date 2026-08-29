package config

import (
	"strings"
	"testing"
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
	got := SearchCapabilities(CapabilityCatalog("v-test"), "隐藏思考")
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
