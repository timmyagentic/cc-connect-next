package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cronReasoningAPIRequest(t *testing.T, handler http.HandlerFunc, method, path string, body any, management bool) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(method, path, &buf))
	data := rec.Body.Bytes()
	if management && rec.Code == http.StatusOK {
		var envelope mgmtResponse
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatal(err)
		}
		if !envelope.OK {
			t.Fatalf("management request failed: %s", data)
		}
		data = envelope.Data
	}
	return rec.Code, data
}

func TestCronReasoningAPIRoundTrip(t *testing.T) {
	for _, management := range []bool{false, true} {
		name := "local"
		if management {
			name = "management"
		}
		t.Run(name, func(t *testing.T) {
			scheduler := newCronReasoningScheduler(t, &cronModelAgent{})
			api := &APIServer{cron: scheduler}
			mgmt := &ManagementServer{cronScheduler: scheduler}
			addPath, addHandler := "/cron/add", api.handleCronAdd
			if management {
				addPath, addHandler = "/api/v1/cron", mgmt.handleCron
			}
			body := map[string]any{"project": "p", "session_key": "test:chat", "cron_expr": "0 9 * * *", "prompt": "report", "reasoning_effort": " HIGH "}
			status, data := cronReasoningAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusOK {
				t.Fatalf("create: %d %s", status, data)
			}
			var job CronJob
			if err := json.Unmarshal(data, &job); err != nil {
				t.Fatal(err)
			}
			if job.ReasoningEffort != "high" || job.Model != "" {
				t.Fatalf("created effort-only job = %+v", job)
			}
			readPath, readHandler := "/cron/info?id="+job.ID, api.handleCronInfo
			if management {
				readPath, readHandler = "/api/v1/cron?project=p", mgmt.handleCron
			}
			status, data = cronReasoningAPIRequest(t, readHandler, http.MethodGet, readPath, nil, management)
			if status != http.StatusOK || !bytes.Contains(data, []byte(`"reasoning_effort":"high"`)) {
				t.Fatalf("read: %d %s", status, data)
			}
			edit := func(updates map[string]any) (int, []byte) {
				t.Helper()
				if management {
					return cronReasoningAPIRequest(t, mgmt.handleCronByID, http.MethodPatch, "/api/v1/cron/"+job.ID, updates, true)
				}
				if len(updates) != 1 {
					t.Fatal("local edit expects one field")
				}
				for field, value := range updates {
					return cronReasoningAPIRequest(t, api.handleCronEdit, http.MethodPost, "/cron/edit", map[string]any{"id": job.ID, "field": field, "value": value}, false)
				}
				panic("unreachable")
			}
			status, data = edit(map[string]any{"reasoning_effort": " LoW "})
			if status != http.StatusOK || !bytes.Contains(data, []byte(`"reasoning_effort":"low"`)) {
				t.Fatalf("edit: %d %s", status, data)
			}
			for _, value := range []any{"invalid", 12, nil} {
				status, data = edit(map[string]any{"reasoning_effort": value})
				if status != http.StatusBadRequest || scheduler.store.Snapshot(job.ID).ReasoningEffort != "low" {
					t.Fatalf("invalid edit: %d %s", status, data)
				}
			}
			status, data = edit(map[string]any{"reasoning_effort": ""})
			if status != http.StatusOK || bytes.Contains(data, []byte(`"reasoning_effort"`)) || scheduler.store.Snapshot(job.ID).ReasoningEffort != "" {
				t.Fatalf("clear: %d %s", status, data)
			}
			body["reasoning_effort"] = "invalid"
			status, data = cronReasoningAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusBadRequest || !strings.Contains(string(data), "invalid reasoning_effort") || len(scheduler.store.List()) != 1 {
				t.Fatalf("invalid create: %d %s", status, data)
			}
			body["prompt"], body["exec"], body["reasoning_effort"] = "", "echo hi", "high"
			status, data = cronReasoningAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusBadRequest || !strings.Contains(string(data), "only supported for prompt jobs") {
				t.Fatalf("shell create: %d %s", status, data)
			}
			if !management {
				return
			}
			status, data = edit(map[string]any{"reasoning_effort": "high", "model": "model-B"})
			if status != http.StatusOK {
				t.Fatalf("set both overrides: %d %s", status, data)
			}
			status, data = edit(map[string]any{"exec": "echo hi", "prompt": "", "model": "", "description": "must not persist"})
			if status != http.StatusBadRequest {
				t.Fatalf("partial conversion accepted: %d %s", status, data)
			}
			if got := scheduler.store.Snapshot(job.ID); got.Prompt != "report" || got.Exec != "" || got.Model != "model-B" || got.ReasoningEffort != "high" || got.Description != "" {
				t.Fatalf("partial conversion persisted: %+v", got)
			}
			status, data = edit(map[string]any{"exec": "echo hi", "prompt": "", "model": "", "reasoning_effort": ""})
			if status != http.StatusOK || bytes.Contains(data, []byte(`"reasoning_effort"`)) || bytes.Contains(data, []byte(`"model"`)) {
				t.Fatalf("atomic conversion: %d %s", status, data)
			}
			if got := scheduler.store.Snapshot(job.ID); got.Exec != "echo hi" || got.Prompt != "" {
				t.Fatalf("conversion not persisted: %+v", got)
			}
		})
	}
}
