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

func TestBuildConfigurationCapabilityBrief_TeachesNaturalLanguageLookup(t *testing.T) {
	catalog := ConfigCatalog{
		Version:      "v2.0.0",
		Capabilities: []ConfigCapability{{ID: "display", Title: "Display", TitleZH: "消息展示", Description: "Control replies.", DescriptionZH: "控制回复展示。"}},
		Options: []ConfigOption{
			{Path: "projects.agent.options.model", Key: "model", Scope: ConfigScopeAgent, Owner: "codex", Description: "Select model", DescriptionZH: "选择模型"},
			{Path: "projects.platforms.options.allow_from", Key: "allow_from", Scope: ConfigScopePlatform, Owner: "feishu", Description: "Allow users", DescriptionZH: "限制用户"},
		},
	}
	brief := BuildConfigurationCapabilityBrief(catalog, "codex", []string{"feishu"})
	for _, want := range []string{
		"[cc-connect-next capability brief]", "v2.0.0", "natural-language", "config capabilities",
		"--agent codex", "--platform feishu", "do not invent", "model", "allow_from", "/feedback",
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
