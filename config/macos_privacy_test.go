package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestMacOSPrivacyConfigDefaultsAndValidation(t *testing.T) {
	var cfg Config
	if _, err := toml.Decode("[macos_privacy]\nallow_directories = [\"relative\"]", &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.validateInternal(true); err == nil || !strings.Contains(err.Error(), "macos_privacy.allow_directories") {
		t.Fatal("unknown privacy directory accepted")
	}
	if cfg.MacOSPrivacy.Enabled != nil {
		t.Fatal("omission must retain the default-enabled policy")
	}
	if _, err := toml.Decode("[macos_privacy]\nenabled = \"false\"", &cfg); err == nil {
		t.Fatal("wrong enabled type accepted")
	}
	cfg = Config{}
	if _, err := toml.Decode("[macos_privacy]\nenabled = false\nallow_directories = [\"relative\"]", &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Projects = []ProjectConfig{{Name: "privacy-test", Agent: AgentConfig{Type: "codex"}}}
	if err := cfg.validateInternal(true); err != nil {
		t.Fatal("disabled privacy settings should not prevent loading:", err)
	}
}

func TestMacOSPrivacyIsDiscoverableWithoutPublishingConfiguredPaths(t *testing.T) {
	results := core.SearchConfigCatalog(CapabilityCatalog("test"), "隐私弹窗")
	for _, option := range results.Options {
		if option.Path == "macos_privacy.enabled" {
			if option.Default != "true" || option.ApplyMode != core.ConfigApplyRestart {
				t.Fatalf("wrong privacy default/apply contract: %+v", option)
			}
			return
		}
	}
	t.Fatal("privacy policy is missing from capability search")
}
