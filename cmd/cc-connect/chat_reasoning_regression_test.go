package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

type reasoningConfigRegressionAgent struct{ stubMainAgent }

func (*reasoningConfigRegressionAgent) Name() string                         { return "codex" }
func (*reasoningConfigRegressionAgent) SupportsPerTurnReasoningEffort() bool { return true }
func (*reasoningConfigRegressionAgent) GetReasoningEffort() string           { return "medium" }
func (*reasoningConfigRegressionAgent) SetReasoningEffort(string)            {}
func (*reasoningConfigRegressionAgent) AvailableReasoningEfforts() []string {
	return []string{"medium", "high"}
}

// This user-reported regression can also be run unchanged on the pre-fix code:
// a successful chat effort command must update config.toml, not project JSON.
func TestReasoningChatConfig_CommandWritesTOMLInsteadOfState(t *testing.T) {
	path, statePath := writeChatModelConfigFixture(t, chatModelConfigFixture)
	store := core.NewProjectStateStore(statePath)
	if err := configureChatModels(store, "demo"); err != nil {
		t.Fatal(err)
	}
	platform := &chatConfigCommandPlatform{}
	engine := core.NewEngine("demo", &reasoningConfigRegressionAgent{}, []core.Platform{platform}, filepath.Join(t.TempDir(), "sessions.json"), core.LangEnglish)
	engine.SetProjectStateStore(store)
	t.Cleanup(func() { _ = engine.Stop() })
	engine.ReceiveMessage(platform, &core.Message{Platform: "feishu", SessionKey: "feishu:group", GroupID: "group", UserID: "owner", MessageID: "set-effort", Content: "/reasoning chat high"})
	if !strings.Contains(strings.Join(platform.replies, "\n"), "set to `high`") {
		t.Fatalf("command did not succeed: %v", platform.replies)
	}
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), `reasoning_effort = "high"`) {
		t.Fatal("successful chat effort was not written to config.toml")
	}
	if state, err := os.ReadFile(statePath); err == nil && strings.Contains(string(state), "group_reasoning_bindings") {
		t.Fatal("chat effort was persisted as runtime state")
	}
}
