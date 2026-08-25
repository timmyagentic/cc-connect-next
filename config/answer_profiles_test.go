package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAnswerProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeConfigFileForTest(t, path, `
[[projects]]
name = "demo"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/demo"
model = "balanced-model"
reasoning_effort = "medium"
service_tier = "default"

[projects.agent.answer_profiles.fast]
model = "fast-model"
reasoning_effort = "low"
service_tier = "fast"

[projects.agent.answer_profiles.quality]
model = "quality-model"
reasoning_effort = "max"
service_tier = "default"

[[projects.platforms]]
type = "feishu"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	profiles := cfg.Projects[0].Agent.AnswerProfiles
	if profiles.Fast == nil || profiles.Fast.Model != "fast-model" || profiles.Fast.ReasoningEffort != "low" || profiles.Fast.ServiceTier != "fast" {
		t.Fatalf("fast profile = %#v", profiles.Fast)
	}
	if profiles.Quality == nil || profiles.Quality.Model != "quality-model" || profiles.Quality.ReasoningEffort != "max" || profiles.Quality.ServiceTier != "default" {
		t.Fatalf("quality profile = %#v", profiles.Quality)
	}
}

func TestValidateAnswerProfilesRejectsEmptyProfile(t *testing.T) {
	p := validProject("demo")
	p.Agent.AnswerProfiles.Fast = &AnswerProfileConfig{}
	cfg := Config{Projects: []ProjectConfig{p}}

	assertErrContains(t, cfg.Validate(), "projects[0].agent.answer_profiles.fast must override at least one setting")
}

func TestValidateAnswerProfilesRejectsInvalidReasoningEffort(t *testing.T) {
	p := validProject("demo")
	p.Agent.AnswerProfiles.Quality = &AnswerProfileConfig{ReasoningEffort: "extreme"}
	cfg := Config{Projects: []ProjectConfig{p}}

	assertErrContains(t, cfg.Validate(), "projects[0].agent.answer_profiles.quality.reasoning_effort must be low, medium, high, xhigh, or max")
}

func TestCloneAgentConfigCopiesAnswerProfiles(t *testing.T) {
	original := AgentConfig{AnswerProfiles: AnswerProfilesConfig{
		Fast:    &AnswerProfileConfig{Model: "fast-model"},
		Quality: &AnswerProfileConfig{ReasoningEffort: "max"},
	}}
	clone := cloneAgentConfig(original)
	clone.AnswerProfiles.Fast.Model = "changed"
	clone.AnswerProfiles.Quality.ReasoningEffort = "low"

	if original.AnswerProfiles.Fast.Model != "fast-model" || original.AnswerProfiles.Quality.ReasoningEffort != "max" {
		t.Fatalf("clone mutated original profiles: %#v", original.AnswerProfiles)
	}
}

func TestSaveAgentModelPreservesAnswerProfiles(t *testing.T) {
	writeTestConfig(t, `
[[projects]]
name = "demo"
[projects.agent]
type = "codex"
[projects.agent.options]
model = "balanced-model"
[projects.agent.answer_profiles.fast]
model = "fast-model"
[projects.agent.answer_profiles.quality]
reasoning_effort = "max"
[[projects.platforms]]
type = "feishu"
`)

	if err := SaveAgentModel("demo", "new-balanced-model"); err != nil {
		t.Fatalf("SaveAgentModel() error = %v", err)
	}
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`model = "new-balanced-model"`,
		`[projects.agent.answer_profiles.fast]`,
		`model = "fast-model"`,
		`[projects.agent.answer_profiles.quality]`,
		`reasoning_effort = "max"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("updated config missing %q:\n%s", want, text)
		}
	}
}

func writeConfigFileForTest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
