package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

const chatModelsFixture = `# preserved header
[[projects]]
name = "alpha"
unknown_setting = "keep-me"
[projects.agent]
type = "codex"
[projects.agent.options]
provider = "cpa"
model = "default-a"
[[projects.agent.providers]]
name = "cpa"
api_key = "${CHAT_MODELS_TEST_SECRET}"
model = "default-a"
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
app_id = "test"
app_secret = "test"

[[projects]]
name = "beta"
[projects.agent]
type = "codex"
[[projects.platforms]]
type = "feishu"

# preserved global table
[log]
level = "debug"
`

func setupChatModelsConfig(t *testing.T, raw string) string {
	t.Helper()
	t.Setenv("CHAT_MODELS_TEST_SECRET", "must-not-expand-on-disk")
	path := writeConfigFixture(t, raw)
	patchConfigPath(t, path)
	return path
}

func TestChatModels_SaveUpdateResetPreservesProjectAndConfig(t *testing.T) {
	path := setupChatModelsConfig(t, chatModelsFixture)
	want := map[string]core.ChatModelBinding{
		"feishu:group-a":   {Model: "model-a", Provider: "codex:cpa"},
		"feishu:group-b":   {Model: "model-b", Provider: "codex:cpa"},
		"feishu:dm:user-a": {Model: "model-private", Provider: "codex:cpa"},
	}
	for key, binding := range want {
		if _, err := SaveChatModel("alpha", key, binding); err != nil {
			t.Fatal(err)
		}
	}
	beta := core.ChatModelBinding{Model: "native-model", Provider: "codex:"}
	if _, err := SaveChatModel("beta", "feishu:group-a", beta); err != nil {
		t.Fatal(err)
	}
	got, err := LoadChatModels("alpha")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded = %#v, %v; want %#v", got, err, want)
	}
	updated := core.ChatModelBinding{Model: "replacement", Provider: "codex:cpa"}
	want["feishu:group-a"] = updated
	if got, err = SaveChatModel("alpha", "feishu:group-a", updated); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("update = %#v, %v", got, err)
	}
	if _, err := SaveChatModel("alpha", "feishu:group-a", core.ChatModelBinding{}); err != nil {
		t.Fatal(err)
	}
	delete(want, "feishu:group-a")
	got, err = LoadChatModels("alpha")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("reset = %#v, %v", got, err)
	}
	for key := range want {
		if _, err := SaveChatModel("alpha", key, core.ChatModelBinding{}); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := LoadChatModels("alpha"); err != nil || len(got) != 0 {
		t.Fatalf("last reset = %#v, %v", got, err)
	}
	if got, err := LoadChatModels("beta"); err != nil || got["feishu:group-a"] != beta {
		t.Fatalf("other project changed = %#v, %v", got, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, retained := range []string{"# preserved header", `unknown_setting = "keep-me"`, "${CHAT_MODELS_TEST_SECRET}", "# preserved global table", "[log]", `level = "debug"`, `model = "default-a"`} {
		if !strings.Contains(string(data), retained) {
			t.Errorf("lost %q in %s", retained, data)
		}
	}
	if strings.Contains(string(data), "must-not-expand-on-disk") {
		t.Fatal("expanded secret while saving unrelated config")
	}
}

func TestChatModels_ConcurrentSavesAndFreshReload(t *testing.T) {
	path := setupChatModelsConfig(t, chatModelsFixture)
	var wg sync.WaitGroup
	for _, key := range []string{"feishu:a", "feishu:b"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			if _, err := SaveChatModel("alpha", key, core.ChatModelBinding{Model: key, Provider: "codex:cpa"}); err != nil {
				t.Error(err)
			}
		}(key)
	}
	wg.Wait()
	got, err := LoadChatModels("alpha")
	if err != nil || len(got) != 2 {
		t.Fatalf("lost concurrent save: %#v, %v", got, err)
	}
	data, _ := os.ReadFile(path)
	data = []byte(strings.ReplaceAll(string(data), `model = "feishu:a"`, `model = "manual-edit"`))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = LoadChatModels("alpha")
	if err != nil || got["feishu:a"].Model != "manual-edit" {
		t.Fatalf("manual edit not loaded: %#v, %v", got, err)
	}
}

func TestChatModels_EmptyArrayAndNestedUnknownReset(t *testing.T) {
	raw := strings.Replace(chatModelsFixture, `name = "alpha"`, "name = \"alpha\"\nchat_models = [] # empty preferences", 1)
	path := setupChatModelsConfig(t, raw)
	if _, err := SaveChatModel("alpha", "feishu:a", core.ChatModelBinding{Model: "model", Provider: "codex:cpa"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	data = []byte(strings.Replace(string(data), "[projects.agent]", "[projects.chat_models.extra]\nvalue = \"binding-owned\"\n[projects.agent]", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveChatModel("alpha", "feishu:a", core.ChatModelBinding{}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "binding-owned") || !strings.Contains(string(data), "# empty preferences") {
		t.Fatalf("reset lost comments or retained orphan child table: %s", data)
	}
}

func TestChatModels_LoadManualNativeAndReferencedProvider(t *testing.T) {
	raw := `[[providers]]
name = "shared"
[[projects]]
name = "alpha"
[[projects.chat_models]]
platform = "feishu"
chat_id = "group"
model = "shared-model"
provider = "shared"
[[projects.chat_models]]
platform = "feishu"
user_id = "user"
model = "native-model"
agent = "original-agent"
[projects.agent]
type = "codex"
provider_refs = ["shared"]
[[projects.platforms]]
type = "feishu"
`
	setupChatModelsConfig(t, raw)
	got, err := LoadChatModels("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got["feishu:group"] != (core.ChatModelBinding{Model: "shared-model", Provider: "codex:shared"}) || got["feishu:dm:user"].Provider != "original-agent:" {
		t.Fatalf("provider identity not retained: %#v", got)
	}
}

func TestChatModels_Validation(t *testing.T) {
	valid := ChatModelConfig{Platform: "telegram", ChatID: "group", Model: "model"}
	for _, tc := range []struct {
		name   string
		modify func(*ChatModelConfig)
	}{
		{"missing target", func(c *ChatModelConfig) { c.ChatID = "" }},
		{"ambiguous target", func(c *ChatModelConfig) { c.UserID = "user" }},
		{"empty model", func(c *ChatModelConfig) { c.Model = "" }},
		{"whitespace", func(c *ChatModelConfig) { c.ChatID = " group " }},
		{"empty platform", func(c *ChatModelConfig) { c.Platform = "" }},
		{"compound platform", func(c *ChatModelConfig) { c.Platform = "feishu:chat" }},
		{"reserved namespace", func(c *ChatModelConfig) { c.ChatID = "dm:user" }},
		{"compound agent", func(c *ChatModelConfig) { c.Agent = "codex:cpa" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.modify(&c)
			proj := validProject("alpha")
			proj.ChatModels = []ChatModelConfig{c}
			cfg := Config{Projects: []ProjectConfig{proj}}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "chat_models") {
				t.Fatalf("invalid binding accepted: %#v, %v", c, err)
			}
		})
	}
	proj := validProject("alpha")
	proj.ChatModels = []ChatModelConfig{valid, valid}
	if err := (&Config{Projects: []ProjectConfig{proj}}).Validate(); err == nil {
		t.Fatal("duplicate target accepted")
	}
	path := setupChatModelsConfig(t, strings.Replace(chatModelsFixture, `name = "alpha"`, "name = \"alpha\"\nchat_models = 42", 1))
	if _, err := LoadChatModels("alpha"); err == nil {
		t.Fatalf("invalid field type accepted in %s", path)
	}
}

func TestChatModels_ResetOwnsUnknownChildArrayTables(t *testing.T) {
	path := setupChatModelsConfig(t, chatModelsFixture)
	for _, key := range []string{"feishu:group-b", "feishu:group-a"} {
		if _, err := SaveChatModel("alpha", key, core.ChatModelBinding{Model: key, Provider: "codex:cpa"}); err != nil {
			t.Fatal(err)
		}
	}
	// Insertion puts A before B. Attach an unknown future array table to B.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "[projects.agent]", "[[projects.chat_models.extra]]\nowner = \"B-only\"\n\n[projects.agent]", 1))
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	bindings, err := SaveChatModel("alpha", "feishu:group-b", core.ChatModelBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings["feishu:group-a"].Model != "feishu:group-a" {
		t.Fatalf("bindings=%+v", bindings)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "B-only") || strings.Contains(string(after), "chat_models.extra") {
		t.Fatal("reset reattached B's child array to A")
	}
}

func TestChatModels_InvalidChangeDoesNotWrite(t *testing.T) {
	path := setupChatModelsConfig(t, chatModelsFixture)
	before, _ := os.ReadFile(path)
	for _, tc := range []struct {
		project, key string
		binding      core.ChatModelBinding
	}{
		{"missing", "feishu:a", core.ChatModelBinding{Model: "model", Provider: "codex:cpa"}},
		{"alpha", "missing-separator", core.ChatModelBinding{Model: "model", Provider: "codex:cpa"}},
		{"alpha", "feishu:a", core.ChatModelBinding{Model: "bad model", Provider: "codex:cpa"}},
		{"alpha", "feishu:a", core.ChatModelBinding{Model: "model", Provider: "cpa"}},
		{"alpha", "feishu:dm:", core.ChatModelBinding{Model: "model", Provider: "codex:cpa"}},
	} {
		if _, err := SaveChatModel(tc.project, tc.key, tc.binding); err == nil {
			t.Fatalf("invalid save succeeded: %#v", tc)
		}
		after, _ := os.ReadFile(path)
		if string(before) != string(after) {
			t.Fatal("invalid save changed config")
		}
	}
}

func TestChatModels_WriteFailureKeepsOldConfig(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires an unprivileged POSIX user for directory permissions")
	}
	path := setupChatModelsConfig(t, chatModelsFixture)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	got, err := SaveChatModel("alpha", "feishu:a", core.ChatModelBinding{Model: "new", Provider: "codex:cpa"})
	if err == nil || got != nil {
		t.Fatalf("write failure returned a new snapshot: %#v, %v", got, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("write failure changed config: %v", err)
	}
}

func TestChatModels_MergeLegacyExplicitWinsAndIdempotent(t *testing.T) {
	path := setupChatModelsConfig(t, chatModelsFixture)
	explicit := core.ChatModelBinding{Model: "manual", Provider: "codex:cpa"}
	if _, err := SaveChatModel("alpha", "feishu:a", explicit); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]core.ChatModelBinding{
		"feishu:a":       {Model: "old", Provider: "codex:cpa"},
		"feishu:b":       {Model: "imported", Provider: "codex:cpa"},
		"feishu:dm:user": {Model: "private", Provider: "codex:cpa"},
	}
	if err := MergeLegacyChatModels("alpha", legacy, nil); err != nil {
		t.Fatal(err)
	}
	got, err := LoadChatModels("alpha")
	if err != nil || len(got) != 3 || got["feishu:a"] != explicit || got["feishu:b"] != legacy["feishu:b"] || got["feishu:dm:user"] != legacy["feishu:dm:user"] {
		t.Fatalf("migration = %#v, %v", got, err)
	}
	before, _ := os.ReadFile(path)
	if err := MergeLegacyChatModels("alpha", legacy, nil); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("second migration changed config")
	}
}
