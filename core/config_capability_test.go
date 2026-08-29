package core

import (
	"strings"
	"testing"
)

func TestConfigOptionRegistry_NormalizesAndClones(t *testing.T) {
	const agentName = "test-config-catalog-agent"
	input := []ConfigOption{{Key: "model", Type: "string", Description: "Model", DescriptionZH: "模型"}}
	RegisterAgentConfigOptions(agentName, input)
	input[0].Key = "mutated"

	got := AgentConfigOptions(agentName)
	if len(got) != 1 {
		t.Fatalf("AgentConfigOptions() len = %d, want 1: %#v", len(got), got)
	}
	if got[0].Path != "projects.agent.options.model" || got[0].Owner != agentName || got[0].Scope != ConfigScopeAgent {
		t.Fatalf("normalized model option = %#v", got[0])
	}
	got[0].Key = "changed"
	if again := AgentConfigOptions(agentName); again[0].Key != "model" {
		t.Fatalf("registry leaked caller mutation: %#v", again)
	}
}

func TestDescribeAgentAndPlatformOptions_HaveAgentFriendlyMetadata(t *testing.T) {
	agent := DescribeAgentOptions([]string{"work_dir", "service_tier"})
	platform := DescribePlatformOptions([]string{"allow_from", "proxy_password"})
	for _, option := range append(agent, platform...) {
		if option.Description == "" || option.DescriptionZH == "" {
			t.Errorf("%s missing bilingual description: %#v", option.Key, option)
		}
		if option.Type == "" || option.ApplyMode == "" {
			t.Errorf("%s missing type/apply metadata: %#v", option.Key, option)
		}
	}
	if !platform[1].Sensitive {
		t.Fatal("proxy_password must be marked sensitive")
	}
}

func TestSearchConfigCatalog_MapsNaturalLanguageIntentToOptions(t *testing.T) {
	catalog := ConfigCatalog{
		Version: "v9.9.9",
		Capabilities: []ConfigCapability{{
			ID: "message-presentation", Title: "Message presentation", TitleZH: "消息展示",
			Description: "Control reasoning and status footers.", DescriptionZH: "控制思考过程和状态栏。",
			Keywords: []string{"隐藏思考", "footer"}, Paths: []string{"display.thinking_messages", "display.reply_footer"},
		}},
		Options: []ConfigOption{
			{Path: "display.thinking_messages", Key: "thinking_messages", Description: "Show reasoning", DescriptionZH: "显示思考过程"},
			{Path: "display.reply_footer", Key: "reply_footer", Description: "Show footer", DescriptionZH: "显示底部状态栏"},
			{Path: "data_dir", Key: "data_dir", Description: "Data directory", DescriptionZH: "数据目录"},
		},
	}
	got := SearchConfigCatalog(catalog, "怎么隐藏思考")
	if len(got.Capabilities) != 1 || len(got.Options) != 2 {
		t.Fatalf("natural-language search = %#v", got)
	}
	spaced := SearchConfigCatalog(catalog, "隐藏 思考 footer")
	if len(spaced.Capabilities) != 1 || len(spaced.Options) != 2 {
		t.Fatalf("multi-keyword search = %#v", spaced)
	}
}

func TestBuildAgentCapabilityBrief_TeachesUnifiedNaturalLanguageLookup(t *testing.T) {
	catalog := ConfigCatalog{
		Version:      "v2.0.0",
		Capabilities: []ConfigCapability{{ID: "display", Title: "Display", TitleZH: "消息展示", Description: "Control replies.", DescriptionZH: "控制回复展示。"}},
		Options: []ConfigOption{
			{Path: "projects.agent.options.model", Key: "model", Scope: ConfigScopeAgent, Owner: "codex", Description: "Select model", DescriptionZH: "选择模型"},
			{Path: "projects.platforms.options.allow_from", Key: "allow_from", Scope: ConfigScopePlatform, Owner: "feishu", Description: "Allow users", DescriptionZH: "限制用户"},
		},
	}
	brief := BuildAgentCapabilityBrief(catalog, "codex", []string{"feishu"})
	for _, want := range []string{
		"[cc-connect-next capability brief]", "v2.0.0", "natural-language", "cc-connect-next capabilities",
		"Skills", "side effects", "do not invent", "validated example", "model", "allow_from", "/feedback",
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q:\n%s", want, brief)
		}
	}
}

func TestRenderConfigCatalogMarkdown_CoalescesEquivalentAdapterOptions(t *testing.T) {
	catalog := ConfigCatalog{Version: "v1", Options: []ConfigOption{
		{Path: "projects.platforms.options.allow_from", Key: "allow_from", Scope: ConfigScopePlatform, Owner: "feishu", Type: "string", Default: "unset", Description: "Allow users", DescriptionZH: "限制用户", ApplyMode: ConfigApplyRestart},
		{Path: "projects.platforms.options.allow_from", Key: "allow_from", Scope: ConfigScopePlatform, Owner: "lark", Type: "string", Default: "unset", Description: "Allow users", DescriptionZH: "限制用户", ApplyMode: ConfigApplyRestart},
	}}
	markdown := RenderConfigCatalogMarkdown(catalog, "en")
	if strings.Count(markdown, "### `projects.platforms.options.allow_from`") != 1 {
		t.Fatalf("equivalent options were not coalesced:\n%s", markdown)
	}
	if !strings.Contains(markdown, "`feishu, lark`") {
		t.Fatalf("owner list missing:\n%s", markdown)
	}
}

func TestRenderConfigCatalogMarkdown_RendersStructuredContract(t *testing.T) {
	minimum, maximum := 1.0, 60.0
	catalog := ConfigCatalog{Version: "v1", Options: []ConfigOption{{
		Path: "projects.heartbeat.interval_mins", Key: "interval_mins", Scope: ConfigScopeProject,
		Type: "integer", Default: "30", DefaultSource: ConfigDefaultBuiltin,
		Requirement: ConfigRequirementConditional, RequiredWhen: []string{"projects.heartbeat.enabled = true"},
		Requires: []string{"projects.heartbeat.session_key"}, ConflictsWith: []string{"projects.heartbeat.disabled"},
		Minimum: &minimum, Maximum: &maximum, Unit: "minutes",
		Description: "Heartbeat interval.", DescriptionZH: "心跳间隔。", ApplyMode: ConfigApplyRestart,
		Example: `interval_mins = 30`, PresetValues: []ConfigPresetValue{{
			Preset: "starter", Value: "15", Description: "Fresh Starter value.", DescriptionZH: "新 Starter 值。",
		}},
	}}}

	for language, wants := range map[string][]string{
		"en": {"Requirement: `conditional`", "Required when: `projects.heartbeat.enabled = true`", "Requires: `projects.heartbeat.session_key`", "Conflicts with: `projects.heartbeat.disabled`", "Range: `1` to `60` `minutes`", "Default source: `builtin`", "Preset `starter`: `15`", "Example: `interval_mins = 30`"},
		"zh": {"要求：`条件必填`", "必填条件: `projects.heartbeat.enabled = true`", "依赖: `projects.heartbeat.session_key`", "冲突: `projects.heartbeat.disabled`", "范围: `1` 到 `60` `minutes`", "默认值来源：`builtin`", "预设 `starter`: `15`", "示例: `interval_mins = 30`"},
	} {
		markdown := RenderConfigCatalogMarkdown(catalog, language)
		for _, want := range wants {
			if !strings.Contains(markdown, want) {
				t.Errorf("%s contract markdown missing %q:\n%s", language, want, markdown)
			}
		}
	}
}
