package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCronReasoningEffortCLIHelper(t *testing.T) {
	if os.Getenv("CC_CRON_REASONING_TEST_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			runCron(os.Args[i+1:])
			os.Exit(0)
		}
	}
	t.Fatal("missing CLI arguments")
}

func runCronReasoningCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, append([]string{"-test.run=^TestCronReasoningEffortCLIHelper$", "--"}, args...)...)
	command.Env = append(os.Environ(), "CC_CRON_REASONING_TEST_HELPER=1")
	output, err := command.CombinedOutput()
	return string(output), err
}

func cronReasoningTestServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket fixture")
	}
	// Avoid long test names in the Unix socket path.
	dir, err := os.MkdirTemp("", "cr-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.MkdirAll(filepath.Join(dir, "run"), 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", resolveSocketPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return dir
}

func TestCronReasoningEffortCLIRequests(t *testing.T) {
	type request struct {
		path string
		body map[string]any
	}
	requests := make(chan request, 1)
	dir := cronReasoningTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request{r.URL.Path, body}
		_ = json.NewEncoder(w).Encode(body)
	})
	base := []string{"add", "--project", "p", "--session-key", "test:chat", "--cron", "0 9 * * *", "--prompt", "report"}
	for _, tc := range []struct {
		name  string
		flags []string
		want  any
		model any
	}{
		{"effort without model", []string{"--reasoning-effort", " provider-effort "}, "provider-effort", nil},
		{"effort with model", []string{"--model", "model-B", "--reasoning-effort", "provider-effort"}, "provider-effort", "model-B"},
		{"omitted inherits", nil, nil, nil},
		{"empty inherits", []string{"--reasoning-effort", ""}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, base...), tc.flags...)
			output, err := runCronReasoningCLI(t, append(args, "--data-dir", dir)...)
			if err != nil {
				t.Fatalf("add: %v\n%s", err, output)
			}
			got := <-requests
			if got.path != "/cron/add" || got.body["reasoning_effort"] != tc.want || got.body["model"] != tc.model {
				t.Fatalf("add request = %#v", got)
			}
			wantDisplay := "inherit project default"
			if tc.want != nil {
				wantDisplay = tc.want.(string)
			}
			if !strings.Contains(output, "Reasoning effort: "+wantDisplay) {
				t.Fatalf("add output = %q", output)
			}
		})
	}
	for _, value := range []string{"provider-effort", ""} {
		output, err := runCronReasoningCLI(t, "edit", "job", "reasoning_effort", value, "--data-dir", dir)
		if err != nil {
			t.Fatalf("edit %q: %v\n%s", value, err, output)
		}
		got := <-requests
		if got.path != "/cron/edit" || got.body["id"] != "job" || got.body["field"] != "reasoning_effort" || got.body["value"] != value {
			t.Fatalf("edit request = %#v", got)
		}
	}
}

func TestCronReasoningEffortCLIRejectsInvalidArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"shell", []string{"add", "--cron", "0 9 * * *", "--exec", "echo hi", "--reasoning-effort", "provider-effort"}, "--reasoning-effort is only supported with --prompt"},
		{"missing add value", []string{"add", "--reasoning-effort"}, "--reasoning-effort requires a value"},
		{"next flag", []string{"add", "--reasoning-effort", "--silent"}, "--reasoning-effort requires a value"},
		{"missing edit value", []string{"edit", "job", "reasoning_effort"}, "value is required"},
		{"unrelated empty edit", []string{"edit", "job", "prompt", ""}, "value is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := runCronReasoningCLI(t, tc.args...)
			if err == nil || !strings.Contains(output, tc.want) {
				t.Fatalf("err=%v output=%s", err, output)
			}
		})
	}
}

func TestCronReasoningEffortCLIServerValidation(t *testing.T) {
	dir := cronReasoningTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unsupported reasoning effort: provider-effort", http.StatusBadRequest)
	})
	for _, args := range [][]string{
		{"add", "--cron", "0 9 * * *", "--prompt", "report", "--reasoning-effort", "provider-effort"},
		{"edit", "job", "reasoning_effort", "provider-effort"},
	} {
		output, err := runCronReasoningCLI(t, append(args, "--data-dir", dir)...)
		if err == nil || !strings.Contains(output, "unsupported reasoning effort: provider-effort") {
			t.Fatalf("server validation: err=%v output=%s", err, output)
		}
	}
}

func TestCronReasoningEffortCLIInfoAndList(t *testing.T) {
	jobs := []map[string]any{
		{"id": "legacy", "prompt": "report"},
		{"id": "empty", "prompt": "report", "reasoning_effort": ""},
		{"id": "explicit", "prompt": "report", "reasoning_effort": "provider-effort"},
		{"id": "shell", "exec": "echo hi"},
	}
	dir := cronReasoningTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cron/list" {
			_ = json.NewEncoder(w).Encode(jobs)
			return
		}
		for _, job := range jobs {
			if job["id"] == r.URL.Query().Get("id") {
				_ = json.NewEncoder(w).Encode(job)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
	for _, id := range []string{"legacy", "empty", "explicit"} {
		output, err := runCronReasoningCLI(t, "info", id, "reasoning_effort", "--data-dir", dir)
		want := "inherit project default\n"
		if id == "explicit" {
			want = "provider-effort\n"
		}
		if err != nil || output != want {
			t.Fatalf("info %s: err=%v output=%q", id, err, output)
		}
		output, err = runCronReasoningCLI(t, "info", id, "--data-dir", dir)
		var job map[string]any
		if err != nil || json.Unmarshal([]byte(output), &job) != nil || job["id"] != id {
			t.Fatalf("full info must remain JSON: err=%v output=%q", err, output)
		}
		if id == "legacy" {
			if _, present := job["reasoning_effort"]; present {
				t.Fatalf("full info must preserve omitted field: %#v", job)
			}
		}
	}
	output, err := runCronReasoningCLI(t, "list", "--data-dir", dir)
	if err != nil || strings.Count(output, "Reasoning effort: inherit project default") != 2 || strings.Count(output, "Reasoning effort: provider-effort") != 1 || strings.Count(output, "Reasoning effort:") != 3 {
		t.Fatalf("list: err=%v output=%s", err, output)
	}
}

func TestCronReasoningEffortCLIHelp(t *testing.T) {
	for _, tc := range []struct {
		action string
		terms  []string
	}{
		{"add", []string{"--reasoning-effort <value>", "Prompt-only effort", "inherit project default", "Model optional", "validated by server"}},
		{"edit", []string{"reasoning_effort", `"" restores inheritance`, "model optional", "validated by the server"}},
	} {
		output, err := runCronReasoningCLI(t, tc.action, "--help")
		if err != nil {
			t.Fatalf("help: %v\n%s", err, output)
		}
		for _, term := range tc.terms {
			if !strings.Contains(output, term) {
				t.Errorf("%s help missing %q: %s", tc.action, term, output)
			}
		}
	}
}
