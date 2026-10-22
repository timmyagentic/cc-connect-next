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

func TestCronModelCLIRequests(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket fixture")
	}
	// Keep the socket name short even under a long TMPDIR.
	dir, err := os.MkdirTemp("", "cm-")
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
	requests := make(chan map[string]any, 3)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		requests <- body
		body["id"] = "job"
		_ = json.NewEncoder(w).Encode(body)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	runCronAdd([]string{"--project", "p", "--session-key", "test:chat", "--cron", "0 9 * * *", "--prompt", "report", "--model", " model-B ", "--data-dir", dir})
	if got := (<-requests)["model"]; got != "model-B" {
		t.Fatalf("add model = %#v", got)
	}
	runCronEdit([]string{"job", "model", "", "--data-dir", dir})
	clear := <-requests
	if clear["field"] != "model" || clear["value"] != "" {
		t.Fatalf("clear request = %#v", clear)
	}
	runCronAdd([]string{"--project", "p", "--session-key", "test:chat", "--cron", "0 9 * * *", "--prompt", "report", "--data-dir", dir})
	if old := <-requests; old["model"] != nil {
		t.Fatalf("legacy add should omit model: %#v", old)
	}
}

func TestCronModelCLIRejectsInvalidArguments(t *testing.T) {
	if os.Getenv("CC_CRON_MODEL_TEST_HELPER") == "1" {
		separator := 0
		for i, arg := range os.Args {
			if arg == "--" {
				separator = i + 1
				break
			}
		}
		runCronAdd(os.Args[separator:])
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"shell", []string{"--cron", "0 9 * * *", "--exec", "echo hi", "--model", "model-B"}, "only supported with --prompt"},
		{"missing", []string{"--cron", "0 9 * * *", "--prompt", "report", "--model"}, "requires a model ID"},
		{"next flag", []string{"--model", "--silent"}, "requires a model ID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestCronModelCLIRejectsInvalidArguments$", "--"}, tc.args...)
			command := exec.Command(executable, args...)
			command.Env = append(os.Environ(), "CC_CRON_MODEL_TEST_HELPER=1")
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tc.want) {
				t.Fatalf("err=%v output=%s", err, output)
			}
		})
	}
}

func TestCronModelDisplayAndClearValue(t *testing.T) {
	if got := cronModelDisplay(map[string]any{}); got != "inherit project default" {
		t.Fatalf("inherit = %q", got)
	}
	if got := cronModelDisplay(map[string]any{"model": "model-B"}); got != "model-B" {
		t.Fatalf("model = %q", got)
	}
	value, err := parseCronEditValue("model", "")
	if err != nil || value != "" {
		t.Fatalf("clear = %#v, %v", value, err)
	}
}
