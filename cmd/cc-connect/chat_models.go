package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

// migrateChatModels runs before the project starts. Commit TOML first, then
// remove only the legacy model fields. If either write fails, startup stops:
// commands cannot reset a binding while a stale migration source still exists.
// A crash between the writes is safe to retry; explicit TOML entries win.
func migrateChatModels(projectName, statePath string) error {
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy chat models: %w", err)
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parse legacy chat models: %w", err)
	}
	legacy := make(map[string]core.ChatModelBinding)
	found := false
	for _, field := range []string{"group_model_bindings", "chat_model_bindings"} {
		encoded, ok := state[field]
		if !ok {
			continue
		}
		found = true
		var bindings map[string]core.ChatModelBinding
		if err := json.Unmarshal(encoded, &bindings); err != nil {
			return fmt.Errorf("parse legacy %s: %w", field, err)
		}
		for key, binding := range bindings {
			legacy[key] = binding
		}
		delete(state, field)
	}
	if !found {
		return nil
	}
	if err := config.MergeLegacyChatModels(projectName, legacy); err != nil {
		return fmt.Errorf("migrate chat models to config.toml: %w", err)
	}
	remaining, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	info, err := os.Stat(statePath)
	if err != nil {
		return err
	}
	if err := core.AtomicWriteFile(statePath, remaining, info.Mode().Perm()); err != nil {
		return fmt.Errorf("finish chat model migration: %w", err)
	}
	return nil
}

func configureChatModels(store *core.ProjectStateStore, projectName string) error {
	return store.ConfigureChatModels(
		func() (map[string]core.ChatModelBinding, error) { return config.LoadChatModels(projectName) },
		func(key string, binding core.ChatModelBinding) (map[string]core.ChatModelBinding, error) {
			return config.SaveChatModel(projectName, key, binding)
		},
	)
}
