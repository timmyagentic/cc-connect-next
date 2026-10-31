package config

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestChatReasoningConfig_IndependentSettingsAndReset(t *testing.T) {
	for _, key := range []string{"feishu:group", "feishu:dm:user"} {
		t.Run(key, func(t *testing.T) {
			path := setupChatModelsConfig(t, chatModelsFixture)
			model := core.ChatModelBinding{Model: "selected", Provider: "codex:cpa"}
			effort := core.GroupReasoningBinding{Effort: "high", Provider: "codex:cpa"}
			if _, err := SaveChatReasoningEffort("alpha", key, effort); err != nil {
				t.Fatal(err)
			}
			if _, err := SaveChatModel("alpha", key, model); err != nil {
				t.Fatal(err)
			}
			got, err := LoadChatModels("alpha")
			if err != nil || got[key].Model != model.Model || got[key].ReasoningEffort != "high" {
				t.Fatalf("combined = %+v, %v", got, err)
			}
			if _, err := SaveChatModel("alpha", key, core.ChatModelBinding{}); err != nil {
				t.Fatal(err)
			}
			got, err = LoadChatModels("alpha")
			if err != nil || got[key].Model != "" || got[key].ReasoningEffort != "high" || got[key].Provider != effort.Provider {
				t.Fatalf("model reset lost effort: %+v, %v", got, err)
			}
			if _, err := SaveChatModel("alpha", key, model); err != nil {
				t.Fatal(err)
			}
			if _, err := SaveChatReasoningEffort("alpha", key, core.GroupReasoningBinding{}); err != nil {
				t.Fatal(err)
			}
			got, err = LoadChatModels("alpha")
			if err != nil || got[key] != model {
				t.Fatalf("effort reset lost model: %+v, %v", got, err)
			}
			if _, err := SaveChatModel("alpha", key, core.ChatModelBinding{}); err != nil {
				t.Fatal(err)
			}
			got, err = LoadChatModels("alpha")
			if err != nil || len(got) != 0 {
				t.Fatalf("last reset left binding: %+v, %v", got, err)
			}
			data, _ := os.ReadFile(path)
			if !strings.Contains(string(data), "# preserved header") {
				t.Fatal("operator comment lost")
			}
		})
	}
}

func TestChatReasoningConfig_ConcurrentFieldsPreserveEachOther(t *testing.T) {
	setupChatModelsConfig(t, chatModelsFixture)
	var wg sync.WaitGroup
	for _, save := range []func() error{
		func() error {
			_, err := SaveChatModel("alpha", "feishu:group", core.ChatModelBinding{Model: "selected", Provider: "codex:cpa"})
			return err
		},
		func() error {
			_, err := SaveChatReasoningEffort("alpha", "feishu:group", core.GroupReasoningBinding{Effort: "high", Provider: "codex:cpa"})
			return err
		},
	} {
		wg.Add(1)
		go func(save func() error) {
			defer wg.Done()
			if err := save(); err != nil {
				t.Error(err)
			}
		}(save)
	}
	wg.Wait()
	got, err := LoadChatModels("alpha")
	if err != nil || got["feishu:group"].Model != "selected" || got["feishu:group"].ReasoningEffort != "high" {
		t.Fatalf("concurrent fields = %+v, %v", got, err)
	}
}

func TestChatReasoningConfig_ProviderConflictDoesNotRetargetOtherField(t *testing.T) {
	path := setupChatModelsConfig(t, chatModelsFixture)
	if _, err := SaveChatModel("alpha", "feishu:group", core.ChatModelBinding{Model: "selected", Provider: "codex:cpa"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if _, err := SaveChatReasoningEffort("alpha", "feishu:group", core.GroupReasoningBinding{Effort: "high", Provider: "codex:other"}); err == nil {
		t.Fatal("conflicting provider accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("conflict changed config")
	}
	if _, err := SaveChatReasoningEffort("alpha", "feishu:group", core.GroupReasoningBinding{Effort: "high", Provider: "codex:cpa"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveChatModel("alpha", "feishu:group", core.ChatModelBinding{Model: "other", Provider: "codex:other"}); err == nil {
		t.Fatal("model change retargeted effort")
	}
}

func TestChatReasoningConfig_ValidationDoesNotChangeFile(t *testing.T) {
	for _, value := range []string{`reasoning_effort = 42`, `reasoning_effort = "high extra"`, `reasoning_effort = "high\u0000"`} {
		setupChatModelsConfig(t, chatModelsFixture+"\n[[projects.chat_models]]\nplatform = \"feishu\"\nchat_id = \"group\"\n"+value+"\n")
		if _, err := LoadChatModels("alpha"); err == nil {
			t.Fatalf("invalid effort accepted: %s", value)
		}
	}
	path := setupChatModelsConfig(t, chatModelsFixture)
	before, _ := os.ReadFile(path)
	if _, err := SaveChatReasoningEffort("alpha", "feishu:dm:", core.GroupReasoningBinding{Effort: "high", Provider: "codex:cpa"}); err == nil {
		t.Fatal("empty private user accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid effort write changed configuration")
	}
}

func TestChatReasoningConfig_PreservesOtherFieldsEnvironmentPlaceholder(t *testing.T) {
	for _, field := range []string{"model", "reasoning_effort", "model-reset", "effort-reset", "migration"} {
		t.Run(field, func(t *testing.T) {
			t.Setenv("CHAT_SELECTED_MODEL", "selected")
			t.Setenv("CHAT_SELECTED_EFFORT", "high")
			t.Setenv("CHAT_SELECTED_PROVIDER", "cpa")
			t.Setenv("CHAT_SELECTED_AGENT", "codex")
			entry := `
[[projects.chat_models]]
platform = "feishu"
chat_id = "group"
provider = "${CHAT_SELECTED_PROVIDER}"
agent = "${CHAT_SELECTED_AGENT}"
model = "${CHAT_SELECTED_MODEL}"
reasoning_effort = "${CHAT_SELECTED_EFFORT}"
`
			if field == "migration" {
				entry = strings.ReplaceAll(entry, `reasoning_effort = "${CHAT_SELECTED_EFFORT}"`, "")
			}
			raw := strings.Replace(chatModelsFixture, "[[projects]]\nname = \"beta\"", entry+"\n[[projects]]\nname = \"beta\"", 1)
			path := setupChatModelsConfig(t, raw)
			preserved := "${CHAT_SELECTED_MODEL}"
			var err error
			switch field {
			case "model":
				preserved = "${CHAT_SELECTED_EFFORT}"
				_, err = SaveChatModel("alpha", "feishu:group", core.ChatModelBinding{Model: "replacement", Provider: "codex:cpa"})
			case "model-reset":
				preserved = "${CHAT_SELECTED_EFFORT}"
				_, err = SaveChatModel("alpha", "feishu:group", core.ChatModelBinding{})
			case "reasoning_effort":
				_, err = SaveChatReasoningEffort("alpha", "feishu:group", core.GroupReasoningBinding{Effort: "low", Provider: "codex:cpa"})
			case "effort-reset":
				_, err = SaveChatReasoningEffort("alpha", "feishu:group", core.GroupReasoningBinding{})
			case "migration":
				err = MergeLegacyChatModels("alpha", nil, map[string]core.GroupReasoningBinding{"feishu:group": {Effort: "high", Provider: "codex:cpa"}})
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, placeholder := range []string{preserved, "${CHAT_SELECTED_PROVIDER}", "${CHAT_SELECTED_AGENT}"} {
				if !strings.Contains(string(data), placeholder) {
					t.Fatalf("%s replaced untouched placeholder %s", field, placeholder)
				}
			}
		})
	}
}
