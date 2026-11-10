package core

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestScheduledHistory(t *testing.T) *ScheduledHistory {
	t.Helper()
	h, err := NewScheduledHistory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func historyCount(t *testing.T, h *ScheduledHistory, query string) int {
	t.Helper()
	var count int
	if err := h.db.QueryRow(query).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestScheduledHistory_VersionsChangeOnlyWithDefinitions(t *testing.T) {
	h := newTestScheduledHistory(t)
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.SetHistory(h)
	job := cronModelJob("job", "model-A")
	if err := store.Add(job); err != nil {
		t.Fatal(err)
	}
	store.MarkRun("job", errors.New("first failure"))
	store.MarkRun("job", nil)
	if got := historyCount(t, h, "SELECT count(*) FROM job_versions"); got != 1 {
		t.Fatalf("runtime outcomes created %d config versions", got)
	}
	if !store.Update("job", "description", "updated definition") {
		t.Fatal("definition update failed")
	}
	if got := historyCount(t, h, "SELECT count(*) FROM job_versions"); got != 2 {
		t.Fatalf("versions=%d", got)
	}
	var snapshot string
	if err := h.db.QueryRow("SELECT snapshot_json FROM job_versions LIMIT 1").Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(snapshot, "last_run") || strings.Contains(snapshot, "last_error") {
		t.Fatalf("runtime state in definition: %s", snapshot)
	}
	if !store.Remove(job.ID) {
		t.Fatal("remove failed")
	}
	if got := historyCount(t, h, "SELECT count(*) FROM job_versions WHERE archived_at IS NOT NULL"); got != 2 {
		t.Fatalf("removed definition versions were not retained and archived: %d", got)
	}
}

func TestScheduledHistory_ConcurrentManualWorkersHaveSeparateRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	h := newTestScheduledHistory(t)
	store, _ := NewCronStore(t.TempDir())
	store.SetHistory(h)
	platform := &scheduledShellPlatform{stubPlatformEngine{n: "test"}}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	t.Cleanup(func() { _ = engine.Stop() })
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test", engine)
	job := &CronJob{ID: "worker", Project: "test", SessionKey: "test:scheduled", Exec: "test -n \"$CC_SCHEDULED_RUN_ID\"", Mute: true}
	if err := store.Add(job); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() { scheduler.runJob(cloneCronJob(job), true) })
	}
	wg.Wait()
	if got := historyCount(t, h, "SELECT count(DISTINCT run_id) FROM runs WHERE trigger='manual' AND scheduler_status='succeeded' AND execution_status='succeeded'"); got != 3 {
		t.Fatalf("successful independent runs=%d", got)
	}
	if len(platform.getSent()) != 0 {
		t.Fatal("history changed muted delivery")
	}
}

func TestScheduledHistory_FailureIsNotOverwrittenByLaterSuccess(t *testing.T) {
	h := newTestScheduledHistory(t)
	job := cronModelJob("same-job", "fixed-model")
	first := h.begin("cron", job, job.SessionKey, false)
	first.executionStarted()
	cause := errors.New("shell: exit code 7; app_secret=sk1234567890abcdef")
	first.executionFinished(cause)
	first.finish("failed", cause)
	second := h.begin("cron", job, job.SessionKey, false)
	second.executionStarted()
	second.executionFinished(nil)
	second.finish("succeeded", nil)
	if got := historyCount(t, h, "SELECT count(*) FROM runs WHERE scheduler_status='failed'"); got != 1 {
		t.Fatalf("failed records=%d", got)
	}
	var stored string
	if err := h.db.QueryRow("SELECT scheduler_error FROM runs WHERE run_id=?", first.id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "sk1234567890abcdef") || !strings.Contains(stored, "exit code 7") {
		t.Fatalf("incorrect diagnostic redaction: %s", stored)
	}
}

func TestScheduledHistory_TimeoutRetainsLateExecutionOutcome(t *testing.T) {
	h := newTestScheduledHistory(t)
	job := cronModelJob("slow", "fixed-model")
	run := h.begin("cron", job, job.SessionKey, false)
	run.executionStarted()
	run.finish("timed_out", errors.New("scheduler wait expired"))
	run.executionFinished(nil)
	var scheduler, execution string
	if err := h.db.QueryRow("SELECT scheduler_status,execution_status FROM runs WHERE run_id=?", run.id).Scan(&scheduler, &execution); err != nil {
		t.Fatal(err)
	}
	if scheduler != "timed_out" || execution != "succeeded" {
		t.Fatalf("wait=%s execution=%s", scheduler, execution)
	}
}

func TestScheduledHistory_RestartDoesNotInventExecutionCompletion(t *testing.T) {
	dir := t.TempDir()
	h, err := NewScheduledHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	run := h.begin("cron", cronModelJob("lost", "model"), "test:user", false)
	run.executionStarted()
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	resumed, err := NewScheduledHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	resumed.RecoverInterrupted()
	var scheduler, execution string
	var finished, observed sql.NullString
	if err := resumed.db.QueryRow(`SELECT scheduler_status,execution_status,execution_finished_at,interrupted_observed_at
 FROM runs WHERE run_id=?`, run.id).Scan(&scheduler, &execution, &finished, &observed); err != nil {
		t.Fatal(err)
	}
	if scheduler != "interrupted" || execution != "unknown" || finished.Valid || !observed.Valid {
		t.Fatalf("scheduler=%s execution=%s finished=%v observed=%v", scheduler, execution, finished, observed)
	}
	mode, err := os.Stat(filepath.Join(dir, "history.sqlite"))
	if err != nil || mode.Mode().Perm() != 0o600 {
		t.Fatalf("database permissions: %v %v", mode, err)
	}
}

func TestScheduledHistory_PreflightFailureStillHasItsOwnRecord(t *testing.T) {
	h := newTestScheduledHistory(t)
	store, _ := NewCronStore(t.TempDir())
	store.SetHistory(h)
	scheduler := NewCronScheduler(store)
	job := cronModelJob("missing-project", "model")
	scheduler.runJob(job, false)
	if got := historyCount(t, h, "SELECT count(*) FROM runs WHERE scheduler_status='failed' AND execution_status='not_started' AND scheduler_error IS NOT NULL"); got != 1 {
		t.Fatalf("failed preflight records=%d", got)
	}
}

func TestScheduledHistory_AgentSessionLinksAndVersionsSurviveReopen(t *testing.T) {
	h := newTestScheduledHistory(t)
	a := &cronModelAgent{model: "project-model"}
	engine, _ := newCronModelEngine(t, a, "")
	store, _ := NewCronStore(t.TempDir())
	store.SetHistory(h)
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test", engine)
	job := cronModelJob("isolated", "task-model")
	job.Mute, job.SessionMode = true, "new_per_run"
	scheduler.runJob(job, true)
	var cc, agent string
	if err := h.db.QueryRow("SELECT cc_session_id,agent_session_id FROM runs").Scan(&cc, &agent); err != nil {
		t.Fatal(err)
	}
	if cc == "" || agent == "" {
		t.Fatalf("missing execution linkage: cc=%q agent=%q", cc, agent)
	}
}

func TestScheduledHistory_QueuedTurnDoesNotReplaceScheduledSessionProof(t *testing.T) {
	h := newTestScheduledHistory(t)
	a := &cronModelAgent{model: "project-model"}
	engine, platform := newCronModelEngine(t, a, "")
	store, _ := NewCronStore(t.TempDir())
	store.SetHistory(h)
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test", engine)
	a.onSend = func(backend *cronModelSession, call cronModelCall) error {
		if call.prompt == "scheduled" {
			engine.ReceiveMessage(platform, answerProfileMessage("queued", "queued"))
		} else if call.prompt == "queued" {
			engine.sessions.GetOrCreateActive("test:user").SetAgentInfo("queued-agent", "test", "")
		}
		backend.events <- Event{Type: EventResult, Content: "completed", Done: true}
		return nil
	}
	job := cronModelJob("scheduled", "task-model")
	job.Mute = true
	scheduler.runJob(job, true)
	var agent string
	if err := h.db.QueryRow("SELECT agent_session_id FROM runs").Scan(&agent); err != nil {
		t.Fatal(err)
	}
	if agent == "" || agent == "queued-agent" {
		t.Fatalf("a queued turn replaced the scheduled execution identity: %q", agent)
	}
}

func TestScheduledHistory_TimerAndStorageFailurePreserveExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	h := newTestScheduledHistory(t)
	platform := &scheduledShellPlatform{stubPlatformEngine{n: "test"}}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	t.Cleanup(func() { _ = engine.Stop() })
	store, _ := NewTimerStore(t.TempDir())
	store.SetHistory(h)
	scheduler := NewTimerScheduler(store)
	scheduler.RegisterEngine("test", engine)
	job := &TimerJob{ID: "once", Project: "test", SessionKey: "test:scheduled", Exec: "true", Mute: true, ScheduledAt: time.Now()}
	if err := store.Add(job); err != nil {
		t.Fatal(err)
	}
	scheduler.executeJob(job.ID)
	if got := historyCount(t, h, "SELECT count(*) FROM runs WHERE kind='timer' AND scheduler_status='succeeded'"); got != 1 {
		t.Fatalf("successful timer records=%d", got)
	}
	_ = h.Close()
	second := &TimerJob{ID: "storage-unavailable", Project: "test", SessionKey: "test:scheduled", Exec: "true", Mute: true, ScheduledAt: time.Now()}
	if err := store.Add(second); err != nil {
		t.Fatal(err)
	}
	scheduler.executeJob(second.ID)
	if !second.Fired || second.LastError != "" || len(platform.getSent()) != 0 {
		t.Fatalf("history failure changed timer execution: %+v", second)
	}
}
