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

type chatModelTestAgent struct{ cronModelAgent }

func (a *chatModelTestAgent) AvailableModels(context.Context) []ModelOption {
	return []ModelOption{{Name: "project-A"}, {Name: "project-C"}, {Name: "group-B", Alias: "pin"}, {Name: "group-C"}, {Name: "fast-D"}}
}

func newChatModelEngine(t *testing.T, path string) (*Engine, *cronModelPlatform, *chatModelTestAgent) {
	t.Helper()
	a := &chatModelTestAgent{cronModelAgent: cronModelAgent{model: "project-A"}}
	e, p := newCronModelEngine(t, a, path)
	e.SetProjectStateStore(NewProjectStateStore(filepath.Join(filepath.Dir(e.sessions.StorePath()), "project.json")))
	return e, p, a
}

func chatModelMessage(group, topic, id, content string) *Message {
	key := "test:" + group
	if topic != "" {
		key += ":root:" + topic
	}
	return &Message{SessionKey: key, Platform: "test", GroupID: group, UserID: "owner", MessageID: id, Content: content, ReplyCtx: key}
}

func chatModelTurn(t *testing.T, e *Engine, p *cronModelPlatform, a *chatModelTestAgent, msg *Message, expected string) {
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

func TestChatModel_OneShotModelPrecedenceAndPrivateIsolation(t *testing.T) {
	e, p, a := newChatModelEngine(t, "")
	e.ReceiveMessage(p, chatModelMessage("A", "topic-1", "bind", "/model group-B"))
	e.SetAnswerProfiles(AnswerProfiles{Fast: &AnswerProfileOptions{Model: "fast-D"}, Quality: &AnswerProfileOptions{ReasoningEffort: "high"}})
	chatModelTurn(t, e, p, a, chatModelMessage("A", "topic-2", "fast", "/fast hello"), "fast-D")
	chatModelTurn(t, e, p, a, chatModelMessage("A", "topic-2", "quality", "/quality hello"), "group-B")
	explicit := chatModelMessage("A", "topic-2", "task", "/fast hello")
	explicit.ModelOverride = "group-C"
	chatModelTurn(t, e, p, a, explicit, "group-C")
	chatModelTurn(t, e, p, a, chatModelMessage("A", "topic-2", "ordinary", "hello"), "group-B")
	private := chatModelMessage("private", "", "private", "hello")
	private.GroupID, private.IsDirect = "", true
	chatModelTurn(t, e, p, a, private, "project-A")
	_, _, setters := a.snapshot()
	if setters != 0 || a.GetModel() != "project-A" {
		t.Fatal("group/one-shot selection changed the shared Agent model")
	}
}

func TestChatModel_QueuedTurnReadsLatestBindingAndResetCannotSteer(t *testing.T) {
	for _, selection := range []string{"group-C", "reset"} {
		t.Run(selection, func(t *testing.T) {
			e, p, a := newChatModelEngine(t, "")
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
			e.ReceiveMessage(p, chatModelMessage("A", "t", "bind", "/model group-B"))
			e.ReceiveMessage(p, chatModelMessage("A", "t", "first", "first"))
			waitAnswerProfileTest(t, "first started", func() bool { calls, _, _ := a.snapshot(); return len(calls) == 1 })
			e.ReceiveMessage(p, chatModelMessage("A", "t", "queued", "queued"))
			e.ReceiveMessage(p, chatModelMessage("A", "t", "change", "/model "+selection))
			e.ReceiveMessage(p, chatModelMessage("A", "t", "queued-after", "queued-after"))
			chatModelTurn(t, e, p, a, chatModelMessage("B", "", "other", "other"), "project-A")
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

func TestChatModel_RestartAndNewSessionKeepBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	e, p, a := newChatModelEngine(t, path)
	e.ReceiveMessage(p, chatModelMessage("A", "t", "bind", "/model group-B"))
	chatModelTurn(t, e, p, a, chatModelMessage("A", "t", "before", "before"), "group-B")
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	e, p, a = newChatModelEngine(t, path)
	// A callback after restart has no new event carrying ChatType/GroupID.
	callback := chatModelMessage("A", "t", "card", "/model")
	callback.GroupID, callback.IsCardAction = "", true
	e.ReceiveMessage(p, callback)
	if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, "group-B") {
		t.Fatalf("binding missing after restart: %s", got)
	}
	e.ReceiveMessage(p, chatModelMessage("A", "t", "new", "/new"))
	chatModelTurn(t, e, p, a, chatModelMessage("A", "t", "after", "after"), "group-B")
	chatModelTurn(t, e, p, a, chatModelMessage("A", "new-topic", "topic", "topic"), "group-B")
	// Another bot/project sharing the same working directory owns a different store.
	other, op, oa := newChatModelEngine(t, "")
	chatModelTurn(t, other, op, oa, chatModelMessage("A", "t", "other-bot", "other-bot"), "project-A")
}

func TestChatModel_InvalidChoiceProviderAndStorageFailClosed(t *testing.T) {
	e, p, a := newChatModelEngine(t, "")
	private := chatModelMessage("private", "", "private", "/model group-B")
	private.GroupID, private.IsDirect, private.UserID = "", true, ""
	e.ReceiveMessage(p, private)
	if _, ok := e.chatModelBinding(private.SessionKey); ok {
		t.Fatal("unverified private chat acquired binding")
	}
	e.ReceiveMessage(p, chatModelMessage("A", "", "bind", "/model group-B"))
	for _, choice := range []string{"typo", "group-B extra"} {
		e.ReceiveMessage(p, chatModelMessage("A", "", "bad", "/model "+choice))
		if binding, _ := e.chatModelBinding("test:A"); binding.Model != "group-B" {
			t.Fatalf("invalid choice changed binding: %+v", binding)
		}
	}
	if err := e.projectState.SaveChatModelBinding("test:A", ChatModelBinding{Model: "group-B", Provider: "different-provider"}); err != nil {
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
	if _, err := e.setChatModel(a, "test:A", "group-B"); err == nil {
		t.Fatal("reported success on persistence failure")
	}
}

func TestChatModel_BackendFailureDoesNotRetryDefault(t *testing.T) {
	e, p, a := newChatModelEngine(t, "")
	a.onSend = func(s *cronModelSession, call cronModelCall) error { return errors.New("model unavailable") }
	e.ReceiveMessage(p, chatModelMessage("A", "", "bind", "/model group-B"))
	e.ReceiveMessage(p, chatModelMessage("A", "", "request", "hello"))
	waitAnswerProfileTest(t, "error returned", func() bool {
		return strings.Contains(strings.Join(p.getSent(), "\n"), "model unavailable") && !e.sessions.GetOrCreateActive("test:A").Busy()
	})
	calls, _, _ := a.snapshot()
	if len(calls) != 1 || calls[0].model != "group-B" {
		t.Fatalf("unexpected fallback: %+v", calls)
	}
}

func TestChatModel_MetadataPersistsAcrossSessionResetAndRollsBackOnWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	sm := NewSessionManager(path)
	sm.UpdateUserMeta("test:A:root:t", "owner", "group A")
	if err := sm.UpdateChatKey("test:A:root:t", "test:A"); err != nil {
		t.Fatal(err)
	}
	sm.NewSession("test:A:root:t", "new")
	loaded := NewSessionManager(path)
	if meta := loaded.GetUserMeta("test:A:root:t"); meta == nil || meta.ChatKey != "test:A" || meta.UserName != "owner" {
		t.Fatalf("lost group metadata: %+v", meta)
	}
	blocked := filepath.Join(t.TempDir(), "block")
	if err := os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded.storePath = filepath.Join(blocked, "sessions.json")
	if err := loaded.UpdateChatKey("test:A:root:t", "test:B"); err == nil {
		t.Fatal("expected write failure")
	}
	if got := loaded.GetUserMeta("test:A:root:t").ChatKey; got != "test:A" {
		t.Fatalf("failed save changed memory: %s", got)
	}
}

func TestChatModel_CardActionsUseCommandScopeAndStableModelIdentity(t *testing.T) {
	e, p, a := newChatModelEngine(t, "")
	e.ReceiveMessage(p, chatModelMessage("A", "topic", "query", "/model"))
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
			if option.Value == "cmd:/model group-B" {
				selectCommand = strings.TrimPrefix(option.Value, "cmd:")
			}
			if !strings.HasPrefix(option.Value, "cmd:/model ") {
				t.Fatalf("card bypassed command scope: %+v", option)
			}
		}
	}
	if selectors != 1 || selectCommand == "" {
		t.Fatalf("expected exactly one current-chat selector: %+v", card)
	}
	callback := chatModelMessage("A", "topic", "select", selectCommand)
	callback.GroupID, callback.IsCardAction = "", true
	e.ReceiveMessage(p, callback)
	chatModelTurn(t, e, p, a, chatModelMessage("A", "other-topic", "reply", "reply"), "group-B")
	if a.GetModel() != "project-A" {
		t.Fatal("card selection changed default model")
	}
	callback.Content = "/model reset"
	e.ReceiveMessage(p, callback)
	chatModelTurn(t, e, p, a, chatModelMessage("A", "other-topic", "after-reset", "after-reset"), "project-A")
}

type chatModelScheduledPlatform struct{ cronModelPlatform }

func (p *chatModelScheduledPlatform) ResolveCronReplyTarget(key, _ string) (string, any, error) {
	return key + ":root:scheduled-topic", "scheduled-context", nil
}

func TestChatModel_ScheduledNewTopicInheritsAndReportsFailure(t *testing.T) {
	e, p, a := newChatModelEngine(t, "")
	e.ReceiveMessage(p, chatModelMessage("A", "", "bind", "/model group-B"))
	scheduledPlatform := &chatModelScheduledPlatform{cronModelPlatform{stubPlatformEngine{n: "test"}}}
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

type groupSpeedTestAgent struct{ chatModelTestAgent }

func (a *groupSpeedTestAgent) ServiceTierCapabilities(model string) (ServiceTierCapabilities, error) {
	return ServiceTierCapabilities{Model: model, Default: "default", Options: []ServiceTierOption{{Name: "default", Value: "default"}}}, nil
}

func TestChatModel_SpeedUsesCurrentChatModelCatalog(t *testing.T) {
	e, p, _ := newChatModelEngine(t, "")
	a := &groupSpeedTestAgent{chatModelTestAgent{cronModelAgent{model: "project-A"}}}
	e.agent = a
	e.ReceiveMessage(p, chatModelMessage("A", "", "bind", "/model group-B"))
	p.clearSent()
	e.ReceiveMessage(p, chatModelMessage("A", "", "speed", "/speed"))
	if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, "group-B") || strings.Contains(got, "project-A") {
		t.Fatalf("speed inspected wrong model: %s", got)
	}
}

func TestChatModel_PinWinsOverAgentDefaultAndUnsupportedBackendsReject(t *testing.T) {
	e, p, a := newChatModelEngine(t, "")
	e.ReceiveMessage(p, chatModelMessage("A", "", "bind", "/model group-B"))
	workspace := &chatModelTestAgent{cronModelAgent{model: "project-A"}}
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
	if len(calls) != 1 || calls[0].model != "group-B" || a.GetModel() != "project-A" || workspace.GetModel() != "project-A" {
		t.Fatalf("chat pin did not override project default: %+v", calls)
	}
	if _, err := e.setChatModel(&stubAgent{}, "test:A", "group-C"); err == nil {
		t.Fatal("unsupported backend accepted binding")
	}
	if _, err := e.prepareSessionTurnOverrides(&stubAgent{}, e.sessions, session, backend, "test:A", "", "", "", ""); err == nil {
		t.Fatal("unsupported backend silently inherited default")
	}
}
