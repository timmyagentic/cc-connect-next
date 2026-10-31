package core

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatReasoningConfig_AuthoritativeReloadAndDurableSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"group_reasoning_bindings":{"test:old":{"effort":"max"}},"work_dir_override":"/work"}`), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewProjectStateStore(path)
	disk := map[string]ChatModelBinding{
		"test:group":   {Model: "selected", Provider: "agent:provider", ReasoningEffort: "high"},
		"test:dm:user": {Provider: "agent:provider", ReasoningEffort: "low"},
	}
	errIO := errors.New("configuration unavailable")
	failLoad, failSave := false, false
	load := func() (map[string]ChatModelBinding, error) {
		if failLoad {
			return nil, errIO
		}
		return maps.Clone(disk), nil
	}
	saveReasoning := func(key string, effort GroupReasoningBinding) (map[string]ChatModelBinding, error) {
		if failSave {
			return nil, errIO
		}
		binding := disk[key]
		binding.ReasoningEffort = effort.Effort
		disk[key] = binding
		return maps.Clone(disk), nil
	}
	if err := store.ConfigureChatModels(load, func(string, ChatModelBinding) (map[string]ChatModelBinding, error) { return nil, errIO }, saveReasoning); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.GroupReasoningBinding("test:old"); ok {
		t.Fatal("legacy state overrode config")
	}
	if _, ok := store.ChatModelBinding("test:dm:user"); ok {
		t.Fatal("effort-only config appeared as a fixed model")
	}
	if got, ok := store.GroupReasoningBinding("test:dm:user"); !ok || got.Effort != "low" {
		t.Fatalf("private effort = %+v, %v", got, ok)
	}
	if err := store.SaveGroupReasoningBinding("test:group", GroupReasoningBinding{Effort: "medium", Provider: "agent:provider"}); err != nil {
		t.Fatal(err)
	}
	if disk["test:group"].Model != "selected" || disk["test:group"].ReasoningEffort != "medium" {
		t.Fatal("effort save changed the model or skipped config")
	}
	failSave = true
	if err := store.SaveGroupReasoningBinding("test:group", GroupReasoningBinding{}); !errors.Is(err, errIO) {
		t.Fatal(err)
	}
	if got, _ := store.GroupReasoningBinding("test:group"); got.Effort != "medium" {
		t.Fatal("failed save changed published effort")
	}
	failLoad = true
	if _, err := store.ReloadChatModels(); !errors.Is(err, errIO) {
		t.Fatal(err)
	}
	if got, _ := store.GroupReasoningBinding("test:group"); got.Effort != "medium" {
		t.Fatal("failed reload changed effort")
	}
	failLoad = false
	disk["test:group"] = ChatModelBinding{Model: "manual", Provider: "agent:provider", ReasoningEffort: "high"}
	delete(disk, "test:dm:user")
	if _, err := store.ReloadChatModels(); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.ChatModelBinding("test:group"); got.Model != "manual" {
		t.Fatal("model was not reloaded")
	}
	if got, _ := store.GroupReasoningBinding("test:group"); got.Effort != "high" {
		t.Fatal("effort was not reloaded")
	}
	if _, ok := store.GroupReasoningBinding("test:dm:user"); ok {
		t.Fatal("deleted preference survived reload")
	}
	store.SetWorkspaceDirOverride("workspace", "/another")
	store.Save()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "model_bindings") || strings.Contains(string(data), "reasoning_bindings") {
		t.Fatalf("JSON became a second preference source: %s", data)
	}
	if store.WorkDirOverride() != "/work" {
		t.Fatal("directory state was lost")
	}
}
