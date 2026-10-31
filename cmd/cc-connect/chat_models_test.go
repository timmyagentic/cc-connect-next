package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

type chatConfigCommandAgent struct {
	stubMainAgent
	setCalls          int
	reasoningSetCalls int
}

func (a *chatConfigCommandAgent) Name() string               { return "codex" }
func (a *chatConfigCommandAgent) SupportsPerTurnModel() bool { return true }
func (a *chatConfigCommandAgent) GetModel() string           { return "default-model" }
func (a *chatConfigCommandAgent) SetModel(string)            { a.setCalls++ }
func (a *chatConfigCommandAgent) AvailableModels(context.Context) []core.ModelOption {
	return []core.ModelOption{{Name: "selected"}}
}
func (a *chatConfigCommandAgent) SupportsPerTurnReasoningEffort() bool { return true }
func (a *chatConfigCommandAgent) GetReasoningEffort() string           { return "medium" }
func (a *chatConfigCommandAgent) SetReasoningEffort(string)            { a.reasoningSetCalls++ }
func (a *chatConfigCommandAgent) AvailableReasoningEfforts() []string {
	return []string{"low", "medium", "high"}
}

type chatConfigCommandPlatform struct{ replies []string }

func (p *chatConfigCommandPlatform) Name() string                    { return "feishu" }
func (p *chatConfigCommandPlatform) Start(core.MessageHandler) error { return nil }
func (p *chatConfigCommandPlatform) Stop() error                     { return nil }
func (p *chatConfigCommandPlatform) Reply(_ context.Context, _ any, content string) error {
	p.replies = append(p.replies, content)
	return nil
}
func (p *chatConfigCommandPlatform) Send(ctx context.Context, target any, content string) error {
	return p.Reply(ctx, target, content)
}

func TestChatModelConfigWiring_CommandsResetModelAndReasoningIndependently(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		message   core.Message
	}{
		{"group-topic", "feishu:group-a", core.Message{SessionKey: "feishu:group-a:root:topic", GroupID: "group-a", UserID: "owner"}},
		{"verified-dm", "feishu:dm:owner", core.Message{SessionKey: "feishu:dm:owner", IsDirect: true, UserID: "owner"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
			store := core.NewProjectStateStore(statePath)
			if err := configureChatModels(store, "demo"); err != nil {
				t.Fatal(err)
			}
			agent, platform := &chatConfigCommandAgent{}, &chatConfigCommandPlatform{}
			engine := core.NewEngine("demo", agent, []core.Platform{platform}, filepath.Join(t.TempDir(), "sessions.json"), core.LangEnglish)
			engine.SetProjectStateStore(store)
			t.Cleanup(func() { _ = engine.Stop() })
			for i, step := range []struct{ command, model, effort, reply string }{
				{"/model selected", "selected", "", "Model for this chat set to `selected`"},
				{"/reasoning chat high", "selected", "high", "Reasoning effort for this chat set to `high`"},
				{"/model reset", "", "high", "This chat now follows the default model"},
				{"/reasoning", "", "high", "Fixed reasoning effort for this chat: `high`"},
				{"/model selected", "selected", "high", "Model for this chat set to `selected`"},
				{"/reasoning chat reset", "selected", "", "This chat follows the default reasoning effort again"},
				{"/model", "selected", "", "Current model: `selected`"},
				{"/model reset", "", "", "This chat now follows the default model"},
			} {
				msg := tc.message
				msg.Platform, msg.MessageID, msg.Content = "feishu", fmt.Sprint(i), step.command
				repliesBefore := len(platform.replies)
				engine.ReceiveMessage(platform, &msg)
				if len(platform.replies) <= repliesBefore || !strings.Contains(strings.Join(platform.replies[repliesBefore:], "\n"), step.reply) {
					t.Fatalf("%s: replies=%v; want %q", step.command, platform.replies[repliesBefore:], step.reply)
				}
				bindings, err := config.LoadChatModels("demo")
				if err != nil {
					t.Fatal(err)
				}
				binding, ok := bindings[tc.key]
				if binding.Model != step.model || binding.ReasoningEffort != step.effort || ok != (step.model != "" || step.effort != "") {
					t.Fatalf("%s: persisted bindings=%+v", step.command, bindings)
				}
				if ok && binding.Provider != "codex:" {
					t.Fatalf("%s: provider=%q", step.command, binding.Provider)
				}
				// Restart must preserve each independent reset, including a
				// reasoning-only entry after /model reset.
				restarted := core.NewProjectStateStore(statePath)
				if err := configureChatModels(restarted, "demo"); err != nil {
					t.Fatal(err)
				}
				for _, current := range []*core.ProjectStateStore{store, restarted} {
					model, modelBound := current.ChatModelBinding(tc.key)
					reasoning, reasoningBound := current.GroupReasoningBinding(tc.key)
					if model.Model != step.model || modelBound != (step.model != "") || reasoning.Effort != step.effort || reasoningBound != (step.effort != "") {
						t.Fatalf("%s: model=%+v (%v), reasoning=%+v (%v)", step.command, model, modelBound, reasoning, reasoningBound)
					}
				}
			}
			if agent.setCalls != 0 || agent.reasoningSetCalls != 0 {
				t.Fatal("chat command changed a shared model or reasoning default")
			}
		})
	}
}

const chatModelConfigFixture = `# keep this operator note
[[projects]]
name = "demo"
[projects.agent]
type = "codex"
[projects.agent.options]
work_dir = "/work"
[[projects.agent.providers]]
name = "cpa"
model = "default-model"
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
app_id = "fixture"
app_secret = "fixture"
`

func writeChatModelConfigFixture(t *testing.T, text string) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	old := config.ConfigPath
	config.ConfigPath = path
	t.Cleanup(func() { config.ConfigPath = old })
	return path, filepath.Join(root, "demo.state.json")
}

func TestChatModelMigration_BothVersionsConfigWinsAndResetDoesNotResurrect(t *testing.T) {
	path, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture+`
[[projects.chat_models]]
platform = "feishu"
chat_id = "group-a"
model = "manual"
provider = "cpa"
`)
	legacy := `{
	"group_model_bindings":{"feishu:group-a":{"model":"older","provider":"codex:cpa"},"feishu:group-b":{"model":"older-b","provider":"codex:cpa"},"feishu:dm:user-a":{"model":"private","provider":"codex:cpa"}},
	"chat_model_bindings":{"feishu:group-a":{"model":"old","provider":"codex:cpa"},"feishu:group-b":{"model":"second","provider":"codex:cpa"}},
	"work_dir_override":"/existing",
	"workspace_dir_overrides":{"design":"/existing/design"},
	"group_reasoning_bindings":{"feishu:group-a":{"effort":"high","provider":"codex:cpa"}},
	"future_setting":{"keep":true}
}`
	if err := os.WriteFile(statePath, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	if err := migrateChatModels("demo", statePath); err != nil {
		t.Fatal(err)
	}
	bindings, err := config.LoadChatModels("demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 3 || bindings["feishu:group-a"].Model != "manual" || bindings["feishu:group-a"].ReasoningEffort != "high" || bindings["feishu:group-b"].Model != "second" || bindings["feishu:dm:user-a"].Model != "private" {
		t.Fatalf("bindings = %+v", bindings)
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"group_model_bindings", "chat_model_bindings", "group_reasoning_bindings"} {
		if _, ok := state[key]; ok {
			t.Fatalf("legacy field survived: %s", key)
		}
	}
	var unrelated map[string]any
	if err := json.Unmarshal(data, &unrelated); err != nil {
		t.Fatal(err)
	}
	wantUnrelated := map[string]any{
		"work_dir_override":       "/existing",
		"workspace_dir_overrides": map[string]any{"design": "/existing/design"},
		"future_setting":          map[string]any{"keep": true},
	}
	if !reflect.DeepEqual(unrelated, wantUnrelated) {
		t.Fatalf("unrelated state changed: %+v", unrelated)
	}
	before, _ := os.ReadFile(path)
	if err := migrateChatModels("demo", statePath); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("second migration rewrote config")
	}
	if _, err := config.SaveChatModel("demo", "feishu:group-b", core.ChatModelBinding{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateChatModels("demo", statePath); err != nil {
		t.Fatal(err)
	}
	bindings, err = config.LoadChatModels("demo")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bindings["feishu:group-b"]; ok {
		t.Fatal("reset binding resurrected from legacy state")
	}
	if _, err := config.SaveChatReasoningEffort("demo", "feishu:group-a", core.GroupReasoningBinding{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateChatModels("demo", statePath); err != nil {
		t.Fatal(err)
	}
	store := core.NewProjectStateStore(statePath)
	if err := configureChatModels(store, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.GroupReasoningBinding("feishu:group-a"); ok {
		t.Fatal("reset reasoning resurrected from legacy state")
	}
	if got, _ := store.ChatModelBinding("feishu:group-a"); got.Model != "manual" {
		t.Fatalf("reasoning reset changed model: %+v", got)
	}
	if store.WorkDirOverride() != "/existing" || store.WorkspaceDirOverride("design") != "/existing/design" {
		t.Fatal("migration changed directory state")
	}
}

func TestChatModelMigration_FailurePreservesSource(t *testing.T) {
	for _, mode := range []string{"invalid-config", "write-failure", "invalid-state", "invalid-reasoning", "legacy-provider-conflict", "toml-provider-conflict"} {
		t.Run(mode, func(t *testing.T) {
			path, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
			legacy := `{"group_model_bindings":{"feishu:group-a":{"model":"older","provider":"codex:cpa"}},"chat_model_bindings":{"feishu:group-a":{"model":"old","provider":"codex:cpa"}},"group_reasoning_bindings":{"feishu:group-a":{"effort":"high","provider":"codex:cpa"}}}`
			switch mode {
			case "invalid-config":
				if err := os.WriteFile(path, []byte("invalid = ["), 0600); err != nil {
					t.Fatal(err)
				}
			case "write-failure":
				config.ConfigPath = filepath.Join(path, "impossible")
			case "invalid-state":
				legacy = `{"chat_model_bindings":42}`
			case "invalid-reasoning":
				legacy = strings.Replace(legacy, `"effort":"high"`, `"effort":42`, 1)
			case "legacy-provider-conflict":
				legacy = strings.Replace(legacy, `"effort":"high","provider":"codex:cpa"`, `"effort":"high","provider":"codex:other"`, 1)
			case "toml-provider-conflict":
				if err := os.WriteFile(path, []byte(chatModelConfigFixture+`
[[projects.chat_models]]
platform = "feishu"
chat_id = "group-a"
model = "manual"
provider = "other"
`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(statePath, []byte(legacy), 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := migrateChatModels("demo", statePath); err == nil {
				t.Fatal("expected migration failure")
			}
			data, err := os.ReadFile(statePath)
			if err != nil || string(data) != legacy {
				t.Fatalf("source changed on failure: %s, %v", data, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("config changed on failure: %s, %v", after, err)
			}
		})
	}
}

func TestChatModelMigration_ReasoningOnlyGroupAndVerifiedDM(t *testing.T) {
	_, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
	legacy := `{"group_reasoning_bindings":{"feishu:group-a":{"effort":"high","provider":"codex:cpa"},"feishu:dm:user-a":{"effort":"low","provider":"codex:cpa"}}}`
	if err := os.WriteFile(statePath, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	if err := migrateChatModels("demo", statePath); err != nil {
		t.Fatal(err)
	}
	bindings, err := config.LoadChatModels("demo")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]core.ChatModelBinding{
		"feishu:group-a":   {Provider: "codex:cpa", ReasoningEffort: "high"},
		"feishu:dm:user-a": {Provider: "codex:cpa", ReasoningEffort: "low"},
	}
	if !reflect.DeepEqual(bindings, want) {
		t.Fatalf("reasoning-only migration = %+v", bindings)
	}
	store := core.NewProjectStateStore(statePath)
	if err := configureChatModels(store, "demo"); err != nil {
		t.Fatal(err)
	}
	for key, binding := range want {
		if got, ok := store.GroupReasoningBinding(key); !ok || got.Effort != binding.ReasoningEffort || got.Provider != binding.Provider {
			t.Fatalf("reasoning not restored for %s: %+v", key, got)
		}
		if _, ok := store.ChatModelBinding(key); ok {
			t.Fatalf("reasoning-only migration fixed a model for %s", key)
		}
	}
	data, err := os.ReadFile(statePath)
	if err != nil || strings.TrimSpace(string(data)) != "{}" {
		t.Fatalf("legacy reasoning survived cleanup: %s, %v", data, err)
	}
}

func TestChatModelMigration_ExplicitTOMLReasoningWinsIncludingEmptyReset(t *testing.T) {
	for _, tc := range []struct {
		name, model, effort string
	}{
		{"explicit-effort", "manual", "low"},
		{"explicit-empty-reset", "manual", ""},
		{"reasoning-only-keeps-model-inherited", "", "low"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			modelLine := ""
			if tc.model != "" {
				modelLine = fmt.Sprintf("model = %q\n", tc.model)
			}
			_, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture+fmt.Sprintf(`
[[projects.chat_models]]
platform = "feishu"
chat_id = "group-a"
%sprovider = "cpa"
reasoning_effort = %q
`, modelLine, tc.effort))
			legacy := `{"chat_model_bindings":{"feishu:group-a":{"model":"old","provider":"codex:cpa"}},"group_reasoning_bindings":{"feishu:group-a":{"effort":"high","provider":"codex:cpa"}}}`
			if err := os.WriteFile(statePath, []byte(legacy), 0600); err != nil {
				t.Fatal(err)
			}
			if err := migrateChatModels("demo", statePath); err != nil {
				t.Fatal(err)
			}
			bindings, err := config.LoadChatModels("demo")
			if err != nil {
				t.Fatal(err)
			}
			want := core.ChatModelBinding{Model: tc.model, Provider: "codex:cpa", ReasoningEffort: tc.effort}
			if got := bindings["feishu:group-a"]; got != want {
				t.Fatalf("explicit TOML overridden: %+v; want %+v", got, want)
			}
		})
	}
}

func TestChatModelMigration_CleanupFailureKeepsSourceAndRetryKeepsCommittedConfig(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires an unprivileged POSIX user for directory permissions")
	}
	path, _ := writeChatModelConfigFixture(t, chatModelConfigFixture)
	stateDir := filepath.Join(filepath.Dir(path), "state")
	if err := os.Mkdir(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateDir, "demo.state.json")
	legacy := `{"group_model_bindings":{"feishu:group-a":{"model":"old","provider":"codex:cpa"}},"group_reasoning_bindings":{"feishu:group-a":{"effort":"high","provider":"codex:cpa"}},"future_setting":{"keep":true}}`
	if err := os.WriteFile(statePath, []byte(legacy), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(stateDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(stateDir, 0700) })
	if err := migrateChatModels("demo", statePath); err == nil || !strings.Contains(err.Error(), "finish chat preference migration") {
		t.Fatalf("expected cleanup failure after config commit: %v", err)
	}
	data, err := os.ReadFile(statePath)
	if err != nil || string(data) != legacy {
		t.Fatalf("cleanup failure changed source: %s, %v", data, err)
	}
	bindings, err := config.LoadChatModels("demo")
	if err != nil {
		t.Fatal(err)
	}
	if got := bindings["feishu:group-a"]; got.Model != "old" || got.ReasoningEffort != "high" {
		t.Fatalf("cleanup attempted before config commit: %+v", got)
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// An operator can reset reasoning in TOML before retrying startup. The
	// surviving legacy source must not resurrect it on the retry.
	edited := strings.Replace(string(committed), `reasoning_effort = "high"`, `reasoning_effort = ""`, 1)
	if edited == string(committed) {
		t.Fatal("committed config is missing the migrated effort")
	}
	if err := os.WriteFile(path, []byte(edited), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := migrateChatModels("demo", statePath); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != edited {
		t.Fatalf("retry changed explicit config: %s, %v", after, err)
	}
	data, err = os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, map[string]any{"future_setting": map[string]any{"keep": true}}) {
		t.Fatalf("retry changed unrelated state or retained legacy preferences: %+v", state)
	}
	info, err := os.Stat(statePath)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("cleanup changed state permissions: info=%v, err=%v", info, err)
	}
}

func TestChatModelMigration_RetainsDormantProviderAndPlatformIdentity(t *testing.T) {
	_, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
	legacy := `{"group_model_bindings":{"feishu:group-a":{"model":"old","provider":"codex:removed-provider"},"telegram:group-b":{"model":"other","provider":"original-agent:"}},"group_reasoning_bindings":{"feishu:group-a":{"effort":"high","provider":"codex:removed-provider"},"telegram:group-b":{"effort":"low","provider":"original-agent:"}}}`
	if err := os.WriteFile(statePath, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	if err := migrateChatModels("demo", statePath); err != nil {
		t.Fatalf("dormant binding blocked startup: %v", err)
	}
	bindings, err := config.LoadChatModels("demo")
	if err != nil {
		t.Fatal(err)
	}
	if bindings["feishu:group-a"].Provider != "codex:removed-provider" || bindings["telegram:group-b"].Provider != "original-agent:" || bindings["feishu:group-a"].ReasoningEffort != "high" || bindings["telegram:group-b"].ReasoningEffort != "low" {
		t.Fatalf("migration rebound stale identities: %+v", bindings)
	}
}

func TestChatModelConfigWiring_StartupCommandManualReloadAndRestart(t *testing.T) {
	path, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
	store := core.NewProjectStateStore(statePath)
	if err := configureChatModels(store, "demo"); err != nil {
		t.Fatal(err)
	}
	selection := core.ChatModelBinding{Model: "selected", Provider: "codex:"}
	if err := store.SaveChatModelBinding("feishu:group-a", selection); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveGroupReasoningBinding("feishu:group-a", core.GroupReasoningBinding{Effort: "high", Provider: "codex:"}); err != nil {
		t.Fatal(err)
	}
	restarted := core.NewProjectStateStore(statePath)
	if err := configureChatModels(restarted, "demo"); err != nil {
		t.Fatal(err)
	}
	if got, ok := restarted.ChatModelBinding("feishu:group-a"); !ok || got != selection {
		t.Fatalf("restart lost selection: %+v", got)
	}
	if got, ok := restarted.GroupReasoningBinding("feishu:group-a"); !ok || got.Effort != "high" || got.Provider != "codex:" {
		t.Fatalf("restart lost reasoning selection: %+v", got)
	}
	agent, platform := &chatConfigCommandAgent{}, &chatConfigCommandPlatform{}
	engine := core.NewEngine("demo", agent, []core.Platform{platform}, filepath.Join(t.TempDir(), "sessions.json"), core.LangEnglish)
	engine.SetProjectStateStore(store)
	t.Cleanup(func() { _ = engine.Stop() })
	messageID := 0
	assertVisible := func(command, want string) {
		t.Helper()
		messageID++
		before := len(platform.replies)
		engine.ReceiveMessage(platform, &core.Message{Platform: "feishu", SessionKey: "feishu:group-a:root:topic", GroupID: "group-a", UserID: "owner", MessageID: fmt.Sprint(messageID), Content: command})
		if len(platform.replies) <= before || !strings.Contains(strings.Join(platform.replies[before:], "\n"), want) {
			t.Fatalf("%s: replies=%v; want %q", command, platform.replies[before:], want)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.ReplaceAll(string(data), `model = "selected"`, `model = "manual"`)
	edited = strings.ReplaceAll(edited, `reasoning_effort = "high"`, `reasoning_effort = "low"`)
	if err := os.WriteFile(path, []byte(edited), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := reloadConfig(path, "demo", engine)
	if err != nil || result.ChatModelsUpdated != 1 {
		t.Fatalf("reload=%+v, %v", result, err)
	}
	if got, _ := store.ChatModelBinding("feishu:group-a"); got.Model != "manual" {
		t.Fatalf("manual edit ignored: %+v", got)
	}
	if got, _ := store.GroupReasoningBinding("feishu:group-a"); got.Effort != "low" {
		t.Fatalf("manual reasoning edit ignored: %+v", got)
	}
	assertVisible("/model", "Current model: `manual`")
	assertVisible("/reasoning", "Fixed reasoning effort for this chat: `low`")
	if err := os.WriteFile(path, []byte(edited+"\n[broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reloadConfig(path, "demo", engine); err == nil {
		t.Fatal("invalid reload accepted")
	}
	if got, _ := store.ChatModelBinding("feishu:group-a"); got.Model != "manual" {
		t.Fatalf("invalid reload changed model: %+v", got)
	}
	if got, _ := store.GroupReasoningBinding("feishu:group-a"); got.Effort != "low" {
		t.Fatalf("invalid reload changed reasoning: %+v", got)
	}
	// Removing only model must preserve the independently configured effort.
	reasoningOnly := strings.Replace(edited, "model = \"manual\"\n", "", 1)
	if err := os.WriteFile(path, []byte(reasoningOnly), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reloadConfig(path, "demo", engine); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.ChatModelBinding("feishu:group-a"); ok {
		t.Fatal("deleted model binding still active")
	}
	if got, _ := store.GroupReasoningBinding("feishu:group-a"); got.Effort != "low" {
		t.Fatalf("removing model changed reasoning: %+v", got)
	}
	assertVisible("/model", "Current model: `default-model`")
	assertVisible("/reasoning", "Fixed reasoning effort for this chat: `low`")
	// Removing all preference tables must restore both inherited defaults.
	if err := os.WriteFile(path, []byte(chatModelConfigFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reloadConfig(path, "demo", engine); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.ChatModelBinding("feishu:group-a"); ok {
		t.Fatal("deleted binding still active")
	}
	if _, ok := store.GroupReasoningBinding("feishu:group-a"); ok {
		t.Fatal("deleted reasoning binding still active")
	}
	assertVisible("/model", "Current model: `default-model`")
	assertVisible("/reasoning", "Reasoning effort for this chat: following the workspace/project default")
	restarted = core.NewProjectStateStore(statePath)
	if err := configureChatModels(restarted, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := restarted.ChatModelBinding("feishu:group-a"); ok {
		t.Fatal("restart resurrected a deleted model binding")
	}
	if _, ok := restarted.GroupReasoningBinding("feishu:group-a"); ok {
		t.Fatal("restart resurrected a deleted reasoning binding")
	}
}
