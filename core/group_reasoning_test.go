package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newGroupReasoningEngine(t *testing.T, path string) (*Engine, *cronModelPlatform, *groupModelTestAgent) {
	t.Helper()
	e, p, a := newGroupModelEngine(t, path)
	a.effort = "medium"
	a.onSend = func(s *cronModelSession, call cronModelCall) error {
		effort := call.options.ReasoningEffort
		if !call.explicit {
			effort = "medium"
		}
		s.events <- Event{Type: EventResult, Content: "effort=" + effort + " model=" + call.model, Done: true}
		return nil
	}
	return e, p, a
}

func reasoningChat(t *testing.T, e *Engine, p *cronModelPlatform, a *groupModelTestAgent, msg *Message, effort string) {
	t.Helper()
	p.clearSent()
	groupModelChat(t, e, p, a, msg, "project-A")
	if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, "effort="+effort) {
		t.Fatalf("user saw wrong effort: %s", got)
	}
}

func TestCUJ_GroupReasoning1_PersistentGroupEffortAndIndependentModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	e, p, a := newGroupReasoningEngine(t, path)
	e.ReceiveMessage(p, groupModelMessage("A", "one", "pin", "/reasoning group high"))
	if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, "high") {
		t.Fatalf("missing confirmation: %s", got)
	}
	reasoningChat(t, e, p, a, groupModelMessage("A", "two", "a", "hello"), "high")
	reasoningChat(t, e, p, a, groupModelMessage("B", "one", "b", "hello"), "medium")
	private := groupModelMessage("private", "", "dm", "hello")
	private.GroupID, private.IsDirect = "", true
	reasoningChat(t, e, p, a, private, "medium")
	e.ReceiveMessage(p, groupModelMessage("A", "one", "model", "/model group group-B"))
	e.ReceiveMessage(p, groupModelMessage("A", "one", "model-reset", "/model group reset"))
	e.ReceiveMessage(p, groupModelMessage("A", "two", "new", "/new"))
	reasoningChat(t, e, p, a, groupModelMessage("A", "two", "after-new", "hello"), "high")
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	e, p, a = newGroupReasoningEngine(t, path)
	reasoningChat(t, e, p, a, groupModelMessage("A", "fresh", "restart", "hello"), "high")
	e.ReceiveMessage(p, groupModelMessage("A", "fresh", "model2", "/model group group-B"))
	e.ReceiveMessage(p, groupModelMessage("A", "fresh", "reset", "/reasoning group reset"))
	groupModelChat(t, e, p, a, groupModelMessage("A", "fresh", "after", "hello"), "group-B")
	calls, _, setters := a.snapshot()
	if calls[len(calls)-1].options.ReasoningEffort != "medium" || setters != 0 || a.GetReasoningEffort() != "medium" {
		t.Fatalf("reset did not restore defaults without mutation: %+v", calls)
	}
	other, op, oa := newGroupReasoningEngine(t, "")
	reasoningChat(t, other, op, oa, groupModelMessage("A", "fresh", "other-bot", "hello"), "medium")
}

func TestGroupReasoning_ProfileAndTaskOverridesAreIndependentOfModel(t *testing.T) {
	e, p, a := newGroupReasoningEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("A", "", "pin", "/reasoning group high"))
	e.SetAnswerProfiles(AnswerProfiles{Fast: &AnswerProfileOptions{ReasoningEffort: "low"}, Quality: &AnswerProfileOptions{Model: "project-A"}})
	reasoningChat(t, e, p, a, groupModelMessage("A", "", "fast", "/fast hello"), "low")
	explicit := groupModelMessage("A", "", "explicit", "/fast hello")
	explicit.ReasoningEffortOverride = "xhigh"
	reasoningChat(t, e, p, a, explicit, "xhigh")
	reasoningChat(t, e, p, a, groupModelMessage("A", "", "quality", "/quality hello"), "high")
	reasoningChat(t, e, p, a, groupModelMessage("A", "", "ordinary", "hello"), "high")
}

func TestGroupReasoning_QueuedTurnResolvesChangeAndResetWithoutSteering(t *testing.T) {
	for _, choice := range []string{"low", "reset"} {
		t.Run(choice, func(t *testing.T) {
			e, p, a := newGroupReasoningEngine(t, "")
			e.SetBusyMessageMode(BusyMessageModeSteer)
			release := make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			a.onSend = func(s *cronModelSession, call cronModelCall) error {
				if call.prompt == "first" {
					<-release
				}
				s.events <- Event{Type: EventResult, Content: "ok", Done: true}
				return nil
			}
			e.ReceiveMessage(p, groupModelMessage("A", "t", "pin", "/reasoning group high"))
			e.ReceiveMessage(p, groupModelMessage("A", "t", "first", "first"))
			waitAnswerProfileTest(t, "turn started", func() bool { calls, _, _ := a.snapshot(); return len(calls) == 1 })
			e.ReceiveMessage(p, groupModelMessage("A", "t", "queued", "queued"))
			e.ReceiveMessage(p, groupModelMessage("A", "t", "change", "/reasoning group "+choice))
			e.ReceiveMessage(p, groupModelMessage("A", "t", "queued2", "queued2"))
			once.Do(func() { close(release) })
			waitAnswerProfileTest(t, "drained", func() bool {
				calls, _, _ := a.snapshot()
				return len(calls) == 3 && !e.sessions.GetOrCreateActive("test:A:root:t").Busy()
			})
			calls, sessions, _ := a.snapshot()
			want := choice
			if choice == "reset" {
				want = "medium"
			}
			if calls[0].options.ReasoningEffort != "high" || calls[1].options.ReasoningEffort != want || calls[2].options.ReasoningEffort != want {
				t.Fatalf("queued effort leaked: %+v", calls)
			}
			for _, s := range sessions {
				s.mu.Lock()
				steers := s.steerCalls
				s.mu.Unlock()
				if steers != 0 {
					t.Fatalf("steered into previous effort: %d", steers)
				}
			}
		})
	}
}

func TestGroupReasoning_InvalidContextBackendProviderAndStorageFailClosed(t *testing.T) {
	e, p, a := newGroupReasoningEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("A", "", "pin", "/reasoning group high"))
	for _, choice := range []string{"typo", "high extra"} {
		e.ReceiveMessage(p, groupModelMessage("A", "", "bad", "/reasoning group "+choice))
		if binding, _ := e.groupReasoningBinding("test:A"); binding.Effort != "high" {
			t.Fatal("invalid selection changed binding")
		}
	}
	private := groupModelMessage("private", "", "private", "/reasoning group low")
	private.GroupID, private.IsDirect = "", true
	e.ReceiveMessage(p, private)
	if _, bound := e.groupReasoningBinding(private.SessionKey); bound {
		t.Fatal("private chat bound effort")
	}
	backend, _ := a.StartSession(context.Background(), "")
	session := e.sessions.GetOrCreateActive("test:A")
	if _, err := e.prepareSessionTurnOverrides(&stubAgent{}, e.sessions, session, backend, "test:A", "", "", "", ""); err == nil {
		t.Fatal("unsupported backend fell back")
	}
	if _, err := e.groupDefaultReasoning(a, "test:A", &groupProviderSession{provider: "wrong"}); err == nil {
		t.Fatal("conversation Provider mismatch fell back")
	}
	if err := e.projectState.SaveGroupReasoningBinding("test:A", GroupReasoningBinding{Effort: "high", Provider: "wrong"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.prepareSessionTurnOverrides(a, e.sessions, session, backend, "test:A", "", "", "", ""); err == nil {
		t.Fatal("saved Provider mismatch fell back")
	}
	if _, err := e.prepareSessionTurnOverrides(a, e.sessions, session, backend, "test:A", "", "", "low", ""); err != nil {
		t.Fatalf("explicit effort must override saved binding: %v", err)
	}
	block := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(block, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	e.projectState.storePath = filepath.Join(block, "state.json")
	if _, err := e.setGroupReasoning(a, "test:A", "low"); err == nil {
		t.Fatal("failed save reported success")
	}
	if binding, _ := e.groupReasoningBinding("test:A"); binding.Effort != "high" {
		t.Fatal("failed save changed memory")
	}
}

func TestGroupReasoning_ScheduledNewTopicAndExplicitTaskEffort(t *testing.T) {
	e, p, a := newGroupReasoningEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("A", "", "pin", "/reasoning group high"))
	e.platforms = []Platform{&groupModelScheduledPlatform{cronModelPlatform{stubPlatformEngine{n: "test"}}}}
	job := &CronJob{ID: "group", Project: "test", SessionKey: "test:A", Prompt: "scheduled", SessionMode: "new_per_run"}
	if err := e.ExecuteCronJob(job); err != nil {
		t.Fatal(err)
	}
	job.ReasoningEffort = "low"
	if err := e.ExecuteCronJob(job); err != nil {
		t.Fatal(err)
	}
	calls, _, setters := a.snapshot()
	if len(calls) != 2 || calls[0].options.ReasoningEffort != "high" || calls[1].options.ReasoningEffort != "low" || setters != 0 {
		t.Fatalf("scheduled effort precedence: %+v", calls)
	}
}

func TestGroupReasoning_CardActionsAndManifest(t *testing.T) {
	e, p, a := newGroupReasoningEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("A", "topic", "query", "/reasoning group"))
	card := e.renderReasoningCardForAgent("test:A:root:topic", a)
	var command string
	for _, element := range card.Elements {
		if box, ok := element.(CardSelect); ok {
			for _, option := range box.Options {
				if option.Value == "cmd:/reasoning group high" {
					command = strings.TrimPrefix(option.Value, "cmd:")
				}
			}
		}
	}
	if command == "" {
		t.Fatal("group reasoning card missing")
	}
	callback := groupModelMessage("A", "topic", "select", command)
	callback.GroupID, callback.IsCardAction = "", true
	e.ReceiveMessage(p, callback)
	reasoningChat(t, e, p, a, groupModelMessage("A", "another", "after", "hello"), "high")
	meta := findManifestCommand(t, e.QueryAgentCapabilityManifest("", "", false).Commands, "reasoning")
	if !strings.Contains(meta.Usage, "group") || !hasManifestEffect(meta.SideEffects, "persistent_state") || meta.Permission != CapabilityPermissionMember {
		t.Fatalf("incorrect manifest: %+v", meta)
	}
}

func TestGroupReasoningI18n_AllLanguages(t *testing.T) {
	for _, key := range []MsgKey{MsgReasoningGroupOnly, MsgReasoningGroupStorageUnavailable, MsgReasoningGroupUnsupported, MsgReasoningGroupProviderChanged, MsgReasoningGroupUsage, MsgReasoningGroupFailed, MsgReasoningGroupReset, MsgReasoningGroupChanged, MsgReasoningGroupCurrent, MsgReasoningGroupDefault, MsgReasoningGroupSelect} {
		for _, lang := range []Language{LangEnglish, LangChinese, LangTraditionalChinese, LangJapanese, LangSpanish} {
			text := messages[key][lang]
			if text == "" {
				t.Fatalf("missing %s/%s", key, lang)
			}
			var args []any
			if key == MsgReasoningGroupCurrent || key == MsgReasoningGroupChanged || key == MsgReasoningGroupFailed {
				args = []any{"value"}
			}
			if strings.Contains(fmt.Sprintf(text, args...), "%!") {
				t.Fatalf("format mismatch: %s", text)
			}
		}
	}
}

// A card must route stable effort names through the same workspace resolver as text.
func TestGroupReasoning_DefaultCardChoiceUsesWorkspaceAgent(t *testing.T) {
	p := &stubPlatformEngine{n: "feishu"}
	global := &stubModelModeAgent{reasoningEffort: "low"}
	e := NewEngine("test", global, []Platform{p}, "", LangEnglish)
	t.Cleanup(e.cancel)
	e.SetMultiWorkspace(t.TempDir(), filepath.Join(t.TempDir(), "bindings.json"))
	wsDir := normalizeWorkspacePath(t.TempDir())
	e.workspaceBindings.Bind("project:test", "channel", "name", wsDir)
	ws := e.workspacePool.GetOrCreate(wsDir)
	local := &stubModelModeAgent{reasoningEffort: "high"}
	ws.agent, ws.sessions = local, NewSessionManager("")
	key := "feishu:channel:user"
	e.sessions.GetOrCreateActive(key).SetAgentSessionID("root-history", "test")
	card := e.handleCardNav("nav:/reasoning", key)
	command := ""
	for _, element := range card.Elements {
		if box, ok := element.(CardSelect); ok {
			if box.InitValue != "cmd:/reasoning high" {
				t.Fatalf("card read wrong Agent: %+v", box)
			}
			for _, option := range box.Options {
				if option.Value == "cmd:/reasoning medium" {
					command = strings.TrimPrefix(option.Value, "cmd:")
				}
			}
		}
	}
	if command == "" {
		t.Fatal("card did not use stable command choices")
	}
	e.ReceiveMessage(p, &Message{SessionKey: key, MessageID: "card", UserID: "user", IsCardAction: true, Content: command, ReplyCtx: key})
	if local.GetReasoningEffort() != "medium" || global.GetReasoningEffort() != "low" {
		t.Fatalf("card changed wrong default: local=%s global=%s", local.GetReasoningEffort(), global.GetReasoningEffort())
	}
	if e.sessions.GetOrCreateActive(key).GetAgentSessionID() != "root-history" {
		t.Fatal("card reset root history")
	}
}
