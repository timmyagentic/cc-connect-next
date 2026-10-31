package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/timmyagentic/cc-connect-next/core"
)

// ChatModelConfig holds model and effort preferences for one verified group or
// direct-message user. Topics inherit their group's preference. Agent pins the
// adapter identity; omitting it uses the project's configured agent type.
type ChatModelConfig struct {
	Platform        string  `toml:"platform"`
	ChatID          string  `toml:"chat_id,omitempty"`
	UserID          string  `toml:"user_id,omitempty"`
	Model           string  `toml:"model,omitempty"`
	ReasoningEffort *string `toml:"reasoning_effort,omitempty"`
	Provider        string  `toml:"provider,omitempty"`
	Agent           string  `toml:"agent,omitempty"`
}

func (c ChatModelConfig) key() string {
	if c.UserID != "" {
		return c.Platform + ":dm:" + c.UserID
	}
	return c.Platform + ":" + c.ChatID
}

func (c ChatModelConfig) binding(agent string) core.ChatModelBinding {
	if c.Agent != "" {
		agent = c.Agent
	}
	return core.ChatModelBinding{Model: c.Model, Provider: agent + ":" + c.Provider, ReasoningEffort: c.effort()}
}

func (c ChatModelConfig) effort() string {
	if c.ReasoningEffort != nil {
		return *c.ReasoningEffort
	}
	return ""
}

func validateChatModels(prefix string, proj ProjectConfig) error {
	seen := map[string]bool{}
	for i, c := range proj.ChatModels {
		path := fmt.Sprintf("%s.chat_models[%d]", prefix, i)
		for _, field := range []struct{ name, value string }{
			{"platform", c.Platform}, {"chat_id", c.ChatID}, {"user_id", c.UserID},
			{"model", c.Model}, {"provider", c.Provider}, {"agent", c.Agent},
			{"reasoning_effort", c.effort()},
		} {
			if strings.IndexFunc(field.value, unicode.IsSpace) >= 0 || strings.ContainsRune(field.value, 0) {
				return fmt.Errorf("config: %s.%s must not contain whitespace or NUL", path, field.name)
			}
		}
		if c.Platform == "" || strings.Contains(c.Platform, ":") {
			return fmt.Errorf("config: %s.platform must be a nonempty platform name without a colon", path)
		}
		if (c.ChatID == "") == (c.UserID == "") {
			return fmt.Errorf("config: %s must specify exactly one of chat_id or user_id", path)
		}
		if strings.HasPrefix(c.ChatID, "dm:") {
			return fmt.Errorf("config: %s.chat_id uses the reserved dm: namespace; use user_id", path)
		}
		if c.Model == "" && c.effort() == "" {
			return fmt.Errorf("config: %s requires model or reasoning_effort", path)
		}
		if strings.Contains(c.Agent, ":") {
			return fmt.Errorf("config: %s.agent must be an agent type without a provider suffix", path)
		}
		// Keep dormant platform/provider identities after configuration changes.
		// Runtime guards reject only the affected chat instead of preventing all
		// projects from starting (including while migrating older bindings).
		if seen[c.key()] {
			return fmt.Errorf("config: %s contains a duplicate chat model target", path)
		}
		seen[c.key()] = true
	}
	return nil
}

func chatModelMap(proj ProjectConfig) map[string]core.ChatModelBinding {
	bindings := make(map[string]core.ChatModelBinding, len(proj.ChatModels))
	for _, c := range proj.ChatModels {
		bindings[c.key()] = c.binding(proj.Agent.Type)
	}
	return bindings
}

func findChatModelProject(cfg *Config, name string) (int, error) {
	for i := range cfg.Projects {
		if cfg.Projects[i].Name == name {
			return i, nil
		}
	}
	return -1, fmt.Errorf("project %q not found in config", name)
}

// LoadChatModels validates the complete current config before returning an
// independent snapshot. Callers serialize reading and publishing this snapshot
// with command writes so a concurrent reload cannot publish an older version.
func LoadChatModels(projectName string) (map[string]core.ChatModelBinding, error) {
	configMu.Lock()
	defer configMu.Unlock()
	if ConfigPath == "" {
		return nil, fmt.Errorf("config path not set")
	}
	cfg, err := Load(ConfigPath)
	if err != nil {
		return nil, err
	}
	i, err := findChatModelProject(cfg, projectName)
	if err != nil {
		return nil, err
	}
	return chatModelMap(cfg.Projects[i]), nil
}

// SaveChatModel changes only the model. An empty Model preserves pinned effort.
// The complete resulting map is returned only after durable config replacement.
func SaveChatModel(projectName, key string, binding core.ChatModelBinding) (map[string]core.ChatModelBinding, error) {
	return saveChatPreference(projectName, key, &binding, nil)
}

// SaveChatReasoningEffort changes only the effort, preserving the chat model.
func SaveChatReasoningEffort(projectName, key string, binding core.GroupReasoningBinding) (map[string]core.ChatModelBinding, error) {
	return saveChatPreference(projectName, key, nil, &binding)
}

func saveChatPreference(projectName, key string, model *core.ChatModelBinding, reasoning *core.GroupReasoningBinding) (map[string]core.ChatModelBinding, error) {
	configMu.Lock()
	defer configMu.Unlock()
	raw, cfg, idx, err := readChatModelConfig(projectName)
	if err != nil {
		return nil, err
	}
	binding := chatModelMap(cfg.Projects[idx])[key]
	if model != nil {
		if model.Model != "" {
			if binding.ReasoningEffort != "" && binding.Provider != model.Provider {
				return nil, fmt.Errorf("chat model and reasoning effort must use the same provider; reset the chat effort before changing provider")
			}
			binding.Provider = model.Provider
		}
		binding.Model = model.Model
	}
	if reasoning != nil {
		if reasoning.Effort != "" {
			if binding.Model != "" && binding.Provider != reasoning.Provider {
				return nil, fmt.Errorf("chat model and reasoning effort must use the same provider; reset the chat model before changing provider")
			}
			binding.Provider = reasoning.Provider
		}
		binding.ReasoningEffort = reasoning.Effort
	}
	field := "model"
	if reasoning != nil {
		field = "reasoning_effort"
	}
	raw, err = patchChatModel(raw, cfg, idx, key, binding, field)
	if err != nil {
		return nil, err
	}
	updated, err := decodeChatModelConfig(raw)
	if err != nil {
		return nil, err
	}
	if err := writeRawConfig(raw); err != nil {
		return nil, err
	}
	return chatModelMap(updated.Projects[idx]), nil
}

// MergeLegacyChatModels imports missing legacy targets in one config write.
// Explicit TOML targets always win. The caller removes old state only after
// this succeeds and must finish that cleanup before accepting commands.
func MergeLegacyChatModels(projectName string, legacy map[string]core.ChatModelBinding, reasoning map[string]core.GroupReasoningBinding) error {
	configMu.Lock()
	defer configMu.Unlock()
	raw, cfg, idx, err := readChatModelConfig(projectName)
	if err != nil {
		return err
	}
	existing := make(map[string]ChatModelConfig)
	for _, entry := range cfg.Projects[idx].ChatModels {
		existing[entry.key()] = entry
	}
	targets := make(map[string]bool)
	for key := range legacy {
		targets[key] = true
	}
	for key := range reasoning {
		targets[key] = true
	}
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	changed := false
	sort.Strings(keys)
	for _, key := range keys {
		entry, configured := existing[key]
		binding := entry.binding(cfg.Projects[idx].Agent.Type)
		if !configured {
			binding = legacy[key]
		}
		if old := reasoning[key]; old.Effort != "" && (!configured || entry.ReasoningEffort == nil) {
			if binding.Model != "" && binding.Provider != old.Provider {
				return fmt.Errorf("migrate chat preferences for %s: model and reasoning effort have different providers", key)
			}
			binding.ReasoningEffort = old.Effort
			binding.Provider = old.Provider
		}
		if binding.Model == "" && binding.ReasoningEffort == "" || configured && binding == entry.binding(cfg.Projects[idx].Agent.Type) {
			continue
		}
		field := ""
		if configured {
			field = "reasoning_effort"
		}
		raw, err = patchChatModel(raw, cfg, idx, key, binding, field)
		if err != nil {
			return err
		}
		cfg, err = decodeChatModelConfig(raw)
		if err != nil {
			return err
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return writeRawConfig(raw)
}

func readChatModelConfig(projectName string) (string, *Config, int, error) {
	if ConfigPath == "" {
		return "", nil, -1, fmt.Errorf("config path not set")
	}
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return "", nil, -1, err
	}
	cfg, err := decodeChatModelConfig(string(data))
	if err != nil {
		return "", nil, -1, err
	}
	idx, err := findChatModelProject(cfg, projectName)
	return string(data), cfg, idx, err
}

func decodeChatModelConfig(raw string) (*Config, error) {
	var cfg Config
	if _, err := toml.Decode(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.ResolveEnvironment()
	cfg.ResolveProviderRefs()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func chatModelFromBinding(key string, binding core.ChatModelBinding) (ChatModelConfig, error) {
	platform, target, ok := strings.Cut(key, ":")
	if !ok || platform == "" || target == "" {
		return ChatModelConfig{}, fmt.Errorf("invalid verified chat key")
	}
	c := ChatModelConfig{Platform: platform, Model: binding.Model}
	if binding.ReasoningEffort != "" {
		c.ReasoningEffort = &binding.ReasoningEffort
	}
	if strings.HasPrefix(target, "dm:") {
		c.UserID = strings.TrimPrefix(target, "dm:")
	} else {
		c.ChatID = target
	}
	if binding.Model != "" || binding.ReasoningEffort != "" {
		var hasAgent bool
		c.Agent, c.Provider, hasAgent = strings.Cut(binding.Provider, ":")
		if !hasAgent || c.Agent == "" {
			return ChatModelConfig{}, fmt.Errorf("chat model provider must include its agent identity")
		}
	}
	return c, nil
}

// patchChatModel edits an array table without re-encoding the surrounding
// configuration (comments, secrets, environment placeholders, and unknown
// settings remain in their original text). Unsupported layouts fail closed.
func patchChatModel(raw string, cfg *Config, idx int, key string, binding core.ChatModelBinding, changedField string) (string, error) {
	entry, err := chatModelFromBinding(key, binding)
	if err != nil {
		return "", err
	}
	lines, trailing := splitConfigLines(raw)
	spans := buildRawProjectSpans(lines)
	if idx >= len(spans) {
		return "", fmt.Errorf("project located in parsed config but not raw file")
	}
	span := spans[idx]
	starts := []int{}
	for i := span.start + 1; i <= span.end; i++ {
		if matchTableHeader(lines[i], "[[projects.chat_models]]") {
			starts = append(starts, i)
		}
	}
	if len(starts) != len(cfg.Projects[idx].ChatModels) {
		return "", fmt.Errorf("chat model changes require [[projects.chat_models]] array tables")
	}
	for i, current := range cfg.Projects[idx].ChatModels {
		if current.key() != key {
			continue
		}
		start, end := starts[i], span.end+1
		for j := start + 1; j <= span.end; j++ {
			if isAnyTableHeader(lines[j]) {
				end = j
				break
			}
		}
		if binding.Model == "" && binding.ReasoningEffort == "" {
			// Unknown child tables belong to this entry as well. Leaving one
			// behind would reattach it to a different target after reset.
			for end <= span.end && (strings.HasPrefix(strings.TrimSpace(lines[end]), "[projects.chat_models.") || strings.HasPrefix(strings.TrimSpace(lines[end]), "[[projects.chat_models.")) {
				end++
				for end <= span.end && !isAnyTableHeader(lines[end]) {
					end++
				}
			}
			// Keep comments between this entry and the following table.
			for end > start+1 && (strings.TrimSpace(lines[end-1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[end-1]), "#")) {
				end--
			}
			lines = append(lines[:start], lines[end:]...)
		} else {
			for _, field := range [][2]string{{"model", entry.Model}, {"reasoning_effort", entry.effort()}, {"provider", entry.Provider}, {"agent", entry.Agent}} {
				if changedField != "" && field[0] != changedField && (field[0] == "model" || field[0] == "reasoning_effort") {
					continue
				}
				present := false
				for _, line := range lines[start+1 : end] {
					present = present || matchTomlStringKey(line, field[0])
				}
				if present && (field[0] == "provider" || field[0] == "agent") && current.binding(cfg.Projects[idx].Agent.Type).Provider == binding.Provider {
					// Preserve explicit identity expressions when the identity
					// itself did not change, including environment placeholders.
					continue
				}
				if !present && field[1] == "" && (field[0] == "model" || field[0] == "reasoning_effort") {
					continue
				}
				before := len(lines)
				lines = upsertTomlStringKey(lines, start+1, end-1, field[0], field[1])
				end += len(lines) - before
			}
		}
		return joinConfigLines(lines, trailing), nil
	}
	if binding.Model == "" && binding.ReasoningEffort == "" {
		return raw, nil
	}
	// Insert after the project's own fields, before its first child or global
	// table, so later unrelated table headers never absorb the new preference.
	insert := span.end + 1
	for i := span.start + 1; i <= span.end; i++ {
		if isAnyTableHeader(lines[i]) {
			insert = i
			break
		}
	}
	// An explicitly empty array is valid hand-written configuration. Remove
	// its one-line declaration before adding the first array table.
	for i := span.start + 1; i < insert; i++ {
		if !matchTomlStringKey(lines[i], "chat_models") {
			continue
		}
		_, rhs, _ := strings.Cut(lines[i], "=")
		value, comment, hasComment := strings.Cut(rhs, "#")
		if strings.TrimSpace(value) != "[]" {
			return "", fmt.Errorf("chat model changes require [[projects.chat_models]] array tables or chat_models = []")
		}
		if hasComment {
			lines[i] = "#" + comment
		} else {
			lines[i] = ""
		}
	}
	newLines := []string{"[[projects.chat_models]]", "platform = " + quoteTomlString(entry.Platform)}
	if entry.UserID != "" {
		newLines = append(newLines, "user_id = "+quoteTomlString(entry.UserID))
	} else {
		newLines = append(newLines, "chat_id = "+quoteTomlString(entry.ChatID))
	}
	if entry.Model != "" {
		newLines = append(newLines, "model = "+quoteTomlString(entry.Model))
	}
	if entry.effort() != "" {
		newLines = append(newLines, "reasoning_effort = "+quoteTomlString(entry.effort()))
	}
	newLines = append(newLines, "provider = "+quoteTomlString(entry.Provider), "agent = "+quoteTomlString(entry.Agent), "")
	lines = append(lines[:insert], append(newLines, lines[insert:]...)...)
	return joinConfigLines(lines, trailing), nil
}
