package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func receiveScheduledProgress(t *testing.T, events <-chan HookEvent) HookEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("scheduled progress was not delivered")
		return HookEvent{}
	}
}

func progressHook(t *testing.T, handle func(HookEvent)) *HookManager {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event HookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("decode progress: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		handle(event)
	}))
	t.Cleanup(server.Close)
	return NewHookManager("test", []HookConfig{{Event: "scheduled.progress", Type: "http", URL: server.URL, Async: boolPtr(false)}}, "sh", "-c", "")
}

func TestScheduledProgress_SlowHookCoalescesAndRetainsTerminal(t *testing.T) {
	events := make(chan HookEvent, 10)
	release := make(chan struct{})
	var once sync.Once
	var requests atomic.Int32
	hooks := progressHook(t, func(event HookEvent) {
		first := requests.Add(1) == 1
		events <- event
		if first {
			<-release
		}
	})
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	p := newScheduledProgress(hooks, scheduledJobSpec{kind: "cron", id: "job", sessionKey: "test:owner", model: "model", reasoningEffort: "medium"})
	first := receiveScheduledProgress(t, events)
	for range 10000 {
		p.observe(Event{Type: EventToolUse, ToolName: "SECRET_TOOL", ToolInput: "SECRET_ARGUMENT"})
	}
	p.observe(Event{Type: EventThinking, Content: "SECRET_THINKING"})
	p.observe(Event{Type: EventThinking, ThinkingKind: ThinkingText, Content: "SECRET_COMMENTARY"})
	p.observe(Event{Type: EventThinking, ThinkingKind: ThinkingItem})
	p.observe(Event{Type: EventThinking, ThinkingKind: ThinkingItem})
	p.observe(Event{Type: EventText, Content: "SECRET_RESULT"})
	p.finish(nil)
	p.finish(errors.New("late failure"))
	p.observe(Event{Type: EventToolUse})
	if requests.Load() != 1 {
		t.Fatal("slow consumer spawned extra hook calls")
	}
	once.Do(func() { close(release) })
	last := receiveScheduledProgress(t, events)
	select {
	case <-p.done:
	case <-time.After(time.Second):
		t.Fatal("observer did not terminate")
	}
	if requests.Load() != 2 || last.Extra["tool_count"] != float64(10000) || last.Extra["thinking_count"] != float64(3) || last.Extra["status"] != "succeeded" || last.Extra["phase"] != "finished" {
		t.Fatalf("wrong terminal snapshot: %#v (%d calls)", last, requests.Load())
	}
	if first.Extra["run_id"] == "" || first.Extra["run_id"] != last.Extra["run_id"] || last.Extra["sequence"].(float64) <= first.Extra["sequence"].(float64) || last.SessionKey != "test:owner" || last.Project != "test" {
		t.Fatalf("unstable progress identity: %#v -> %#v", first, last)
	}
	body, _ := json.Marshal(last)
	if strings.Contains(string(body), "SECRET_") || last.Content != "" || last.Error != "" {
		t.Fatalf("progress leaked execution content: %s", body)
	}
}

func TestScheduledProgress_RateLimitKeepsLatestAndTerminalBypassesDelay(t *testing.T) {
	events := make(chan HookEvent, 10)
	hooks := progressHook(t, func(event HookEvent) { events <- event })
	p := newScheduledProgress(hooks, scheduledJobSpec{kind: "timer", id: "job"})
	receiveScheduledProgress(t, events)
	for range 200 {
		p.observe(Event{Type: EventToolUse})
	}
	select {
	case event := <-events:
		t.Fatalf("intermediate snapshot bypassed rate limit: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
	update := receiveScheduledProgress(t, events)
	if update.Extra["tool_count"] != float64(200) || update.Extra["phase"] != "tool" {
		t.Fatalf("did not retain latest intermediate snapshot: %#v", update)
	}
	p.finish(context.Canceled)
	select {
	case final := <-events:
		if final.Extra["status"] != "interrupted" {
			t.Fatalf("cancellation reported as success: %#v", final)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("terminal event waited for progress interval")
	}
	<-p.done
}

func TestScheduledProgress_ExplicitOptInAndHistoryIndependentIdentity(t *testing.T) {
	for _, pattern := range []string{"*", "cron.triggered", ""} {
		hooks := NewHookManager("test", []HookConfig{{Event: pattern, Type: "command", Command: "unused"}}, "sh", "-c", "")
		if p := newScheduledProgress(hooks, scheduledJobSpec{}); p != nil {
			p.finish(nil)
			t.Fatalf("pattern %q enabled progress", pattern)
		}
	}
	if p := newScheduledProgress(nil, scheduledJobSpec{}); p != nil {
		t.Fatal("nil hooks enabled observer")
	}
	events := make(chan HookEvent, 10)
	hooks := progressHook(t, func(event HookEvent) { events <- event })
	history := newTestScheduledHistory(t)
	run := history.begin("cron", cronModelJob("job", ""), "test:user", true)
	if err := history.db.Close(); err != nil {
		t.Fatal(err)
	}
	failedHistoryRun := history.begin("cron", cronModelJob("job", ""), "test:user", true)
	seen := map[string]bool{}
	for _, r := range []*scheduledRun{run, nil, failedHistoryRun} {
		p := newScheduledProgress(hooks, scheduledJobSpec{kind: "cron", id: "job", historyRun: r})
		if p == nil {
			t.Fatal("missing progress without history")
		}
		id := p.current.runID
		if id == "" || seen[id] || r != nil && id != r.runID() {
			t.Fatalf("invalid run identity %q", id)
		}
		seen[id] = true
		p.finish(nil)
		<-p.done
	}
}

func TestScheduledProgress_ConcurrentExecutionsDoNotWaitForSlowHook(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	events := make(chan HookEvent, 20)
	hooks := progressHook(t, func(event HookEvent) {
		events <- event
		if event.Extra["job_id"] == "slow" && event.Extra["phase"] == "started" {
			<-release
		}
	})
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-A"}
	slowStarted := make(chan struct{})
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.prompt == "slow" {
			<-slowStarted
		}
		s.events <- Event{Type: EventThinking, Content: "private"}
		s.events <- Event{Type: EventToolUse, ToolName: "private"}
		s.events <- Event{Type: EventResult, Content: "complete", Done: true}
		return nil
	}
	e, _ := newCronModelEngine(t, a, "")
	e.SetHooks(hooks)
	done := make(chan error, 3)
	slow := cronModelJob("slow", "model")
	slow.SessionMode, slow.Mute = "new_per_run", true
	go func() { done <- e.ExecuteCronJob(slow) }()
	if event := receiveScheduledProgress(t, events); event.Extra["job_id"] != "slow" {
		t.Fatalf("unexpected start: %#v", event)
	}
	close(slowStarted)
	for _, id := range []string{"fast-1", "fast-2"} {
		job := cronModelJob(id, "model")
		job.SessionMode, job.Mute = "new_per_run", true
		go func() { done <- e.ExecuteCronJob(job) }()
	}
	for range 3 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("execution waited for slow Hook")
		}
	}
	finished := map[string]string{}
	for len(finished) != 2 {
		event := receiveScheduledProgress(t, events)
		if event.Extra["phase"] == "finished" {
			id := event.Extra["job_id"].(string)
			finished[id] = event.Extra["run_id"].(string)
			if event.Extra["tool_count"] != float64(1) || event.Extra["thinking_count"] != float64(1) {
				t.Fatalf("counts crossed executions: %#v", event)
			}
		}
	}
	if finished["fast-1"] == "" || finished["fast-1"] == finished["fast-2"] {
		t.Fatalf("run identity reused: %#v", finished)
	}
	once.Do(func() { close(release) })
	if event := receiveScheduledProgress(t, events); event.Extra["job_id"] != "slow" || event.Extra["phase"] != "finished" {
		t.Fatalf("slow terminal lost: %#v", event)
	}
}

func TestScheduledProgress_DetachesBeforeReusedOrdinaryTurn(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-A"}
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.model == "task-B" {
			close(started)
			<-release
		}
		s.events <- Event{Type: EventToolUse, ToolName: "private"}
		s.events <- Event{Type: EventResult, Content: call.prompt + " complete", Done: true}
		return nil
	}
	e, platform := newCronModelEngine(t, a, "")
	events := make(chan HookEvent, 20)
	hooks := progressHook(t, func(event HookEvent) {
		if event.Extra["phase"] == "finished" {
			history := e.sessions.GetOrCreateActive("test:user").GetHistory(0)
			found := false
			for _, entry := range history {
				found = found || entry.Role == "assistant" && entry.Content == "task complete"
			}
			if !found {
				t.Error("terminal event preceded result commit")
			}
		}
		events <- event
	})
	e.SetHooks(hooks)
	done := make(chan error, 1)
	go func() { done <- e.ExecuteCronJob(cronModelJob("task", "task-B")) }()
	<-started
	e.ReceiveMessage(platform, answerProfileMessage("ordinary", "ordinary"))
	waitAnswerProfileTest(t, "ordinary message queued", func() bool { return cronModelQueueDepth(e) == 1 })
	once.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for {
		event := receiveScheduledProgress(t, events)
		if event.Extra["phase"] == "finished" {
			if event.Extra["tool_count"] != float64(1) {
				t.Fatalf("ordinary tools leaked into scheduled progress: %#v", event)
			}
			break
		}
	}
	calls, _, _ := a.snapshot()
	if len(calls) != 2 || calls[1].prompt != "ordinary" {
		t.Fatalf("ordinary queued turn changed: %#v", calls)
	}
}

func TestScheduledProgress_FailureHasNoRawErrorAndIsDiscoverable(t *testing.T) {
	events := make(chan HookEvent, 10)
	e, _ := newCronModelEngine(t, &cronModelAgent{startErr: errors.New("SECRET_START_ERROR")}, "")
	hooks := progressHook(t, func(event HookEvent) { events <- event })
	for _, pattern := range []string{"*", "scheduled.progress"} {
		e.SetHooks(NewHookManager("test", []HookConfig{{Event: pattern, Type: "command", Command: "SECRET_COMMAND"}}, "sh", "-c", ""))
		capability := findRuntimeFeature(t, e.agentRuntimeCapabilities().Capabilities, "scheduled_progress_hook")
		want := CapabilityUnavailable
		if pattern == "scheduled.progress" {
			want = CapabilityConditional
		}
		if capability.Availability.State != want || capability.ReadOnly == nil || *capability.ReadOnly || capability.Fallback.Mode != "skip" {
			t.Fatalf("incorrect capability: %#v", capability)
		}
		body, _ := json.Marshal(e.agentRuntimeCapabilities())
		if strings.Contains(string(body), "SECRET_COMMAND") {
			t.Fatal("manifest exposed hook command")
		}
	}
	e.SetHooks(hooks)
	if err := e.ExecuteCronJob(cronModelJob("failed", "")); err == nil {
		t.Fatal("start failure was swallowed")
	}
	for {
		event := receiveScheduledProgress(t, events)
		if event.Extra["phase"] == "finished" {
			if event.Extra["status"] != "failed" || event.Error != "" {
				t.Fatalf("incorrect failure: %#v", event)
			}
			break
		}
	}
}

func TestScheduledProgress_ExecutionDeadlineDoesNotReportSuccess(t *testing.T) {
	events := make(chan HookEvent, 10)
	a := &cronModelAgent{onSend: func(*cronModelSession, cronModelCall) error { return nil }}
	e, _ := newCronModelEngine(t, a, "")
	e.eventIdleTimeout = 30 * time.Millisecond
	e.SetHooks(progressHook(t, func(event HookEvent) { events <- event }))
	job := cronModelJob("deadline", "")
	job.Mute = true
	if err := e.ExecuteCronJob(job); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected execution deadline, got %v", err)
	}
	for {
		event := receiveScheduledProgress(t, events)
		if event.Extra["phase"] == "finished" {
			if event.Extra["status"] != "interrupted" {
				t.Fatalf("execution deadline reported as success: %#v", event)
			}
			break
		}
	}
}

func TestScheduledProgress_HookFailureDoesNotChangeExecutionResult(t *testing.T) {
	called := make(chan bool, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event HookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Error(err)
		}
		called <- event.Extra["phase"] == "finished"
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	e, _ := newCronModelEngine(t, &cronModelAgent{}, "")
	e.SetHooks(NewHookManager("test", []HookConfig{{Event: "scheduled.progress", Type: "http", URL: server.URL, Async: boolPtr(false)}}, "sh", "-c", ""))
	if err := e.ExecuteCronJob(cronModelJob("success", "")); err != nil {
		t.Fatalf("Hook failure changed execution result: %v", err)
	}
	for {
		select {
		case terminal := <-called:
			if terminal {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("failing terminal Hook was not exercised")
		}
	}
}
