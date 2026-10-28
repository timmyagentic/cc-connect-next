package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestChatModelBinding_SaveLoad(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "projects", "demo.state.json")
	store := NewProjectStateStore(statePath)
	if got, ok := store.ChatModelBinding("missing"); ok || got != (ChatModelBinding{}) {
		t.Fatalf("missing binding = %+v, %v, want zero value, false", got, ok)
	}

	bindings := map[string]ChatModelBinding{
		"platform:bot-a:group-a": {Model: "model-a", Provider: "provider-a"},
		"platform:bot-b:group-a": {Model: "model-b"},
		"platform:bot-a:group-b": {Model: "model-c", Provider: "provider-c"},
	}
	for key, binding := range bindings {
		if err := store.SaveChatModelBinding(key, binding); err != nil {
			t.Fatal(err)
		}
	}

	reloaded := NewProjectStateStore(statePath)
	for key, want := range bindings {
		assertChatModelBinding(t, store, key, want, true)
		assertChatModelBinding(t, reloaded, key, want, true)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted struct {
		Bindings map[string]map[string]string `json:"chat_model_bindings"`
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if got := persisted.Bindings["platform:bot-a:group-a"]; got["model"] != "model-a" || got["provider"] != "provider-a" {
		t.Fatalf("persisted binding = %v, want model and provider fields", got)
	}
	if _, ok := persisted.Bindings["platform:bot-b:group-a"]["provider"]; ok {
		t.Fatal("empty provider should be omitted")
	}

	key := "platform:bot-a:group-a"
	updated := ChatModelBinding{Model: "updated-model"}
	if err := reloaded.SaveChatModelBinding(key, updated); err != nil {
		t.Fatal(err)
	}
	assertChatModelBinding(t, NewProjectStateStore(statePath), key, updated, true)
}

func TestChatModelBinding_Delete(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	store := NewProjectStateStore(statePath)
	binding := ChatModelBinding{Model: "model", Provider: "provider"}
	for _, key := range []string{"group-a", "group-b"} {
		if err := store.SaveChatModelBinding(key, binding); err != nil {
			t.Fatal(err)
		}
	}

	// Deletion depends only on Model, even if Provider is still populated.
	for _, key := range []string{"missing", "group-a", "group-a"} {
		if err := store.SaveChatModelBinding(key, ChatModelBinding{Provider: "provider"}); err != nil {
			t.Fatal(err)
		}
		assertChatModelBinding(t, store, key, ChatModelBinding{}, false)
		assertChatModelBinding(t, NewProjectStateStore(statePath), key, ChatModelBinding{}, false)
		assertChatModelBinding(t, NewProjectStateStore(statePath), "group-b", binding, true)
	}
	if err := store.SaveChatModelBinding("group-b", ChatModelBinding{}); err != nil {
		t.Fatal(err)
	}
	assertChatModelBinding(t, NewProjectStateStore(statePath), "group-b", ChatModelBinding{}, false)
}

func TestChatModelBinding_WriteFailureRollback(t *testing.T) {
	for _, failure := range []string{"create-directory", "atomic-rename"} {
		t.Run(failure, func(t *testing.T) {
			statePath := filepath.Join(t.TempDir(), "projects", "state.json")
			store := NewProjectStateStore(statePath)
			store.SetWorkDirOverride("/workdir")
			store.SetWorkspaceDirOverride("workspace", "/override")
			original := ChatModelBinding{Model: "original", Provider: "original-provider"}
			for _, key := range []string{"group", "other-group"} {
				if err := store.SaveChatModelBinding(key, original); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}

			// Use filesystem shape instead of permissions so failure is reliable
			// on Windows and when the tests run with elevated privileges.
			blockedPath := statePath
			if failure == "create-directory" {
				blockedPath = filepath.Dir(statePath)
			}
			backupPath := blockedPath + ".saved"
			if err := os.Rename(blockedPath, backupPath); err != nil {
				t.Fatal(err)
			}
			if failure == "create-directory" {
				err = os.WriteFile(blockedPath, []byte("not a directory"), 0o644)
			} else {
				err = os.Mkdir(blockedPath, 0o755)
			}
			if err != nil {
				t.Fatal(err)
			}

			for _, attempt := range []struct {
				name    string
				key     string
				binding ChatModelBinding
			}{
				{"update", "group", ChatModelBinding{Model: "replacement", Provider: "replacement-provider"}},
				{"insert", "new-group", ChatModelBinding{Model: "new"}},
				{"delete", "group", ChatModelBinding{}},
			} {
				t.Run(attempt.name, func(t *testing.T) {
					if err := store.SaveChatModelBinding(attempt.key, attempt.binding); err == nil {
						t.Fatal("SaveChatModelBinding should return a write error")
					}
					assertChatModelBinding(t, store, "group", original, true)
					assertChatModelBinding(t, store, "other-group", original, true)
					assertChatModelBinding(t, store, "new-group", ChatModelBinding{}, false)
				})
			}
			if got := store.WorkDirOverride(); got != "/workdir" {
				t.Fatalf("workdir after failure = %q", got)
			}
			if got := store.WorkspaceDirOverride("workspace"); got != "/override" {
				t.Fatalf("workspace dir after failure = %q", got)
			}

			if err := os.Remove(blockedPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(backupPath, blockedPath); err != nil {
				t.Fatal(err)
			}
			// A later legacy Save must not flush any of the failed changes.
			store.Save()
			after, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatalf("state changed after failed writes:\nbefore: %s\nafter: %s", before, after)
			}
			updated := ChatModelBinding{Model: "retry"}
			if err := store.SaveChatModelBinding("group", updated); err != nil {
				t.Fatalf("retry after restoring path: %v", err)
			}
			assertChatModelBinding(t, NewProjectStateStore(statePath), "group", updated, true)
		})
	}
}

func TestChatModelBinding_PreservesExistingState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	legacy := []byte(`{
		"work_dir_override": "/legacy",
		"workspace_dir_overrides": {"workspace": "/legacy/override"}
	}`)
	if err := os.WriteFile(statePath, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewProjectStateStore(statePath)
	binding := ChatModelBinding{Model: "group-model", Provider: "group-provider"}
	if err := store.SaveChatModelBinding("group", binding); err != nil {
		t.Fatal(err)
	}
	reloaded := NewProjectStateStore(statePath)
	if got := reloaded.WorkDirOverride(); got != "/legacy" {
		t.Fatalf("legacy workdir = %q", got)
	}
	if got := reloaded.WorkspaceDirOverride("workspace"); got != "/legacy/override" {
		t.Fatalf("legacy workspace dir = %q", got)
	}

	reloaded.SetWorkDirOverride("/updated")
	reloaded.ClearWorkspaceDirOverride("workspace")
	reloaded.Save()
	reloaded = NewProjectStateStore(statePath)
	assertChatModelBinding(t, reloaded, "group", binding, true)
	if err := reloaded.SaveChatModelBinding("group", ChatModelBinding{}); err != nil {
		t.Fatal(err)
	}
	reloaded = NewProjectStateStore(statePath)
	if got := reloaded.WorkDirOverride(); got != "/updated" {
		t.Fatalf("updated workdir = %q", got)
	}
	if got := reloaded.WorkspaceDirOverride("workspace"); got != "" {
		t.Fatalf("cleared workspace dir = %q", got)
	}
}

func TestChatModelBinding_ProjectIsolation(t *testing.T) {
	dir := t.TempDir()
	key := "platform:bot:group"
	for _, project := range []string{"project-a", "project-b"} {
		store := NewProjectStateStore(filepath.Join(dir, project+".json"))
		if err := store.SaveChatModelBinding(key, ChatModelBinding{Model: project}); err != nil {
			t.Fatal(err)
		}
	}
	for _, project := range []string{"project-a", "project-b"} {
		store := NewProjectStateStore(filepath.Join(dir, project+".json"))
		assertChatModelBinding(t, store, key, ChatModelBinding{Model: project}, true)
	}
}

func TestChatModelBinding_ConcurrentAccess(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	store := NewProjectStateStore(statePath)
	const writers = 8
	const iterations = 12
	start := make(chan struct{})
	done := make(chan struct{})
	var writes, reads sync.WaitGroup
	for i := range writers {
		key := fmt.Sprintf("bot:%d:group", i)
		writes.Add(1)
		go func() {
			defer writes.Done()
			<-start
			for j := range iterations {
				model := fmt.Sprintf("%s-model-%d", key, j)
				binding := ChatModelBinding{Model: model, Provider: "provider-" + model}
				if err := store.SaveChatModelBinding(key, binding); err != nil {
					t.Errorf("save %s: %v", key, err)
					return
				}
				if got, ok := store.ChatModelBinding(key); !ok || got != binding {
					t.Errorf("binding %s = %+v, %v, want %+v, true", key, got, ok, binding)
					return
				}
				if j < iterations-1 {
					if err := store.SaveChatModelBinding(key, ChatModelBinding{}); err != nil {
						t.Errorf("delete %s: %v", key, err)
						return
					}
				}
			}
		}()
		reads.Add(1)
		go func() {
			defer reads.Done()
			<-start
			for {
				select {
				case <-done:
					return
				default:
					if got, ok := store.ChatModelBinding(key); ok && (got.Model == "" || got.Provider != "provider-"+got.Model) {
						t.Errorf("partially updated binding %s = %+v", key, got)
						return
					}
				}
			}
		}()
	}
	writes.Add(1)
	go func() {
		defer writes.Done()
		<-start
		for range iterations {
			store.SetWorkDirOverride("/workdir")
			store.SetWorkspaceDirOverride("workspace", "/override")
			store.Save()
		}
	}()
	close(start)
	writes.Wait()
	close(done)
	reads.Wait()

	reloaded := NewProjectStateStore(statePath)
	for i := range writers {
		key := fmt.Sprintf("bot:%d:group", i)
		model := fmt.Sprintf("%s-model-%d", key, iterations-1)
		want := ChatModelBinding{Model: model, Provider: "provider-" + model}
		assertChatModelBinding(t, store, key, want, true)
		assertChatModelBinding(t, reloaded, key, want, true)
	}
	if reloaded.WorkDirOverride() != "/workdir" || reloaded.WorkspaceDirOverride("workspace") != "/override" {
		t.Fatal("concurrent saves lost existing state")
	}
}

func TestChatModelBinding_MemoryOnly(t *testing.T) {
	store := NewProjectStateStore("")
	binding := ChatModelBinding{Model: "model"}
	if err := store.SaveChatModelBinding("group", binding); err != nil {
		t.Fatal(err)
	}
	store.Save()
	assertChatModelBinding(t, store, "group", binding, true)
	if err := store.SaveChatModelBinding("group", ChatModelBinding{}); err != nil {
		t.Fatal(err)
	}
	assertChatModelBinding(t, store, "group", ChatModelBinding{}, false)
}

func assertChatModelBinding(t *testing.T, store *ProjectStateStore, key string, want ChatModelBinding, wantOK bool) {
	t.Helper()
	if got, ok := store.ChatModelBinding(key); got != want || ok != wantOK {
		t.Fatalf("ChatModelBinding(%q) = %+v, %v, want %+v, %v", key, got, ok, want, wantOK)
	}
}
