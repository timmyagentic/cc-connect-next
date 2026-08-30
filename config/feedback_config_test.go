package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Unknown TOML keys used to be silently ignored; they now surface via
// UnknownConfigKeys so the feedback channel can prompt the user.
func TestLoad_RecordsUnknownConfigKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[log]
level = "info"
sparkle = true

[feedbak]
enabled = true

[[projects]]
name = "demo"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "` + dir + `"
totally_made_up_option = "ignored-by-design"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "t"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadPermissive(path)
	if err != nil {
		t.Fatalf("LoadPermissive: %v", err)
	}

	for _, want := range []string{"log.sparkle", "feedbak.enabled"} {
		if !slices.Contains(cfg.UnknownConfigKeys, want) {
			t.Errorf("UnknownConfigKeys missing %q, got %v", want, cfg.UnknownConfigKeys)
		}
	}
	// Agent options are a free-form map by design; their keys are consumed
	// by the map and must NOT be reported as unknown.
	for _, k := range cfg.UnknownConfigKeys {
		if k == "projects.agent.options.totally_made_up_option" {
			t.Errorf("agent option keys must not be reported as unknown: %v", cfg.UnknownConfigKeys)
		}
	}
}

func TestFeedbackEnabled_DefaultTrue(t *testing.T) {
	c := &Config{}
	if !c.FeedbackEnabled() {
		t.Error("feedback must default to enabled")
	}
	f := false
	c.Feedback.Enabled = &f
	if c.FeedbackEnabled() {
		t.Error("explicit false must disable feedback")
	}
}

func TestValidateFeedbackEndpointRequiresExactSecureV1Route(t *testing.T) {
	valid := []string{
		"https://relay.example/v1/feedback",
		"http://localhost:8787/v1/feedback",
		"http://127.0.0.1:8787/v1/feedback",
	}
	for _, endpoint := range valid {
		if err := validateFeedbackEndpoint(endpoint); err != nil {
			t.Errorf("validateFeedbackEndpoint(%q): %v", endpoint, err)
		}
	}
	invalid := []string{
		"http://relay.example/v1/feedback",
		"https://relay.example/feedback",
		"https://relay.example/v1/feedback?token=value",
		"https://user:pass@relay.example/v1/feedback",
	}
	for _, endpoint := range invalid {
		if err := validateFeedbackEndpoint(endpoint); err == nil {
			t.Errorf("validateFeedbackEndpoint(%q) succeeded", endpoint)
		}
	}
}
