package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

type chatConfigCommandAgent struct {
	stubMainAgent
	setCalls int
}

func (a *chatConfigCommandAgent) Name() string               { return "codex" }
func (a *chatConfigCommandAgent) SupportsPerTurnModel() bool { return true }
func (a *chatConfigCommandAgent) GetModel() string           { return "default-model" }
func (a *chatConfigCommandAgent) SetModel(string)            { a.setCalls++ }
func (a *chatConfigCommandAgent) AvailableModels(context.Context) []core.ModelOption {
	return []core.ModelOption{{Name: "selected"}}
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

func TestChatModelConfigWiring_CommandsPersistWithoutChangingDefault(t *testing.T) {
	_, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
	store := core.NewProjectStateStore(statePath)
	if err := configureChatModels(store, "demo"); err != nil {
		t.Fatal(err)
	}
	agent, platform := &chatConfigCommandAgent{}, &chatConfigCommandPlatform{}
	engine := core.NewEngine("demo", agent, []core.Platform{platform}, filepath.Join(t.TempDir(), "sessions.json"), core.LangEnglish)
	engine.SetProjectStateStore(store)
	t.Cleanup(func() { _ = engine.Stop() })
	for _, model := range []string{"selected", "reset"} {
		engine.ReceiveMessage(platform, &core.Message{Platform: "feishu", SessionKey: "feishu:group-a:root:topic", GroupID: "group-a", UserID: "owner", MessageID: model, Content: "/model " + model})
		bindings, err := config.LoadChatModels("demo")
		if err != nil {
			t.Fatal(err)
		}
		binding, ok := bindings["feishu:group-a"]
		if model == "selected" && (!ok || binding.Model != model || binding.Provider != "codex:") {
			t.Fatalf("command not persisted: %+v; replies=%v", bindings, platform.replies)
		}
		if model == "reset" && ok {
			t.Fatal("reset left a binding")
		}
	}
	if agent.setCalls != 0 {
		t.Fatal("chat command changed shared default")
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
	"group_model_bindings":{"feishu:group-a":{"model":"older","provider":"codex:cpa"},"feishu:dm:user-a":{"model":"private","provider":"codex:cpa"}},
	"chat_model_bindings":{"feishu:group-a":{"model":"old","provider":"codex:cpa"},"feishu:group-b":{"model":"second","provider":"codex:cpa"}},
	"work_dir_override":"/existing",
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
	if len(bindings) != 3 || bindings["feishu:group-a"].Model != "manual" || bindings["feishu:group-b"].Model != "second" || bindings["feishu:dm:user-a"].Model != "private" {
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
	for _, key := range []string{"group_model_bindings", "chat_model_bindings"} {
		if _, ok := state[key]; ok {
			t.Fatalf("legacy field survived: %s", key)
		}
	}
	for _, key := range []string{"work_dir_override", "group_reasoning_bindings", "future_setting"} {
		if _, ok := state[key]; !ok {
			t.Fatalf("unrelated field removed: %s", key)
		}
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
}

func TestChatModelMigration_FailurePreservesSource(t *testing.T) {
	for _, mode := range []string{"invalid-config", "write-failure", "invalid-state"} {
		t.Run(mode, func(t *testing.T) {
			path, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
			legacy := `{"group_model_bindings":{"feishu:group-a":{"model":"old","provider":"codex:cpa"}}}`
			switch mode {
			case "invalid-config":
				if err := os.WriteFile(path, []byte("invalid = ["), 0600); err != nil {
					t.Fatal(err)
				}
			case "write-failure":
				config.ConfigPath = filepath.Join(path, "impossible")
			case "invalid-state":
				legacy = `{"chat_model_bindings":42}`
			}
			if err := os.WriteFile(statePath, []byte(legacy), 0600); err != nil {
				t.Fatal(err)
			}
			if err := migrateChatModels("demo", statePath); err == nil {
				t.Fatal("expected migration failure")
			}
			data, err := os.ReadFile(statePath)
			if err != nil || string(data) != legacy {
				t.Fatalf("source changed on failure: %s, %v", data, err)
			}
		})
	}
}

func TestChatModelMigration_RetainsDormantProviderAndPlatformIdentity(t *testing.T) {
	_, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
	legacy := `{"group_model_bindings":{"feishu:group-a":{"model":"old","provider":"codex:removed-provider"},"telegram:group-b":{"model":"other","provider":"original-agent:"}}}`
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
	if bindings["feishu:group-a"].Provider != "codex:removed-provider" || bindings["telegram:group-b"].Provider != "original-agent:" {
		t.Fatalf("migration rebound stale identities: %+v", bindings)
	}
}

func TestChatModelConfigWiring_StartupCommandManualReloadAndRestart(t *testing.T) {
	path, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
	store := core.NewProjectStateStore(statePath)
	if err := configureChatModels(store, "demo"); err != nil {
		t.Fatal(err)
	}
	selection := core.ChatModelBinding{Model: "selected", Provider: "codex:cpa"}
	if err := store.SaveChatModelBinding("feishu:group-a", selection); err != nil {
		t.Fatal(err)
	}
	restarted := core.NewProjectStateStore(statePath)
	if err := configureChatModels(restarted, "demo"); err != nil {
		t.Fatal(err)
	}
	if got, ok := restarted.ChatModelBinding("feishu:group-a"); !ok || got != selection {
		t.Fatalf("restart lost selection: %+v", got)
	}
	engine := core.NewEngine("demo", &stubMainAgent{}, nil, filepath.Join(t.TempDir(), "sessions.json"), core.LangEnglish)
	engine.SetProjectStateStore(store)
	t.Cleanup(func() { _ = engine.Stop() })
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.ReplaceAll(string(data), `model = "selected"`, `model = "manual"`)
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
	// Removing all model tables must restore inheritance rather than merge.
	if err := os.WriteFile(path, []byte(chatModelConfigFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reloadConfig(path, "demo", engine); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.ChatModelBinding("feishu:group-a"); ok {
		t.Fatal("deleted binding still active")
	}
	if err := os.WriteFile(path, []byte(edited+"\n[broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reloadConfig(path, "demo", engine); err == nil {
		t.Fatal("invalid reload accepted")
	}
	if _, ok := store.ChatModelBinding("feishu:group-a"); ok {
		t.Fatal("invalid reload changed runtime")
	}
}
