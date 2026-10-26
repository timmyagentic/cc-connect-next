package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type groupModelTestAgent struct{ cronModelAgent }

func (a *groupModelTestAgent) AvailableModels(context.Context) []ModelOption {
	return []ModelOption{{Name: "project-A"}, {Name: "project-C"}, {Name: "group-B", Alias: "group"}, {Name: "group-C"}, {Name: "fast-D"}}
}

func newGroupModelEngine(t *testing.T, path string) (*Engine, *cronModelPlatform, *groupModelTestAgent) {
	t.Helper()
	a := &groupModelTestAgent{cronModelAgent: cronModelAgent{model: "project-A"}}
	e, p := newCronModelEngine(t, a, path)
	e.SetProjectStateStore(NewProjectStateStore(filepath.Join(filepath.Dir(e.sessions.StorePath()), "project.json")))
	return e, p, a
}

func groupModelMessage(group, topic, id, content string) *Message {
	key := "test:" + group
	if topic != "" {
		key += ":root:" + topic
	}
	return &Message{SessionKey: key, Platform: "test", GroupID: group, UserID: "owner", MessageID: id, Content: content, ReplyCtx: key}
}

func groupModelChat(t *testing.T, e *Engine, p *cronModelPlatform, a *groupModelTestAgent, msg *Message, expected string) {
	t.Helper()
	before, _, _ := a.snapshot()
	e.ReceiveMessage(p, msg)
	waitAnswerProfileTest(t, msg.MessageID, func() bool {
		calls, _, _ := a.snapshot()
		return len(calls) == len(before)+1 && !e.sessions.GetOrCreateActive(msg.SessionKey).Busy()
	})
	calls, _, _ := a.snapshot()
	if got := calls[len(calls)-1].model; got != expected {
		t.Fatalf("%s model=%q, want %q", msg.MessageID, got, expected)
	}
}

func TestGroupModel_OneShotModelPrecedenceAndPrivateIsolation(t *testing.T) {
	e, p, a := newGroupModelEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("A", "topic-1", "bind", "/model group group-B"))
	e.SetAnswerProfiles(AnswerProfiles{Fast: &AnswerProfileOptions{Model: "fast-D"}, Quality: &AnswerProfileOptions{ReasoningEffort: "high"}})
	groupModelChat(t, e, p, a, groupModelMessage("A", "topic-2", "fast", "/fast hello"), "fast-D")
	groupModelChat(t, e, p, a, groupModelMessage("A", "topic-2", "quality", "/quality hello"), "group-B")
	explicit := groupModelMessage("A", "topic-2", "task", "/fast hello")
	explicit.ModelOverride = "group-C"
	groupModelChat(t, e, p, a, explicit, "group-C")
	groupModelChat(t, e, p, a, groupModelMessage("A", "topic-2", "ordinary", "hello"), "group-B")
	private := groupModelMessage("private", "", "private", "hello")
	private.GroupID, private.IsDirect = "", true
	groupModelChat(t, e, p, a, private, "project-A")
	_, _, setters := a.snapshot()
	if setters != 0 || a.GetModel() != "project-A" {
		t.Fatal("group/one-shot selection changed the shared Agent model")
	}
}

func TestGroupModel_QueuedTurnReadsLatestBindingAndResetCannotSteer(t *testing.T) {
	for _, selection := range []string{"group-C", "reset"} {
		t.Run(selection, func(t *testing.T) {
			e, p, a := newGroupModelEngine(t, "")
			e.SetBusyMessageMode(BusyMessageModeSteer)
			release := make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			a.onSend = func(s *cronModelSession, call cronModelCall) error {
				if call.prompt == "first" {
					<-release
				}
				s.events <- Event{Type: EventResult, Content: "model=" + call.model, Done: true}
				return nil
			}
			e.ReceiveMessage(p, groupModelMessage("A", "t", "bind", "/model group group-B"))
			e.ReceiveMessage(p, groupModelMessage("A", "t", "first", "first"))
			waitAnswerProfileTest(t, "first started", func() bool { calls, _, _ := a.snapshot(); return len(calls) == 1 })
			e.ReceiveMessage(p, groupModelMessage("A", "t", "queued", "queued"))
			e.ReceiveMessage(p, groupModelMessage("A", "t", "change", "/model group "+selection))
			e.ReceiveMessage(p, groupModelMessage("A", "t", "queued-after", "queued-after"))
			groupModelChat(t, e, p, a, groupModelMessage("B", "", "other", "other"), "project-A")
			once.Do(func() { close(release) })
			waitAnswerProfileTest(t, "queue drained", func() bool {
				calls, _, _ := a.snapshot()
				return len(calls) == 4 && !e.sessions.GetOrCreateActive("test:A:root:t").Busy()
			})
			calls, sessions, setters := a.snapshot()
			want := selection
			if selection == "reset" {
				want = "project-A"
			}
			if calls[0].model != "group-B" || calls[2].model != want || calls[3].model != want || setters != 0 {
				t.Fatalf("models leaked across queue/group: %+v setters=%d", calls, setters)
			}
			for _, s := range sessions {
				s.mu.Lock()
				steers := s.steerCalls
				s.mu.Unlock()
				if steers != 0 {
					t.Fatalf("steered into old model: %d", steers)
				}
			}
		})
	}
}

func TestGroupModel_RestartAndNewSessionKeepBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	e, p, a := newGroupModelEngine(t, path)
	e.ReceiveMessage(p, groupModelMessage("A", "t", "bind", "/model group group-B"))
	groupModelChat(t, e, p, a, groupModelMessage("A", "t", "before", "before"), "group-B")
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	e, p, a = newGroupModelEngine(t, path)
	// A callback after restart has no new event carrying ChatType/GroupID.
	callback := groupModelMessage("A", "t", "card", "/model")
	callback.GroupID, callback.IsCardAction = "", true
	e.ReceiveMessage(p, callback)
	if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, "group-B") {
		t.Fatalf("binding missing after restart: %s", got)
	}
	e.ReceiveMessage(p, groupModelMessage("A", "t", "new", "/new"))
	groupModelChat(t, e, p, a, groupModelMessage("A", "t", "after", "after"), "group-B")
	groupModelChat(t, e, p, a, groupModelMessage("A", "new-topic", "topic", "topic"), "group-B")
	// Another bot/project sharing the same working directory owns a different store.
	other, op, oa := newGroupModelEngine(t, "")
	groupModelChat(t, other, op, oa, groupModelMessage("A", "t", "other-bot", "other-bot"), "project-A")
}

func TestGroupModel_InvalidChoiceProviderAndStorageFailClosed(t *testing.T) {
	e, p, a := newGroupModelEngine(t, "")
	private := groupModelMessage("private", "", "private", "/model group group-B")
	private.GroupID, private.IsDirect = "", true
	e.ReceiveMessage(p, private)
	if _, ok := e.groupModelBinding(private.SessionKey); ok {
		t.Fatal("private chat acquired group binding")
	}
	e.ReceiveMessage(p, groupModelMessage("A", "", "bind", "/model group group-B"))
	for _, choice := range []string{"typo", "group-B extra"} {
		e.ReceiveMessage(p, groupModelMessage("A", "", "bad", "/model group "+choice))
		if binding, _ := e.groupModelBinding("test:A"); binding.Model != "group-B" {
			t.Fatalf("invalid choice changed binding: %+v", binding)
		}
	}
	if err := e.projectState.SaveGroupModelBinding("test:A", GroupModelBinding{Model: "group-B", Provider: "different-provider"}); err != nil {
		t.Fatal(err)
	}
	backend, err := a.StartSession(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	session := e.sessions.GetOrCreateActive("test:A")
	if _, err := e.prepareSessionTurnOverrides(a, e.sessions, session, backend, "test:A", "", "", "", ""); err == nil {
		t.Fatal("provider mismatch silently inherited default")
	}
	if _, err := e.prepareSessionTurnOverrides(a, e.sessions, session, backend, "test:A", "", "project-C", "", ""); err != nil {
		t.Fatalf("explicit task model should override binding: %v", err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	e.projectState = NewProjectStateStore(filepath.Join(blocked, "state.json"))
	if _, err := e.setGroupModel(a, "test:A", "group-B"); err == nil {
		t.Fatal("reported success on persistence failure")
	}
}

func TestGroupModel_BackendFailureDoesNotRetryDefault(t *testing.T) {
	e, p, a := newGroupModelEngine(t, "")
	a.onSend = func(s *cronModelSession, call cronModelCall) error { return errors.New("model unavailable") }
	e.ReceiveMessage(p, groupModelMessage("A", "", "bind", "/model group group-B"))
	e.ReceiveMessage(p, groupModelMessage("A", "", "request", "hello"))
	waitAnswerProfileTest(t, "error returned", func() bool {
		return strings.Contains(strings.Join(p.getSent(), "\n"), "model unavailable") && !e.sessions.GetOrCreateActive("test:A").Busy()
	})
	calls, _, _ := a.snapshot()
	if len(calls) != 1 || calls[0].model != "group-B" {
		t.Fatalf("unexpected fallback: %+v", calls)
	}
}

func TestGroupModel_MetadataPersistsAcrossSessionResetAndRollsBackOnWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	sm := NewSessionManager(path)
	sm.UpdateUserMeta("test:A:root:t", "owner", "group A")
	if err := sm.UpdateGroupModelKey("test:A:root:t", "test:A"); err != nil {
		t.Fatal(err)
	}
	sm.NewSession("test:A:root:t", "new")
	loaded := NewSessionManager(path)
	if meta := loaded.GetUserMeta("test:A:root:t"); meta == nil || meta.GroupModelKey != "test:A" || meta.UserName != "owner" {
		t.Fatalf("lost group metadata: %+v", meta)
	}
	blocked := filepath.Join(t.TempDir(), "block")
	if err := os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded.storePath = filepath.Join(blocked, "sessions.json")
	if err := loaded.UpdateGroupModelKey("test:A:root:t", "test:B"); err == nil {
		t.Fatal("expected write failure")
	}
	if got := loaded.GetUserMeta("test:A:root:t").GroupModelKey; got != "test:A" {
		t.Fatalf("failed save changed memory: %s", got)
	}
}

func TestGroupModel_CardActionsUseCommandScopeAndStableModelIdentity(t *testing.T) {
	e, p, a := newGroupModelEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("A", "topic", "query", "/model"))
	card := e.renderModelCard("test:A:root:topic")
	var selectCommand string
	var selectors int
	for _, element := range card.Elements {
		selectBox, ok := element.(CardSelect)
		if !ok {
			continue
		}
		selectors++
		for _, option := range selectBox.Options {
			if option.Value == "cmd:/model group group-B" {
				selectCommand = strings.TrimPrefix(option.Value, "cmd:")
			}
			if !strings.HasPrefix(option.Value, "cmd:/model ") {
				t.Fatalf("card bypassed command scope: %+v", option)
			}
		}
	}
	if selectors != 2 || selectCommand == "" {
		t.Fatalf("missing group/default selectors: %+v", card)
	}
	callback := groupModelMessage("A", "topic", "select", selectCommand)
	callback.GroupID, callback.IsCardAction = "", true
	e.ReceiveMessage(p, callback)
	groupModelChat(t, e, p, a, groupModelMessage("A", "other-topic", "reply", "reply"), "group-B")
	if a.GetModel() != "project-A" {
		t.Fatal("card selection changed default model")
	}
	callback.Content = "/model group reset"
	e.ReceiveMessage(p, callback)
	groupModelChat(t, e, p, a, groupModelMessage("A", "other-topic", "after-reset", "after-reset"), "project-A")
}

type groupModelScheduledPlatform struct{ cronModelPlatform }

func (p *groupModelScheduledPlatform) ResolveCronReplyTarget(key, _ string) (string, any, error) {
	return key + ":root:scheduled-topic", "scheduled-context", nil
}

func TestGroupModel_ScheduledNewTopicInheritsAndReportsFailure(t *testing.T) {
	e, p, a := newGroupModelEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("A", "", "bind", "/model group group-B"))
	scheduledPlatform := &groupModelScheduledPlatform{cronModelPlatform{stubPlatformEngine{n: "test"}}}
	e.platforms = []Platform{scheduledPlatform}
	job := &CronJob{ID: "group", Project: "test", SessionKey: "test:A", Prompt: "scheduled", SessionMode: "new_per_run"}
	if err := e.ExecuteCronJob(job); err != nil {
		t.Fatal(err)
	}
	job.Model = "group-C"
	if err := e.ExecuteCronJob(job); err != nil {
		t.Fatal(err)
	}
	calls, _, _ := a.snapshot()
	if len(calls) != 2 || calls[0].model != "group-B" || calls[1].model != "group-C" {
		t.Fatalf("scheduled model precedence: %+v", calls)
	}
	job.Model = ""
	a.onSend = func(*cronModelSession, cronModelCall) error { return errors.New("group model unavailable") }
	if err := e.ExecuteCronJob(job); err == nil || !strings.Contains(err.Error(), "group model unavailable") {
		t.Fatalf("job did not report failure: %v", err)
	}
}

type groupSpeedTestAgent struct{ groupModelTestAgent }

func (a *groupSpeedTestAgent) ServiceTierCapabilities(model string) (ServiceTierCapabilities, error) {
	return ServiceTierCapabilities{Model: model, Default: "default", Options: []ServiceTierOption{{Name: "default", Value: "default"}}}, nil
}

func TestGroupModel_SpeedUsesGroupModelCatalog(t *testing.T) {
	e, p, _ := newGroupModelEngine(t, "")
	a := &groupSpeedTestAgent{groupModelTestAgent{cronModelAgent{model: "project-A"}}}
	e.agent = a
	e.ReceiveMessage(p, groupModelMessage("A", "", "bind", "/model group group-B"))
	p.clearSent()
	e.ReceiveMessage(p, groupModelMessage("A", "", "speed", "/speed"))
	if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, "group-B") || strings.Contains(got, "project-A") {
		t.Fatalf("speed inspected wrong model: %s", got)
	}
}

func TestGroupModel_WorkspaceDefaultsAreLowerPriorityAndUnsupportedBackendsReject(t *testing.T) {
	e, p, a := newGroupModelEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("A", "", "bind", "/model group group-B"))
	workspace := &groupModelTestAgent{cronModelAgent{model: "workspace-C"}}
	backend, err := workspace.StartSession(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	session := e.sessions.GetOrCreateActive("test:A")
	overrides, err := e.prepareSessionTurnOverrides(workspace, e.sessions, session, backend, "test:A", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.sendAgentTurnWithOverrides(workspace, backend, "workspace", nil, nil, overrides); err != nil {
		t.Fatal(err)
	}
	calls, _, _ := workspace.snapshot()
	if len(calls) != 1 || calls[0].model != "group-B" || a.GetModel() != "project-A" || workspace.GetModel() != "workspace-C" {
		t.Fatalf("workspace default leaked: %+v", calls)
	}
	if _, err := e.setGroupModel(&stubAgent{}, "test:A", "group-C"); err == nil {
		t.Fatal("unsupported backend accepted binding")
	}
	if _, err := e.prepareSessionTurnOverrides(&stubAgent{}, e.sessions, session, backend, "test:A", "", "", "", ""); err == nil {
		t.Fatal("unsupported backend silently inherited default")
	}
}
