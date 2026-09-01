package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
	"github.com/timmyagentic/cc-connect-next/daemon"
)

func TestWriteConfigCapabilities_FiltersCurrentAdaptersAndSearches(t *testing.T) {
	var out bytes.Buffer
	if err := writeConfigCapabilities(&out, []string{
		"--agent", "codex", "--platform", "feishu", "--search", "service tier", "--format", "json",
	}); err != nil {
		t.Fatal(err)
	}
	var catalog core.ConfigCatalog
	if err := json.Unmarshal(out.Bytes(), &catalog); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out.String())
	}
	if !slices.Contains(catalog.Agents, "codex") || len(catalog.Agents) != 1 {
		t.Fatalf("agents = %v", catalog.Agents)
	}
	if !slices.Contains(catalog.Platforms, "feishu") || len(catalog.Platforms) != 1 {
		t.Fatalf("platforms = %v", catalog.Platforms)
	}
	found := false
	for _, option := range catalog.Options {
		if option.Owner != "" && option.Owner != "codex" && option.Owner != "feishu" {
			t.Fatalf("unexpected adapter option: %#v", option)
		}
		if option.Owner == "codex" && option.Key == "service_tier" {
			found = true
		}
	}
	if !found {
		t.Fatalf("service_tier missing: %#v", catalog.Options)
	}
}

func TestConfigurationCapabilityBrief_IsBoundedAndActiveAdapterSpecific(t *testing.T) {
	brief := core.BuildAgentCapabilityBrief(config.CapabilityCatalog("v-test"), "codex", []string{"feishu"})
	if len(brief) > 16_000 {
		t.Fatalf("capability brief is too large: %d bytes", len(brief))
	}
	for _, want := range []string{"cc-connect-next capabilities", "Skills", "side effects", "codex.service_tier", "feishu.allow_from", "do not invent"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q", want)
		}
	}
	if strings.Contains(brief, "telegram.token") || strings.Contains(brief, "claudecode.allowed_tools") {
		t.Fatalf("brief leaked inactive adapter options:\n%s", brief)
	}
}

func TestConfigurationCapabilityBrief_RemainsBoundedForEveryPlatform(t *testing.T) {
	catalog := config.CapabilityCatalog("v-test")
	brief := core.BuildAgentCapabilityBrief(catalog, "codex", catalog.Platforms)
	if len(brief) > 16_000 {
		t.Fatalf("all-platform capability brief is too large: %d bytes", len(brief))
	}
	if !strings.Contains(brief, "more are available through the Manifest command") {
		t.Fatalf("all-platform brief did not explain truncation:\n%s", brief)
	}
}

func TestCompiledCapabilityGroupsReferenceDeclaredPaths(t *testing.T) {
	catalog := config.CapabilityCatalog("v-test")
	paths := make(map[string]bool, len(catalog.Options))
	for _, option := range catalog.Options {
		paths[option.Path] = true
	}
	for _, capability := range catalog.Capabilities {
		for _, path := range capability.Paths {
			if !paths[path] {
				t.Errorf("capability %s references undeclared path %s", capability.ID, path)
			}
		}
	}
}

func TestConfigurationCatalogHighRiskDefaultsMatchRuntime(t *testing.T) {
	catalog := config.CapabilityCatalog("v-test")
	defaults := make(map[string]string)
	for _, option := range catalog.Options {
		if option.Owner == "" {
			defaults[option.Path] = option.Default
		}
	}
	if got, want := defaults["projects.reset_on_idle_mins"], strconv.Itoa(defaultResetOnIdleMins); got != want {
		t.Errorf("catalog reset_on_idle_mins default = %q, runtime = %q", got, want)
	}
	if got, want := defaults["queue.busy_message_mode"], (&config.Config{}).ResolveBusyMessageMode(nil); got != want {
		t.Errorf("catalog busy_message_mode default = %q, runtime = %q", got, want)
	}
	_, _, _, _, _, _, showFooter, _ := config.EffectiveDisplay(&config.Config{}, &config.ProjectConfig{})
	if got, want := defaults["display.reply_footer"], strconv.FormatBool(showFooter); got != want {
		t.Errorf("catalog reply_footer default = %q, runtime = %q", got, want)
	}
}

func TestCodexModeCatalogDistinguishesStarterFromOmissionFallback(t *testing.T) {
	mode := findCatalogOption(t, core.AgentConfigOptions("codex"), "mode")
	if mode.Default != "suggest" || mode.DefaultSource != core.ConfigDefaultAdapter {
		t.Errorf("Codex mode omitted default = %q source=%q", mode.Default, mode.DefaultSource)
	}
	if !hasPresetValue(mode.PresetValues, "starter", "yolo") {
		t.Errorf("Codex mode presets = %#v, want starter=yolo", mode.PresetValues)
	}
	if !strings.Contains(mode.Description, "Omitting") || !strings.Contains(mode.Description, "fresh generated configs") {
		t.Errorf("Codex mode description does not explain both semantics: %q", mode.Description)
	}
	if !strings.Contains(config.StarterConfigTOML(), `mode = "yolo"`) {
		t.Fatal("generated Starter config no longer writes mode = yolo")
	}
}

func TestFeishuThreadIsolationCatalogDistinguishesProfileFromOmissionFallback(t *testing.T) {
	for _, owner := range []string{"feishu", "lark"} {
		option := findCatalogOption(t, core.PlatformConfigOptions(owner), "thread_isolation")
		if option.Default != "off" || option.DefaultSource != core.ConfigDefaultBuiltin {
			t.Errorf("%s thread_isolation omitted default = %q source=%q", owner, option.Default, option.DefaultSource)
		}
		if !hasPresetValue(option.PresetValues, "starter/recommended-feishu", "topics_only") {
			t.Errorf("%s thread_isolation presets = %#v", owner, option.PresetValues)
		}
		if option.Type != "string | boolean (legacy)" {
			t.Errorf("%s thread_isolation type = %q", owner, option.Type)
		}
		for _, value := range []string{"off", "topics_only", "topic_per_message"} {
			if !slices.Contains(option.Values, value) {
				t.Errorf("%s thread_isolation allowed values %v missing %q", owner, option.Values, value)
			}
		}
		if !strings.Contains(option.Description, "Omitting") || !strings.Contains(option.Description, "workspace binding") || !strings.Contains(strings.ToLower(option.Description), "ordinary group messages") || !strings.Contains(option.Description, "P2P topics") {
			t.Errorf("%s description does not explain compatibility and topic scope: %q", owner, option.Description)
		}
		if !strings.Contains(option.DescriptionZH, "省略") || !strings.Contains(option.DescriptionZH, "工作区绑定") || !strings.Contains(option.DescriptionZH, "普通群消息") || !strings.Contains(option.DescriptionZH, "P2P 私聊话题") {
			t.Errorf("%s Chinese description does not explain compatibility and topic scope: %q", owner, option.DescriptionZH)
		}
	}

	discord := findCatalogOption(t, core.PlatformConfigOptions("discord"), "thread_isolation")
	if discord.Default != "false" || strings.Contains(discord.Description, "Starter") {
		t.Fatalf("Feishu-specific default leaked into Discord metadata: %#v", discord)
	}
	if !strings.Contains(config.StarterConfigTOML(), `thread_isolation = "topics_only"`) {
		t.Fatal("generated Starter config no longer writes thread_isolation = topics_only")
	}

	var out bytes.Buffer
	if err := writeConfigCapabilities(&out, []string{"--platform", "feishu", "--search", "同一个群多个话题", "--lang", "zh"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"projects.platforms.options.thread_isolation", "topics_only", "topic_per_message", "新 Starter", "省略该键"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("natural-language topic query missing %q:\n%s", want, out.String())
		}
	}
}

func hasPresetValue(values []core.ConfigPresetValue, preset, value string) bool {
	for _, candidate := range values {
		if candidate.Preset == preset && candidate.Value == value {
			return true
		}
	}
	return false
}

func TestFeishuCatalogMatchesRuntimeConfigurationContract(t *testing.T) {
	owners := map[string]string{
		"feishu": lark.FeishuBaseUrl,
		"lark":   lark.LarkBaseUrl,
	}
	wantDefaults := map[string]string{
		"allow_chat":                      "empty",
		"app_id":                          "none",
		"app_secret":                      "none",
		"callback_path":                   "/feishu/webhook",
		"done_emoji":                      "Done",
		"enable_feishu_card":              "true",
		"group_only":                      "false",
		"group_reply_all":                 "false",
		"group_reply_all_chats":           "empty",
		"image_batch_window_ms":           "500",
		"mention_map":                     "empty",
		"peer_bots":                       "empty",
		"port":                            "8080",
		"progress_style":                  "legacy",
		"reaction_emoji":                  "OnIt",
		"reply_to_trigger":                "true",
		"require_mention":                 "true",
		"resolve_mentions":                "false",
		"respond_to_at_everyone_and_here": "false",
		"share_session_in_channel":        "false",
	}
	wantTypes := map[string]string{
		"group_reply_all_chats": "string | string[]",
		"image_batch_window_ms": "integer",
		"mention_map":           "table",
		"peer_bots":             "table",
		"port":                  "string",
	}

	for owner, domain := range owners {
		options := core.PlatformConfigOptions(owner)
		for key, want := range wantDefaults {
			if got := findCatalogOption(t, options, key).Default; got != want {
				t.Errorf("%s.%s default = %q, want %q", owner, key, got, want)
			}
		}
		if got := findCatalogOption(t, options, "domain").Default; got != domain {
			t.Errorf("%s.domain default = %q, want SDK default %q", owner, got, domain)
		}
		for key, want := range wantTypes {
			if got := findCatalogOption(t, options, key).Type; got != want {
				t.Errorf("%s.%s type = %q, want %q", owner, key, got, want)
			}
		}
		if !findCatalogOption(t, options, "app_secret").Sensitive {
			t.Errorf("%s.app_secret must remain sensitive", owner)
		}
		for key, fragments := range map[string][]string{
			"allow_chat":            {"comma-separated", "empty or '*'"},
			"encrypt_key":           {"unset", "WebSocket", "webhook"},
			"group_reply_all_chats": {"takes precedence", "group_reply_all"},
			"mention_map":           {"resolve_mentions = true", "open_id"},
			"peer_bots":             {"app_id", "alias"},
			"reply_to_trigger":      {"real topic", "thread_id"},
			"require_mention":       {"false", "group_reply_all = true"},
		} {
			option := findCatalogOption(t, options, key)
			for _, fragment := range fragments {
				if !strings.Contains(option.Description, fragment) {
					t.Errorf("%s.%s description %q missing %q", owner, key, option.Description, fragment)
				}
			}
			if option.DescriptionZH == "" {
				t.Errorf("%s.%s is missing a Chinese description", owner, key)
			}
		}
	}
}

func TestWriteConfigCapabilities_AnswersChineseIntentWithoutReadingConfig(t *testing.T) {
	var out bytes.Buffer
	if err := writeConfigCapabilities(&out, []string{"--search", "隐藏思考", "--lang", "zh"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"display.thinking_messages", "显示或隐藏 Agent 思考进度消息", "不读取或显示本机配置值"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestWriteConfigCapabilities_ExactUnknownKeyIsAnHonestNoMatch(t *testing.T) {
	var out bytes.Buffer
	if err := writeConfigCapabilities(&out, []string{"--key", "card_theme_color", "--lang", "zh"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "当前构建没有声明") || strings.Contains(out.String(), "projects.platforms.options.card_theme_color") {
		t.Fatalf("unexpected unsupported-key output:\n%s", out.String())
	}
}

func TestWriteConfigCapabilities_RejectsAdapterMissingFromThisBuild(t *testing.T) {
	var out bytes.Buffer
	err := writeConfigCapabilities(&out, []string{"--agent", "not-compiled"})
	if err == nil || !strings.Contains(err.Error(), "unknown Agent adapter") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteConfigCapabilities_UnsupportedNaturalLanguageWishIsNoMatch(t *testing.T) {
	var out bytes.Buffer
	if err := writeConfigCapabilities(&out, []string{"--search", "卡片主题颜色", "--lang", "zh"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "当前构建没有声明") || strings.Contains(out.String(), "card_mode") {
		t.Fatalf("unsupported wish was presented as a supported card option:\n%s", out.String())
	}
}

func TestWriteConfigCapabilities_CommonNaturalLanguageIntentsReachExactOptions(t *testing.T) {
	tests := []struct {
		query string
		path  string
	}{
		{query: "把思考过程隐藏", path: "display.thinking_messages"},
		{query: "消息忙的时候直接追加给当前回答", path: "queue.busy_message_mode"},
		{query: "群里不 @ 也回复", path: "projects.platforms.options.group_reply_all"},
		{query: "飞书机器人不要发表情", path: "projects.platforms.options.reaction_emoji"},
		{query: "飞书回复不要引用原消息", path: "projects.platforms.options.reply_to_trigger"},
		{query: "限制每分钟收到的消息", path: "rate_limit.max_messages"},
		{query: "Webhook 接口需要认证", path: "webhook.token"},
		{query: "管理后台端口", path: "management.port"},
		{query: "项目空闲后重置会话", path: "projects.reset_on_idle_mins"},
		{query: "启动后执行 hook", path: "hooks.event"},
		{query: "微信发送次数限制", path: "projects.platforms.options.burst_limit"},
		{query: "日志文件最大多大", path: "CC_LOG_MAX_SIZE"},
		{query: "daemon 安装时不要保存环境密钥", path: "CC_DAEMON_NO_CAPTURE_SECRETS"},
		{query: "Codex home 放在哪里", path: "CODEX_HOME"},
		{query: "飞书只允许我使用", path: "projects.platforms.options.allow_from"},
		{query: "企业微信 websocket 模式需要什么凭证", path: "projects.platforms.options.bot_id"},
		{query: "钉钉工作通知 Agent ID", path: "projects.platforms.options.agent_id"},
		{query: "多工作区和 work_dir 冲突", path: "projects.base_dir"},
		{query: "自定义命令 prompt 和 exec 二选一", path: "commands.prompt"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			canonical := core.SearchConfigCatalog(config.CapabilityCatalog("v-test"), tt.query)
			canonicalLimit := min(10, len(canonical.Options))
			canonicalFound := false
			for _, option := range canonical.Options[:canonicalLimit] {
				if option.Path == tt.path {
					canonicalFound = true
					break
				}
			}
			if !canonicalFound {
				t.Fatalf("canonical search did not rank %s in its first ten options; got %v", tt.path, catalogOptionPaths(canonical.Options[:canonicalLimit]))
			}

			var out bytes.Buffer
			if err := writeConfigCapabilities(&out, []string{"--all", "--search", tt.query, "--format", "json"}); err != nil {
				t.Fatal(err)
			}
			var catalog core.ConfigCatalog
			if err := json.Unmarshal(out.Bytes(), &catalog); err != nil {
				t.Fatalf("decode JSON: %v", err)
			}
			for index, option := range catalog.Options {
				if option.Path == tt.path {
					if index >= 10 {
						t.Fatalf("query %q reached %s only at result %d; first options=%v", tt.query, tt.path, index+1, catalogOptionPaths(catalog.Options[:10]))
					}
					if option.Requirement == "" || option.DefaultSource == "" || option.Example == "" || option.Placement == "" {
						t.Fatalf("query %q reached incomplete contract for %s: %#v", tt.query, tt.path, option)
					}
					return
				}
			}
			t.Fatalf("query %q did not reach %s; options=%v", tt.query, tt.path, catalogOptionPaths(catalog.Options))
		})
	}
}

func catalogOptionPaths(options []core.ConfigOption) []string {
	paths := make([]string, 0, len(options))
	for _, option := range options {
		paths = append(paths, option.Path)
	}
	return paths
}

func TestRegisteredPluginConfigSchemasCoverLiteralOptionReads(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	agents, platforms := core.RegisteredConfigOptions()
	for _, name := range core.ListRegisteredAgents() {
		assertPluginOptionReadsCovered(t, filepath.Join(repoRoot, "agent", name), name, agents[name])
	}
	for _, name := range core.ListRegisteredPlatforms() {
		dir := name
		switch name {
		case "lark":
			dir = "feishu"
		case "wps-xiezuo":
			dir = "wps-xiezuo"
		}
		assertPluginOptionReadsCovered(t, filepath.Join(repoRoot, "platform", dir), name, platforms[name])
	}
}

func TestRegisteredPluginConfigSchemasMatchDirectRuntimeTypes(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	agents, platforms := core.RegisteredConfigOptions()
	for _, name := range core.ListRegisteredAgents() {
		assertPluginOptionTypesMatch(t, filepath.Join(repoRoot, "agent", name), name, agents[name])
	}
	for _, name := range core.ListRegisteredPlatforms() {
		dir := name
		if name == "lark" {
			dir = "feishu"
		}
		assertPluginOptionTypesMatch(t, filepath.Join(repoRoot, "platform", dir), name, platforms[name])
	}
}

func TestReviewedPlatformOptionTypesMatchConstructors(t *testing.T) {
	for _, owner := range []string{"feishu", "lark", "line", "wecom"} {
		port := findCatalogOption(t, core.PlatformConfigOptions(owner), "port")
		if port.Type != "string" {
			t.Errorf("%s port type = %q, want string", owner, port.Type)
		}
	}
	interval := findCatalogOption(t, core.PlatformConfigOptions("max"), "webhook_resubscribe_interval")
	if interval.Type != "string" || interval.Default != "5m" || !strings.Contains(interval.Example, `"5m"`) {
		t.Fatalf("MAX resubscribe interval metadata = %#v", interval)
	}
}

func TestCompiledAdapterContractsAreActionable(t *testing.T) {
	agents, platforms := core.RegisteredConfigOptions()
	for kind, owners := range map[string]map[string][]core.ConfigOption{"agent": agents, "platform": platforms} {
		for owner, options := range owners {
			for _, option := range options {
				if option.Internal {
					continue
				}
				identity := kind + " " + owner + "." + option.Key
				if option.Requirement == "" || option.DefaultSource == "" || option.Example == "" || option.Placement == "" {
					t.Errorf("%s incomplete contract: %#v", identity, option)
				}
				if option.Source == core.ConfigSourceTOML && option.Placement == "config.toml root" {
					t.Errorf("%s lost its adapter TOML placement", identity)
				}
				if option.Example != "" && option.Source == core.ConfigSourceTOML {
					var decoded map[string]any
					if _, err := toml.Decode(option.Example, &decoded); err != nil {
						t.Errorf("%s example is invalid TOML: %q: %v", identity, option.Example, err)
					}
				}
				if option.Default == "unset / adapter default" {
					t.Errorf("%s still uses ambiguous adapter default", identity)
				}
			}
		}
	}

	for _, tc := range []struct {
		owner, key, wantType string
		wantRequirement      core.ConfigRequirement
	}{
		{owner: "dingtalk", key: "agent_id", wantType: "integer", wantRequirement: core.ConfigRequirementConditional},
		{owner: "discord", key: "group_reply_all_guilds", wantType: "string", wantRequirement: core.ConfigRequirementOptional},
		{owner: "claudecode", key: "plugin_dir", wantType: "string | string[]", wantRequirement: core.ConfigRequirementOptional},
		{owner: "tmux", key: "session", wantType: "string", wantRequirement: core.ConfigRequirementRequired},
		{owner: "telegram", key: "token", wantType: "string", wantRequirement: core.ConfigRequirementRequired},
		{owner: "slack", key: "bot_token", wantType: "string", wantRequirement: core.ConfigRequirementRequired},
		{owner: "wecom", key: "bot_id", wantType: "string", wantRequirement: core.ConfigRequirementConditional},
	} {
		registry := platforms
		if tc.owner == "claudecode" || tc.owner == "tmux" {
			registry = agents
		}
		option := findCatalogOption(t, registry[tc.owner], tc.key)
		if option.Type != tc.wantType || option.Requirement != tc.wantRequirement {
			t.Errorf("%s.%s contract = type %q requirement %q, want %q/%q", tc.owner, tc.key, option.Type, option.Requirement, tc.wantType, tc.wantRequirement)
		}
	}
}

func TestCompiledAdapterContractsEncodeEveryRuntimeRequirement(t *testing.T) {
	agents, platforms := core.RegisteredConfigOptions()
	required := map[string][]string{
		"dingtalk":   {"client_id", "client_secret"},
		"discord":    {"token"},
		"feishu":     {"app_id", "app_secret"},
		"lark":       {"app_id", "app_secret"},
		"line":       {"channel_secret", "channel_token"},
		"matrix":     {"homeserver", "access_token"},
		"max":        {"token"},
		"qqbot":      {"app_id", "app_secret"},
		"slack":      {"bot_token", "app_token"},
		"telegram":   {"token"},
		"webex":      {"token"},
		"weibo":      {"app_id", "app_secret"},
		"weixin":     {"token"},
		"wps-xiezuo": {"app_id", "app_secret"},
	}
	for owner, keys := range required {
		for _, key := range keys {
			option := findCatalogOption(t, platforms[owner], key)
			if option.Requirement != core.ConfigRequirementRequired || option.DefaultSource != core.ConfigDefaultNone {
				t.Errorf("%s.%s required contract = %#v", owner, key, option)
			}
		}
	}
	for _, key := range []string{"corp_id", "corp_secret", "agent_id", "callback_token", "callback_aes_key", "bot_id", "bot_secret"} {
		option := findCatalogOption(t, platforms["wecom"], key)
		if option.Requirement != core.ConfigRequirementConditional || len(option.RequiredWhen) == 0 {
			t.Errorf("wecom.%s conditional contract = %#v", key, option)
		}
	}
	for _, key := range []string{"cmd", "cli_path", "command"} {
		option := findCatalogOption(t, agents["acp"], key)
		if option.Requirement != core.ConfigRequirementConditional || len(option.RequiredWhen) == 0 {
			t.Errorf("acp.%s command requirement = %#v", key, option)
		}
	}
	if option := findCatalogOption(t, agents["tmux"], "session"); option.Requirement != core.ConfigRequirementRequired {
		t.Errorf("tmux.session required contract = %#v", option)
	}
}

func TestCompiledAdapterCatalogDefaultsMatchConstructors(t *testing.T) {
	_, platforms := core.RegisteredConfigOptions()
	wants := map[string]map[string]string{
		"dingtalk":   {"robot_code": "client_id", "reaction_emoji": "🤔Thinking", "card_template_key": "content", "card_throttle_ms": "300"},
		"discord":    {"progress_style": "compact", "group_reply_all_guilds": "empty"},
		"line":       {"port": "8080", "callback_path": "/callback"},
		"max":        {"api_base": "https://platform-api.max.ru", "webhook_path": "/webhook", "webhook_resubscribe_interval": "5m"},
		"qq":         {"ws_url": "ws://127.0.0.1:3001"},
		"qqbot":      {"intents": "100663296", "sandbox": "false", "markdown_support": "false"},
		"telegram":   {"progress_style": "compact"},
		"wecom":      {"mode": "callback", "port": "8081", "callback_path": "/wecom/callback", "api_base_url": "https://qyapi.weixin.qq.com", "enable_markdown": "false"},
		"weibo":      {"name": "weibo", "token_endpoint": "https://open-im.api.weibo.com/open/auth/ws_token", "ws_endpoint": "ws://open-im.api.weibo.com/ws/stream"},
		"weixin":     {"base_url": "https://ilinkai.weixin.qq.com", "cdn_base_url": "https://novac2c.cdn.weixin.qq.com/c2c", "burst_limit": "4", "burst_window_secs": "86400", "long_poll_timeout_ms": "35000"},
		"wps-xiezuo": {"base_url": "https://openapi.wps.cn", "clean_reply": "false"},
	}
	for owner, values := range wants {
		for key, want := range values {
			if got := findCatalogOption(t, platforms[owner], key).Default; got != want {
				t.Errorf("%s.%s default = %q, want %q", owner, key, got, want)
			}
		}
	}

	agents, _ := core.RegisteredConfigOptions()
	for key, want := range map[string]string{
		"auto_create": "true", "pane": "0", "poll_interval_ms": "200", "strip_input_block": "true", "window_per_session": "false",
	} {
		if got := findCatalogOption(t, agents["tmux"], key).Default; got != want {
			t.Errorf("tmux.%s default = %q, want %q", key, got, want)
		}
	}
}

func TestFeishuAccessControlContractRejectsSilentArrayFallback(t *testing.T) {
	base := map[string]any{"app_id": "id", "app_secret": "secret"}
	for _, key := range []string{"allow_from", "allow_chat"} {
		opts := make(map[string]any, len(base)+1)
		for baseKey, value := range base {
			opts[baseKey] = value
		}
		opts[key] = []any{"ou_or_oc_value"}
		err := core.ValidatePlatformOptions("feishu", opts)
		if err == nil || !strings.Contains(err.Error(), `option "`+key+`" must be string`) {
			t.Errorf("ValidatePlatformOptions(%s array) error = %v, want fail-closed type error", key, err)
		}
	}
}

func TestEnvironmentContractDefaultsMatchRuntimeConstants(t *testing.T) {
	catalog := config.CapabilityCatalog("v-test")
	find := func(path string) core.ConfigOption {
		t.Helper()
		for _, option := range catalog.Options {
			if option.Path == path {
				return option
			}
		}
		t.Fatalf("environment contract %s not found", path)
		return core.ConfigOption{}
	}
	if got, want := find("CC_LOG_MAX_SIZE").Default, strconv.FormatInt(int64(daemon.DefaultLogMaxSize)/(1024*1024), 10)+"MB"; got != want {
		t.Errorf("CC_LOG_MAX_SIZE default = %q, want %q", got, want)
	}
	if got, want := find("CC_LOG_MAX_BACKUPS").Default, strconv.Itoa(daemon.DefaultLogMaxBackups); got != want {
		t.Errorf("CC_LOG_MAX_BACKUPS default = %q, want %q", got, want)
	}
	if got, want := find("--log-max-size").Default, strconv.FormatInt(int64(daemon.DefaultLogMaxSize)/(1024*1024), 10)+"MB"; got != want {
		t.Errorf("--log-max-size default = %q, want %q", got, want)
	}
	if got, want := find("--log-max-backups").Default, strconv.Itoa(daemon.DefaultLogMaxBackups); got != want {
		t.Errorf("--log-max-backups default = %q, want %q", got, want)
	}
	if got, want := find("daemon install --log-max-size").Default, strconv.FormatInt(int64(daemon.DefaultLogMaxSize)/(1024*1024), 10); got != want {
		t.Errorf("daemon install --log-max-size default = %q, want %q", got, want)
	}
}

func TestCompiledCatalogIncludesOwnedEnvironmentContracts(t *testing.T) {
	catalog := config.CapabilityCatalog("v-test")
	wants := map[string]string{
		"CODEX_HOME":                    "codex",
		"CLAUDE_CONFIG_DIR":             "claudecode",
		"PI_CODING_AGENT_DIR":           "pi",
		"MATRIX_CROSS_SIGNING_PASSWORD": "matrix",
	}
	for path, owner := range wants {
		found := false
		for _, option := range catalog.Options {
			if option.Path == path && option.Owner == owner {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("compiled catalog missing %s owner=%s", path, owner)
		}
	}
}

func findCatalogOption(t *testing.T, options []core.ConfigOption, key string) core.ConfigOption {
	t.Helper()
	for _, option := range options {
		if option.Key == key {
			return option
		}
	}
	t.Fatalf("catalog option %q not found", key)
	return core.ConfigOption{}
}

func TestAnnotateUnknownPluginConfigKeys_ReportsDynamicMapTypos(t *testing.T) {
	cfg := &config.Config{Projects: []config.ProjectConfig{{
		Name: "demo",
		Agent: config.AgentConfig{Type: "codex", Options: map[string]any{
			"model": "gpt-test", "provider": "local", "totally_made_up": true,
		}},
		Platforms: []config.PlatformConfig{{Type: "feishu", Options: map[string]any{
			"app_id": "id", "app_secret": "secret", "card_theme_color": "blue",
		}}},
	}}}
	annotateUnknownPluginConfigKeys(cfg)
	for _, want := range []string{
		"projects[0].agent.options.totally_made_up",
		"projects[0].platforms[0].options.card_theme_color",
	} {
		if !slices.Contains(cfg.UnknownConfigKeys, want) {
			t.Errorf("UnknownConfigKeys missing %q: %v", want, cfg.UnknownConfigKeys)
		}
	}
	for _, valid := range []string{"model", "provider", "app_id", "app_secret"} {
		for _, unknown := range cfg.UnknownConfigKeys {
			if strings.HasSuffix(unknown, "."+valid) {
				t.Errorf("valid key %q reported unknown: %v", valid, cfg.UnknownConfigKeys)
			}
		}
	}
}

func TestAnnotateUnknownPluginConfigKeys_LoadedDynamicTablesReportOnlyUnknownOptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[[projects]]
name = "demo"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "` + dir + `"
totally_made_up = true

[projects.agent.options.env]
GITLAB_TOKEN = "fixture"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "fixture"
app_secret = "fixture"
resolve_mentions = true
card_theme_color = "blue"

[projects.platforms.options.mention_map]
reviewer = "ou_fixture"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadPermissive(path)
	if err != nil {
		t.Fatalf("LoadPermissive: %v", err)
	}

	annotateUnknownPluginConfigKeys(cfg)
	want := []string{
		"projects[0].agent.options.totally_made_up",
		"projects[0].platforms[0].options.card_theme_color",
	}
	if !slices.Equal(cfg.UnknownConfigKeys, want) {
		t.Fatalf("UnknownConfigKeys = %v, want only real top-level option gaps %v", cfg.UnknownConfigKeys, want)
	}
}

func TestGeneratedConfigCapabilityDocsAreCurrent(t *testing.T) {
	docs := generatedConfigCapabilityDocs()
	for name, want := range docs {
		path := filepath.Join("..", "..", "docs", name)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read generated doc %s: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("%s is stale; run `go generate ./cmd/cc-connect`", path)
		}
	}
}

func TestGeneratedWebConfigContractIsCurrent(t *testing.T) {
	path := filepath.Join("..", "..", "web", "src", "generated", "configContract.ts")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated Web contract %s: %v", path, err)
	}
	if want := generatedWebConfigContract(); string(got) != want {
		t.Errorf("%s is stale; run `go generate ./cmd/cc-connect`", path)
	}
}

func assertPluginOptionReadsCovered(t *testing.T, dir, name string, options []core.ConfigOption) {
	t.Helper()
	if len(options) == 0 {
		t.Errorf("%s has no registered config option schema", name)
		return
	}
	known := make(map[string]core.ConfigOption, len(options))
	for _, option := range options {
		known[option.Key] = option
		if option.Description == "" || option.DescriptionZH == "" {
			t.Errorf("%s.%s has no bilingual description", name, option.Key)
		}
		if strings.HasPrefix(option.Description, "Configure the ") && strings.HasSuffix(option.Description, " adapter option.") {
			t.Errorf("%s.%s still uses the generic fallback description", name, option.Key)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		filename := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(filename, ".go") || strings.HasSuffix(filename, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, filename), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			index, ok := node.(*ast.IndexExpr)
			if !ok {
				return true
			}
			ident, ok := index.X.(*ast.Ident)
			if !ok || ident.Name != "opts" {
				return true
			}
			literal, ok := index.Index.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			key, err := strconv.Unquote(literal.Value)
			if err == nil {
				if _, ok := known[key]; !ok {
					t.Errorf("%s reads opts[%q] in %s but the catalog does not declare it", name, key, filename)
				}
			}
			return true
		})
	}
}

func assertPluginOptionTypesMatch(t *testing.T, dir, name string, options []core.ConfigOption) {
	t.Helper()
	known := make(map[string]core.ConfigOption, len(options))
	for _, option := range options {
		known[option.Key] = option
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		filename := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(filename, ".go") || strings.HasSuffix(filename, "_test.go") || filename == "option_schema.go" {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, filename), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			assertion, ok := node.(*ast.TypeAssertExpr)
			if !ok || assertion.Type == nil {
				return true
			}
			index, ok := assertion.X.(*ast.IndexExpr)
			if !ok {
				return true
			}
			ident, ok := index.X.(*ast.Ident)
			if !ok || ident.Name != "opts" {
				return true
			}
			literal, ok := index.Index.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			key, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			option, ok := known[key]
			if !ok {
				return true // key coverage reports this separately
			}
			asserted := configASTType(assertion.Type)
			if asserted != "" && !configContractAcceptsType(option.Type, asserted) {
				t.Errorf("%s reads opts[%q].(%s) in %s but catalog type is %q", name, key, asserted, filename, option.Type)
			}
			return true
		})
	}
}

func configASTType(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		switch typed.Name {
		case "string":
			return "string"
		case "bool":
			return "boolean"
		case "int", "int32", "int64", "uint", "uint32", "uint64", "float32", "float64":
			return "numeric"
		}
	case *ast.ArrayType:
		return "string[]"
	case *ast.MapType:
		return "table"
	}
	return ""
}

func configContractAcceptsType(contract, asserted string) bool {
	for _, candidate := range strings.Split(contract, "|") {
		candidate = strings.TrimSpace(candidate)
		if idx := strings.Index(candidate, " "); idx >= 0 {
			candidate = candidate[:idx]
		}
		if candidate == asserted || (asserted == "numeric" && (candidate == "integer" || candidate == "number")) {
			return true
		}
	}
	return false
}
