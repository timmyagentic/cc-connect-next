package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Effort-only support proves that an override does not require model support.
type cronReasoningStorageAgent struct {
	stubAgent
	supported bool
	values    []string
}

func (a *cronReasoningStorageAgent) SupportsPerTurnReasoningEffort() bool { return a.supported }
func (a *cronReasoningStorageAgent) AvailableReasoningEfforts() []string  { return a.values }
func (*cronReasoningStorageAgent) GetReasoningEffort() string             { return "medium" }
func (*cronReasoningStorageAgent) SetReasoningEffort(string) {
	panic("cron validation must not mutate project effort")
}

type cronReasoningNoCatalogAgent struct{ stubAgent }

func (*cronReasoningNoCatalogAgent) SupportsPerTurnReasoningEffort() bool { return true }

func newCronReasoningScheduler(t *testing.T, agent Agent) *CronScheduler {
	t.Helper()
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	engine := NewEngine("p", agent, nil, "", LangEnglish)
	t.Cleanup(engine.cancel)
	scheduler.RegisterEngine("p", engine)
	return scheduler
}

func cronReasoningStorageJob(id, effort string) *CronJob {
	return &CronJob{ID: id, Project: "p", SessionKey: "test:chat", CronExpr: "0 9 * * *", Prompt: "report", ReasoningEffort: effort}
}

func TestCronReasoningPersistenceAndClear(t *testing.T) {
	a := &cronReasoningStorageAgent{supported: true, values: []string{"low", "medium", "high"}}
	scheduler := newCronReasoningScheduler(t, a)
	job := cronReasoningStorageJob("effort", "  HIGH  ")
	if err := scheduler.AddJob(job); err != nil {
		t.Fatal(err)
	}
	reload := &CronStore{path: scheduler.store.path}
	reload.load()
	if got := reload.Snapshot(job.ID); got.ReasoningEffort != "high" || got.Model != "" {
		t.Fatalf("persisted effort-only job = %+v", got)
	}
	snapshot := scheduler.store.Snapshot(job.ID)
	if err := scheduler.UpdateJob(job.ID, "reasoning_effort", " LoW "); err != nil {
		t.Fatal(err)
	}
	if snapshot.ReasoningEffort != "high" || scheduler.store.Snapshot(job.ID).ReasoningEffort != "low" {
		t.Fatal("edit changed an existing run snapshot or was not normalized")
	}
	if err := scheduler.UpdateJob(job.ID, "reasoning_effort", " \t "); err != nil {
		t.Fatal(err)
	}
	reload = &CronStore{path: scheduler.store.path}
	reload.load()
	data, err := json.Marshal(reload.Snapshot(job.ID))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"reasoning_effort"`)) {
		t.Fatalf("cleared override must be omitted: %s", data)
	}
	var legacy CronJob
	if err := json.Unmarshal([]byte(`{"id":"old","prompt":"report"}`), &legacy); err != nil || legacy.ReasoningEffort != "" {
		t.Fatalf("legacy effort = %q, error = %v", legacy.ReasoningEffort, err)
	}
}

func TestCronReasoningValidationAtCreateAndEdit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent Agent
		value string
		want  string
	}{
		{"missing capability", &stubAgent{}, "high", ErrReasoningEffortOverrideUnsupported.Error()},
		{"disabled capability", &cronReasoningStorageAgent{values: []string{"high"}}, "high", ErrReasoningEffortOverrideUnsupported.Error()},
		{"missing catalog", &cronReasoningNoCatalogAgent{}, "high", ErrReasoningEffortOverrideUnsupported.Error()},
		{"empty catalog", &cronReasoningStorageAgent{supported: true}, "high", "invalid reasoning_effort"},
		{"invalid value", &cronReasoningStorageAgent{supported: true, values: []string{"low"}}, "high", "invalid reasoning_effort"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheduler := newCronReasoningScheduler(t, tc.agent)
			if err := scheduler.AddJob(cronReasoningStorageJob("bad", tc.value)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("create error = %v, want %q", err, tc.want)
			}
			if scheduler.store.Snapshot("bad") != nil {
				t.Fatal("invalid creation was persisted")
			}
			if err := scheduler.AddJob(cronReasoningStorageJob("legacy", "")); err != nil {
				t.Fatalf("inherited effort rejected: %v", err)
			}
			err := scheduler.UpdateJobFields("legacy", map[string]any{"reasoning_effort": tc.value, "description": "must not persist"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("edit error = %v, want %q", err, tc.want)
			}
			if got := scheduler.store.Snapshot("legacy"); got.ReasoningEffort != "" || got.Description != "" {
				t.Fatalf("invalid edit was partially persisted: %+v", got)
			}
		})
	}
}

func TestCronReasoningAtomicShellConversion(t *testing.T) {
	scheduler := newCronReasoningScheduler(t, &cronModelAgent{})
	job := cronReasoningStorageJob("both", "high")
	job.Model = "model-B"
	if err := scheduler.AddJob(job); err != nil {
		t.Fatal(err)
	}
	before := scheduler.store.Snapshot(job.ID)
	beforeFile, err := os.ReadFile(scheduler.store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, updates := range []map[string]any{
		{"exec": "echo hi", "prompt": "", "model": ""},
		{"exec": "echo hi", "prompt": "", "reasoning_effort": ""},
		{"prompt": ""},
		{"reasoning_effort": 12, "description": "invalid"},
		{"reasoning_effort": "invalid", "model": "model-C"},
		{"reasoning_effort": "low", "id": "changed"},
	} {
		if err := scheduler.UpdateJobFields(job.ID, updates); err == nil {
			t.Fatalf("accepted invalid edit: %#v", updates)
		}
		afterFile, err := os.ReadFile(scheduler.store.path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, scheduler.store.Snapshot(job.ID)) || !bytes.Equal(beforeFile, afterFile) {
			t.Fatalf("invalid edit mutated memory or disk: %#v", updates)
		}
	}
	if err := scheduler.UpdateJobFields(job.ID, map[string]any{"exec": "echo hi", "prompt": "", "model": "", "reasoning_effort": ""}); err != nil {
		t.Fatal(err)
	}
	got := scheduler.store.Snapshot(job.ID)
	if got.Exec != "echo hi" || got.Prompt != "" || got.Model != "" || got.ReasoningEffort != "" {
		t.Fatalf("converted job = %+v", got)
	}
	if err := scheduler.UpdateJob(job.ID, "reasoning_effort", "low"); err == nil {
		t.Fatal("shell effort edit accepted")
	}
	job = cronReasoningStorageJob("shell", "high")
	job.Prompt, job.Exec = "", "echo hi"
	if err := scheduler.AddJob(job); err == nil || !strings.Contains(err.Error(), "only supported for prompt jobs") {
		t.Fatalf("shell effort create error = %v", err)
	}
}

func TestCronReasoningProjectChangeIsAtomic(t *testing.T) {
	scheduler := newCronReasoningScheduler(t, &cronReasoningStorageAgent{supported: true, values: []string{"high", " bespoke "}})
	job := cronReasoningStorageJob("effort", " BESPOKE ")
	if err := scheduler.AddJob(job); err != nil {
		t.Fatalf("agent-provided value rejected: %v", err)
	}
	other := NewEngine("other", &cronReasoningStorageAgent{supported: true, values: []string{"low"}}, nil, "", LangEnglish)
	t.Cleanup(other.cancel)
	scheduler.RegisterEngine("other", other)
	if err := scheduler.UpdateJob(job.ID, "project", "other"); err == nil {
		t.Fatal("project with incompatible catalog accepted")
	}
	if got := scheduler.store.Snapshot(job.ID); got.Project != "p" || got.ReasoningEffort != "bespoke" {
		t.Fatalf("failed project change mutated job: %+v", got)
	}
	if err := scheduler.UpdateJobFields(job.ID, map[string]any{"project": "other", "reasoning_effort": " LOW "}); err != nil {
		t.Fatal(err)
	}
	if got := scheduler.store.Snapshot(job.ID); got.Project != "other" || got.ReasoningEffort != "low" {
		t.Fatalf("project/effort update = %+v", got)
	}
}

func TestCronReasoningLoadedJobsRevalidateBeforeEnableAndRun(t *testing.T) {
	for _, kind := range []string{"unsupported", "invalid", "shell"} {
		t.Run(kind, func(t *testing.T) {
			a := &cronReasoningStorageAgent{supported: kind != "unsupported", values: []string{"low"}}
			scheduler := newCronReasoningScheduler(t, a)
			job := cronReasoningStorageJob("loaded", "high")
			want := "invalid reasoning_effort"
			if kind == "unsupported" {
				want = ErrReasoningEffortOverrideUnsupported.Error()
			}
			if kind == "shell" {
				job.Prompt, job.Exec = "", "echo must-not-run"
				want = "only supported for prompt jobs"
			}
			if err := scheduler.store.Add(job); err != nil {
				t.Fatal(err)
			}
			scheduler.store.load()
			for _, enable := range []func() error{
				func() error { return scheduler.EnableJob(job.ID) },
				func() error { return scheduler.UpdateJob(job.ID, "enabled", true) },
				func() error { return scheduler.RunJobNow(job.ID) },
			} {
				err := enable()
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("loaded job validation error = %v, want %q", err, want)
				}
				if kind == "unsupported" && !errors.Is(err, ErrReasoningEffortOverrideUnsupported) {
					t.Fatalf("missing capability error identity: %v", err)
				}
				if scheduler.store.Snapshot(job.ID).Enabled || len(scheduler.entries) != 0 {
					t.Fatal("invalid job was enabled or scheduled")
				}
			}
			// Simulate an enabled job loaded at scheduler startup.
			scheduler.store.SetEnabled(job.ID, true)
			scheduler.executeJob(job.ID)
			if got := scheduler.store.Snapshot(job.ID); got.LastRun.IsZero() || !strings.Contains(got.LastError, want) {
				t.Fatalf("invalid scheduled run was not recorded: %+v", got)
			}
			if kind != "shell" {
				if err := scheduler.UpdateJob(job.ID, "enabled", false); err != nil {
					t.Fatalf("cannot disable stale job: %v", err)
				}
			}
			if err := scheduler.UpdateJob(job.ID, "reasoning_effort", ""); err != nil {
				t.Fatalf("cannot clear invalid override: %v", err)
			}
		})
	}
}
