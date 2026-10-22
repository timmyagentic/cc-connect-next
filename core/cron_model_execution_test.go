package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type cronModelCall struct {
	prompt   string
	model    string
	explicit bool
	options  TurnOptions
}

type cronModelAgent struct {
	mu          sync.Mutex
	model       string
	nativeModel string
	resumeModel string
	setCalls    int
	calls       []cronModelCall
	sessions    []*cronModelSession
	startErr    error
	defaultsErr error
	onSend      func(*cronModelSession, cronModelCall) error
}

func (a *cronModelAgent) Name() string                                  { return "cron-model-test" }
func (a *cronModelAgent) Stop() error                                   { return nil }
func (a *cronModelAgent) SupportsPerTurnModel() bool                    { return true }
func (a *cronModelAgent) AvailableModels(context.Context) []ModelOption { return nil }
func (a *cronModelAgent) ListSessions(context.Context) ([]AgentSessionInfo, error) {
	return nil, nil
}
func (a *cronModelAgent) GetModel() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.model
}
func (a *cronModelAgent) SetModel(model string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.model = model
	a.setCalls++
}
func (a *cronModelAgent) StartSession(_ context.Context, id string) (AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.startErr != nil {
		return nil, a.startErr
	}
	legacyModel := a.model
	if legacyModel == "" {
		legacyModel = a.nativeModel
	}
	if id != "" && a.resumeModel != "" {
		legacyModel = a.resumeModel
	}
	s := &cronModelSession{agent: a, events: make(chan Event, 16), legacyModel: legacyModel, id: id}
	if s.id == "" {
		s.id = fmt.Sprintf("cron-model-session-%d", len(a.sessions)+1)
	}
	a.sessions = append(a.sessions, s)
	return s, nil
}
func (a *cronModelAgent) snapshot() ([]cronModelCall, []*cronModelSession, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]cronModelCall(nil), a.calls...), append([]*cronModelSession(nil), a.sessions...), a.setCalls
}

type cronModelSession struct {
	agent       *cronModelAgent
	events      chan Event
	legacyModel string
	id          string
	mu          sync.Mutex
	closed      bool
	steerCalls  int
}

func (s *cronModelSession) Send(prompt string, _ []ImageAttachment, _ []FileAttachment) error {
	return s.send(cronModelCall{prompt: prompt, model: s.legacyModel})
}
func (s *cronModelSession) SendWithTurnOptions(prompt string, _ []ImageAttachment, _ []FileAttachment, options TurnOptions) error {
	model := options.Model
	if model == "" {
		model = s.agent.nativeModel
	}
	return s.send(cronModelCall{prompt: prompt, model: model, explicit: true, options: options})
}
func (s *cronModelSession) send(call cronModelCall) error {
	s.agent.mu.Lock()
	s.agent.calls = append(s.agent.calls, call)
	onSend := s.agent.onSend
	s.agent.mu.Unlock()
	if onSend != nil {
		return onSend(s, call)
	}
	s.events <- Event{Type: EventResult, Content: "model=" + call.model + " " + call.prompt, Done: true}
	return nil
}
func (s *cronModelSession) DefaultTurnOptions() TurnOptions {
	return TurnOptions{Model: s.agent.nativeModel}
}
func (s *cronModelSession) ValidateTurnDefaults() error {
	s.agent.mu.Lock()
	defer s.agent.mu.Unlock()
	return s.agent.defaultsErr
}
func (s *cronModelSession) RespondPermission(string, PermissionResult) error { return nil }
func (s *cronModelSession) Events() <-chan Event                             { return s.events }
func (s *cronModelSession) CurrentSessionID() string                         { return s.id }
func (s *cronModelSession) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed
}
func (s *cronModelSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
func (s *cronModelSession) Steer(string, []ImageAttachment, []FileAttachment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steerCalls++
	return nil
}

type cronModelPlatform struct{ stubPlatformEngine }

func (p *cronModelPlatform) ReconstructReplyCtx(key string) (any, error) { return key, nil }

func newCronModelEngine(t *testing.T, a Agent, storePath string) (*Engine, *cronModelPlatform) {
	t.Helper()
	if storePath == "" {
		storePath = filepath.Join(t.TempDir(), "sessions.json")
	}
	p := &cronModelPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
	e := NewEngine("test", a, []Platform{p}, storePath, LangEnglish)
	e.SetDataDir(filepath.Dir(storePath))
	t.Cleanup(func() { _ = e.Stop() })
	return e, p
}

func cronModelJob(id, model string) *CronJob {
	return &CronJob{ID: id, Project: "test", SessionKey: "test:user", CronExpr: "0 6 * * *", Prompt: id, Model: model, Enabled: true}
}

func waitCronModelChat(t *testing.T, e *Engine, p *cronModelPlatform, a *cronModelAgent, id string, count int) {
	t.Helper()
	e.ReceiveMessage(p, answerProfileMessage(id, id))
	waitAnswerProfileTest(t, id, func() bool {
		calls, _, _ := a.snapshot()
		return len(calls) >= count && !e.sessions.GetOrCreateActive("test:user").Busy()
	})
}

func cronModelQueueDepth(e *Engine) int {
	e.interactiveMu.Lock()
	state := e.interactiveStates["test:user"]
	e.interactiveMu.Unlock()
	if state == nil {
		return 0
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return len(state.pendingMessages)
}

func TestCronModelOverride_ReuseRestoresDefaultsAndPreservesLegacyJobs(t *testing.T) {
	a := &cronModelAgent{model: "project-A"}
	e, p := newCronModelEngine(t, a, "")
	var saved int
	e.SetModelSaveFunc(func(string) error { saved++; return nil })
	if err := e.ExecuteCronJob(cronModelJob("legacy-before", "")); err != nil {
		t.Fatal(err)
	}
	if e.sessions.GetOrCreateActive("test:user").TurnDefaultsRequired() {
		t.Fatal("legacy job changed the model recovery marker")
	}
	if err := e.ExecuteCronJob(cronModelJob("pinned", "task-B")); err != nil {
		t.Fatal(err)
	}
	waitCronModelChat(t, e, p, a, "ordinary-after", 3)
	if err := e.ExecuteCronJob(cronModelJob("legacy-after", "")); err != nil {
		t.Fatal(err)
	}
	calls, sessions, setCalls := a.snapshot()
	if len(sessions) != 1 || len(calls) != 4 {
		t.Fatalf("sessions=%d calls=%+v", len(sessions), calls)
	}
	for i, want := range []string{"project-A", "task-B", "project-A", "project-A"} {
		if calls[i].model != want || calls[i].explicit != (i > 0) {
			t.Fatalf("call[%d]=%+v, want model %q explicit=%v", i, calls[i], want, i > 0)
		}
	}
	if setCalls != 0 || saved != 0 || a.GetModel() != "project-A" {
		t.Fatalf("Project defaults mutated: setters=%d saves=%d model=%q", setCalls, saved, a.GetModel())
	}
}

func TestCronModelOverride_NewPerRunConcurrentIsolation(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-A"}
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.model != "project-A" {
			started <- call.model
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "model=" + call.model, Done: true}
		return nil
	}
	e, p := newCronModelEngine(t, a, "")
	waitCronModelChat(t, e, p, a, "before", 1)
	activeID := e.sessions.GetOrCreateActive("test:user").ID
	done := make(chan error, 2)
	for _, model := range []string{"task-B", "task-C"} {
		job := cronModelJob(model, model)
		job.SessionMode = "new_per_run"
		go func() { done <- e.ExecuteCronJob(job) }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("isolated tasks did not start concurrently")
		}
	}
	waitCronModelChat(t, e, p, a, "while-tasks-run", 4)
	if s := e.sessions.GetOrCreateActive("test:user"); s.ID != activeID || s.TurnDefaultsRequired() {
		t.Fatal("side task replaced or modified the active ordinary session")
	}
	once.Do(func() { close(release) })
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	calls, sessions, setCalls := a.snapshot()
	if len(sessions) != 3 || setCalls != 0 {
		t.Fatalf("sessions=%d model setters=%d", len(sessions), setCalls)
	}
	counts := map[string]int{}
	for _, call := range calls {
		counts[call.model]++
	}
	if counts["project-A"] != 2 || counts["task-B"] != 1 || counts["task-C"] != 1 {
		t.Fatalf("models crossed tasks: %+v", calls)
	}
}

func TestCronModelOverride_ReuseQueuesOrdinaryWithoutSteering(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-A"}
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.model == "task-B" {
			close(started)
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "model=" + call.model, Done: true}
		return nil
	}
	e, p := newCronModelEngine(t, a, "")
	e.SetBusyMessageMode(BusyMessageModeSteer)
	done := make(chan error, 1)
	go func() { done <- e.ExecuteCronJob(cronModelJob("pinned", "task-B")) }()
	<-started
	e.ReceiveMessage(p, answerProfileMessage("ordinary", "ordinary"))
	waitAnswerProfileTest(t, "ordinary input queued", func() bool {
		return cronModelQueueDepth(e) == 1
	})
	once.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	calls, sessions, _ := a.snapshot()
	if len(calls) != 2 || calls[0].model != "task-B" || calls[1].model != "project-A" || !calls[1].explicit {
		t.Fatalf("queued follow-up inherited task model: %+v", calls)
	}
	sessions[0].mu.Lock()
	steers := sessions[0].steerCalls
	sessions[0].mu.Unlock()
	if steers != 0 {
		t.Fatalf("ordinary input steered into task model turn: %d", steers)
	}
}

func TestCronModelOverride_QueuedModelIsPreserved(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-A"}
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.prompt == "first" {
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "model=" + call.model, Done: true}
		return nil
	}
	e, p := newCronModelEngine(t, a, "")
	e.SetBusyMessageMode(BusyMessageModeQueue)
	e.ReceiveMessage(p, answerProfileMessage("first", "first"))
	waitAnswerProfileTest(t, "first turn starts", func() bool { calls, _, _ := a.snapshot(); return len(calls) == 1 })
	pinned := answerProfileMessage("queued-pinned", "queued-pinned")
	pinned.ModelOverride = "task-B"
	e.ReceiveMessage(p, pinned)
	e.ReceiveMessage(p, answerProfileMessage("after", "after"))
	waitAnswerProfileTest(t, "both messages queued", func() bool { return cronModelQueueDepth(e) == 2 })
	once.Do(func() { close(release) })
	waitAnswerProfileTest(t, "queue drains", func() bool {
		calls, _, _ := a.snapshot()
		return len(calls) == 3 && !e.sessions.GetOrCreateActive("test:user").Busy()
	})
	calls, _, _ := a.snapshot()
	for i, want := range []string{"project-A", "task-B", "project-A"} {
		if calls[i].model != want {
			t.Fatalf("call[%d]=%+v want %q", i, calls[i], want)
		}
	}
}

func TestCronModelOverride_QueuedFailureDoesNotReplaceJobSuccess(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a := &cronModelAgent{model: "project-A"}
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.model != "task-B" {
			return errors.New("ordinary follow-up failed")
		}
		close(started)
		<-release
		s.events <- Event{Type: EventResult, Content: "task succeeded", Done: true}
		return nil
	}
	e, p := newCronModelEngine(t, a, "")
	done := make(chan error, 1)
	go func() { done <- e.ExecuteCronJob(cronModelJob("pinned", "task-B")) }()
	<-started
	e.ReceiveMessage(p, answerProfileMessage("ordinary-failure", "ordinary-failure"))
	waitAnswerProfileTest(t, "ordinary follow-up queued", func() bool { return cronModelQueueDepth(e) == 1 })
	once.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatalf("ordinary follow-up failure replaced the completed scheduled result: %v", err)
	}
	calls, _, _ := a.snapshot()
	if len(calls) != 2 || calls[1].model != "project-A" {
		t.Fatalf("follow-up did not use Project default: %+v", calls)
	}
}

func TestCronModelOverride_UnsupportedAgentKeepsUnpinnedJobs(t *testing.T) {
	e, _ := newCronModelEngine(t, &resultAgent{session: newResultAgentSession("legacy success")}, "")
	if err := e.ExecuteCronJob(cronModelJob("legacy", "")); err != nil {
		t.Fatalf("unpinned task on a legacy adapter changed behavior: %v", err)
	}
}

func TestCronModelOverride_DefaultRestorationSurvivesRestart(t *testing.T) {
	for _, restart := range []string{"idle", "daemon"} {
		t.Run(restart, func(t *testing.T) {
			a := &cronModelAgent{nativeModel: "native-A", resumeModel: "task-B"}
			e, p := newCronModelEngine(t, a, "")
			if err := e.ExecuteCronJob(cronModelJob("pinned", "task-B")); err != nil {
				t.Fatal(err)
			}
			storePath := e.sessions.StorePath()
			count := 2
			if restart == "daemon" {
				_ = e.Stop()
				a = &cronModelAgent{nativeModel: "native-A", resumeModel: "task-B"}
				e, p = newCronModelEngine(t, a, storePath)
				count = 1
			} else {
				e.cleanupInteractiveState("test:user")
			}
			waitCronModelChat(t, e, p, a, "ordinary-after-restart", count)
			calls, _, _ := a.snapshot()
			last := calls[len(calls)-1]
			if !last.explicit || last.options.Model != "native-A" || last.model != "native-A" {
				t.Fatalf("resumed session retained the previous task model: %+v", last)
			}
		})
	}
}

func TestCronModelOverride_MutedFailuresReachScheduler(t *testing.T) {
	for _, failure := range []string{"unsupported", "start", "send", "backend", "result", "closed", "idle", "cancel", "late-send"} {
		t.Run(failure, func(t *testing.T) {
			a := &cronModelAgent{model: "project-A"}
			var engine *Engine
			want := "model unavailable"
			a.onSend = func(s *cronModelSession, call cronModelCall) error {
				switch failure {
				case "send":
					return errors.New(want)
				case "backend":
					s.events <- Event{Type: EventError, Error: errors.New(want)}
				case "result":
					s.events <- Event{Type: EventResult, Content: "failed", Error: errors.New(want), Done: true}
				case "closed":
					close(s.events)
				case "cancel":
					engine.cancel()
				case "late-send":
					s.events <- Event{Type: EventResult, Content: "premature success", Done: true}
					time.Sleep(30 * time.Millisecond)
					return errors.New(want)
				}
				return nil
			}
			var agent Agent = a
			switch failure {
			case "unsupported":
				agent = &stubAgent{}
				want = "does not support per-turn model"
			case "start":
				a.startErr = errors.New(want)
			case "closed":
				want = "event channel closed"
			case "idle":
				want = "idle timeout"
			case "cancel":
				want = "context canceled"
			}
			var p *cronModelPlatform
			engine, p = newCronModelEngine(t, agent, "")
			if failure == "idle" {
				engine.SetEventIdleTimeout(20 * time.Millisecond)
			}
			store, err := NewCronStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			scheduler := NewCronScheduler(store)
			scheduler.RegisterEngine("test", engine)
			job := cronModelJob("failure-"+failure, "task-B")
			job.Mute = true
			if err := store.Add(job); err != nil {
				t.Fatal(err)
			}
			scheduler.runJob(job, true)
			stored := store.Get(job.ID)
			if stored.LastRun.IsZero() || !strings.Contains(stored.LastError, want) {
				t.Fatalf("muted failure recorded as success: %+v, want error %q", stored, want)
			}
			if got := p.getSent(); len(got) != 0 {
				t.Fatalf("muted task emitted messages: %q", got)
			}
			calls, _, setCalls := a.snapshot()
			for _, call := range calls {
				if !call.explicit || call.model != "task-B" {
					t.Fatalf("failed model fell back to default: %+v", calls)
				}
			}
			if setCalls != 0 {
				t.Fatalf("failed task changed the Project model %d times", setCalls)
			}
		})
	}
}

func TestCronModelOverride_RecoveryMarkerSavedBeforeSend(t *testing.T) {
	a := &cronModelAgent{model: "project-A"}
	e, _ := newCronModelEngine(t, a, "")
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		reloaded := NewSessionManager(e.sessions.StorePath())
		if !reloaded.GetOrCreateActive("test:user").TurnDefaultsRequired() {
			return errors.New("model sent before recovery marker was persisted")
		}
		s.events <- Event{Type: EventResult, Content: "ok", Done: true}
		return nil
	}
	if err := e.ExecuteCronJob(cronModelJob("pinned", "task-B")); err != nil {
		t.Fatal(err)
	}
}

func TestCronModelOverride_UnresolvedDefaultsDoNotMarkOrBlockOrdinaryChat(t *testing.T) {
	a := &cronModelAgent{nativeModel: "legacy-A", defaultsErr: errors.New("native default model cannot be resolved")}
	e, p := newCronModelEngine(t, a, "")
	if err := e.ExecuteCronJob(cronModelJob("pinned", "task-B")); err == nil || !strings.Contains(err.Error(), "cannot be resolved") {
		t.Fatalf("task without restorable defaults error=%v", err)
	}
	if e.sessions.GetOrCreateActive("test:user").TurnDefaultsRequired() {
		t.Fatal("failed defaults preflight marked the ordinary session")
	}
	calls, _, _ := a.snapshot()
	if len(calls) != 0 {
		t.Fatalf("task sent without restorable defaults: %+v", calls)
	}
	waitCronModelChat(t, e, p, a, "ordinary-still-works", 1)
	calls, _, _ = a.snapshot()
	if calls[0].explicit || calls[0].model != "legacy-A" {
		t.Fatalf("failed preflight changed ordinary sending: %+v", calls)
	}
}

func TestCronModelOverride_MarkedSessionCannotFallBackToLastModel(t *testing.T) {
	a := &cronModelAgent{nativeModel: "native-A", resumeModel: "task-B"}
	e, p := newCronModelEngine(t, a, "")
	if err := e.ExecuteCronJob(cronModelJob("pinned", "task-B")); err != nil {
		t.Fatal(err)
	}
	e.cleanupInteractiveState("test:user")
	a.mu.Lock()
	a.defaultsErr = errors.New("native default model cannot be resolved")
	a.mu.Unlock()
	p.clearSent()
	e.ReceiveMessage(p, answerProfileMessage("ordinary", "ordinary"))
	waitAnswerProfileTest(t, "restore failed visibly", func() bool {
		return len(p.getSent()) > 0 && !e.sessions.GetOrCreateActive("test:user").Busy()
	})
	calls, _, _ := a.snapshot()
	if len(calls) != 1 || !strings.Contains(strings.Join(p.getSent(), "\n"), "cannot be resolved") {
		t.Fatalf("marked session silently reused task model: calls=%+v replies=%q", calls, p.getSent())
	}
}

func TestCronModelOverride_RecoveryMarkerSaveFailurePreventsSend(t *testing.T) {
	a := &cronModelAgent{model: "project-A"}
	e, _ := newCronModelEngine(t, a, "")
	if err := os.Mkdir(e.sessions.StorePath(), 0o755); err != nil {
		t.Fatal(err)
	}
	err := e.ExecuteCronJob(cronModelJob("pinned", "task-B"))
	if err == nil || !strings.Contains(err.Error(), "save task model recovery marker") {
		t.Fatalf("ExecuteCronJob error=%v, want persistent recovery marker failure", err)
	}
	calls, _, _ := a.snapshot()
	if len(calls) != 0 || e.sessions.GetOrCreateActive("test:user").TurnDefaultsRequired() {
		t.Fatalf("failed save leaked into execution: %+v", calls)
	}
}
