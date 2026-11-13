package core

import (
	"context"
	"sync"
	"testing"
)

type cronPriorityAgent struct{ *cronModelAgent }

func (a *cronPriorityAgent) ServiceTierCapabilities(model string) (ServiceTierCapabilities, error) {
	return ServiceTierCapabilities{Model: model, Default: "default", Options: []ServiceTierOption{{Name: "Standard", Value: "default"}, {Name: "Fast", Value: "priority"}}}, nil
}

func TestCronServiceTierReviewCatalogIDsAndIsolation(t *testing.T) {
	for _, mode := range []string{"reuse", "new_per_run"} {
		t.Run(mode, func(t *testing.T) {
			base := &cronModelAgent{model: "project-model", nativeModel: "project-model", effort: "medium"}
			a := &cronPriorityAgent{base}
			e, p := newCronModelEngine(t, a, "")
			active := e.sessions.GetOrCreateActive("test:user")
			if err := e.sessions.SetServiceTier(active, "default"); err != nil {
				t.Fatal(err)
			}
			job := cronModelJob("priority-task", "task-model")
			job.ServiceTier = "priority"
			job.SessionMode = mode
			if err := e.ExecuteCronJob(job); err != nil {
				t.Fatal(err)
			}
			waitCronModelChat(t, e, p, base, "after-priority", 2)
			calls, _, setters := base.snapshot()
			if calls[0].options.ServiceTier != "priority" || calls[1].options.ServiceTier != "default" || setters != 0 || e.sessions.ServiceTier(active) != "default" {
				t.Fatalf("tier leaked or catalog ID lost: %+v setters=%d", calls, setters)
			}
			for _, invalid := range []string{"fast", "unsupported"} {
				bad := cronModelJob("bad-tier", "")
				bad.ServiceTier = invalid
				if err := e.ExecuteCronJob(bad); err == nil {
					t.Fatalf("accepted noncatalog tier %q", invalid)
				}
			}
			calls, _, _ = base.snapshot()
			if len(calls) != 2 {
				t.Fatalf("invalid tier reached backend: %+v", calls)
			}
		})
	}
}

func TestCronServiceTierReviewUnsupportedBackendFailsClosed(t *testing.T) {
	base := &cronModelAgent{model: "project-model"}
	e, _ := newCronModelEngine(t, base, "")
	job := cronModelJob("unsupported-backend", "")
	job.ServiceTier = "priority"
	if err := e.ExecuteCronJob(job); err == nil {
		t.Fatal("backend without catalog accepted override")
	}
	calls, _, _ := base.snapshot()
	if len(calls) != 0 {
		t.Fatal("unsupported override submitted")
	}
}

func TestCronServiceTierReviewQueuePreservesPriorityAndRestoresDefault(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	base := &cronModelAgent{model: "project-model", nativeModel: "project-model"}
	base.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.prompt == "first" {
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "ok", Done: true}
		return nil
	}
	e, p := newCronModelEngine(t, &cronPriorityAgent{base}, "")
	e.SetBusyMessageMode(BusyMessageModeSteer)
	active := e.sessions.GetOrCreateActive("test:user")
	if err := e.sessions.SetServiceTier(active, "default"); err != nil {
		t.Fatal(err)
	}
	e.ReceiveMessage(p, answerProfileMessage("first", "first"))
	waitAnswerProfileTest(t, "first running", func() bool { calls, _, _ := base.snapshot(); return len(calls) == 1 })
	msg := answerProfileMessage("priority-queued", "priority-queued")
	msg.ServiceTierOverride = "priority"
	e.ReceiveMessage(p, msg)
	waitAnswerProfileTest(t, "priority queued", func() bool { return cronModelQueueDepth(e) == 1 })
	once.Do(func() { close(release) })
	waitAnswerProfileTest(t, "priority completed", func() bool { calls, _, _ := base.snapshot(); return len(calls) == 2 && !active.Busy() })
	waitCronModelChat(t, e, p, base, "ordinary-after", 3)
	calls, sessions, setters := base.snapshot()
	if calls[1].options.ServiceTier != "priority" || calls[2].options.ServiceTier != "default" || setters != 0 {
		t.Fatalf("queue tier lost/leaked: %+v", calls)
	}
	sessions[0].mu.Lock()
	steers := sessions[0].steerCalls
	sessions[0].mu.Unlock()
	if steers != 0 {
		t.Fatalf("explicit tier steered into active turn: %d", steers)
	}
}

func TestCronServiceTierReviewClearingStoredOverrideRestoresInheritance(t *testing.T) {
	base := &cronModelAgent{model: "project-model", nativeModel: "project-model"}
	e, _ := newCronModelEngine(t, &cronPriorityAgent{base}, "")
	active := e.sessions.GetOrCreateActive("test:user")
	if err := e.sessions.SetServiceTier(active, "default"); err != nil {
		t.Fatal(err)
	}
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	job := cronModelJob("clear-priority", "")
	job.ServiceTier = "priority"
	if err := store.Add(job); err != nil {
		t.Fatal(err)
	}
	if !store.Update(job.ID, "service_tier", "") {
		t.Fatal("clear failed")
	}
	if err := e.ExecuteCronJob(store.Get(job.ID)); err != nil {
		t.Fatal(err)
	}
	calls, _, _ := base.snapshot()
	if len(calls) != 1 || calls[0].options.ServiceTier != "default" {
		t.Fatalf("clear did not inherit: %+v", calls)
	}
}

type cronStickyTierAgent struct{ *cronPriorityAgent }

func (a *cronStickyTierAgent) StartSession(ctx context.Context, id string) (AgentSession, error) {
	raw, err := a.cronModelAgent.StartSession(ctx, id)
	if err != nil {
		return nil, err
	}
	return &cronStickyTierSession{cronModelSession: raw.(*cronModelSession)}, nil
}

type cronStickyTierSession struct {
	*cronModelSession
	tierMu sync.Mutex
	tier   string
}

func (s *cronStickyTierSession) SendWithTurnOptions(prompt string, images []ImageAttachment, files []FileAttachment, options TurnOptions) error {
	s.tierMu.Lock()
	s.tier = options.ServiceTier
	s.tierMu.Unlock()
	return s.cronModelSession.SendWithTurnOptions(prompt, images, files, options)
}
func (s *cronStickyTierSession) Send(prompt string, _ []ImageAttachment, _ []FileAttachment) error {
	s.tierMu.Lock()
	tier := s.tier
	s.tierMu.Unlock()
	return s.send(cronModelCall{prompt: prompt, options: TurnOptions{ServiceTier: tier}})
}
func TestCronServiceTierReviewTierOnlyRestoresUnconfiguredNativeDefault(t *testing.T) {
	base := &cronModelAgent{}
	a := &cronStickyTierAgent{&cronPriorityAgent{base}}
	e, p := newCronModelEngine(t, a, "")
	job := cronModelJob("tier-only", "")
	job.ServiceTier = "priority"
	if err := e.ExecuteCronJob(job); err != nil {
		t.Fatal(err)
	}
	waitCronModelChat(t, e, p, base, "ordinary-native-default", 2)
	calls, _, setters := base.snapshot()
	if calls[0].options.ServiceTier != "priority" {
		t.Fatalf("task tier missing: %+v", calls)
	}
	if calls[1].options.ServiceTier != "" || !calls[1].explicit || setters != 0 {
		t.Fatalf("sticky task tier leaked into next ordinary turn: %+v", calls)
	}
}

func TestCronServiceTierReviewQueueUsesWorkspaceAgentCatalog(t *testing.T) {
	for _, workspaceHasCatalog := range []bool{true, false} {
		name := "workspace-supported"
		if !workspaceHasCatalog {
			name = "workspace-unsupported"
		}
		t.Run(name, func(t *testing.T) {
			base := &cronModelAgent{model: "workspace-model"}
			supported := &cronPriorityAgent{base}
			var project, workspace Agent = base, supported
			if !workspaceHasCatalog {
				project, workspace = supported, base
			}
			e, p := newCronModelEngine(t, project, "")
			native, err := workspace.StartSession(context.Background(), "")
			if err != nil {
				t.Fatal(err)
			}
			state := &interactiveState{agent: workspace, agentSession: native}
			e.interactiveStates["test:user"] = state
			msg := answerProfileMessage("workspace-priority", "workspace-priority")
			msg.ServiceTierOverride = "priority"
			if !e.queueMessageForBusySession(p, msg, "test:user", e.sessions.GetOrCreateActive("test:user")) {
				t.Fatal("queue did not handle message")
			}
			want := 0
			if workspaceHasCatalog {
				want = 1
			}
			if len(state.pendingMessages) != want {
				t.Fatalf("queue used project catalog instead of workspace: got %d want %d", len(state.pendingMessages), want)
			}
		})
	}
}
