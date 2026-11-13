package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestCronServiceTierCLIRequests(t *testing.T) {
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
		{"tier without model", []string{"--service-tier", " priority "}, "priority", nil},
		{"tier with model", []string{"--model", "model-B", "--service-tier", "priority"}, "priority", "model-B"},
		{"omitted inherits", nil, nil, nil},
		{"empty inherits", []string{"--service-tier", ""}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, base...), tc.flags...)
			output, err := runCronReasoningCLI(t, append(args, "--data-dir", dir)...)
			if err != nil {
				t.Fatalf("add: %v\n%s", err, output)
			}
			got := <-requests
			if got.path != "/cron/add" || got.body["service_tier"] != tc.want || got.body["model"] != tc.model {
				t.Fatalf("add request = %#v", got)
			}
		})
	}
	for _, value := range []string{"priority", ""} {
		output, err := runCronReasoningCLI(t, "edit", "job", "service_tier", value, "--data-dir", dir)
		if err != nil {
			t.Fatalf("edit %q: %v\n%s", value, err, output)
		}
		got := <-requests
		if got.path != "/cron/edit" || got.body["id"] != "job" || got.body["field"] != "service_tier" || got.body["value"] != value {
			t.Fatalf("edit request = %#v", got)
		}
	}
}

func TestCronServiceTierCLIRejectsInvalidArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"shell", []string{"add", "--cron", "0 9 * * *", "--exec", "echo hi", "--service-tier", "priority"}, "--service-tier is only supported with --prompt"},
		{"missing add value", []string{"add", "--service-tier"}, "--service-tier requires a value"},
		{"next flag", []string{"add", "--service-tier", "--silent"}, "--service-tier requires a value"},
		{"missing edit value", []string{"edit", "job", "service_tier"}, "value is required"},
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

func TestCronServiceTierCLIServerValidation(t *testing.T) {
	dir := cronReasoningTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unsupported service tier: priority", http.StatusBadRequest)
	})
	for _, args := range [][]string{
		{"add", "--cron", "0 9 * * *", "--prompt", "report", "--service-tier", "priority"},
		{"edit", "job", "service_tier", "priority"},
	} {
		output, err := runCronReasoningCLI(t, append(args, "--data-dir", dir)...)
		if err == nil || !strings.Contains(output, "unsupported service tier: priority") {
			t.Fatalf("server validation: err=%v output=%s", err, output)
		}
	}
}

func TestCronServiceTierCLIHelp(t *testing.T) {
	for _, tc := range []struct {
		action string
		terms  []string
	}{
		{"add", []string{"--service-tier <tier>", "Prompt-only service tier", "validated against model catalog"}},
		{"edit", []string{"service_tier", `"" restores inheritance`, "catalog ID", "<catalog-tier-id>"}},
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
