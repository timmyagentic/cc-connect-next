package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCronModelPersistenceAndClear(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCronStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	job := &CronJob{ID: "model", Project: "p", SessionKey: "test:chat", CronExpr: "0 9 * * *", Prompt: "report", Model: "  model-B  "}
	if err := scheduler.AddJob(job); err != nil {
		t.Fatal(err)
	}
	reload, err := NewCronStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reload.Snapshot(job.ID).Model; got != "model-B" {
		t.Fatalf("persisted model = %q", got)
	}
	if err := scheduler.UpdateJob(job.ID, "model", ""); err != nil {
		t.Fatal(err)
	}
	reload, err = NewCronStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(reload.Snapshot(job.ID))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"model":`) {
		t.Fatalf("cleared override must be omitted: %s", data)
	}
	var legacy CronJob
	if err := json.Unmarshal([]byte(`{"id":"old","prompt":"report"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Model != "" {
		t.Fatalf("legacy job model = %q", legacy.Model)
	}
}

func TestCronModelEditsAreValidatedAtomically(t *testing.T) {
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	job := &CronJob{ID: "model", Project: "p", SessionKey: "test:chat", CronExpr: "0 9 * * *", Prompt: "report", Model: "model-B"}
	if err := scheduler.AddJob(job); err != nil {
		t.Fatal(err)
	}
	for _, update := range []map[string]any{
		{"exec": "echo hello"}, {"prompt": ""}, {"model": 12},
		{"model": "model-C", "exec": "echo hello", "description": "must not persist"},
		{"model": "model-C", "id": "changed"},
	} {
		if err := scheduler.UpdateJobFields(job.ID, update); err == nil {
			t.Fatalf("accepted invalid edit: %#v", update)
		}
		got := store.Snapshot(job.ID)
		if got.Model != "model-B" || got.Exec != "" || got.Prompt != "report" || got.Description != "" {
			t.Fatalf("partial edit: %+v", got)
		}
	}
	if err := scheduler.UpdateJobFields(job.ID, map[string]any{"model": "", "prompt": "", "exec": "echo hello"}); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.UpdateJob(job.ID, "model", "model-C"); err == nil {
		t.Fatal("accepted model on shell job")
	}
	if err := scheduler.AddJob(&CronJob{ID: "bad", SessionKey: "test:chat", CronExpr: "0 9 * * *", Exec: "echo hello", Model: "model-B"}); err == nil {
		t.Fatal("accepted shell model at creation")
	}
	got := store.Snapshot(job.ID)
	if got.Model != "" || got.Prompt != "" || got.Exec != "echo hello" {
		t.Fatalf("conversion = %+v", got)
	}
}

func TestCronModelSnapshotIsIndependentOfConcurrentEdits(t *testing.T) {
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	timeout, silent := 30, false
	if err := store.Add(&CronJob{ID: "model", Prompt: "report", Model: "model-B", TimeoutMins: &timeout, Silent: &silent}); err != nil {
		t.Fatal(err)
	}
	run := store.Snapshot("model")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 25; i++ {
			if !store.Update("model", "model", "model-C") {
				t.Error("update failed")
			}
			if got := store.Snapshot("model").Model; got != "model-C" {
				t.Errorf("next run model = %q", got)
			}
		}
	}()
	for i := 0; i < 25; i++ {
		if run.Model != "model-B" {
			t.Errorf("running job changed model: %q", run.Model)
		}
		_ = store.Snapshot("model")
	}
	wg.Wait()
	*run.TimeoutMins, *run.Silent = 1, true
	saved := store.Snapshot("model")
	if *saved.TimeoutMins != 30 || *saved.Silent {
		t.Fatal("snapshot pointers alias persisted settings")
	}
}

func TestCronModelLocalAPIRoundTrip(t *testing.T) {
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	api := &APIServer{cron: NewCronScheduler(store)}
	request := func(path, body string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body)))
		return rec
	}
	rec := request("/cron/add", `{"project":"p","session_key":"test:chat","cron_expr":"0 9 * * *","prompt":"report","model":"model-B"}`, api.handleCronAdd)
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	var job CronJob
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Model != "model-B" {
		t.Fatalf("model = %q", job.Model)
	}
	rec = request("/cron/edit", `{"id":"`+job.ID+`","field":"model","value":""}`, api.handleCronEdit)
	if rec.Code != http.StatusOK || store.Snapshot(job.ID).Model != "" {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body)
	}
	rec = request("/cron/add", `{"project":"p","session_key":"test:chat","cron_expr":"0 9 * * *","exec":"echo hi","model":"model-B"}`, api.handleCronAdd)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("shell model: %d %s", rec.Code, rec.Body)
	}
}

func TestCronModelManagementAPIRoundTrip(t *testing.T) {
	mgmt, server, engine := testManagementServer(t, "tok")
	engine.agent = &cronModelStorageAgent{}
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test-project", engine)
	mgmt.SetCronScheduler(scheduler)
	result := mgmtPost(t, server.URL+"/api/v1/cron", "tok", map[string]any{"project": "test-project", "session_key": "test:chat", "cron_expr": "0 9 * * *", "prompt": "report", "model": "model-B"})
	if !result.OK {
		t.Fatal(result.Error)
	}
	var job CronJob
	if err := json.Unmarshal(result.Data, &job); err != nil {
		t.Fatal(err)
	}
	if job.Model != "model-B" {
		t.Fatalf("model = %q", job.Model)
	}
	result = mgmtPatch(t, server.URL+"/api/v1/cron/"+job.ID, "tok", map[string]any{"model": "", "prompt": "", "exec": "echo hi"})
	if !result.OK {
		t.Fatal(result.Error)
	}
	// Decode into a fresh value: omitted model must mean no override.
	var updated CronJob
	if err := json.Unmarshal(result.Data, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Model != "" || updated.Exec != "echo hi" {
		t.Fatalf("updated = %+v", updated)
	}
	result = mgmtPatch(t, server.URL+"/api/v1/cron/"+job.ID, "tok", map[string]any{"model": "model-C", "description": "invalid"})
	if result.OK || store.Snapshot(job.ID).Description != "" {
		t.Fatalf("invalid patch applied: %+v", result)
	}
}

type cronModelStorageAgent struct{ stubAgent }

func (*cronModelStorageAgent) SupportsPerTurnModel() bool { return true }

func TestCronModelUnsupportedProjectRejectsOnlyOverrides(t *testing.T) {
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	engine := NewEngine("p", &stubAgent{}, nil, "", LangEnglish)
	defer engine.cancel()
	scheduler.RegisterEngine("p", engine)
	job := &CronJob{ID: "legacy", Project: "p", SessionKey: "test:chat", CronExpr: "0 9 * * *", Prompt: "report"}
	if err := scheduler.AddJob(job); err != nil {
		t.Fatalf("legacy job rejected: %v", err)
	}
	if err := scheduler.UpdateJob(job.ID, "model", "model-B"); err == nil {
		t.Fatal("unsupported override accepted")
	}
	if store.Snapshot(job.ID).Model != "" {
		t.Fatal("unsupported edit was persisted")
	}
	job = &CronJob{ID: "model", Project: "p", SessionKey: "test:chat", CronExpr: "0 9 * * *", Prompt: "report", Model: "model-B"}
	if err := scheduler.AddJob(job); err == nil {
		t.Fatal("unsupported creation accepted")
	}
	// A previously persisted job can outlive a change to an unsupported agent.
	if err := store.Add(job); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.RunJobNow(job.ID); err == nil {
		t.Fatal("unsupported manual run accepted")
	}
	if err := scheduler.UpdateJob(job.ID, "enabled", false); err != nil {
		t.Fatalf("disabling an unsupported stored job failed: %v", err)
	}
	if err := scheduler.UpdateJob(job.ID, "mute", true); err != nil {
		t.Fatalf("muting an unsupported stored job failed: %v", err)
	}
	if err := scheduler.UpdateJob(job.ID, "model", ""); err != nil {
		t.Fatalf("clear rejected: %v", err)
	}
}

// Both the Web PATCH path and chat enable/disable commands can edit a job.
func TestCronModelConcurrentEnableAndPatchKeepScheduleConsistent(t *testing.T) {
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	job := &CronJob{ID: "model", Project: "p", SessionKey: "test:chat", CronExpr: "0 9 * * *", Prompt: "report", Model: "model-B"}
	if err := scheduler.AddJob(job); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := scheduler.EnableJob(job.ID); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := scheduler.UpdateJob(job.ID, "enabled", false); err != nil {
				t.Error(err)
			}
		}()
		wg.Wait()
		scheduler.mu.RLock()
		_, scheduled := scheduler.entries[job.ID]
		scheduler.mu.RUnlock()
		if enabled := store.Snapshot(job.ID).Enabled; enabled != scheduled {
			t.Fatalf("enabled=%v but scheduled=%v after concurrent edits", enabled, scheduled)
		}
	}
}
