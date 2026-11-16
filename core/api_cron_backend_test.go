package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

type cronBackendAPIAgent struct{ *cronModelAgent }

func (a *cronBackendAPIAgent) WithSessionBackend(backend string) (Agent, error) {
	if backend != "exec" && backend != "app_server" {
		return nil, fmt.Errorf("unsupported backend %q", backend)
	}
	return a, nil
}

func TestCronBackendAPIRoundTrip(t *testing.T) {
	for _, management := range []bool{false, true} {
		name := "local"
		if management {
			name = "management"
		}
		t.Run(name, func(t *testing.T) {
			scheduler := newCronReasoningScheduler(t, &cronBackendAPIAgent{&cronModelAgent{}})
			api := &APIServer{cron: scheduler}
			mgmt := &ManagementServer{cronScheduler: scheduler}
			addPath, addHandler := "/cron/add", api.handleCronAdd
			if management {
				addPath, addHandler = "/api/v1/cron", mgmt.handleCron
			}
			body := map[string]any{"project": "p", "session_key": "test:chat", "cron_expr": "0 9 * * *", "prompt": "report", "session_mode": "new_per_run", "backend": "exec"}
			status, data := cronTierAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusOK {
				t.Fatalf("create: %d %s", status, data)
			}
			var job CronJob
			if err := json.Unmarshal(data, &job); err != nil {
				t.Fatal(err)
			}
			if job.Backend != "exec" {
				t.Fatalf("created job = %+v", job)
			}
			readPath, readHandler := "/cron/info?id="+job.ID, api.handleCronInfo
			if management {
				readPath, readHandler = "/api/v1/cron?project=p", mgmt.handleCron
			}
			status, data = cronTierAPIRequest(t, readHandler, http.MethodGet, readPath, nil, management)
			if status != http.StatusOK || !bytes.Contains(data, []byte(`"backend":"exec"`)) {
				t.Fatalf("read: %d %s", status, data)
			}
			edit := func(updates map[string]any) (int, []byte) {
				t.Helper()
				if management {
					return cronTierAPIRequest(t, mgmt.handleCronByID, http.MethodPatch, "/api/v1/cron/"+job.ID, updates, true)
				}
				for field, value := range updates {
					return cronTierAPIRequest(t, api.handleCronEdit, http.MethodPost, "/cron/edit", map[string]any{"id": job.ID, "field": field, "value": value}, false)
				}
				panic("missing edit")
			}
			status, data = edit(map[string]any{"backend": "app_server"})
			if status != http.StatusOK || scheduler.store.Snapshot(job.ID).Backend != "app_server" {
				t.Fatalf("edit: %d %s", status, data)
			}
			for _, value := range []any{"invalid", 12, nil} {
				status, data = edit(map[string]any{"backend": value})
				if status != http.StatusBadRequest || scheduler.store.Snapshot(job.ID).Backend != "app_server" {
					t.Fatalf("invalid edit: %d %s", status, data)
				}
			}
			status, data = edit(map[string]any{"backend": ""})
			if status != http.StatusOK || bytes.Contains(data, []byte(`"backend"`)) || scheduler.store.Snapshot(job.ID).Backend != "" {
				t.Fatalf("clear: %d %s", status, data)
			}
			body["backend"] = "invalid"
			status, data = cronTierAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusBadRequest || len(scheduler.store.List()) != 1 {
				t.Fatalf("invalid create: %d %s", status, data)
			}
			body["prompt"], body["exec"], body["backend"] = "", "echo hi", "exec"
			status, data = cronTierAPIRequest(t, addHandler, http.MethodPost, addPath, body, management)
			if status != http.StatusBadRequest {
				t.Fatalf("shell create: %d %s", status, data)
			}
			if !management {
				return
			}
			status, data = edit(map[string]any{"backend": "exec"})
			if status != http.StatusOK {
				t.Fatalf("restore backend: %d %s", status, data)
			}
			status, data = edit(map[string]any{"exec": "echo hi", "prompt": "", "description": "must not persist"})
			if status != http.StatusBadRequest || scheduler.store.Snapshot(job.ID).Description != "" {
				t.Fatalf("partial shell conversion: %d %s", status, data)
			}
			status, data = edit(map[string]any{"exec": "echo hi", "prompt": "", "backend": ""})
			if status != http.StatusOK || scheduler.store.Snapshot(job.ID).Backend != "" || scheduler.store.Snapshot(job.ID).Exec != "echo hi" {
				t.Fatalf("atomic shell conversion: %d %s", status, data)
			}
		})
	}
}
