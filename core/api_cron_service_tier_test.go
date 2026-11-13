package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cronTierAPIRequest(t *testing.T, handler http.HandlerFunc, method, path string, body any, management bool) (int, []byte) {
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

func TestCronServiceTierAPIRoundTrip(t *testing.T) {
	for _, management := range []bool{false, true} {
		name := "local"
		if management {
			name = "management"
		}
		t.Run(name, func(t *testing.T) {
			scheduler := newCronReasoningScheduler(t, &cronSpeedAgent{&cronModelAgent{}})
			api := &APIServer{cron: scheduler}
			mgmt := &ManagementServer{cronScheduler: scheduler}
			addPath, addHandler := "/cron/add", api.handleCronAdd
			if management {
				addPath, addHandler = "/api/v1/cron", mgmt.handleCron
			}
			body := map[string]any{"project": "p", "session_key": "test:chat", "cron_expr": "0 9 * * *", "prompt": "report", "service_tier": " fast "}
			status, data := cronTierAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusOK {
				t.Fatalf("create: %d %s", status, data)
			}
			var job CronJob
			if err := json.Unmarshal(data, &job); err != nil {
				t.Fatal(err)
			}
			if job.ServiceTier != "fast" || job.Model != "" {
				t.Fatalf("created tier-only job = %+v", job)
			}
			readPath, readHandler := "/cron/info?id="+job.ID, api.handleCronInfo
			if management {
				readPath, readHandler = "/api/v1/cron?project=p", mgmt.handleCron
			}
			status, data = cronTierAPIRequest(t, readHandler, http.MethodGet, readPath, nil, management)
			if status != http.StatusOK || !bytes.Contains(data, []byte(`"service_tier":"fast"`)) {
				t.Fatalf("read: %d %s", status, data)
			}
			edit := func(updates map[string]any) (int, []byte) {
				t.Helper()
				if management {
					return cronTierAPIRequest(t, mgmt.handleCronByID, http.MethodPatch, "/api/v1/cron/"+job.ID, updates, true)
				}
				if len(updates) != 1 {
					t.Fatal("local edit expects one field")
				}
				for field, value := range updates {
					return cronTierAPIRequest(t, api.handleCronEdit, http.MethodPost, "/cron/edit", map[string]any{"id": job.ID, "field": field, "value": value}, false)
				}
				panic("unreachable")
			}
			status, data = edit(map[string]any{"service_tier": " standard "})
			if status != http.StatusOK || !bytes.Contains(data, []byte(`"service_tier":"standard"`)) {
				t.Fatalf("edit: %d %s", status, data)
			}
			for _, value := range []any{"invalid", 12, nil} {
				status, data = edit(map[string]any{"service_tier": value})
				if status != http.StatusBadRequest || scheduler.store.Snapshot(job.ID).ServiceTier != "standard" {
					t.Fatalf("invalid edit: %d %s", status, data)
				}
			}
			status, data = edit(map[string]any{"service_tier": ""})
			if status != http.StatusOK || bytes.Contains(data, []byte(`"service_tier"`)) || scheduler.store.Snapshot(job.ID).ServiceTier != "" {
				t.Fatalf("clear: %d %s", status, data)
			}
			body["service_tier"] = "invalid"
			status, data = cronTierAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusBadRequest || !strings.Contains(string(data), "invalid service_tier") || len(scheduler.store.List()) != 1 {
				t.Fatalf("invalid create: %d %s", status, data)
			}
			body["prompt"], body["exec"], body["service_tier"] = "", "echo hi", "fast"
			status, data = cronTierAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusBadRequest || !strings.Contains(string(data), "only supported for prompt jobs") {
				t.Fatalf("shell create: %d %s", status, data)
			}
			if !management {
				return
			}
			status, data = edit(map[string]any{"service_tier": "fast", "model": "model-B"})
			if status != http.StatusOK {
				t.Fatalf("set both overrides: %d %s", status, data)
			}
			status, data = edit(map[string]any{"exec": "echo hi", "prompt": "", "model": "", "description": "must not persist"})
			if status != http.StatusBadRequest {
				t.Fatalf("partial conversion accepted: %d %s", status, data)
			}
			if got := scheduler.store.Snapshot(job.ID); got.Prompt != "report" || got.Exec != "" || got.Model != "model-B" || got.ServiceTier != "fast" || got.Description != "" {
				t.Fatalf("partial conversion persisted: %+v", got)
			}
			status, data = edit(map[string]any{"exec": "echo hi", "prompt": "", "model": "", "service_tier": ""})
			if status != http.StatusOK || bytes.Contains(data, []byte(`"service_tier"`)) || bytes.Contains(data, []byte(`"model"`)) {
				t.Fatalf("atomic conversion: %d %s", status, data)
			}
			if got := scheduler.store.Snapshot(job.ID); got.Exec != "echo hi" || got.Prompt != "" {
				t.Fatalf("conversion not persisted: %+v", got)
			}
		})
	}
}
