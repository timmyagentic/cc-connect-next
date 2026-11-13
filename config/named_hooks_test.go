package config

import (
	"strings"
	"testing"
)

func TestNamedHookConfigValidatesScopeAndInput(t *testing.T) {
	valid := HookConfig{Name: "reply_confirmation", Project: "demo", Event: "card.action", Type: "command", Command: "python3 handler.py", Input: "json"}
	tests := []struct {
		name string
		edit func(*HookConfig)
		want string
	}{
		{"valid", func(*HookConfig) {}, ""},
		{"missing name", func(h *HookConfig) { h.Name = "" }, "requires name and project"},
		{"missing project", func(h *HookConfig) { h.Project = "" }, "requires name and project"},
		{"unknown project", func(h *HookConfig) { h.Project = "unknown" }, "configured project"},
		{"invalid name", func(h *HookConfig) { h.Name = "../handler" }, "identifier"},
		{"long name", func(h *HookConfig) { h.Name = strings.Repeat("a", 65) }, "identifier"},
		{"missing JSON stdin", func(h *HookConfig) { h.Input = "" }, "require input=json"},
		{"invalid input", func(h *HookConfig) { h.Input = "xml" }, "input must be"},
		{"HTTP uses JSON body", func(h *HookConfig) { h.Type, h.Command, h.URL, h.Input = "http", "", "https://example.com/hook", "" }, ""},
		{"HTTP rejects command input", func(h *HookConfig) { h.Type, h.Command, h.URL = "http", "", "https://example.com/hook" }, "only supported for command"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hook := valid
			tc.edit(&hook)
			cfg := Config{Projects: []ProjectConfig{validProject("demo")}, Hooks: []HookConfig{hook}}
			err := cfg.validate()
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	cfg := Config{Projects: []ProjectConfig{validProject("demo")}, Hooks: []HookConfig{valid, valid}}
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate hook error = %v", err)
	}
	cfg.Projects = append(cfg.Projects, validProject("second"))
	cfg.Hooks[1].Project = "second"
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
}
