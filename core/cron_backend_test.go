package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type cronBackendAgent struct {
	*cronModelAgent
	backendMu sync.Mutex
	backends  map[string]string
}

type cronBackendAdapter struct {
	*cronBackendAgent
	backend string
}

func (a *cronBackendAgent) WithSessionBackend(backend string) (Agent, error) {
	if backend != "exec" && backend != "app_server" {
		return nil, fmt.Errorf("unsupported backend %q", backend)
	}
	return &cronBackendAdapter{cronBackendAgent: a, backend: backend}, nil
}

func (a *cronBackendAgent) start(ctx context.Context, id, backend string) (AgentSession, error) {
	session, err := a.cronModelAgent.StartSession(ctx, id)
	if err == nil {
		a.backendMu.Lock()
		a.backends[session.CurrentSessionID()] = backend
		a.backendMu.Unlock()
	}
	return session, err
}

func (a *cronBackendAgent) StartSession(ctx context.Context, id string) (AgentSession, error) {
	return a.start(ctx, id, "app_server")
}

func (a *cronBackendAdapter) StartSession(ctx context.Context, id string) (AgentSession, error) {
	return a.start(ctx, id, a.backend)
}

func newBackendAgent() *cronBackendAgent {
	return &cronBackendAgent{cronModelAgent: &cronModelAgent{model: "chat-model"}, backends: map[string]string{}}
}

func backendJob(id, backend string) *CronJob {
	job := cronModelJob(id, "task-model")
	job.SessionMode, job.Backend = "new_per_run", backend
	return job
}

func TestCronBackendValidationAndPersistence(t *testing.T) {
	path := t.TempDir()
	store, err := NewCronStore(path)
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	agent := newBackendAgent()
	engine, _ := newCronModelEngine(t, agent, "")
	scheduler.RegisterEngine("test", engine)
	for _, value := range []string{"exec", "app_server", ""} {
		job := backendJob("job-"+value, value)
		job.Enabled = false
		if err := scheduler.AddJob(job); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := NewCronStore(path)
	if err != nil || loaded.Get("job-exec").Backend != "exec" {
		t.Fatalf("backend did not survive reload: %v", err)
	}
	if err := scheduler.UpdateJob("job-exec", "backend", "invalid"); err == nil {
		t.Fatal("invalid backend edit was accepted")
	}
	if store.Get("job-exec").Backend != "exec" {
		t.Fatal("invalid edit changed persisted job")
	}
	if err := scheduler.UpdateJob("job-exec", "session_mode", "reuse"); err == nil {
		t.Fatal("backend override could borrow ordinary session")
	}
	if err := scheduler.UpdateJob("job-exec", "backend", ""); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.UpdateJob("job-exec", "session_mode", "reuse"); err != nil {
		t.Fatal(err)
	}
	job := backendJob("shell", "exec")
	job.Exec = "echo ignored"
	if err := scheduler.AddJob(job); err == nil {
		t.Fatal("shell job accepted backend")
	}
	job = backendJob("inherited-session", "exec")
	job.SessionMode = ""
	scheduler.SetDefaultSessionMode("new_per_run")
	if err := scheduler.AddJob(job); err != nil {
		t.Fatal(err)
	}
	if got := store.Get("inherited-session").Backend; got != "exec" {
		t.Fatal(got)
	}
}

func TestCronBackendRejectsUnsupportedAgentAtRegistrationAndExecution(t *testing.T) {
	agent := &cronModelAgent{model: "chat-model"}
	engine, _ := newCronModelEngine(t, agent, "")
	store, _ := NewCronStore(t.TempDir())
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test", engine)
	job := backendJob("unsupported", "exec")
	if err := scheduler.AddJob(job); err == nil {
		t.Fatal("unsupported agent accepted task backend")
	}
	if err := engine.ExecuteCronJob(job); err == nil {
		t.Fatal("unsupported agent ran task backend")
	}
	calls, sessions, _ := agent.snapshot()
	if len(calls) != 0 || len(sessions) != 0 {
		t.Fatal("invalid backend launched work")
	}
}

func TestCUJ_E3_CronBackendDoesNotChangeOrdinaryChat(t *testing.T) {
	agent := newBackendAgent()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	agent.onSend = func(s *cronModelSession, call cronModelCall) error {
		agent.backendMu.Lock()
		backend := agent.backends[s.id]
		agent.backendMu.Unlock()
		if call.prompt == "task-exec" {
			started <- struct{}{}
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "backend=" + backend + " model=" + call.model, Done: true}
		return nil
	}
	engine, platform := newCronModelEngine(t, agent, "")
	store, _ := NewCronStore(t.TempDir())
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test", engine)
	engine.SetCronScheduler(scheduler)
	job := backendJob("task-exec", "exec")
	job.Enabled = false
	if err := scheduler.AddJob(job); err != nil {
		t.Fatal(err)
	}

	waitCronModelChat(t, engine, platform, agent.cronModelAgent, "ordinary-before", 1)
	active := engine.sessions.GetOrCreateActive("test:user").ID
	engine.ReceiveMessage(platform, answerProfileMessage("trigger", "/cron exec task-exec"))
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("Cron did not start")
	}
	waitCronModelChat(t, engine, platform, agent.cronModelAgent, "ordinary-during", 3)
	once.Do(func() { close(release) })
	waitAnswerProfileTest(t, "task response", func() bool {
		return strings.Contains(strings.Join(platform.getSent(), "\n"), "backend=exec model=task-model")
	})
	waitCronModelChat(t, engine, platform, agent.cronModelAgent, "ordinary-after", 4)
	if got := engine.sessions.GetOrCreateActive("test:user").ID; got != active {
		t.Fatalf("ordinary session changed from %s to %s", active, got)
	}
	sent := strings.Join(platform.getSent(), "\n")
	if strings.Count(sent, "backend=app_server model=chat-model") != 3 {
		t.Fatalf("ordinary chat backend changed: %s", sent)
	}
	calls, sessions, setCalls := agent.snapshot()
	if len(calls) != 4 || len(sessions) != 2 || setCalls != 0 {
		t.Fatalf("calls=%d sessions=%d setters=%d", len(calls), len(sessions), setCalls)
	}
}
