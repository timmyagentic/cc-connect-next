package core

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// ChatModelBinding is a configuration snapshot for a verified chat's preferences.
type ChatModelBinding struct {
	Model           string `json:"model"`
	Provider        string `json:"provider,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// GroupReasoningBinding is independent of the chat's model selection.
type GroupReasoningBinding struct {
	Effort   string `json:"effort"`
	Provider string `json:"provider,omitempty"`
}

type projectStateData struct {
	WorkDirOverride        string                           `json:"work_dir_override,omitempty"`
	WorkspaceDirOverrides  map[string]string                `json:"workspace_dir_overrides,omitempty"`
	ChatModelBindings      map[string]ChatModelBinding      `json:"chat_model_bindings,omitempty"`
	GroupReasoningBindings map[string]GroupReasoningBinding `json:"group_reasoning_bindings,omitempty"`
}

// ProjectStateStore persists lightweight runtime state for one project.
type ProjectStateStore struct {
	mu                sync.RWMutex
	storePath         string
	state             projectStateData
	chatModelLoad     func() (map[string]ChatModelBinding, error)
	chatModelSave     func(string, ChatModelBinding) (map[string]ChatModelBinding, error)
	chatReasoningSave func(string, GroupReasoningBinding) (map[string]ChatModelBinding, error)
}

func NewProjectStateStore(path string) *ProjectStateStore {
	ps := &ProjectStateStore{storePath: path}
	if path != "" {
		ps.load()
	}
	return ps
}

func (ps *ProjectStateStore) WorkDirOverride() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.state.WorkDirOverride
}

func (ps *ProjectStateStore) SetWorkDirOverride(dir string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.state.WorkDirOverride = dir
}

func (ps *ProjectStateStore) WorkspaceDirOverride(workspace string) string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if ps.state.WorkspaceDirOverrides == nil {
		return ""
	}
	return ps.state.WorkspaceDirOverrides[workspace]
}

func (ps *ProjectStateStore) SetWorkspaceDirOverride(workspace, dir string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.state.WorkspaceDirOverrides == nil {
		ps.state.WorkspaceDirOverrides = make(map[string]string)
	}
	ps.state.WorkspaceDirOverrides[workspace] = dir
}

func (ps *ProjectStateStore) ClearWorkspaceDirOverride(workspace string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.state.WorkspaceDirOverrides == nil {
		return
	}
	delete(ps.state.WorkspaceDirOverrides, workspace)
	if len(ps.state.WorkspaceDirOverrides) == 0 {
		ps.state.WorkspaceDirOverrides = nil
	}
}

func (ps *ProjectStateStore) ClearWorkDirOverride() {
	ps.SetWorkDirOverride("")
}

// ChatModelBinding looks up the caller-provided verified chat key.
func (ps *ProjectStateStore) ChatModelBinding(key string) (ChatModelBinding, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	binding, ok := ps.state.ChatModelBindings[key]
	return binding, ok
}

// SaveChatModelBinding persists a binding before making it visible in memory.
// An empty Model removes the binding. Stores without a path remain memory-only.
func (ps *ProjectStateStore) SaveChatModelBinding(key string, binding ChatModelBinding) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.chatModelSave != nil {
		bindings, err := ps.chatModelSave(key, binding)
		if err != nil {
			return err
		}
		ps.publishChatBindings(bindings)
		return nil
	}

	next := ps.state
	next.ChatModelBindings = make(map[string]ChatModelBinding, len(ps.state.ChatModelBindings)+1)
	for k, v := range ps.state.ChatModelBindings {
		next.ChatModelBindings[k] = v
	}
	if binding.Model == "" {
		delete(next.ChatModelBindings, key)
		if len(next.ChatModelBindings) == 0 {
			next.ChatModelBindings = nil
		}
	} else {
		next.ChatModelBindings[key] = binding
	}

	if err := ps.saveLocked(next); err != nil {
		return err
	}
	ps.state = next
	return nil
}

// ConfigureChatModels makes config authoritative for models and effort, including
// when either field has no bindings. Call before accepting messages. Callbacks run under
// the same lock as commands and reload, so an older reload cannot overwrite a
// selection that has just been saved. They must not call back into this store.
func (ps *ProjectStateStore) ConfigureChatModels(load func() (map[string]ChatModelBinding, error), save func(string, ChatModelBinding) (map[string]ChatModelBinding, error), saveReasoning func(string, GroupReasoningBinding) (map[string]ChatModelBinding, error)) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if load == nil || save == nil {
		return fmt.Errorf("chat model configuration requires load and save callbacks")
	}
	bindings, err := load()
	if err != nil {
		return err
	}
	ps.chatModelLoad, ps.chatModelSave = load, save
	ps.chatReasoningSave = saveReasoning
	ps.publishChatBindings(bindings)
	return nil
}

// publishChatBindings replaces both preference maps from one config snapshot.
// Callers hold mu through reading, durable writes, and publication.
func (ps *ProjectStateStore) publishChatBindings(bindings map[string]ChatModelBinding) {
	ps.state.ChatModelBindings = make(map[string]ChatModelBinding)
	ps.state.GroupReasoningBindings = make(map[string]GroupReasoningBinding)
	for key, binding := range bindings {
		if binding.Model != "" {
			ps.state.ChatModelBindings[key] = ChatModelBinding{Model: binding.Model, Provider: binding.Provider}
		}
		if binding.ReasoningEffort != "" {
			ps.state.GroupReasoningBindings[key] = GroupReasoningBinding{Effort: binding.ReasoningEffort, Provider: binding.Provider}
		}
	}
}

func (ps *ProjectStateStore) ReloadChatModels() (int, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.chatModelLoad == nil {
		return len(ps.state.ChatModelBindings), nil
	}
	bindings, err := ps.chatModelLoad()
	if err != nil {
		return 0, err
	}
	ps.publishChatBindings(bindings)
	return len(bindings), nil
}

func (ps *ProjectStateStore) GroupReasoningBinding(key string) (GroupReasoningBinding, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	binding, ok := ps.state.GroupReasoningBindings[key]
	return binding, ok
}

// SaveGroupReasoningBinding publishes only after a successful durable write.
// Empty effort removes only the effort binding, preserving any model binding.
func (ps *ProjectStateStore) SaveGroupReasoningBinding(key string, binding GroupReasoningBinding) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.chatModelLoad != nil {
		if ps.chatReasoningSave == nil {
			return fmt.Errorf("chat reasoning configuration requires a save callback")
		}
		bindings, err := ps.chatReasoningSave(key, binding)
		if err != nil {
			return err
		}
		ps.publishChatBindings(bindings)
		return nil
	}
	next := ps.state
	next.GroupReasoningBindings = make(map[string]GroupReasoningBinding, len(ps.state.GroupReasoningBindings)+1)
	for k, v := range ps.state.GroupReasoningBindings {
		next.GroupReasoningBindings[k] = v
	}
	if binding.Effort == "" {
		delete(next.GroupReasoningBindings, key)
		if len(next.GroupReasoningBindings) == 0 {
			next.GroupReasoningBindings = nil
		}
	} else {
		next.GroupReasoningBindings[key] = binding
	}
	if err := ps.saveLocked(next); err != nil {
		return err
	}
	ps.state = next
	return nil
}

func (ps *ProjectStateStore) Save() {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if err := ps.saveLocked(ps.state); err != nil {
		slog.Error("project_state: failed to save", "path", ps.storePath, "error", err)
	}
}

func (ps *ProjectStateStore) saveLocked(state projectStateData) error {
	if ps.storePath == "" {
		return nil
	}
	if ps.chatModelLoad != nil {
		// Directory updates must not recreate preferences owned by config.toml.
		state.ChatModelBindings = nil
		state.GroupReasoningBindings = nil
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("project_state: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(ps.storePath), 0o755); err != nil {
		return fmt.Errorf("project_state: create dir: %w", err)
	}
	if err := AtomicWriteFile(ps.storePath, data, 0o644); err != nil {
		return fmt.Errorf("project_state: write: %w", err)
	}
	return nil
}

func (ps *ProjectStateStore) load() {
	data, err := os.ReadFile(ps.storePath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("project_state: failed to read", "path", ps.storePath, "error", err)
		}
		return
	}

	var state projectStateData
	if err := json.Unmarshal(data, &state); err != nil {
		slog.Error("project_state: failed to unmarshal", "path", ps.storePath, "error", err)
		return
	}
	ps.state = state
}
