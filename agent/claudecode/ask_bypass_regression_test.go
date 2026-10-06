package claudecode

import (
	"context"
	"github.com/timmyagentic/cc-connect-next/core"
	"testing"
)

func TestHandleControlRequestBypassModesStillEmitAskUserQuestion(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{name: "bypassPermissions", mode: "bypassPermissions"},
		{name: "dontAsk", mode: "dontAsk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cs := &claudeSession{
				events:  make(chan core.Event, 1),
				ctx:     ctx,
				ccHooks: newCCPermissionHookRunner(t.TempDir()),
			}
			cs.setPermissionMode(tt.mode)
			cs.handleControlRequest(map[string]any{
				"request_id": "ask-1",
				"request": map[string]any{
					"subtype":   "can_use_tool",
					"tool_name": "AskUserQuestion",
					"input": map[string]any{
						"questions": []any{map[string]any{
							"question":    "Which database?",
							"header":      "Database",
							"multiSelect": false,
							"options": []any{
								map[string]any{"label": "PostgreSQL", "description": "Production"},
								map[string]any{"label": "SQLite", "description": "Local"},
							},
						}},
					},
				},
			})

			select {
			case event := <-cs.events:
				if event.Type != core.EventPermissionRequest || event.ToolName != "AskUserQuestion" {
					t.Fatalf("event = %#v, want AskUserQuestion permission request", event)
				}
				if len(event.Questions) != 1 || event.Questions[0].Question != "Which database?" {
					t.Fatalf("questions = %#v, want parsed user question", event.Questions)
				}
			default:
				t.Fatalf("AskUserQuestion was auto-handled in %s mode instead of emitted for user input", tt.mode)
			}
		})
	}
}
