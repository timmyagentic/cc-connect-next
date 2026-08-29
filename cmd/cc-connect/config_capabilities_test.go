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

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
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
	brief := core.BuildConfigurationCapabilityBrief(config.CapabilityCatalog("v-test"), "codex", []string{"feishu"})
	if len(brief) > 16_000 {
		t.Fatalf("capability brief is too large: %d bytes", len(brief))
	}
	for _, want := range []string{"config capabilities", "codex.service_tier", "feishu.allow_from", "do not invent"} {
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
	brief := core.BuildConfigurationCapabilityBrief(catalog, "codex", catalog.Platforms)
	if len(brief) > 16_000 {
		t.Fatalf("all-platform capability brief is too large: %d bytes", len(brief))
	}
	if !strings.Contains(brief, "more are available through the catalog command") {
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
	for _, want := range []string{"suggest", "starter", "yolo"} {
		if !strings.Contains(mode.Default, want) {
			t.Errorf("Codex mode default %q missing %q", mode.Default, want)
		}
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
		for _, want := range []string{"false", "omitted", "Starter", "recommended", "true"} {
			if !strings.Contains(option.Default, want) {
				t.Errorf("%s thread_isolation default %q missing %q", owner, option.Default, want)
			}
		}
		if !strings.Contains(option.Description, "Omitting") || !strings.Contains(option.Description, "workspace binding") || !strings.Contains(option.Description, "Ordinary group messages") {
			t.Errorf("%s description does not explain compatibility and topic scope: %q", owner, option.Description)
		}
		if !strings.Contains(option.DescriptionZH, "省略") || !strings.Contains(option.DescriptionZH, "工作区绑定") || !strings.Contains(option.DescriptionZH, "普通群消息") {
			t.Errorf("%s Chinese description does not explain compatibility and topic scope: %q", owner, option.DescriptionZH)
		}
	}

	discord := findCatalogOption(t, core.PlatformConfigOptions("discord"), "thread_isolation")
	if discord.Default != "false" || strings.Contains(discord.Description, "Starter") {
		t.Fatalf("Feishu-specific default leaked into Discord metadata: %#v", discord)
	}
	if !strings.Contains(config.StarterConfigTOML(), "thread_isolation = true") {
		t.Fatal("generated Starter config no longer writes thread_isolation = true")
	}

	var out bytes.Buffer
	if err := writeConfigCapabilities(&out, []string{"--platform", "feishu", "--search", "同一个群多个话题", "--lang", "zh"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"projects.platforms.options.thread_isolation", "新 Starter", "省略该键"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("natural-language topic query missing %q:\n%s", want, out.String())
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
