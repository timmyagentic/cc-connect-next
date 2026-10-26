package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestGroupModelBinding_SaveLoad(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "projects", "demo.state.json")
	store := NewProjectStateStore(statePath)
	if got, ok := store.GroupModelBinding("missing"); ok || got != (GroupModelBinding{}) {
		t.Fatalf("missing binding = %+v, %v, want zero value, false", got, ok)
	}

	bindings := map[string]GroupModelBinding{
		"platform:bot-a:group-a": {Model: "model-a", Provider: "provider-a"},
		"platform:bot-b:group-a": {Model: "model-b"},
		"platform:bot-a:group-b": {Model: "model-c", Provider: "provider-c"},
	}
	for key, binding := range bindings {
		if err := store.SaveGroupModelBinding(key, binding); err != nil {
			t.Fatal(err)
		}
	}

	reloaded := NewProjectStateStore(statePath)
	for key, want := range bindings {
		assertGroupModelBinding(t, store, key, want, true)
		assertGroupModelBinding(t, reloaded, key, want, true)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted struct {
		Bindings map[string]map[string]string `json:"group_model_bindings"`
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
	updated := GroupModelBinding{Model: "updated-model"}
	if err := reloaded.SaveGroupModelBinding(key, updated); err != nil {
		t.Fatal(err)
	}
	assertGroupModelBinding(t, NewProjectStateStore(statePath), key, updated, true)
}

func TestGroupModelBinding_Delete(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	store := NewProjectStateStore(statePath)
	binding := GroupModelBinding{Model: "model", Provider: "provider"}
	for _, key := range []string{"group-a", "group-b"} {
		if err := store.SaveGroupModelBinding(key, binding); err != nil {
			t.Fatal(err)
		}
	}

	// Deletion depends only on Model, even if Provider is still populated.
	for _, key := range []string{"missing", "group-a", "group-a"} {
		if err := store.SaveGroupModelBinding(key, GroupModelBinding{Provider: "provider"}); err != nil {
			t.Fatal(err)
		}
		assertGroupModelBinding(t, store, key, GroupModelBinding{}, false)
		assertGroupModelBinding(t, NewProjectStateStore(statePath), key, GroupModelBinding{}, false)
		assertGroupModelBinding(t, NewProjectStateStore(statePath), "group-b", binding, true)
	}
	if err := store.SaveGroupModelBinding("group-b", GroupModelBinding{}); err != nil {
		t.Fatal(err)
	}
	assertGroupModelBinding(t, NewProjectStateStore(statePath), "group-b", GroupModelBinding{}, false)
}

func TestGroupModelBinding_WriteFailureRollback(t *testing.T) {
	for _, failure := range []string{"create-directory", "atomic-rename"} {
		t.Run(failure, func(t *testing.T) {
			statePath := filepath.Join(t.TempDir(), "projects", "state.json")
			store := NewProjectStateStore(statePath)
			store.SetWorkDirOverride("/workdir")
			store.SetWorkspaceDirOverride("workspace", "/override")
			store.SetWorkspaceModelOverride("workspace", "workspace-model")
			original := GroupModelBinding{Model: "original", Provider: "original-provider"}
			for _, key := range []string{"group", "other-group"} {
				if err := store.SaveGroupModelBinding(key, original); err != nil {
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
				binding GroupModelBinding
			}{
				{"update", "group", GroupModelBinding{Model: "replacement", Provider: "replacement-provider"}},
				{"insert", "new-group", GroupModelBinding{Model: "new"}},
				{"delete", "group", GroupModelBinding{}},
			} {
				t.Run(attempt.name, func(t *testing.T) {
					if err := store.SaveGroupModelBinding(attempt.key, attempt.binding); err == nil {
						t.Fatal("SaveGroupModelBinding should return a write error")
					}
					assertGroupModelBinding(t, store, "group", original, true)
					assertGroupModelBinding(t, store, "other-group", original, true)
					assertGroupModelBinding(t, store, "new-group", GroupModelBinding{}, false)
				})
			}
			if got := store.WorkDirOverride(); got != "/workdir" {
				t.Fatalf("workdir after failure = %q", got)
			}
			if got := store.WorkspaceDirOverride("workspace"); got != "/override" {
				t.Fatalf("workspace dir after failure = %q", got)
			}
			if got := store.WorkspaceModelOverride("workspace"); got != "workspace-model" {
				t.Fatalf("workspace model after failure = %q", got)
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
			updated := GroupModelBinding{Model: "retry"}
			if err := store.SaveGroupModelBinding("group", updated); err != nil {
				t.Fatalf("retry after restoring path: %v", err)
			}
			assertGroupModelBinding(t, NewProjectStateStore(statePath), "group", updated, true)
		})
	}
}

func TestGroupModelBinding_PreservesExistingState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	legacy := []byte(`{
		"work_dir_override": "/legacy",
		"workspace_dir_overrides": {"workspace": "/legacy/override"},
		"workspace_model_overrides": {"workspace": "legacy-model"}
	}`)
	if err := os.WriteFile(statePath, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewProjectStateStore(statePath)
	binding := GroupModelBinding{Model: "group-model", Provider: "group-provider"}
	if err := store.SaveGroupModelBinding("group", binding); err != nil {
		t.Fatal(err)
	}
	reloaded := NewProjectStateStore(statePath)
	if got := reloaded.WorkDirOverride(); got != "/legacy" {
		t.Fatalf("legacy workdir = %q", got)
	}
	if got := reloaded.WorkspaceDirOverride("workspace"); got != "/legacy/override" {
		t.Fatalf("legacy workspace dir = %q", got)
	}
	if got := reloaded.WorkspaceModelOverride("workspace"); got != "legacy-model" {
		t.Fatalf("legacy workspace model = %q", got)
	}

	reloaded.SetWorkDirOverride("/updated")
	reloaded.ClearWorkspaceDirOverride("workspace")
	reloaded.SetWorkspaceModelOverride("workspace", "updated-model")
	reloaded.Save()
	reloaded = NewProjectStateStore(statePath)
	assertGroupModelBinding(t, reloaded, "group", binding, true)
	if err := reloaded.SaveGroupModelBinding("group", GroupModelBinding{}); err != nil {
		t.Fatal(err)
	}
	reloaded = NewProjectStateStore(statePath)
	if got := reloaded.WorkDirOverride(); got != "/updated" {
		t.Fatalf("updated workdir = %q", got)
	}
	if got := reloaded.WorkspaceDirOverride("workspace"); got != "" {
		t.Fatalf("cleared workspace dir = %q", got)
	}
	if got := reloaded.WorkspaceModelOverride("workspace"); got != "updated-model" {
		t.Fatalf("updated workspace model = %q", got)
	}
}

func TestGroupModelBinding_ProjectIsolation(t *testing.T) {
	dir := t.TempDir()
	key := "platform:bot:group"
	for _, project := range []string{"project-a", "project-b"} {
		store := NewProjectStateStore(filepath.Join(dir, project+".json"))
		if err := store.SaveGroupModelBinding(key, GroupModelBinding{Model: project}); err != nil {
			t.Fatal(err)
		}
	}
	for _, project := range []string{"project-a", "project-b"} {
		store := NewProjectStateStore(filepath.Join(dir, project+".json"))
		assertGroupModelBinding(t, store, key, GroupModelBinding{Model: project}, true)
	}
}

func TestGroupModelBinding_ConcurrentAccess(t *testing.T) {
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
				binding := GroupModelBinding{Model: model, Provider: "provider-" + model}
				if err := store.SaveGroupModelBinding(key, binding); err != nil {
					t.Errorf("save %s: %v", key, err)
					return
				}
				if got, ok := store.GroupModelBinding(key); !ok || got != binding {
					t.Errorf("binding %s = %+v, %v, want %+v, true", key, got, ok, binding)
					return
				}
				if j < iterations-1 {
					if err := store.SaveGroupModelBinding(key, GroupModelBinding{}); err != nil {
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
					if got, ok := store.GroupModelBinding(key); ok && (got.Model == "" || got.Provider != "provider-"+got.Model) {
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
			store.SetWorkspaceModelOverride("workspace", "workspace-model")
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
		want := GroupModelBinding{Model: model, Provider: "provider-" + model}
		assertGroupModelBinding(t, store, key, want, true)
		assertGroupModelBinding(t, reloaded, key, want, true)
	}
	if reloaded.WorkDirOverride() != "/workdir" || reloaded.WorkspaceDirOverride("workspace") != "/override" || reloaded.WorkspaceModelOverride("workspace") != "workspace-model" {
		t.Fatal("concurrent saves lost existing state")
	}
}

func TestGroupModelBinding_MemoryOnly(t *testing.T) {
	store := NewProjectStateStore("")
	binding := GroupModelBinding{Model: "model"}
	if err := store.SaveGroupModelBinding("group", binding); err != nil {
		t.Fatal(err)
	}
	store.Save()
	assertGroupModelBinding(t, store, "group", binding, true)
	if err := store.SaveGroupModelBinding("group", GroupModelBinding{}); err != nil {
		t.Fatal(err)
	}
	assertGroupModelBinding(t, store, "group", GroupModelBinding{}, false)
}

func assertGroupModelBinding(t *testing.T, store *ProjectStateStore, key string, want GroupModelBinding, wantOK bool) {
	t.Helper()
	if got, ok := store.GroupModelBinding(key); got != want || ok != wantOK {
		t.Fatalf("GroupModelBinding(%q) = %+v, %v, want %+v, %v", key, got, ok, want, wantOK)
	}
}
