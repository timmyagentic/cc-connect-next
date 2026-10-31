package core

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestChatModelConfig_ReplacesLegacyStateAndNeverWritesItBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"chat_model_bindings":{"test:A":{"model":"old"}},"work_dir_override":"/work"}`), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewProjectStateStore(path)
	disk := map[string]ChatModelBinding{"test:B": {Model: "configured"}}
	load := func() (map[string]ChatModelBinding, error) { return maps.Clone(disk), nil }
	if err := store.ConfigureChatModels(load, func(key string, value ChatModelBinding) (map[string]ChatModelBinding, error) {
		if value.Model == "" {
			delete(disk, key)
		} else {
			disk[key] = value
		}
		return maps.Clone(disk), nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	assertChatModelBinding(t, store, "test:A", ChatModelBinding{}, false)
	if err := store.SaveChatModelBinding("test:C", ChatModelBinding{Model: "selected"}); err != nil {
		t.Fatal(err)
	}
	if disk["test:C"].Model != "selected" {
		t.Fatal("selection was not persisted")
	}
	delete(disk, "test:B")
	if n, err := store.ReloadChatModels(); err != nil || n != 1 {
		t.Fatalf("reload = %d, %v", n, err)
	}
	assertChatModelBinding(t, store, "test:B", ChatModelBinding{}, false)
	store.SetWorkspaceDirOverride("workspace", "/another")
	store.Save()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "model_bindings") {
		t.Fatalf("JSON became a second source: %s", data)
	}
	if store.WorkDirOverride() != "/work" {
		t.Fatal("unrelated state lost")
	}
	if err := store.SaveChatModelBinding("test:C", ChatModelBinding{}); err != nil {
		t.Fatal(err)
	}
	if len(disk) != 0 {
		t.Fatal("reset did not remove configuration")
	}
}

func TestChatModelConfig_FailedSaveOrReloadKeepsPublishedBindings(t *testing.T) {
	store := NewProjectStateStore("")
	failLoad := false
	old := ChatModelBinding{Model: "old"}
	errIO := errors.New("write failed")
	if err := store.ConfigureChatModels(func() (map[string]ChatModelBinding, error) {
		if failLoad {
			return nil, errIO
		}
		return map[string]ChatModelBinding{"test:A": old}, nil
	}, func(string, ChatModelBinding) (map[string]ChatModelBinding, error) { return nil, errIO }, nil); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []ChatModelBinding{{Model: "new"}, {}} {
		if err := store.SaveChatModelBinding("test:A", replacement); !errors.Is(err, errIO) {
			t.Fatal(err)
		}
		assertChatModelBinding(t, store, "test:A", old, true)
	}
	failLoad = true
	if _, err := store.ReloadChatModels(); !errors.Is(err, errIO) {
		t.Fatal(err)
	}
	assertChatModelBinding(t, store, "test:A", old, true)
}

func TestChatModelConfig_ReloadAndSaveSerializeReadThroughPublication(t *testing.T) {
	store := NewProjectStateStore("")
	disk := map[string]ChatModelBinding{"test:A": {Model: "old"}}
	reading, release := make(chan struct{}), make(chan struct{})
	block := false
	if err := store.ConfigureChatModels(func() (map[string]ChatModelBinding, error) {
		snapshot := maps.Clone(disk)
		if block {
			close(reading)
			<-release
		}
		return snapshot, nil
	}, func(key string, value ChatModelBinding) (map[string]ChatModelBinding, error) {
		disk[key] = value
		return maps.Clone(disk), nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	block = true
	reloaded, saved := make(chan error, 1), make(chan error, 1)
	go func() { _, err := store.ReloadChatModels(); reloaded <- err }()
	<-reading
	go func() { saved <- store.SaveChatModelBinding("test:A", ChatModelBinding{Model: "new"}) }()
	close(release)
	if err := <-reloaded; err != nil {
		t.Fatal(err)
	}
	if err := <-saved; err != nil {
		t.Fatal(err)
	}
	assertChatModelBinding(t, store, "test:A", ChatModelBinding{Model: "new"}, true)
	if disk["test:A"].Model != "new" {
		t.Fatal("disk and runtime disagree")
	}
}

func TestCUJ_ChatModelConfigReloadAppliesToQueuedTurnAndReset(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(map[bool]string{false: "edit", true: "delete"}[reset], func(t *testing.T) {
			e, p, a := newChatModelEngine(t, "")
			a.effort = "medium"
			disk := map[string]ChatModelBinding{"test:A": {Model: "group-B", Provider: a.Name() + ":", ReasoningEffort: "high"}}
			if err := e.projectState.ConfigureChatModels(func() (map[string]ChatModelBinding, error) { return maps.Clone(disk), nil },
				func(string, ChatModelBinding) (map[string]ChatModelBinding, error) { return nil, errors.New("unused") }, nil); err != nil {
				t.Fatal(err)
			}
			release := make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			a.onSend = func(s *cronModelSession, call cronModelCall) error {
				if call.prompt == "first" {
					<-release
				}
				s.events <- Event{Type: EventResult, Content: call.model, Done: true}
				return nil
			}
			e.ReceiveMessage(p, chatModelMessage("A", "t", "first", "first"))
			waitAnswerProfileTest(t, "started", func() bool { calls, _, _ := a.snapshot(); return len(calls) == 1 })
			e.ReceiveMessage(p, chatModelMessage("A", "t", "queued", "queued"))
			want := "group-C"
			if reset {
				delete(disk, "test:A")
				want = "project-A"
			} else {
				disk["test:A"] = ChatModelBinding{Model: want, Provider: a.Name() + ":", ReasoningEffort: "low"}
			}
			if _, err := e.ReloadChatModels(); err != nil {
				t.Fatal(err)
			}
			once.Do(func() { close(release) })
			waitAnswerProfileTest(t, "drained", func() bool {
				calls, _, _ := a.snapshot()
				return len(calls) == 2 && !e.sessions.GetOrCreateActive("test:A:root:t").Busy()
			})
			calls, sessions, setters := a.snapshot()
			if calls[0].model != "group-B" || calls[1].model != want || setters != 0 {
				t.Fatalf("calls = %+v, setters=%d", calls, setters)
			}
			wantEffort := "low"
			if reset {
				wantEffort = "medium"
			}
			if calls[0].options.ReasoningEffort != "high" || calls[1].options.ReasoningEffort != wantEffort {
				t.Fatalf("reload failed to apply reasoning to the queued turn: %+v", calls)
			}
			for _, session := range sessions {
				session.mu.Lock()
				steers := session.steerCalls
				session.mu.Unlock()
				if steers != 0 {
					t.Fatal("queued input steered into previous model")
				}
			}
		})
	}
}

func TestUserMeta_ReadsLegacyVerifiedChatKey(t *testing.T) {
	for _, input := range []string{
		`{"group_model_key":"test:dm:owner","user_name":"Alice"}`,
		`{"group_model_key":"test:wrong","chat_key":"test:dm:owner","user_name":"Alice"}`,
	} {
		var meta UserMeta
		if err := json.Unmarshal([]byte(input), &meta); err != nil {
			t.Fatal(err)
		}
		if meta.ChatKey != "test:dm:owner" || meta.UserName != "Alice" {
			t.Fatalf("meta = %+v", meta)
		}
		data, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "group_model_key") {
			t.Fatal("legacy key was written back")
		}
	}
}
