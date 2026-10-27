package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type groupProviderAgent struct {
	groupModelTestAgent
	providerMu sync.Mutex
	providers  []ProviderConfig
	active     string
	startGate  func()
}

func (a *groupProviderAgent) SetProviders(values []ProviderConfig) {
	a.providerMu.Lock()
	defer a.providerMu.Unlock()
	a.providers = append([]ProviderConfig(nil), values...)
}
func (a *groupProviderAgent) ListProviders() []ProviderConfig {
	a.providerMu.Lock()
	defer a.providerMu.Unlock()
	return append([]ProviderConfig(nil), a.providers...)
}
func (a *groupProviderAgent) SetActiveProvider(name string) bool {
	a.providerMu.Lock()
	defer a.providerMu.Unlock()
	for _, p := range a.providers {
		if p.Name == name {
			a.active = name
			return true
		}
	}
	return false
}
func (a *groupProviderAgent) GetActiveProvider() *ProviderConfig {
	a.providerMu.Lock()
	defer a.providerMu.Unlock()
	for _, p := range a.providers {
		if p.Name == a.active {
			return &p
		}
	}
	return nil
}
func (a *groupProviderAgent) StartSession(ctx context.Context, id string) (AgentSession, error) {
	provider := ""
	if p := a.GetActiveProvider(); p != nil {
		provider = p.Name
	}
	if a.startGate != nil {
		a.startGate()
	}
	backend, err := a.groupModelTestAgent.StartSession(ctx, id)
	if err != nil {
		return nil, err
	}
	return &groupProviderSession{cronModelSession: backend.(*cronModelSession), provider: provider}, nil
}

type groupProviderSession struct {
	*cronModelSession
	provider string
}

func (s *groupProviderSession) ModelProviderName() string { return s.provider }

func newGroupProviderAgent() *groupProviderAgent {
	return &groupProviderAgent{groupModelTestAgent: groupModelTestAgent{cronModelAgent{model: "project-A"}}, providers: []ProviderConfig{
		{Name: "P", Models: []ModelOption{{Name: "custom-provider-model", Alias: "custom"}}, Model: "custom-default"},
		{Name: "Q"},
	}, active: "P"}
}

func TestGroupModel_ExistingBackendProviderCannotFollowMutableAgent(t *testing.T) {
	e, p, _ := newGroupModelEngine(t, "")
	a := newGroupProviderAgent()
	e.agent = a
	e.ReceiveMessage(p, groupModelMessage("A", "one", "bind", "/model group group-B"))
	groupModelChat(t, e, p, &a.groupModelTestAgent, groupModelMessage("A", "one", "before", "before"), "group-B")
	// Another topic changes the Agent provider; the first backend remains on P.
	e.ReceiveMessage(p, groupModelMessage("A", "two", "provider", "/provider switch Q"))
	e.ReceiveMessage(p, groupModelMessage("A", "two", "rebind", "/model group group-C"))
	before, sessions, _ := a.snapshot()
	p.clearSent()
	e.ReceiveMessage(p, groupModelMessage("A", "one", "after", "after"))
	waitAnswerProfileTest(t, "old backend rejected", func() bool {
		return !e.sessions.GetOrCreateActive("test:A:root:one").Busy() && strings.Contains(strings.Join(p.getSent(), "\n"), "different Provider")
	})
	after, _, _ := a.snapshot()
	if len(after) != len(before) || !sessions[0].Alive() {
		t.Fatalf("stale provider received a request or lost history: %+v", after)
	}
}

func TestGroupModel_ExplicitProviderModelsRemainSelectableWithNativeCatalog(t *testing.T) {
	e, p, _ := newGroupModelEngine(t, "")
	a := newGroupProviderAgent()
	e.agent = a
	for _, selection := range []string{"custom", "custom-default"} {
		e.ReceiveMessage(p, groupModelMessage("A", "", "set", "/model group "+selection))
		want := selection
		if selection == "custom" {
			want = "custom-provider-model"
		}
		groupModelChat(t, e, p, &a.groupModelTestAgent, groupModelMessage("A", "", selection, selection), want)
	}
	if got := e.renderModelCard("test:A").RenderText(); !strings.Contains(got, "custom-provider-model") || !strings.Contains(got, "custom-default") {
		t.Fatalf("configured models hidden by native catalog: %s", got)
	}
}

func TestGroupModel_ScheduledBindingDuringStartupStillReportsFailure(t *testing.T) {
	e, p, _ := newGroupModelEngine(t, "")
	a := newGroupProviderAgent()
	e.agent = a
	e.ReceiveMessage(p, groupModelMessage("A", "", "known-group", "/model"))
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a.startGate = func() { close(started); <-release }
	a.onSend = func(*cronModelSession, cronModelCall) error { return errors.New("late bound model unavailable") }
	job := &CronJob{ID: "late", Project: "test", SessionKey: "test:A", Prompt: "scheduled", Mute: true}
	done := make(chan error, 1)
	go func() { done <- e.ExecuteCronJob(job) }()
	<-started
	// Persist directly while startup holds the engine's interactive-state lock;
	// another group/topic's command publishes the same atomic store operation.
	if err := e.projectState.SaveGroupModelBinding("test:A", GroupModelBinding{Model: "group-B", Provider: groupModelProvider(a)}); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	if err := <-done; err == nil || !strings.Contains(err.Error(), "late bound model unavailable") {
		t.Fatalf("late binding failure reported success: %v", err)
	}
}

func TestGroupModel_RichCardsKeepSafeRecoveryInstructions(t *testing.T) {
	p := &stubCompactProgressPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}, style: "card", supportPayload: true}
	a := &groupModelTestAgent{cronModelAgent{model: "project-A"}}
	dir := t.TempDir()
	e := NewEngine("test", a, []Platform{p}, filepath.Join(dir, "sessions.json"), LangEnglish)
	e.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich"})
	e.SetProjectStateStore(NewProjectStateStore(filepath.Join(dir, "project.json")))
	t.Cleanup(func() { _ = e.Stop() })
	e.ReceiveMessage(p, groupModelMessage("A", "", "bind", "/model group group-B"))
	if err := e.projectState.SaveGroupModelBinding("test:A", GroupModelBinding{Model: "group-B", Provider: "other"}); err != nil {
		t.Fatal(err)
	}
	p.clearSent()
	e.ReceiveMessage(p, groupModelMessage("A", "", "request", "hello"))
	waitAnswerProfileTest(t, "safe group failure", func() bool { return !e.sessions.GetOrCreateActive("test:A").Busy() })
	if len(p.getPreviewStarts()) == 0 {
		t.Fatal("test did not enter rich-card rendering")
	}
	texts := append(p.getPreviewEdits(), p.getSent()...)
	if got := strings.Join(texts, "\n"); !strings.Contains(got, "/model chat reset") {
		t.Fatalf("missing group recovery hint: %s", got)
	}
	calls, _, _ := a.snapshot()
	if len(calls) != 0 {
		t.Fatalf("mismatched binding was sent: %+v", calls)
	}
}
