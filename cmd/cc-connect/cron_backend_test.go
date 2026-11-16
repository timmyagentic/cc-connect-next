package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestCronBackendCLIRequests(t *testing.T) {
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
	base := []string{"add", "--project", "p", "--session-key", "test:chat", "--cron", "0 9 * * *", "--prompt", "report", "--session-mode", "new-per-run"}
	for _, tc := range []struct {
		name  string
		flags []string
		want  any
	}{
		{"exec", []string{"--backend", " exec "}, "exec"},
		{"app-server", []string{"--backend", "app_server"}, "app_server"},
		{"omitted", nil, nil},
		{"empty", []string{"--backend", ""}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, base...), tc.flags...)
			output, err := runCronReasoningCLI(t, append(args, "--data-dir", dir)...)
			if err != nil {
				t.Fatalf("add: %v\n%s", err, output)
			}
			got := <-requests
			if got.path != "/cron/add" || got.body["backend"] != tc.want || got.body["session_mode"] != "new-per-run" {
				t.Fatalf("add request = %#v", got)
			}
			wantDisplay := "inherit project default"
			if tc.want != nil {
				wantDisplay = tc.want.(string)
			}
			if !strings.Contains(output, "Backend: "+wantDisplay) {
				t.Fatalf("add output = %q", output)
			}
		})
	}
	for _, value := range []string{"exec", "app_server", ""} {
		output, err := runCronReasoningCLI(t, "edit", "job", "backend", value, "--data-dir", dir)
		if err != nil {
			t.Fatalf("edit %q: %v\n%s", value, err, output)
		}
		got := <-requests
		if got.path != "/cron/edit" || got.body["id"] != "job" || got.body["field"] != "backend" || got.body["value"] != value {
			t.Fatalf("edit request = %#v", got)
		}
	}
}

func TestCronBackendCLIValidation(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"add", "--cron", "0 9 * * *", "--exec", "echo hi", "--backend", "exec"}, "--backend is only supported with --prompt"},
		{[]string{"add", "--backend"}, "--backend requires a value"},
		{[]string{"add", "--backend", "--silent"}, "--backend requires a value"},
		{[]string{"edit", "job", "backend"}, "value is required"},
	} {
		output, err := runCronReasoningCLI(t, tc.args...)
		if err == nil || !strings.Contains(output, tc.want) {
			t.Fatalf("args=%v err=%v output=%s", tc.args, err, output)
		}
	}
	dir := cronReasoningTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "backend requires new_per_run", http.StatusBadRequest)
	})
	output, err := runCronReasoningCLI(t, "edit", "job", "backend", "exec", "--data-dir", dir)
	if err == nil || !strings.Contains(output, "backend requires new_per_run") {
		t.Fatalf("server validation: err=%v output=%s", err, output)
	}
}

func TestCronBackendCLIInfoListAndHelp(t *testing.T) {
	jobs := []map[string]any{
		{"id": "legacy", "prompt": "report"},
		{"id": "explicit", "prompt": "report", "backend": "exec"},
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
	for _, id := range []string{"legacy", "explicit"} {
		output, err := runCronReasoningCLI(t, "info", id, "backend", "--data-dir", dir)
		want := "inherit project default\n"
		if id == "explicit" {
			want = "exec\n"
		}
		if err != nil || output != want {
			t.Fatalf("info %s: err=%v output=%q", id, err, output)
		}
		output, err = runCronReasoningCLI(t, "info", id, "--data-dir", dir)
		var result map[string]any
		if err != nil || json.Unmarshal([]byte(output), &result) != nil || result["id"] != id {
			t.Fatalf("full info: err=%v output=%q", err, output)
		}
		if id == "legacy" && result["backend"] != nil {
			t.Fatalf("legacy field was added to JSON: %#v", result)
		}
	}
	output, err := runCronReasoningCLI(t, "list", "--data-dir", dir)
	if err != nil || strings.Count(output, "Backend: inherit project default") != 1 || strings.Count(output, "Backend: exec") != 1 || strings.Count(output, "Backend:") != 2 {
		t.Fatalf("list: err=%v output=%s", err, output)
	}
	for _, action := range []string{"add", "edit"} {
		output, err = runCronReasoningCLI(t, action, "--help")
		if err != nil || !strings.Contains(output, "backend") || !strings.Contains(output, "app_server") || !strings.Contains(output, "inherit") {
			t.Fatalf("%s help: err=%v output=%s", action, err, output)
		}
	}
}
