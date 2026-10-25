package codex

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

var _ core.PerTurnReasoningEffortSupporter = (*Agent)(nil)

func TestCronReasoningAppServerResumeDoesNotAdoptPreviousTaskEffort(t *testing.T) {
	for _, model := range []string{"", "project-model"} {
		for _, native := range []string{"", "medium"} {
			t.Run(model+"/"+native, func(t *testing.T) {
				s, stdin := cronModelAppServer(t, model)
				s.effort = ""
				stdin.respond = func(id int64, req map[string]any) {
					var result string
					switch req["method"] {
					case "config/read":
						result = `{"config":{"model":"native-model","model_reasoning_effort":` + cronModelJSON(native) + `}}`
					case "thread/resume":
						result = `{"thread":{"id":"thread-1"},"model":"project-model","reasoningEffort":"high"}`
					case "turn/start":
						result = `{"turn":{"id":"turn-2"}}`
					default:
						t.Fatalf("unexpected RPC: %v", req)
					}
					s.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(result)})
				}
				if err := s.captureDefaultModel("thread-1"); err != nil {
					t.Fatal(err)
				}
				if err := s.ensureThread("thread-1"); err != nil {
					t.Fatal(err)
				}
				if got := s.DefaultTurnOptions().ReasoningEffort; got != native {
					t.Fatalf("default effort=%q, want %q", got, native)
				}
				for _, effort := range []string{"high", native} {
					options := s.DefaultTurnOptions()
					options.ReasoningEffort = effort
					if err := s.SendWithTurnOptions("hello", nil, nil, options); err != nil {
						t.Fatal(err)
					}
					got, present := stdin.lastRequest()["params"].(map[string]any)["effort"]
					var want any = effort
					if effort == "" {
						want = nil
					}
					if !present || got != want {
						t.Fatalf("wire effort=%v present=%v, want %v", got, present, want)
					}
					if s.DefaultTurnOptions().ReasoningEffort != native {
						t.Fatal("one-shot effort mutated defaults")
					}
				}
			})
		}
	}
}

func TestCronReasoningAppServerExplicitDefaultSurvivesResume(t *testing.T) {
	s, stdin := cronModelAppServer(t, "project-model")
	if err := s.captureDefaultModel("thread-1"); err != nil {
		t.Fatal(err)
	}
	if len(stdin.requests) != 0 {
		t.Fatal("explicit defaults should not need config/read")
	}
	effort := "high"
	s.applyThreadRuntimeState("", "project-model", &effort)
	if s.DefaultTurnOptions().ReasoningEffort != "medium" {
		t.Fatal("resume replaced configured effort")
	}
}

func TestCronReasoningAppServerMissingNativeDefaultsBlocksOverride(t *testing.T) {
	s, stdin := cronModelAppServer(t, "project-model")
	s.effort = ""
	stdin.respond = respondError(s, "config unavailable")
	if err := s.captureDefaultModel("thread-1"); err == nil {
		t.Fatal("missing effort default accepted")
	}
	if err := s.ValidateTurnDefaults(); err == nil {
		t.Fatal("unrestorable overrides accepted")
	}
}

func TestCronReasoningExecOptionsThenDefault(t *testing.T) {
	s := &codexSession{model: "project-model", effort: "medium", mode: "suggest", workDir: t.TempDir()}
	for _, effort := range []string{"high", "medium"} {
		options := s.DefaultTurnOptions()
		options.ReasoningEffort = effort
		args := s.launchArgsWithTurnOptions("hello", nil, &options)
		if !slices.Contains(args, `model_reasoning_effort="`+effort+`"`) {
			t.Fatalf("missing task effort in %v", args)
		}
		if s.DefaultTurnOptions().ReasoningEffort != "medium" {
			t.Fatal("task effort mutated defaults")
		}
	}
}
