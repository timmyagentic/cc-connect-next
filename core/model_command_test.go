package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestModelCommand_DefaultReadIsMemberAndWritesRequireAdmin(t *testing.T) {
	for _, entry := range []string{"message", "card", "direct-handler"} {
		t.Run(entry, func(t *testing.T) {
			e, p, a := newChatModelEngine(t, "")
			e.SetAdminFrom("admin")
			var saved []string
			e.SetModelSaveFunc(func(model string) error { saved = append(saved, model); return nil })
			invoke := func(user string, args ...string) string {
				t.Helper()
				p.clearSent()
				msg := chatModelMessage("A", "topic", user+strings.Join(args, "-"), "/model default")
				msg.UserID = user
				msg.IsCardAction = entry == "card"
				if len(args) != 0 {
					msg.Content += " " + strings.Join(args, " ")
				}
				if entry == "direct-handler" {
					e.cmdDefaultModel(p, msg, args)
				} else {
					e.ReceiveMessage(p, msg)
				}
				return strings.Join(p.getSent(), "\n")
			}
			if reply := invoke("member"); !strings.Contains(reply, "project-A") || strings.Contains(strings.ToLower(reply), "permission denied") {
				t.Fatalf("member cannot inspect the default: %s", reply)
			}
			for _, choice := range []string{"project-C", "PIN", "3"} {
				if reply := invoke("member", choice); !strings.Contains(strings.ToLower(reply), "admin") {
					t.Fatalf("member write lacks permission feedback: %s", reply)
				}
				if a.GetModel() != "project-A" || len(saved) != 0 {
					t.Fatalf("member changed default: model=%s saved=%v", a.GetModel(), saved)
				}
			}
			if reply := invoke("admin", "project-C"); !strings.Contains(reply, "project-C") {
				t.Fatalf("missing admin write feedback: %s", reply)
			}
			if a.GetModel() != "project-C" || !reflect.DeepEqual(saved, []string{"project-C"}) {
				t.Fatalf("admin default not applied/persisted: model=%s saved=%v", a.GetModel(), saved)
			}
			if binding, ok := e.chatModelBinding("test:A:root:topic"); ok {
				t.Fatalf("global write unexpectedly pinned the invoking chat: %+v", binding)
			}
		})
	}
}

func TestModelCommand_RemovedCommandsAndStaleCommandCallbacksDoNotMutate(t *testing.T) {
	for _, card := range []bool{false, true} {
		for _, command := range []string{
			"/model chat group-C", "/model group group-C", "/model switch project-C",
			"/model chat reset", "/model group reset", "/model switch 2",
			"/model CHAT group-C", "/model GROUP group-C", "/model SWITCH project-C",
			"/model chat", "/model group", "/model switch",
		} {
			t.Run(fmt.Sprintf("card=%v/%s", card, command), func(t *testing.T) {
				e, p, a := newChatModelEngine(t, "")
				e.SetAdminFrom("owner")
				e.ReceiveMessage(p, chatModelMessage("A", "topic", "pin", "/model group-B"))
				var saves int
				e.SetModelSaveFunc(func(string) error { saves++; return nil })
				p.clearSent()
				msg := chatModelMessage("A", "topic", "old-command", command)
				msg.IsCardAction = card
				e.ReceiveMessage(p, msg)
				binding, ok := e.chatModelBinding(msg.SessionKey)
				if !ok || binding.Model != "group-B" || a.GetModel() != "project-A" || saves != 0 {
					t.Fatalf("removed command changed models: binding=%+v default=%s saves=%d", binding, a.GetModel(), saves)
				}
				if reply := strings.Join(p.getSent(), "\n"); !strings.Contains(reply, "/model") {
					t.Fatalf("rejected command lacks normal usage guidance: %s", reply)
				}
				if calls, _, _ := a.snapshot(); len(calls) != 0 {
					t.Fatalf("removed command was forwarded as a paid agent turn: %+v", calls)
				}
			})
		}
	}
}

func TestModelCommand_UnverifiedRouteNeverChangesSharedDefault(t *testing.T) {
	for _, command := range []string{"/model", "/model group-B", "/model 3", "/model reset"} {
		t.Run(command, func(t *testing.T) {
			e, p, a := newChatModelEngine(t, "")
			e.SetAdminFrom("owner")
			var saves int
			e.SetModelSaveFunc(func(string) error { saves++; return nil })
			msg := &Message{Platform: "test", SessionKey: "test:unverified", UserID: "owner", MessageID: "no-route", Content: command, ReplyCtx: "ctx"}
			e.ReceiveMessage(p, msg)
			if a.GetModel() != "project-A" || saves != 0 || e.chatKey(msg.SessionKey) != "" {
				t.Fatal("unverified command fell back to shared model mutation")
			}
			if reply := strings.Join(p.getSent(), "\n"); !strings.Contains(reply, "/model default") {
				t.Fatalf("unverified chat lacks explicit default entry: %s", reply)
			}
			for _, item := range e.renderModelCard(msg.SessionKey).Elements {
				if _, ok := item.(CardSelect); ok {
					t.Fatal("unverified chat exposes a model mutation selector")
				}
			}
		})
	}
}

func TestModelCommand_ModelSwitcherOnlySupportsDefaultNotChatPinning(t *testing.T) {
	a := &stubModelModeAgent{model: "gpt-4.1-mini"}
	e, p := newCronModelEngine(t, a, "")
	e.SetProjectStateStore(NewProjectStateStore(""))
	e.SetAdminFrom("owner")
	e.ReceiveMessage(p, chatModelMessage("A", "", "chat", "/model gpt-4.1"))
	if a.model != "gpt-4.1-mini" {
		t.Fatal("chat pin silently changed the shared model")
	}
	if _, ok := e.chatModelBinding("test:A"); ok {
		t.Fatal("backend without per-turn support acquired a chat binding")
	}
	if reply := strings.Join(p.getSent(), "\n"); !strings.Contains(reply, e.i18n.T(MsgModelChatUnsupported)) {
		t.Fatalf("unsupported chat pin lacks explanation: %s", reply)
	}
	p.clearSent()
	e.ReceiveMessage(p, chatModelMessage("A", "", "default", "/model default gpt"))
	if a.model != "gpt-4.1" || !strings.Contains(strings.Join(p.getSent(), "\n"), "gpt-4.1") {
		t.Fatal("ModelSwitcher-only backend cannot change project default")
	}
}

func TestModelCommand_DefaultPersistenceFailureDoesNotClaimSuccess(t *testing.T) {
	for _, provider := range []bool{false, true} {
		t.Run(fmt.Sprintf("provider=%v", provider), func(t *testing.T) {
			a := &stubModelModeAgent{model: "gpt-4.1-mini"}
			e, p := newCronModelEngine(t, a, "")
			e.SetAdminFrom("owner")
			var saves int
			if provider {
				a.providers = []ProviderConfig{{Name: "P", Model: "gpt-4.1-mini"}}
				a.active = "P"
				e.SetProviderModelSaveFunc(func(string, string) error { saves++; return errors.New("disk full") })
			} else {
				e.SetModelSaveFunc(func(string) error { saves++; return errors.New("disk full") })
			}
			msg := chatModelMessage("A", "", "save-failure", "/model default gpt")
			session := e.sessions.GetOrCreateActive(msg.SessionKey)
			session.SetAgentSessionID("keep-session", "test")
			session.AddHistory("user", "keep history")
			e.ReceiveMessage(p, msg)
			visible := strings.Join(p.getSent(), "\n")
			if !strings.Contains(visible, "disk full") || strings.Contains(visible, e.i18n.Tf(MsgModelDefaultChanged, "gpt-4.1")) {
				t.Fatalf("failure was hidden or success reported: %s", visible)
			}
			if saves != 1 || a.GetModel() != "gpt-4.1-mini" {
				t.Fatalf("failed config save changed runtime: saves=%d model=%s", saves, a.GetModel())
			}
			if provider && a.GetActiveProvider().Model != "gpt-4.1-mini" {
				t.Fatal("failed provider save changed in-memory provider config")
			}
			if session.GetAgentSessionID() != "keep-session" || len(session.GetHistory(0)) != 1 {
				t.Fatal("failed default change reset the conversation")
			}
		})
	}
}

type modelCommandCatalogAgent struct {
	chatModelTestAgent
	options []ModelOption
}

func (a *modelCommandCatalogAgent) AvailableModels(context.Context) []ModelOption {
	return append([]ModelOption(nil), a.options...)
}

func modelCommandSingleSelect(t *testing.T, card *Card) CardSelect {
	t.Helper()
	var boxes []CardSelect
	for _, item := range card.Elements {
		if box, ok := item.(CardSelect); ok {
			boxes = append(boxes, box)
		}
	}
	if len(boxes) != 1 {
		t.Fatalf("expected one selector per scope, got %d: %+v", len(boxes), card)
	}
	return boxes[0]
}

func TestModelCommand_CardsSeparateScopesAndStableNamesSurviveCatalogReorder(t *testing.T) {
	e, p, _ := newChatModelEngine(t, "")
	a := &modelCommandCatalogAgent{
		chatModelTestAgent: chatModelTestAgent{cronModelAgent{model: "project-A"}},
		options:            []ModelOption{{Name: "project-A"}, {Name: "group-B", Alias: "pin"}, {Name: "project-C"}},
	}
	e.agent = a
	e.SetAdminFrom("owner")
	e.ReceiveMessage(p, chatModelMessage("A", "one", "read", "/model"))
	chatCard := e.renderModelCard("test:A:root:one")
	chatBox := modelCommandSingleSelect(t, chatCard)
	defaultBox := modelCommandSingleSelect(t, e.renderModelDefaultCard())
	if chatBox.InitValue != "cmd:/model reset" || defaultBox.InitValue != "cmd:/model default project-A" {
		t.Fatalf("incorrect initial scope: chat=%s default=%s", chatBox.InitValue, defaultBox.InitValue)
	}
	validNames := map[string]bool{"project-A": true, "group-B": true, "project-C": true}
	for _, scope := range []struct {
		box    CardSelect
		prefix string
		chat   bool
	}{{chatBox, "cmd:/model ", true}, {defaultBox, "cmd:/model default ", false}} {
		for _, choice := range scope.box.Options {
			name := strings.TrimPrefix(choice.Value, scope.prefix)
			if !strings.HasPrefix(choice.Value, scope.prefix) || !validNames[name] && !(scope.chat && name == "reset") {
				t.Fatalf("model card uses an unstable or unsafe callback: %q", choice.Value)
			}
		}
	}
	var defaultButton bool
	for _, row := range chatCard.CollectButtons() {
		for _, button := range row {
			if button.Data == "cmd:/model default" {
				defaultButton = true
			}
		}
	}
	if !defaultButton {
		t.Fatal("chat card lacks a secondary default entry")
	}
	// A previously rendered card must retain model identity as catalog order changes.
	a.options[0], a.options[1] = a.options[1], a.options[0]
	callback := chatModelMessage("A", "one", "select", "/model group-B")
	callback.IsCardAction = true
	e.ReceiveMessage(p, callback)
	chatModelTurn(t, e, p, &a.chatModelTestAgent, chatModelMessage("A", "two", "pinned", "pinned"), "group-B")
	if a.GetModel() != "project-A" {
		t.Fatal("chat card changed project default")
	}
	callback.Content, callback.MessageID = "/model default project-C", "default"
	e.ReceiveMessage(p, callback)
	chatModelTurn(t, e, p, &a.chatModelTestAgent, chatModelMessage("B", "", "inherited", "inherited"), "project-C")
	chatModelTurn(t, e, p, &a.chatModelTestAgent, chatModelMessage("A", "two", "still-pinned", "still-pinned"), "group-B")
}

func TestModelCommand_InlineChoicesUseSameStableScopedCommands(t *testing.T) {
	e, _, a := newChatModelEngine(t, "")
	p := &stubInlineButtonPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
	for _, command := range []string{"/model", "/model default"} {
		p.buttonRows = nil
		e.ReceiveMessage(p, chatModelMessage("A", "", command, command))
		prefix := "cmd:/model "
		if command == "/model default" {
			prefix = "cmd:/model default "
		}
		allowed := map[string]bool{"cmd:/model": true, "cmd:/model default": true, "cmd:/model reset": command == "/model"}
		for _, model := range a.AvailableModels(context.Background()) {
			allowed[prefix+model.Name] = true
		}
		var selection bool
		for _, row := range p.buttonRows {
			for _, button := range row {
				if strings.Contains(button.Data, "/model") && !allowed[button.Data] {
					t.Fatalf("%s contains stale or unsafe inline callback %q", command, button.Data)
				}
				if button.Data == prefix+"group-B" {
					selection = true
				}
			}
		}
		if !selection {
			t.Fatalf("%s lacks stable model choices: %+v", command, p.buttonRows)
		}
	}
}

func TestModelCommand_CurrentTurnAndHistorySurviveChangeNextTurnUsesNewModel(t *testing.T) {
	for _, command := range []string{"/model group-B", "/model default group-B"} {
		t.Run(command, func(t *testing.T) {
			e, p, a := newChatModelEngine(t, "")
			e.SetAdminFrom("owner")
			e.SetBusyMessageMode(BusyMessageModeQueue)
			release := make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			a.onSend = func(s *cronModelSession, call cronModelCall) error {
				if call.prompt == "first" {
					<-release
				}
				s.events <- Event{Type: EventResult, Content: "model=" + call.model + " " + call.prompt, Done: true}
				return nil
			}
			first := chatModelMessage("A", "one", "first", "first")
			e.ReceiveMessage(p, first)
			waitAnswerProfileTest(t, "first turn started", func() bool { calls, _, _ := a.snapshot(); return len(calls) == 1 })
			session := e.sessions.GetOrCreateActive(first.SessionKey)
			id, history := session.GetAgentSessionID(), session.GetHistory(0)
			_, backends, _ := a.snapshot()
			e.ReceiveMessage(p, chatModelMessage("A", "one", "change", command))
			if !session.Busy() || !backends[0].Alive() || session.GetAgentSessionID() != id || !reflect.DeepEqual(history, session.GetHistory(0)) {
				t.Fatal("model command interrupted the active turn or reset history")
			}
			e.ReceiveMessage(p, chatModelMessage("A", "one", "next", "next"))
			once.Do(func() { close(release) })
			waitAnswerProfileTest(t, "next turn completed", func() bool {
				return !session.Busy() && strings.Contains(strings.Join(p.getSent(), "\n"), "model=group-B next")
			})
			if visible := strings.Join(p.getSent(), "\n"); !strings.Contains(visible, "model=project-A first") {
				t.Fatalf("in-flight turn changed model: %s", visible)
			}
			if got := e.sessions.GetOrCreateActive(first.SessionKey); got != session || got.GetAgentSessionID() != id {
				t.Fatal("model change replaced the conversation")
			}
			if command == "/model group-B" && a.GetModel() != "project-A" {
				t.Fatal("chat change changed global default")
			}
		})
	}
}

type modelAwareCommandAgent struct{ chatModelTestAgent }

func (a *modelAwareCommandAgent) StartSession(ctx context.Context, id string) (AgentSession, error) {
	backend, err := a.chatModelTestAgent.StartSession(ctx, id)
	if err != nil {
		return nil, err
	}
	return &modelAwareCommandSession{cronModelSession: backend.(*cronModelSession), model: a.GetModel()}, nil
}

type modelAwareCommandSession struct {
	*cronModelSession
	model string // protected by the embedded session's mutex
}

func (s *modelAwareCommandSession) GetModel() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.model
}

func (s *modelAwareCommandSession) SendWithTurnOptions(prompt string, images []ImageAttachment, files []FileAttachment, options TurnOptions) error {
	s.mu.Lock()
	s.model = options.Model
	s.mu.Unlock()
	return s.cronModelSession.SendWithTurnOptions(prompt, images, files, options)
}

func TestModelCommand_DefaultBusySteeringUsesRunningModel(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("default-changed=%v", changed), func(t *testing.T) {
			a := &modelAwareCommandAgent{chatModelTestAgent{cronModelAgent{model: "project-A"}}}
			e, p := newCronModelEngine(t, a, "")
			e.SetAdminFrom("owner")
			e.SetBusyMessageMode(BusyMessageModeSteer)
			release := make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			a.onSend = func(s *cronModelSession, call cronModelCall) error {
				if call.prompt == "first" {
					<-release
				}
				s.events <- Event{Type: EventResult, Content: "model=" + call.model + " " + call.prompt, Done: true}
				return nil
			}
			first := chatModelMessage("A", "", "first", "first")
			e.ReceiveMessage(p, first)
			waitAnswerProfileTest(t, "running model known", func() bool { calls, _, _ := a.snapshot(); return len(calls) == 1 })
			nextModel := "project-A"
			if changed {
				nextModel = "project-C"
			}
			e.ReceiveMessage(p, chatModelMessage("A", "", "default", "/model default "+nextModel))
			e.ReceiveMessage(p, chatModelMessage("A", "", "next", "next"))
			_, backends, _ := a.snapshot()
			backends[0].mu.Lock()
			steers := backends[0].steerCalls
			backends[0].mu.Unlock()
			if changed && steers != 0 {
				t.Fatal("new-default message was steered into the old running model")
			}
			if !changed && steers != 1 {
				t.Fatalf("unchanged-default steering regressed: calls=%d", steers)
			}
			if !backends[0].Alive() || !e.sessions.GetOrCreateActive(first.SessionKey).Busy() {
				t.Fatal("default update interrupted the in-flight turn")
			}
			once.Do(func() { close(release) })
			waitAnswerProfileTest(t, "turns completed", func() bool {
				return !e.sessions.GetOrCreateActive(first.SessionKey).Busy()
			})
			calls, _, _ := a.snapshot()
			if changed {
				if len(calls) != 2 || calls[0].model != "project-A" || calls[1].model != "project-C" {
					t.Fatalf("new turn did not use the changed default: %+v", calls)
				}
				if visible := strings.Join(p.getSent(), "\n"); !strings.Contains(visible, "model=project-C next") {
					t.Fatalf("queued reply used old model: %s", visible)
				}
			} else if len(calls) != 1 {
				t.Fatalf("steered message was also sent as a new turn: %+v", calls)
			}
		})
	}
}

// Real provider-backed agents may prioritize ProviderConfig.Model over the
// field changed by SetModel. A simple model-string stub misses stale providers.
type modelCommandProviderFirstAgent struct{ chatProviderAgent }

func newModelCommandProviderFirstAgent(model string) *modelCommandProviderFirstAgent {
	return &modelCommandProviderFirstAgent{chatProviderAgent{
		chatModelTestAgent: chatModelTestAgent{cronModelAgent{model: model}},
		providers:          []ProviderConfig{{Name: "P", Model: model}},
		active:             "P",
	}}
}

func (a *modelCommandProviderFirstAgent) GetModel() string {
	if provider := a.GetActiveProvider(); provider != nil && provider.Model != "" {
		return provider.Model
	}
	return a.chatModelTestAgent.GetModel()
}

func newModelCommandWorkspace(t *testing.T) (*Engine, *cronModelPlatform, *modelCommandProviderFirstAgent, *modelCommandProviderFirstAgent, *SessionManager) {
	t.Helper()
	global := newModelCommandProviderFirstAgent("project-A")
	cached := newModelCommandProviderFirstAgent("project-A")
	e, p := newCronModelEngine(t, global, "")
	e.SetAdminFrom("owner")
	e.SetMultiWorkspace(t.TempDir(), filepath.Join(t.TempDir(), "bindings.json"))
	dir := normalizeWorkspacePath(t.TempDir())
	e.workspaceBindings.Bind("project:test", "test:workspace", "workspace", dir)
	ws := e.workspacePool.GetOrCreate(dir)
	ws.agent = cached
	ws.sessions = NewSessionManager(filepath.Join(t.TempDir(), "workspace-sessions.json"))
	return e, p, global, cached, ws.sessions
}

func TestModelCommand_CachedProviderModelFollowsGlobalDefault(t *testing.T) {
	e, p, global, cached, sessions := newModelCommandWorkspace(t)
	var providerSaved, modelSaved string
	e.SetProviderModelSaveFunc(func(provider, model string) error {
		providerSaved, modelSaved = provider, model
		return nil
	})
	e.ReceiveMessage(p, chatModelMessage("workspace", "", "default", "/model default project-C"))
	if global.GetModel() != "project-C" || providerSaved != "P" || modelSaved != "project-C" {
		t.Fatalf("global provider model not persisted: global=%s saved=%s/%s", global.GetModel(), providerSaved, modelSaved)
	}
	p.clearSent()
	message := chatModelMessage("workspace", "", "next", "hello")
	e.ReceiveMessage(p, message)
	waitAnswerProfileTest(t, "workspace reply", func() bool {
		calls, _, _ := cached.snapshot()
		return len(calls) == 1 && !sessions.GetOrCreateActive(message.SessionKey).Busy()
	})
	if got := cached.GetModel(); got != "project-C" {
		t.Fatalf("cached active provider masked new project model: %s", got)
	}
	if got := cached.GetActiveProvider(); got == nil || got.Name != "P" {
		t.Fatalf("model default unexpectedly changed provider identity: %+v", got)
	}
	if visible := strings.Join(p.getSent(), "\n"); !strings.Contains(visible, "model=project-C hello") {
		t.Fatalf("workspace user saw stale model after global change: %s", visible)
	}
}

func TestModelCommand_AlreadyQueuedWorkspaceTurnSeesNewGlobalDefault(t *testing.T) {
	e, p, _, cached, sessions := newModelCommandWorkspace(t)
	e.SetBusyMessageMode(BusyMessageModeQueue)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	cached.onSend = func(s *cronModelSession, call cronModelCall) error {
		if call.prompt == "first" {
			<-release
		}
		s.events <- Event{Type: EventResult, Content: "model=" + call.model + " " + call.prompt, Done: true}
		return nil
	}
	first := chatModelMessage("workspace", "", "first", "first")
	e.ReceiveMessage(p, first)
	waitAnswerProfileTest(t, "workspace busy", func() bool { calls, _, _ := cached.snapshot(); return len(calls) == 1 })
	session := sessions.GetOrCreateActive(first.SessionKey)
	sessionID := session.GetAgentSessionID()
	e.ReceiveMessage(p, chatModelMessage("workspace", "", "queued", "queued-before-change"))
	// Updating from a different chat must reach the existing workspace queue.
	// Do not query /model or resolve the workspace again before releasing it.
	e.ReceiveMessage(p, privateChatMessage("owner", "default", "/model default project-C"))
	_, backends, _ := cached.snapshot()
	if !session.Busy() || !backends[0].Alive() || session.GetAgentSessionID() != sessionID {
		t.Fatal("global default change interrupted the active workspace turn")
	}
	once.Do(func() { close(release) })
	waitAnswerProfileTest(t, "workspace queue drained", func() bool {
		calls, _, _ := cached.snapshot()
		return len(calls) == 2 && !session.Busy()
	})
	calls, _, _ := cached.snapshot()
	if calls[0].model != "project-A" || calls[1].model != "project-C" {
		t.Fatalf("already queued workspace turn used stale default: %+v", calls)
	}
	visible := strings.Join(p.getSent(), "\n")
	if !strings.Contains(visible, "model=project-A first") || !strings.Contains(visible, "model=project-C queued-before-change") {
		t.Fatalf("workspace user did not see old-current/new-next behavior: %s", visible)
	}
	if session.GetAgentSessionID() != sessionID {
		t.Fatal("per-turn-capable workspace lost its backend session")
	}
}

func TestModelCommand_ChangedDefaultCannotReachBackendOfPreviousProvider(t *testing.T) {
	a := newModelCommandProviderFirstAgent("project-A")
	a.SetProviders([]ProviderConfig{{Name: "P", Model: "project-A"}, {Name: "Q", Model: "project-C"}})
	e, p := newCronModelEngine(t, a, "")
	e.SetAdminFrom("owner")
	before := chatModelMessage("A", "", "before", "before")
	e.ReceiveMessage(p, before)
	waitAnswerProfileTest(t, "provider P replied", func() bool {
		return strings.Contains(strings.Join(p.getSent(), "\n"), "model=project-A before") && !e.sessions.GetOrCreateActive(before.SessionKey).Busy()
	})
	// The provider change is invoked from a different chat. A's existing backend
	// still speaks to P and must not receive Q's newly selected global model.
	e.ReceiveMessage(p, chatModelMessage("B", "", "provider", "/provider switch Q"))
	e.ReceiveMessage(p, chatModelMessage("B", "", "default", "/model default group-C"))
	p.clearSent()
	e.ReceiveMessage(p, chatModelMessage("A", "", "after", "after"))
	waitAnswerProfileTest(t, "provider mismatch reported", func() bool {
		return len(p.getSent()) > 0 && !e.sessions.GetOrCreateActive(before.SessionKey).Busy()
	})
	calls, _, _ := a.snapshot()
	if len(calls) != 1 {
		t.Fatalf("sent new-provider model to existing old-provider backend: %+v", calls)
	}
	if visible := strings.Join(p.getSent(), "\n"); !strings.Contains(visible, "/new") {
		t.Fatalf("provider mismatch lacks recovery guidance: %s", visible)
	}
}
