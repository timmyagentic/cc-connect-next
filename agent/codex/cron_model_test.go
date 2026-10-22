package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

var (
	_ core.TurnDefaultsValidator = (*codexSession)(nil)
	_ core.TurnDefaultsValidator = (*appServerSession)(nil)
	_ core.PerTurnModelSupporter = (*Agent)(nil)
)

// Exercise Send at the process boundary, including resumed turns, without
// invoking a real CLI, touching its home, or making model requests.
func cronModelExecSession(t *testing.T, model string) (*codexSession, string, string) {
	t.Helper()
	dir := t.TempDir()
	argsPath, callsPath := filepath.Join(dir, "args"), filepath.Join(dir, "calls")
	writeFakeCodexScript(t, dir, `#!/bin/sh
printf '%s\n' "$@" > "$CRON_TEST_ARGS"
printf 'launch\n' >> "$CRON_TEST_CALLS"
prompt=$(cat)
case "$prompt" in
  fail) printf 'Selected model is at capacity\n' >&2; exit 1 ;;
  cancel) sleep 30; exit 0 ;;
esac
printf '%s\n' '{"type":"turn.completed"}'
`, `
[IO.File]::WriteAllLines($env:CRON_TEST_ARGS, (fakeCodexArgs))
[IO.File]::AppendAllText($env:CRON_TEST_CALLS, "launch`+"`n"+`")
$prompt = [Console]::In.ReadToEnd()
if ($prompt -eq 'fail') { [Console]::Error.WriteLine('Selected model is at capacity'); exit 1 }
if ($prompt -eq 'cancel') { Start-Sleep -Seconds 30; exit 0 }
[Console]::Out.WriteLine('{"type":"turn.completed"}')
`)
	s, err := newCodexSession(context.Background(), codexSessionParams{
		cliBin: filepath.Join(dir, "codex"), workDir: dir, resumeID: "cron-test-thread",
		model: model, effort: "medium", mode: "suggest", modelProvider: "unchanged-provider",
		extraEnv: []string{"CRON_TEST_ARGS=" + argsPath, "CRON_TEST_CALLS=" + callsPath,
			"CODEX_HOME=" + filepath.Join(dir, "codex-home")},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a previously inspected native catalog default without a CLI probe.
	s.runtimeCfgFetched = time.Now()
	s.runtimeCfgModel = "native-A"
	cleanupCodexTestSession(t, s)
	return s, argsPath, callsPath
}

func assertCronExecArgs(t *testing.T, path, model string) {
	t.Helper()
	args := waitForArgsFile(t, path)
	if got := valueAfter(args, "--model"); got != model {
		t.Fatalf("exec --model = %q, want %q; argv=%q", got, model, args)
	}
	if !containsSequence(args, []string{"-c", `model_provider="unchanged-provider"`}) {
		t.Fatalf("per-turn model changed provider: %q", args)
	}
}

func TestCronModelExecSendRestoresDefault(t *testing.T) {
	for _, model := range []string{"default-A", ""} {
		t.Run("default="+model, func(t *testing.T) {
			s, args, calls := cronModelExecSession(t, model)
			wantDefault := model
			if wantDefault == "" {
				wantDefault = "native-A"
			}
			for _, selected := range []string{"task-B", model} {
				options := s.DefaultTurnOptions()
				options.Model = selected
				if err := s.SendWithTurnOptions("hello", nil, nil, options); err != nil {
					t.Fatal(err)
				}
				s.wg.Wait()
				wantWire := selected
				if wantWire == "" {
					wantWire = wantDefault
				}
				assertCronExecArgs(t, args, wantWire)
				if got := s.DefaultTurnOptions().Model; got != wantDefault {
					t.Fatalf("task model mutated default: %q", got)
				}
			}
			data, err := os.ReadFile(calls)
			if err != nil || string(data) != "launch\nlaunch\n" {
				t.Fatalf("process launches = %q, err=%v", data, err)
			}
		})
	}
}

func TestCronModelExecConcurrentSessionsStayIsolated(t *testing.T) {
	a, argsA, _ := cronModelExecSession(t, "default-A")
	b, argsB, _ := cronModelExecSession(t, "default-A")
	var wg sync.WaitGroup
	for _, pair := range []struct {
		session *codexSession
		model   string
	}{{a, "task-B"}, {b, "task-C"}} {
		wg.Add(1)
		go func(s *codexSession, model string) {
			defer wg.Done()
			if err := s.SendWithTurnOptions("hello", nil, nil, core.TurnOptions{Model: model}); err != nil {
				t.Error(err)
			}
		}(pair.session, pair.model)
	}
	wg.Wait()
	a.wg.Wait()
	b.wg.Wait()
	assertCronExecArgs(t, argsA, "task-B")
	assertCronExecArgs(t, argsB, "task-C")
	for _, s := range []*codexSession{a, b} {
		if s.DefaultTurnOptions().Model != "default-A" {
			t.Fatal("concurrent task changed session default")
		}
	}
}

func TestCronModelExecFailureAndCancellationNeverFallback(t *testing.T) {
	for _, prompt := range []string{"fail", "cancel"} {
		t.Run(prompt, func(t *testing.T) {
			s, args, calls := cronModelExecSession(t, "default-A")
			if err := s.SendWithTurnOptions(prompt, nil, nil, core.TurnOptions{Model: "task-B"}); err != nil {
				t.Fatal(err)
			}
			assertCronExecArgs(t, args, "task-B")
			if prompt == "cancel" {
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
			s.wg.Wait()
			data, err := os.ReadFile(calls)
			if err != nil || string(data) != "launch\n" {
				t.Fatalf("failure/cancel launched fallback: %q, err=%v", data, err)
			}
		})
	}
}

func TestCronModelExecUnresolvedDefaultRejectsOverrideButKeepsLegacySend(t *testing.T) {
	s, _, calls := cronModelExecSession(t, "")
	s.runtimeCfgModel = ""
	if err := s.ValidateTurnDefaults(); err == nil {
		t.Fatal("unresolved default passed preflight")
	}
	if err := s.SendWithTurnOptions("task", nil, nil, core.TurnOptions{Model: "task-B"}); err == nil {
		t.Fatal("override started without a restorable default")
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("explicit task launched a process: %v", err)
	}
	if err := s.Send("legacy", nil, nil); err != nil {
		t.Fatal(err)
	}
	s.wg.Wait()
	data, err := os.ReadFile(calls)
	if err != nil || string(data) != "launch\n" {
		t.Fatalf("legacy launches=%q err=%v", data, err)
	}
}

func TestCronModelExecResolvesUnconfiguredNativeDefaultWithoutInference(t *testing.T) {
	dir := t.TempDir()
	writeFakeCodexScript(t, dir, `#!/bin/sh
while IFS= read -r line; do
  case "$line" in
    *'"method":"initialize"'*) printf '%s\n' '{"id":1,"result":{}}' ;;
    *'"method":"config/read"'*) printf '%s\n' '{"id":2,"result":{"config":{"model":null,"model_reasoning_effort":"medium"}}}' ;;
    *'"method":"model/list"'*) printf '%s\n' '{"id":3,"result":{"data":[{"model":"native-A","isDefault":true}],"nextCursor":null}}' ;;
    *'"method":"turn/start"'*) exit 9 ;;
  esac
done
`, `
while ($null -ne ($line = [Console]::In.ReadLine())) {
  $request = $line | ConvertFrom-Json
  switch ($request.method) {
    'initialize' { [Console]::Out.WriteLine('{"id":1,"result":{}}') }
    'config/read' { [Console]::Out.WriteLine('{"id":2,"result":{"config":{"model":null,"model_reasoning_effort":"medium"}}}') }
    'model/list' { [Console]::Out.WriteLine('{"id":3,"result":{"data":[{"model":"native-A","isDefault":true}],"nextCursor":null}}') }
    'turn/start' { exit 9 }
  }
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	model, effort, err := loadCodexRuntimeConfig(ctx, filepath.Join(dir, "codex"), nil, dir, nil)
	if err != nil || model != "native-A" || effort != "medium" {
		t.Fatalf("native defaults=(%q,%q), err=%v", model, effort, err)
	}
}

func TestCronModelCatalogRequiresIdentifiedDefaultAcrossPages(t *testing.T) {
	for _, withDefault := range []bool{false, true} {
		t.Run(map[bool]string{false: "unresolved", true: "page-two-default"}[withDefault], func(t *testing.T) {
			calls := 0
			model, err := codexCatalogDefaultModel(func(method string, params any, out any) error {
				calls++
				if method != "model/list" {
					t.Fatalf("unexpected metadata RPC %s", method)
				}
				payload := `{"data":[{"model":"other-B","isDefault":false}],"nextCursor":"page-2"}`
				if calls == 2 {
					if params.(map[string]any)["cursor"] != "page-2" {
						t.Fatal("missing pagination cursor")
					}
					payload = `{"data":[],"nextCursor":null}`
					if withDefault {
						payload = `{"data":[{"model":"native-A","isDefault":true}],"nextCursor":null}`
					}
				}
				return json.Unmarshal([]byte(payload), out)
			})
			if calls != 2 {
				t.Fatalf("calls=%d", calls)
			}
			if withDefault && (err != nil || model != "native-A") {
				t.Fatalf("model=%q err=%v", model, err)
			}
			if !withDefault && (err == nil || model != "") {
				t.Fatalf("unidentified catalog silently selected %q err=%v", model, err)
			}
		})
	}
}

func cronModelAppServer(t *testing.T, model string) (*appServerSession, *steerFakeStdin) {
	t.Helper()
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "")
	s.model, s.effort, s.modelProvider = model, "medium", "unchanged-provider"
	stdin.respond = respondSuccess(s, `{"turn":{"id":"turn-1"}}`)
	return s, stdin
}

func TestCronModelAppServerSendRestoresConfiguredAndNativeDefaults(t *testing.T) {
	for _, model := range []string{"default-A", ""} {
		t.Run("default="+model, func(t *testing.T) {
			s, stdin := cronModelAppServer(t, model)
			wantDefault := model
			if wantDefault == "" {
				wantDefault = "native-A"
				s.defaultModel = &wantDefault
			}
			for _, selected := range []string{"task-B", model} {
				options := s.DefaultTurnOptions()
				options.Model = selected
				if err := s.SendWithTurnOptions("hello", nil, nil, options); err != nil {
					t.Fatal(err)
				}
				req := stdin.lastRequest()
				if req["method"] != "turn/start" {
					t.Fatalf("unexpected RPC: %v", req)
				}
				params := req["params"].(map[string]any)
				got, exists := params["model"]
				wantWire := selected
				if wantWire == "" {
					wantWire = wantDefault
				}
				if !exists || got != wantWire {
					t.Fatalf("wire model=%v present=%v, want explicit %q", got, exists, wantWire)
				}
				if s.DefaultTurnOptions().Model != wantDefault || s.modelProvider != "unchanged-provider" {
					t.Fatal("one-shot options mutated defaults/provider")
				}
			}
		})
	}
}

func TestCronModelAppServerConcurrentSessionsStayIsolated(t *testing.T) {
	a, stdinA := cronModelAppServer(t, "default-A")
	b, stdinB := cronModelAppServer(t, "default-A")
	var wg sync.WaitGroup
	for _, pair := range []struct {
		session *appServerSession
		model   string
	}{{a, "task-B"}, {b, "task-C"}} {
		wg.Add(1)
		go func(s *appServerSession, model string) {
			defer wg.Done()
			if err := s.SendWithTurnOptions("hello", nil, nil, core.TurnOptions{Model: model}); err != nil {
				t.Error(err)
			}
		}(pair.session, pair.model)
	}
	wg.Wait()
	for _, pair := range []struct {
		session *appServerSession
		stdin   *steerFakeStdin
		model   string
	}{{a, stdinA, "task-B"}, {b, stdinB, "task-C"}} {
		if got := pair.stdin.lastRequest()["params"].(map[string]any)["model"]; got != pair.model {
			t.Fatalf("cross-session model leaked: got %v, want %s", got, pair.model)
		}
		if pair.session.DefaultTurnOptions().Model != "default-A" || pair.session.modelProvider != "unchanged-provider" {
			t.Fatal("concurrent task changed defaults/provider")
		}
	}
}

func TestCronModelAppServerResumeDoesNotAdoptPreviousTaskModel(t *testing.T) {
	for _, native := range []string{"default-A", ""} {
		t.Run("native="+native, func(t *testing.T) {
			s, stdin := cronModelAppServer(t, "")
			stdin.respond = func(id int64, req map[string]any) {
				var result string
				switch req["method"] {
				case "config/read":
					result = `{"config":{"model":` + cronModelJSON(native) + `}}`
				case "model/list":
					result = `{"data":[{"model":"native-A","isDefault":true}],"nextCursor":null}`
				case "thread/resume":
					result = `{"thread":{"id":"thread-1"},"model":"task-B"}`
				case "turn/start":
					result = `{"turn":{"id":"turn-2"}}`
				default:
					t.Errorf("unexpected RPC: %v", req)
				}
				s.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(result)})
			}
			if err := s.captureDefaultModel("thread-1"); err != nil {
				t.Fatal(err)
			}
			if err := s.ensureThread("thread-1"); err != nil {
				t.Fatal(err)
			}
			wantDefault := native
			if wantDefault == "" {
				wantDefault = "native-A"
			}
			if got := s.DefaultTurnOptions().Model; got != wantDefault {
				t.Fatalf("default=%q inherited task-B; want %q", got, wantDefault)
			}
			if err := s.SendWithTurnOptions("ordinary", nil, nil, s.DefaultTurnOptions()); err != nil {
				t.Fatal(err)
			}
			got, present := stdin.lastRequest()["params"].(map[string]any)["model"]
			if !present || got != wantDefault {
				t.Fatalf("restored wire model=%v, native=%q", got, native)
			}
		})
	}
}

func cronModelJSON(text string) string { data, _ := json.Marshal(text); return string(data) }

func TestCronModelAppServerDefaultReadFailureIsRetainedWithoutFallback(t *testing.T) {
	s, stdin := cronModelAppServer(t, "")
	stdin.respond = respondError(s, "config unavailable")
	if err := s.captureDefaultModel("thread-1"); err == nil || !strings.Contains(err.Error(), "default model") {
		t.Fatalf("default configuration failure=%v; must not adopt task model", err)
	}
	if len(stdin.requests) != 1 || stdin.requests[0]["method"] != "config/read" {
		t.Fatalf("unexpected fallback: %v", stdin.requests)
	}
}

func TestCronModelAppServerDefaultReadFailurePreservesLegacySendButRejectsOverrides(t *testing.T) {
	s, stdin := cronModelAppServer(t, "")
	stdin.respond = func(id int64, req map[string]any) {
		if req["method"] == "config/read" {
			s.handleResponse(rpcResponseEnvelope{ID: id, Error: &rpcError{Message: "config unavailable"}})
			return
		}
		result := `{"turn":{"id":"turn-1"}}`
		if req["method"] == "thread/resume" {
			result = `{"thread":{"id":"thread-1"},"model":"previous-task-B"}`
		}
		s.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(result)})
	}
	if err := s.captureDefaultModel("thread-1"); err == nil {
		t.Fatal("expected failed native default lookup")
	}
	// Constructor deliberately continues along this legacy path after recording
	// the lookup error. Neither resume nor Send requires per-turn defaults.
	if err := s.ensureThread("thread-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Send("legacy ordinary message", nil, nil); err != nil {
		t.Fatalf("legacy Send regressed: %v", err)
	}
	if got := stdin.lastRequest()["params"].(map[string]any)["model"]; got != "previous-task-B" {
		t.Fatalf("legacy wire behavior changed: %v", got)
	}
	if got := s.DefaultTurnOptions().Model; got != "" {
		t.Fatalf("failed lookup adopted previous override: %q", got)
	}
	before := len(stdin.requests)
	for _, options := range []core.TurnOptions{{Model: "task-C"}, s.DefaultTurnOptions()} {
		if err := s.SendWithTurnOptions("explicit task or default restore", nil, nil, options); err == nil || !strings.Contains(err.Error(), "per-turn options unavailable") {
			t.Fatalf("explicit turn silently fell back: %v", err)
		}
	}
	if len(stdin.requests) != before {
		t.Fatalf("failed lookup submitted explicit turn/fallback: %v", stdin.requests[before:])
	}
}

func TestCronModelAppServerFailureAndCancellationNeverFallback(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(map[bool]string{false: "failure", true: "cancel"}[cancelRequest], func(t *testing.T) {
			s, stdin := cronModelAppServer(t, "default-A")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.ctx = ctx
			if cancelRequest {
				stdin.respond = func(int64, map[string]any) { cancel() }
			} else {
				stdin.respond = respondError(s, "Selected model is at capacity")
			}
			if err := s.SendWithTurnOptions("hello", nil, nil, core.TurnOptions{Model: "task-B"}); err == nil {
				t.Fatal("failed/cancelled turn reported success")
			}
			if len(stdin.requests) != 1 {
				t.Fatalf("fallback requests: %v", stdin.requests)
			}
			if s.DefaultTurnOptions().Model != "default-A" {
				t.Fatal("failed turn changed default")
			}
		})
	}
}
