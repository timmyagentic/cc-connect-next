package core

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func cronReasoningJob(id, effort string) *CronJob {
	job := cronModelJob(id, "")
	job.ReasoningEffort = effort
	return job
}

func TestCronReasoningOverride_RestoresDefaults(t *testing.T) {
	for _, mode := range []string{"reuse", "new_per_run"} {
		for _, model := range []string{"", "task-model"} {
			t.Run(mode+"/"+model, func(t *testing.T) {
				a := &cronModelAgent{model: "project-model", effort: "medium"}
				e, p := newCronModelEngine(t, a, "")
				job := cronReasoningJob("reasoning-task", "high")
				job.Model, job.SessionMode = model, mode
				if err := e.ExecuteCronJob(job); err != nil {
					t.Fatal(err)
				}
				waitCronModelChat(t, e, p, a, "ordinary-after", 2)
				if err := e.ExecuteCronJob(cronModelJob("legacy-after", "")); err != nil {
					t.Fatal(err)
				}
				calls, _, setters := a.snapshot()
				wantModel := model
				if wantModel == "" {
					wantModel = "project-model"
				}
				if calls[0].options.ReasoningEffort != "high" || calls[0].model != wantModel {
					t.Fatalf("task options=%+v", calls[0])
				}
				for _, call := range calls[1:] {
					if call.model != "project-model" || call.explicit && call.options.ReasoningEffort != "medium" {
						t.Fatalf("task options leaked: %+v", calls)
					}
				}
				if setters != 0 || a.GetReasoningEffort() != "medium" {
					t.Fatal("task mutated persistent defaults")
				}
			})
		}
	}
}

func TestCronReasoningOverride_ConcurrentTasksRemainIsolated(t *testing.T) {
	started, release := make(chan string, 2), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-model", effort: "medium"}
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.options.ReasoningEffort == "low" || call.options.ReasoningEffort == "high" {
			started <- call.options.ReasoningEffort
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "ok", Done: true}
		return nil
	}
	e, p := newCronModelEngine(t, a, "")
	waitCronModelChat(t, e, p, a, "before", 1)
	activeID := e.sessions.GetOrCreateActive("test:user").ID
	done := make(chan error, 2)
	for _, effort := range []string{"low", "high"} {
		job := cronReasoningJob(effort, effort)
		job.SessionMode = "new_per_run"
		go func() { done <- e.ExecuteCronJob(job) }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("tasks did not start concurrently")
		}
	}
	waitCronModelChat(t, e, p, a, "during", 4)
	if s := e.sessions.GetOrCreateActive("test:user"); s.ID != activeID || s.TurnDefaultsRequired() {
		t.Fatal("side task changed ordinary session")
	}
	once.Do(func() { close(release) })
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	calls, _, setters := a.snapshot()
	counts := map[string]int{}
	for _, call := range calls {
		if call.explicit {
			counts[call.options.ReasoningEffort]++
		}
	}
	if counts["low"] != 1 || counts["high"] != 1 || setters != 0 {
		t.Fatalf("cross-task settings: %+v setters=%d", calls, setters)
	}
}

func TestCronReasoningOverride_QueuesOrdinaryWithoutSteering(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-model", effort: "medium"}
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.options.ReasoningEffort == "high" {
			close(started)
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "ok", Done: true}
		return nil
	}
	e, p := newCronModelEngine(t, a, "")
	e.SetBusyMessageMode(BusyMessageModeSteer)
	done := make(chan error, 1)
	go func() { done <- e.ExecuteCronJob(cronReasoningJob("task", "high")) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("task did not start")
	}
	e.ReceiveMessage(p, answerProfileMessage("ordinary", "ordinary"))
	waitAnswerProfileTest(t, "ordinary input queued", func() bool { return cronModelQueueDepth(e) == 1 })
	once.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	calls, sessions, _ := a.snapshot()
	if len(calls) != 2 || calls[1].options.ReasoningEffort != "medium" {
		t.Fatalf("queued input inherited task effort: %+v", calls)
	}
	sessions[0].mu.Lock()
	steers := sessions[0].steerCalls
	sessions[0].mu.Unlock()
	if steers != 0 {
		t.Fatalf("ordinary input steered into task: %d", steers)
	}
}

func TestCronReasoningOverride_QueuedEffortIsPreserved(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-model", effort: "medium"}
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.prompt == "first" {
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "ok", Done: true}
		return nil
	}
	e, p := newCronModelEngine(t, a, "")
	e.SetBusyMessageMode(BusyMessageModeSteer)
	e.ReceiveMessage(p, answerProfileMessage("first", "first"))
	waitAnswerProfileTest(t, "first turn", func() bool { calls, _, _ := a.snapshot(); return len(calls) == 1 })
	msg := answerProfileMessage("queued", "queued")
	msg.ReasoningEffortOverride = "high"
	e.ReceiveMessage(p, msg)
	waitAnswerProfileTest(t, "effort queued", func() bool { return cronModelQueueDepth(e) == 1 })
	once.Do(func() { close(release) })
	waitAnswerProfileTest(t, "queue drained", func() bool {
		calls, _, _ := a.snapshot()
		return len(calls) == 2 && !e.sessions.GetOrCreateActive("test:user").Busy()
	})
	waitCronModelChat(t, e, p, a, "after", 3)
	calls, _, _ := a.snapshot()
	if calls[1].options.ReasoningEffort != "high" || calls[2].options.ReasoningEffort != "medium" {
		t.Fatalf("queue options lost or leaked: %+v", calls)
	}
}

func TestCronReasoningOverride_RestorationSurvivesRestart(t *testing.T) {
	for _, effort := range []string{"", "medium"} {
		t.Run("native-default/"+effort, func(t *testing.T) {
			a := &cronModelAgent{nativeModel: "native-model", nativeEffort: effort}
			e, _ := newCronModelEngine(t, a, "")
			if err := e.ExecuteCronJob(cronReasoningJob("task", "high")); err != nil {
				t.Fatal(err)
			}
			path := e.sessions.StorePath()
			_ = e.Stop()
			a = &cronModelAgent{nativeModel: "native-model", nativeEffort: effort}
			e, p := newCronModelEngine(t, a, path)
			waitCronModelChat(t, e, p, a, "after-restart", 1)
			calls, _, _ := a.snapshot()
			if !calls[0].explicit || calls[0].options.ReasoningEffort != effort {
				t.Fatalf("restart retained task effort: %+v", calls)
			}
		})
	}
}

func TestCronReasoningOverride_MutedFailureReachesScheduler(t *testing.T) {
	a := &cronModelAgent{model: "project-model", effort: "medium"}
	a.onSend = func(_ *cronModelSession, _ cronModelCall) error { return errors.New("effort rejected by upstream") }
	e, p := newCronModelEngine(t, a, "")
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test", e)
	job := cronReasoningJob("failure", "high")
	job.Mute = true
	if err := store.Add(job); err != nil {
		t.Fatal(err)
	}
	scheduler.runJob(job, true)
	if stored := store.Get(job.ID); stored.LastRun.IsZero() || !strings.Contains(stored.LastError, "effort rejected by upstream") {
		t.Fatalf("failed task recorded as success: %+v", stored)
	}
	if len(p.getSent()) != 0 {
		t.Fatal("muted task emitted messages")
	}
}

func TestCronReasoningOverride_UnsupportedSessionDoesNotSend(t *testing.T) {
	e, _ := newCronModelEngine(t, &stubAgent{}, "")
	err := e.ExecuteCronJob(cronReasoningJob("unsupported", "high"))
	if !errors.Is(err, ErrReasoningEffortOverrideUnsupported) {
		t.Fatalf("unsupported session error=%v", err)
	}
}
