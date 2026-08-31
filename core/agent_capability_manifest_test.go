package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type manifestTestPlatform struct {
	stubCardPlatform
}

func (p *manifestTestPlatform) SendImage(context.Context, any, ImageAttachment) error { return nil }
func (p *manifestTestPlatform) SendFile(context.Context, any, FileAttachment) error   { return nil }

type manifestNoNativeSteerAgent struct{ stubAgent }

func (*manifestNoNativeSteerAgent) NativeSteerStatus() (bool, string) {
	return false, "configured backend cannot steer"
}

type manifestSteerSession struct{ stubAgentSession }

func (*manifestSteerSession) Steer(string, []ImageAttachment, []FileAttachment) error { return nil }

func TestBuiltinCommandManifestContractsCoverDispatchSurface(t *testing.T) {
	seen := make(map[string]bool)
	seenNames := make(map[string]string)
	for _, command := range builtinCommands {
		if seen[command.id] {
			t.Errorf("duplicate built-in command %q", command.id)
		}
		seen[command.id] = true
		for _, name := range builtinCommandNames(command) {
			if owner, ok := seenNames[name]; ok {
				t.Errorf("built-in command name %q belongs to both %q and %q", name, owner, command.id)
			}
			seenNames[name] = command.id
		}
		if command.category == "" || command.usage == "" {
			t.Errorf("built-in command %q has incomplete category/usage: %#v", command.id, command)
		}
		if !command.readOnly && len(command.effects) == 0 {
			t.Errorf("mutating built-in command %q declares no side effects", command.id)
		}
		for _, effect := range command.effects {
			if _, ok := sideEffectDescriptions[effect]; !ok {
				t.Errorf("built-in command %q references undocumented side effect %q", command.id, effect)
			}
		}
		for _, subcommand := range command.privilegedWhen {
			if !isPrivilegedCommandInvocation(command.id, []string{subcommand}) {
				t.Errorf("manifest says %s %s is privileged, but dispatch does not", command.id, subcommand)
			}
		}
	}
	for _, tool := range agentToolDefinitions {
		if tool.id == "" || tool.invocation == "" || tool.description == "" || tool.zh == "" {
			t.Errorf("incomplete Agent tool contract: %#v", tool)
		}
		if !tool.readOnly && len(tool.effects) == 0 {
			t.Errorf("mutating Agent tool %q declares no side effects", tool.id)
		}
		for _, effect := range tool.effects {
			if _, ok := sideEffectDescriptions[effect]; !ok {
				t.Errorf("Agent tool %q references undocumented side effect %q", tool.id, effect)
			}
		}
	}
}

func TestAgentCapabilityManifest_CoversContractsWithoutLeakingBodies(t *testing.T) {
	agent := &stubModelModeAgent{}
	platform := &manifestTestPlatform{stubCardPlatform: stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "manifest-platform"}}}
	e := NewEngine("demo", agent, []Platform{platform}, "", LangChinese)
	e.SetAdminFrom("")
	e.SetConfigCatalog(ConfigCatalog{
		Version:   "v-test",
		Agents:    []string{"stub", "inactive-agent"},
		Platforms: []string{"manifest-platform", "inactive-platform"},
		Capabilities: []ConfigCapability{{
			ID: "topics", Title: "Topics", TitleZH: "话题", Description: "Isolate real topics.", DescriptionZH: "只隔离已有真实话题。",
			Paths: []string{"projects.platforms.options.thread_isolation"},
		}},
		Options: []ConfigOption{
			{Path: "language", Key: "language", Scope: ConfigScopeGlobal, Type: "string", Description: "Language", DescriptionZH: "语言"},
			{Path: "projects.agent.options.model", Key: "model", Scope: ConfigScopeAgent, Owner: "stub", Type: "string", Description: "Model", DescriptionZH: "模型"},
			{Path: "projects.agent.options.secret", Key: "secret", Scope: ConfigScopeAgent, Owner: "inactive-agent", Type: "string", Description: "Inactive", DescriptionZH: "未启用"},
			{Path: "projects.platforms.options.thread_isolation", Key: "thread_isolation", Scope: ConfigScopePlatform, Owner: "manifest-platform", Type: "string", Description: "Isolate real topics", DescriptionZH: "只隔离已有真实话题"},
		},
	})

	e.AddCommand("deploy", "Deploy the project", "", "echo DO_NOT_LEAK_EXEC_BODY", "", "config")
	e.AddCommand("summarize", "Summarize the project", "summarize", "", "", "config")
	skillRoot := t.TempDir()
	skillDir := filepath.Join(skillRoot, "release-check")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Verify a release safely\n---\nDO_NOT_LEAK_SKILL_BODY"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.skills.SetDirs([]string{skillRoot})
	commandDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(commandDir, "private-command.md"), []byte("DO_NOT_LEAK_AGENT_COMMAND_BODY\nprivate instructions"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.commands.SetAgentDirs([]string{commandDir})
	e.platformLifecycleMu.Lock()
	e.platformReady[platform] = true
	e.platformLifecycleMu.Unlock()

	manifest := e.QueryAgentCapabilityManifest("manifest-platform:chat:user", "", false)
	if manifest.Schema != AgentCapabilityManifestSchema || !manifest.ReadOnly || !manifest.SessionBound {
		t.Fatalf("manifest header = %#v", manifest)
	}
	if manifest.Configuration.Version != "v-test" || !slices.Equal(manifest.Configuration.Agents, []string{"stub"}) || !slices.Equal(manifest.Configuration.Platforms, []string{"manifest-platform"}) {
		t.Fatalf("active configuration filters = %#v", manifest.Configuration)
	}
	for _, tool := range manifest.Tools {
		if tool.Description == "" || tool.DescriptionZH == "" || tool.Permission == "" || tool.Fallback.Mode == "" || tool.Availability.Reason == "" {
			t.Errorf("incomplete Agent tool Manifest entry: %#v", tool)
		}
	}
	for _, command := range manifest.Commands {
		if command.Source != "builtin" {
			continue
		}
		if command.Description == "" || command.DescriptionZH == "" || command.Permission == "" || command.Fallback.Mode == "" || command.Availability.Reason == "" {
			t.Errorf("incomplete built-in command Manifest entry: %#v", command)
		}
	}
	for _, option := range manifest.Configuration.Options {
		if option.Owner == "inactive-agent" || option.Owner == "inactive-platform" {
			t.Fatalf("inactive adapter leaked into configuration: %#v", option)
		}
	}
	allAdapters := e.QueryAgentCapabilityManifest("", "", true)
	if !allAdapters.AllAdapters || len(allAdapters.Configuration.Options) != 4 {
		t.Fatalf("all-adapter configuration = %#v", allAdapters.Configuration)
	}
	inactive := findRuntimeAdapter(t, allAdapters.Runtime, "agent", "inactive-agent")
	if inactive.State != CapabilityUnavailable || findRuntimeFeature(t, inactive.Capabilities, "activation").Fallback.Mode != "configure-and-restart" {
		t.Fatalf("inactive adapter contract = %#v", inactive)
	}

	shell := findManifestCommand(t, manifest.Commands, "shell")
	if shell.Permission != CapabilityPermissionAdmin || shell.Availability.State != CapabilityUnavailable || !hasManifestEffect(shell.SideEffects, "shell_execution") {
		t.Fatalf("shell contract = %#v", shell)
	}
	daemonRestart := findManifestTool(t, manifest.Tools, "daemon-restart")
	if daemonRestart.Permission != CapabilityPermissionAdmin || daemonRestart.Availability.State != CapabilityUnavailable || !hasManifestEffect(daemonRestart.SideEffects, "process_control") || daemonRestart.Fallback.Mode != "chat-command" {
		t.Fatalf("daemon restart tool contract = %#v", daemonRestart)
	}
	timer := findManifestCommand(t, manifest.Commands, "timer")
	if timer.Permission != CapabilityPermissionConditional || !slices.Contains(timer.PrivilegedWhen, "addexec") {
		t.Fatalf("timer permission contract = %#v", timer)
	}
	deploy := findManifestCommand(t, manifest.Commands, "deploy")
	if deploy.Permission != CapabilityPermissionAdmin || deploy.Availability.State != CapabilityUnavailable || !hasManifestEffect(deploy.SideEffects, "shell_execution") {
		t.Fatalf("custom exec contract = %#v", deploy)
	}
	summarize := findManifestCommand(t, manifest.Commands, "summarize")
	if summarize.Permission != CapabilityPermissionMember || summarize.Availability.State != CapabilityAvailable || !hasManifestEffect(summarize.SideEffects, "agent_turn") {
		t.Fatalf("custom prompt contract = %#v", summarize)
	}
	e.SetAdminFrom("admin-user")
	deployWithAdminConfigured := findManifestCommand(t, e.QueryAgentCapabilityManifest("", "", false).Commands, "deploy")
	if deployWithAdminConfigured.Permission != CapabilityPermissionAdmin || deployWithAdminConfigured.Availability.State != CapabilityConditional {
		t.Fatalf("custom exec with admin_from configured = %#v", deployWithAdminConfigured)
	}
	daemonRestartWithAdminConfigured := findManifestTool(t, e.QueryAgentCapabilityManifest("", "", false).Tools, "daemon-restart")
	if daemonRestartWithAdminConfigured.Permission != CapabilityPermissionAdmin || daemonRestartWithAdminConfigured.Availability.State != CapabilityConditional {
		t.Fatalf("daemon restart tool with admin_from = %#v", daemonRestartWithAdminConfigured)
	}
	if len(manifest.Skills) != 1 || manifest.Skills[0].Name != "release-check" || manifest.Skills[0].Description != "Verify a release safely" {
		t.Fatalf("skills = %#v", manifest.Skills)
	}

	agentAdapter := findRuntimeAdapter(t, manifest.Runtime, "agent", "stub")
	if findRuntimeFeature(t, agentAdapter.Capabilities, "model_switching").Availability.State != CapabilityAvailable {
		t.Fatalf("model switching not detected: %#v", agentAdapter)
	}
	platformAdapter := findRuntimeAdapter(t, manifest.Runtime, "platform", "manifest-platform")
	if findRuntimeFeature(t, platformAdapter.Capabilities, "structured_cards").Availability.State != CapabilityAvailable {
		t.Fatalf("card capability not detected: %#v", platformAdapter)
	}
	audio := findRuntimeFeature(t, platformAdapter.Capabilities, "audio")
	if audio.Availability.State != CapabilityUnavailable || audio.Fallback.Mode != "file" {
		t.Fatalf("audio fallback = %#v", audio)
	}

	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"DO_NOT_LEAK_EXEC_BODY", "DO_NOT_LEAK_SKILL_BODY", "DO_NOT_LEAK_AGENT_COMMAND_BODY", skillRoot, commandDir} {
		if strings.Contains(text, forbidden) {
			t.Errorf("manifest leaked %q", forbidden)
		}
	}
}

func TestSearchAgentCapabilityManifest_FiltersEveryDomain(t *testing.T) {
	manifest := AgentCapabilityManifest{
		Schema: AgentCapabilityManifestSchema,
		Configuration: ConfigCatalog{Version: "v-test", Options: []ConfigOption{{
			Path: "projects.platforms.options.thread_isolation", Key: "thread_isolation", Description: "Isolate topics", DescriptionZH: "只隔离已有话题",
		}}},
		Tools:    []AgentToolCapability{{ID: "timer", Description: "One-shot delay", DescriptionZH: "一次性延迟"}},
		Commands: []AgentCommandCapability{{ID: "cron", Invocation: "/cron", Description: "Recurring schedule", DescriptionZH: "周期任务"}},
		Skills:   []AgentSkillCapability{{Name: "release-check", Description: "Verify release evidence"}},
		Runtime: []RuntimeAdapterCapabilities{{Kind: "platform", Name: "feishu", Capabilities: []RuntimeFeatureCapability{{
			ID: "audio", Description: "Native audio", DescriptionZH: "原生音频", Availability: unavailable("unsupported", "不支持"),
		}}}},
	}

	for _, test := range []struct {
		query string
		check func(AgentCapabilityManifest) bool
	}{
		{"已有话题", func(got AgentCapabilityManifest) bool { return len(got.Configuration.Options) == 1 }},
		{"一次性延迟", func(got AgentCapabilityManifest) bool { return len(got.Tools) == 1 && got.Tools[0].ID == "timer" }},
		{"周期任务", func(got AgentCapabilityManifest) bool { return len(got.Commands) == 1 && got.Commands[0].ID == "cron" }},
		{"release evidence", func(got AgentCapabilityManifest) bool { return len(got.Skills) == 1 }},
		{"原生音频", func(got AgentCapabilityManifest) bool {
			return len(got.Runtime) == 1 && len(got.Runtime[0].Capabilities) == 1
		}},
	} {
		t.Run(test.query, func(t *testing.T) {
			got := SearchAgentCapabilityManifest(manifest, test.query)
			if got.Query != test.query || !test.check(got) {
				t.Fatalf("search %q = %#v", test.query, got)
			}
		})
	}
}

func TestSelectAgentCapabilityManifestSectionsKeepsRequestedProjection(t *testing.T) {
	manifest := AgentCapabilityManifest{
		Configuration: ConfigCatalog{Version: "v-test", Options: []ConfigOption{{Path: "language"}}},
		Tools:         []AgentToolCapability{{ID: "send"}},
		Commands:      []AgentCommandCapability{{ID: "help"}},
		Skills:        []AgentSkillCapability{{Name: "release"}},
		Runtime:       []RuntimeAdapterCapabilities{{Kind: "agent", Name: "codex"}},
	}
	got := SelectAgentCapabilityManifestSections(manifest, "commands, skills")
	if len(got.Commands) != 1 || len(got.Skills) != 1 || len(got.Tools) != 0 || len(got.Runtime) != 0 || len(got.Configuration.Options) != 0 || got.Configuration.Version != "v-test" {
		t.Fatalf("section projection = %#v", got)
	}
}

func TestEngineSearchAgentCapabilityManifestUsesCanonicalConfigSearch(t *testing.T) {
	e := NewEngine("demo", &stubAgent{}, nil, "", LangEnglish)
	e.SetConfigCatalog(ConfigCatalog{Version: "v-test", Options: []ConfigOption{{Path: "queue.busy_message_mode", Key: "busy_message_mode", Description: "Queue mode", DescriptionZH: "排队模式", Keywords: []string{"消息忙的时候直接追加给当前回答"}}}})
	manifest := e.QueryAgentCapabilityManifest("", "消息忙的时候直接追加给当前回答", false)
	if len(manifest.Configuration.Options) != 1 || manifest.Configuration.Options[0].Path != "queue.busy_message_mode" {
		t.Fatalf("canonical configuration search not used: manifest=%#v", manifest.Configuration)
	}
}

func TestAgentCapabilityManifest_RedactsDynamicDescriptionsAndRuntimeErrors(t *testing.T) {
	platform := &stubUnhealthyPlatform{stubPlatformEngine: stubPlatformEngine{n: "broken"}, err: errors.New("token=abcdefghijklmnopqrstuvwxyz123456 /Users/example/private")}
	e := NewEngine("demo", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	e.AddCommand("unsafe-description", "token=abcdefghijklmnopqrstuvwxyz123456", "prompt", "", "", "config")
	e.OnPlatformReady(platform)
	manifest := e.QueryAgentCapabilityManifest("", "", false)
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "abcdefghijklmnopqrstuvwxyz123456") || strings.Contains(string(encoded), "/Users/example/private") || !strings.Contains(string(encoded), "[REDACTED]") {
		t.Fatalf("dynamic metadata was not redacted: %s", encoded)
	}
}

func TestSessionManifestDoesNotAdvertiseInterfaceOnlySteerWhenBackendRejectsIt(t *testing.T) {
	e := NewEngine("demo", &manifestNoNativeSteerAgent{}, nil, "", LangEnglish)
	adapter := e.sessionRuntimeCapabilities(&manifestSteerSession{})
	steer := findRuntimeFeature(t, adapter.Capabilities, "steer")
	if steer.Availability.State != CapabilityUnavailable || !strings.Contains(steer.Availability.Reason, "cannot steer") {
		t.Fatalf("steer availability = %#v", steer.Availability)
	}
}

func TestSessionManifestMarksUnreadableContextUsageUnavailable(t *testing.T) {
	e := NewEngine("demo", &stubAgent{}, nil, "", LangEnglish)
	manifest := e.QueryAgentCapabilityManifest("", "", false)
	session := findRuntimeAdapter(t, manifest.Runtime, "session", "stub:unbound")
	capability := findRuntimeFeature(t, session.Capabilities, "context_usage")
	if capability.Availability.State != CapabilityUnavailable || !strings.Contains(capability.Availability.Reason, "no production tool or API") {
		t.Fatalf("context_usage availability = %#v, want explicit unavailable reason", capability.Availability)
	}
	if capability.Fallback.Mode != "degrade" || !strings.Contains(capability.Fallback.Description, "No live context percentage") {
		t.Fatalf("context_usage fallback = %#v", capability.Fallback)
	}
}

func TestPlatformCommandMenusProjectManifestAvailability(t *testing.T) {
	e := NewEngine("demo", &stubAgent{}, nil, "", LangEnglish)
	e.SetAdminFrom("")
	e.SetDisabledCommands([]string{"help", "hidden-custom"})
	e.AddCommand("visible-custom", "Visible custom command", "prompt", "", "", "config")
	e.AddCommand("hidden-custom", "Hidden custom command", "prompt", "", "", "config")

	commands := e.GetAllCommands()
	present := make(map[string]bool, len(commands))
	for _, command := range commands {
		present[command.Command] = true
	}
	for _, expected := range []string{"current", "visible-custom"} {
		if !present[expected] {
			t.Errorf("Manifest-available command %q missing from platform menu: %#v", expected, commands)
		}
	}
	for _, forbidden := range []string{"help", "hidden-custom", "shell", "model"} {
		if present[forbidden] {
			t.Errorf("Manifest-unavailable command %q leaked into platform menu: %#v", forbidden, commands)
		}
	}
}

func TestRenderAgentCapabilityManifestMarkdown_ExplainsAvailabilityPermissionEffectsAndFallback(t *testing.T) {
	manifest := AgentCapabilityManifest{
		Schema: AgentCapabilityManifestSchema, Version: "v-test", Project: "demo", ReadOnly: true, Query: "shell",
		Commands: []AgentCommandCapability{{
			ID: "shell", Invocation: "/shell", Source: "builtin", Usage: "/shell <command>", Description: "Run shell", DescriptionZH: "执行 Shell",
			Permission: CapabilityPermissionAdmin, ReadOnly: false, SideEffects: expandSideEffects([]string{"shell_execution"}),
			Fallback: defaultRejectFallback(), Availability: unavailable("admin_from missing", "未配置 admin_from"),
		}},
	}
	got := RenderAgentCapabilityManifestMarkdown(manifest, "zh")
	for _, want := range []string{"Agent 能力清单", "/shell", "权限", "admin", "shell_execution", "退化行为", "admin_from"} {
		if !strings.Contains(got, want) {
			t.Errorf("render missing %q:\n%s", want, got)
		}
	}
}

func findManifestCommand(t *testing.T, commands []AgentCommandCapability, id string) AgentCommandCapability {
	t.Helper()
	for _, command := range commands {
		if command.ID == id {
			return command
		}
	}
	t.Fatalf("command %q not found", id)
	return AgentCommandCapability{}
}

func findManifestTool(t *testing.T, tools []AgentToolCapability, id string) AgentToolCapability {
	t.Helper()
	for _, tool := range tools {
		if tool.ID == id {
			return tool
		}
	}
	t.Fatalf("tool %q not found", id)
	return AgentToolCapability{}
}

func hasManifestEffect(effects []CapabilitySideEffect, kind string) bool {
	for _, effect := range effects {
		if effect.Kind == kind {
			return true
		}
	}
	return false
}

func findRuntimeAdapter(t *testing.T, adapters []RuntimeAdapterCapabilities, kind, name string) RuntimeAdapterCapabilities {
	t.Helper()
	for _, adapter := range adapters {
		if adapter.Kind == kind && adapter.Name == name {
			return adapter
		}
	}
	t.Fatalf("runtime adapter %s:%s not found", kind, name)
	return RuntimeAdapterCapabilities{}
}

func findRuntimeFeature(t *testing.T, capabilities []RuntimeFeatureCapability, id string) RuntimeFeatureCapability {
	t.Helper()
	for _, capability := range capabilities {
		if capability.ID == id {
			return capability
		}
	}
	t.Fatalf("runtime capability %q not found", id)
	return RuntimeFeatureCapability{}
}
