package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSpeedUnsupportedIsACommand(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	t.Cleanup(func() { _ = e.Stop() })
	e.ReceiveMessage(p, answerProfileMessage("speed", "/speed fast"))
	if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, "does not support speed") {
		t.Fatalf("unsupported speed must be rejected explicitly, got %q", got)
	}
}

type speedTestAgent struct {
	answerProfileTestAgent
	defaultOnly bool
	catalogErr  error
}

func (a *speedTestAgent) ServiceTierCapabilities(model string) (ServiceTierCapabilities, error) {
	options := []ServiceTierOption{{Name: "default", Value: "default"}}
	if !a.defaultOnly {
		options = append(options, ServiceTierOption{Name: "fast", Value: "priority"})
	}
	return ServiceTierCapabilities{Model: model, Default: "default", Options: options}, a.catalogErr
}

func newSpeedTestEngine(t *testing.T, dir string) (*Engine, *stubPlatformEngine, *speedTestAgent) {
	t.Helper()
	p := &stubPlatformEngine{n: "test"}
	a := &speedTestAgent{answerProfileTestAgent: answerProfileTestAgent{session: newAnswerProfileTestSession()}}
	e := NewEngine("test", a, []Platform{p}, dir, LangEnglish)
	t.Cleanup(func() { _ = e.Stop() })
	return e, p, a
}

func speedCommand(t *testing.T, e *Engine, p *stubPlatformEngine, command string) string {
	t.Helper()
	before := len(p.getSent())
	e.ReceiveMessage(p, answerProfileMessage("speed-command", command))
	return strings.Join(p.getSent()[before:], "\n")
}

func TestSpeedSavedOnlyToSelectedConversation(t *testing.T) {
	e, p, _ := newSpeedTestEngine(t, filepath.Join(t.TempDir(), "sessions.json"))
	if got := speedCommand(t, e, p, "/speed"); !strings.Contains(got, "Next-turn speed: `default`") || !strings.Contains(got, "default, fast") {
		t.Fatalf("query = %q", got)
	}
	if e.sessions.ActiveSessionID("test:user") != "" {
		t.Fatal("read-only query created a session")
	}
	if got := speedCommand(t, e, p, "/speed fast"); !strings.Contains(got, "Saved across bridge restarts") {
		t.Fatalf("change = %q", got)
	}
	original := e.sessions.GetOrCreateActive("test:user")
	original.SetAgentSessionID("preserved-thread", "test")
	original.AddHistory("user", "preserved history")
	if got := speedCommand(t, e, p, "/speed default"); !strings.Contains(got, "`default`") {
		t.Fatalf("default change = %q", got)
	}
	if original.GetAgentSessionID() != "preserved-thread" || original.HistoryLen() != 1 {
		t.Fatal("speed change destroyed session continuity")
	}
	speedCommand(t, e, p, "/speed fast")
	other := e.sessions.GetOrCreateActive("test:other")
	if e.sessions.ServiceTier(other) != "" {
		t.Fatal("speed leaked into another chat")
	}
	fresh := e.sessions.NewSession("test:user", "new")
	if e.sessions.ServiceTier(fresh) != "" {
		t.Fatal("new conversation inherited an old conversation's override")
	}
	if _, err := e.sessions.SwitchSession("test:user", original.ID); err != nil {
		t.Fatal(err)
	}
	reloaded := NewSessionManager(e.sessions.StorePath())
	if got := reloaded.ServiceTier(reloaded.GetOrCreateActive("test:user")); got != "priority" {
		t.Fatalf("speed after reload and switch = %q", got)
	}
}

func TestSpeedChoiceSurvivesImmediateIdleReset(t *testing.T) {
	e, p, a := newSpeedTestEngine(t, "")
	e.resetOnIdle = time.Minute
	session := e.sessions.GetOrCreateActive("test:user")
	session.AddHistory("user", "old conversation")
	session.mu.Lock()
	session.LastUserActivity = time.Now().Add(-time.Hour)
	session.mu.Unlock()
	speedCommand(t, e, p, "/speed fast")
	e.ReceiveMessage(p, answerProfileMessage("after-idle", "continue with the selected speed"))
	waitAnswerProfileTest(t, "turn after speed selection", func() bool {
		calls, _ := a.session.snapshot()
		return len(calls) == 1 && !session.Busy()
	})
	if got := e.sessions.ActiveSessionID("test:user"); got != session.ID {
		t.Fatalf("speed choice discarded by idle rotation: %s -> %s", session.ID, got)
	}
	calls, _ := a.session.snapshot()
	if calls[0].options.ServiceTier != "priority" {
		t.Fatalf("next turn lost selected speed: %+v", calls[0].options)
	}
}

func TestSpeedFailuresDoNotChangePreference(t *testing.T) {
	for _, test := range []struct {
		name, command, want string
		configure           func(*Engine, *speedTestAgent)
	}{
		{"invalid", "/speed turbo", "Unsupported speed", nil},
		{"extra args", "/speed fast now", "Unsupported speed", nil},
		{"unsupported model", "/speed fast", "Unsupported speed", func(_ *Engine, a *speedTestAgent) { a.defaultOnly = true }},
		{"catalog unavailable", "/speed fast", "Cannot verify speed options", func(_ *Engine, a *speedTestAgent) { a.catalogErr = fmt.Errorf("catalog unavailable") }},
		{"disabled", "/speed fast", "disabled", func(e *Engine, _ *speedTestAgent) { e.SetDisabledCommands([]string{"speed"}) }},
		{"save failure", "/speed fast", "Speed was not changed", func(e *Engine, _ *speedTestAgent) { e.sessions.storePath = t.TempDir() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			e, p, a := newSpeedTestEngine(t, "")
			s := e.sessions.GetOrCreateActive("test:user")
			if err := e.sessions.SetServiceTier(s, "default"); err != nil {
				t.Fatal(err)
			}
			if test.configure != nil {
				test.configure(e, a)
			}
			if got := speedCommand(t, e, p, test.command); !strings.Contains(got, test.want) {
				t.Fatalf("reply = %q, want %q", got, test.want)
			}
			if got := e.sessions.ServiceTier(s); got != "default" {
				t.Fatalf("failed change mutated speed: %q", got)
			}
		})
	}
}

func TestSpeedTurnsPreserveModelEffortAndOneShotProfiles(t *testing.T) {
	e, p, a := newSpeedTestEngine(t, "")
	speedCommand(t, e, p, "/speed fast")
	// No answer profiles are configured: ordinary Send must still apply speed.
	sendAndWaitForCall(t, e, p, 1, "first", "first")
	e.SetAnswerProfiles(AnswerProfiles{
		Fast:    &AnswerProfileOptions{ReasoningEffort: "low", ServiceTier: "default"},
		Quality: &AnswerProfileOptions{Model: "quality-model", ReasoningEffort: "max", ServiceTier: "default"},
	})
	sendAndWaitForCall(t, e, p, 2, "fast", "/fast quick")
	sendAndWaitForCall(t, e, p, 3, "ordinary", "ordinary")
	sendAndWaitForCall(t, e, p, 4, "quality", "/quality deep")
	sendAndWaitForCall(t, e, p, 5, "restored", "restored")
	speedCommand(t, e, p, "/speed default")
	sendAndWaitForCall(t, e, p, 6, "default", "default")
	calls, _ := a.session.snapshot()
	for _, index := range []int{0, 2, 4} {
		if got := calls[index].options; got.ServiceTier != "priority" || got.Model != "balanced-model" || got.ReasoningEffort != "medium" || got.AnswerProfile != "" {
			t.Fatalf("ordinary call %d = %+v", index, got)
		}
	}
	if got := calls[1].options; got.ServiceTier != "default" || got.ReasoningEffort != "low" || got.AnswerProfile != AnswerProfileFast {
		t.Fatalf("fast call = %+v", got)
	}
	if got := calls[3].options; got.ServiceTier != "default" || got.Model != "quality-model" || got.ReasoningEffort != "max" {
		t.Fatalf("quality call = %+v", got)
	}
	if got := calls[5].options; got.ServiceTier != "default" || got.Model != "balanced-model" || got.ReasoningEffort != "medium" {
		t.Fatalf("default call = %+v", got)
	}
}

func TestSpeedBusyChangeQueuesAcrossBothTurnHandoffs(t *testing.T) {
	e, p, a := newSpeedTestEngine(t, "")
	e.SetBusyMessageMode(BusyMessageModeSteer)
	a.session.releaseFirst = make(chan struct{})
	a.session.releaseSecond = make(chan struct{})
	first, second := false, false
	t.Cleanup(func() {
		if !first {
			close(a.session.releaseFirst)
		}
		if !second {
			close(a.session.releaseSecond)
		}
	})
	speedCommand(t, e, p, "/speed default")
	e.ReceiveMessage(p, answerProfileMessage("first", "first"))
	waitAnswerProfileTest(t, "first turn", func() bool { calls, _ := a.session.snapshot(); return len(calls) == 1 })
	speedCommand(t, e, p, "/speed fast")
	if got := speedCommand(t, e, p, "/speed"); !strings.Contains(got, "Next-turn speed: `fast`") || !strings.Contains(got, "Running-turn speed: `default`") {
		t.Fatalf("busy query = %q", got)
	}
	e.ReceiveMessage(p, answerProfileMessage("second", "second"))
	if calls, steers := a.session.snapshot(); len(calls) != 1 || steers != 0 || !a.session.Alive() {
		t.Fatalf("speed change interrupted or steered: %d calls, %d steers", len(calls), steers)
	}
	close(a.session.releaseFirst)
	first = true
	waitAnswerProfileTest(t, "queued fast turn", func() bool { calls, _ := a.session.snapshot(); return len(calls) == 2 })
	speedCommand(t, e, p, "/speed default")
	e.ReceiveMessage(p, answerProfileMessage("third", "third"))
	if _, steers := a.session.snapshot(); steers != 0 {
		t.Fatal("message steered across speed boundary of queued turn")
	}
	close(a.session.releaseSecond)
	second = true
	waitAnswerProfileTest(t, "queued default turn", func() bool { calls, _ := a.session.snapshot(); return len(calls) == 3 })
	calls, _ := a.session.snapshot()
	for i, want := range []string{"default", "priority", "default"} {
		if got := calls[i].options.ServiceTier; got != want {
			t.Fatalf("call %d tier = %q, want %q", i, got, want)
		}
	}
}

func TestSpeedRejectsStalePreferenceBeforeSending(t *testing.T) {
	e, _, a := newSpeedTestEngine(t, "")
	a.defaultOnly = true
	if err := e.sendAgentTurn(a, a.session, "task", nil, nil, "", "priority"); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("send = %v", err)
	}
	if calls, _ := a.session.snapshot(); len(calls) != 0 {
		t.Fatal("unsupported speed reached backend")
	}
}

func TestSpeedPersistenceWriteFailureKeepsOriginalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	sm := NewSessionManager(path)
	s := sm.GetOrCreateActive("test:user")
	if err := sm.SetServiceTier(s, "priority"); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A parent path that is a regular file deterministically fails on every OS.
	sm.storePath = filepath.Join(path, "invalid.json")
	if err := sm.SetServiceTier(s, "default"); err == nil {
		t.Fatal("expected save error")
	}
	if sm.ServiceTier(s) != "priority" {
		t.Fatal("save error changed in-memory speed")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(original) {
		t.Fatal("save error changed previous snapshot")
	}
}

func TestSpeedChangeWaitsForAlreadyAcceptedSteer(t *testing.T) {
	e, p, a := newSpeedTestEngine(t, "")
	e.SetBusyMessageMode(BusyMessageModeSteer)
	a.session.releaseFirst = make(chan struct{})
	a.session.releaseSteer = make(chan struct{})
	a.session.steerStarted = make(chan struct{}, 1)
	first, steer := false, false
	t.Cleanup(func() {
		if !steer {
			close(a.session.releaseSteer)
		}
		if !first {
			close(a.session.releaseFirst)
		}
	})
	speedCommand(t, e, p, "/speed default")
	e.ReceiveMessage(p, answerProfileMessage("first", "first"))
	waitAnswerProfileTest(t, "first turn", func() bool { calls, _ := a.session.snapshot(); return len(calls) == 1 })
	go e.ReceiveMessage(p, answerProfileMessage("steer", "additional input"))
	select {
	case <-a.session.steerStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("steer did not start")
	}
	changed := make(chan struct{})
	go func() { e.ReceiveMessage(p, answerProfileMessage("change", "/speed fast")); close(changed) }()
	select {
	case <-changed:
		t.Fatal("speed change raced an in-flight steer")
	case <-time.After(30 * time.Millisecond):
	}
	close(a.session.releaseSteer)
	steer = true
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("speed change did not finish after steer")
	}
	if got := speedCommand(t, e, p, "/speed"); !strings.Contains(got, "Running-turn speed: `default`") || !strings.Contains(got, "Next-turn speed: `fast`") {
		t.Fatalf("post-steer speeds: %q", got)
	}
	close(a.session.releaseFirst)
	first = true
}

type speedNativeDefaultsSession struct{ *answerProfileTestSession }

func (*speedNativeDefaultsSession) DefaultTurnOptions() TurnOptions {
	return TurnOptions{Model: "native-model", ReasoningEffort: "high", ServiceTier: "default"}
}

type speedLiveCatalogSession struct{ *answerProfileTestSession }

func (*speedLiveCatalogSession) ServiceTierCapabilities(model string) (ServiceTierCapabilities, error) {
	return ServiceTierCapabilities{Model: model, Default: "default", Options: []ServiceTierOption{{Name: "default", Value: "default"}}}, nil
}

func TestSpeedLiveCatalogOverridesBootstrapEvidence(t *testing.T) {
	e, p, a := newSpeedTestEngine(t, "")
	live := &speedLiveCatalogSession{a.session}
	e.interactiveStates["test:user"] = &interactiveState{agent: a, agentSession: live}
	if got := speedCommand(t, e, p, "/speed fast"); !strings.Contains(got, "Unsupported speed") {
		t.Fatalf("bootstrap catalog incorrectly overrode live capability: %q", got)
	}
	if err := e.sendAgentTurn(a, live, "task", nil, nil, "", "priority"); err == nil {
		t.Fatal("saved speed bypassed live model validation")
	}
}

type speedInheritedAgent struct{ speedTestAgent }

func (*speedInheritedAgent) GetModel() string           { return "" }
func (*speedInheritedAgent) GetReasoningEffort() string { return "" }

func TestSpeedPreservesCLIInheritedModelAndEffort(t *testing.T) {
	e, _, _ := newSpeedTestEngine(t, "")
	s := &speedNativeDefaultsSession{newAnswerProfileTestSession()}
	a := &speedInheritedAgent{}
	if err := e.sendAgentTurn(a, s, "task", nil, nil, "", "priority"); err != nil {
		t.Fatal(err)
	}
	calls, _ := s.snapshot()
	if got := calls[0].options; got.Model != "native-model" || got.ReasoningEffort != "high" || got.ServiceTier != "priority" {
		t.Fatalf("inherited options = %+v", got)
	}
}

func TestSpeedManifestHelpAndMenuContract(t *testing.T) {
	e, _, _ := newSpeedTestEngine(t, "")
	command := findManifestCommand(t, e.QueryAgentCapabilityManifest("", "", false).Commands, "speed")
	if command.Permission != CapabilityPermissionMember || command.ReadOnly || command.Availability.State != CapabilityConditional || !hasManifestEffect(command.SideEffects, "persistent_state") || hasManifestEffect(command.SideEffects, "agent_process") {
		t.Fatalf("speed manifest = %+v", command)
	}
	for _, lang := range []Language{LangEnglish, LangChinese, LangTraditionalChinese, LangJapanese, LangSpanish} {
		i18n := NewI18n(lang)
		if !strings.Contains(i18n.T(MsgHelp), "/speed") || i18n.T(MsgBuiltinCmdSpeed) == string(MsgBuiltinCmdSpeed) {
			t.Fatalf("speed missing from help/menu for %s", lang)
		}
	}
	found := false
	for _, published := range e.GetBridgePublishedCommands() {
		if published.Name == "speed" {
			found = true
		}
	}
	if !found {
		t.Fatal("speed missing from bridge command projection")
	}
	e.SetDisabledCommands([]string{"speed"})
	command = findManifestCommand(t, e.QueryAgentCapabilityManifest("", "", false).Commands, "speed")
	if command.Availability.State != CapabilityUnavailable {
		t.Fatalf("disabled speed advertised as %+v", command.Availability)
	}
}

func TestSpeedMultiWorkspaceUsesBoundSessionStore(t *testing.T) {
	e, p, _ := newSpeedTestEngine(t, "")
	base := t.TempDir()
	e.SetMultiWorkspace(base, filepath.Join(t.TempDir(), "bindings.json"))
	for _, workspace := range []string{"one", "two"} {
		dir := filepath.Join(base, workspace)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		dir = normalizeWorkspacePath(dir)
		state := e.workspacePool.GetOrCreate(dir)
		state.agent = &speedTestAgent{answerProfileTestAgent: answerProfileTestAgent{session: newAnswerProfileTestSession()}}
		state.sessions = NewSessionManager(filepath.Join(t.TempDir(), "sessions.json"))
		e.workspaceBindings.Bind("project:test", "test:"+workspace, workspace, dir)
	}
	msg := answerProfileMessage("ws-one", "/speed fast")
	msg.ChannelKey = "test:one"
	e.ReceiveMessage(p, msg)
	_, one, _, err := e.commandContext(p, msg)
	if err != nil {
		t.Fatal(err)
	}
	if got := one.ServiceTier(one.GetOrCreateActive(msg.SessionKey)); got != "priority" {
		t.Fatalf("bound workspace speed = %q; replies %q", got, p.getSent())
	}
	msg.ChannelKey = "test:two"
	_, two, _, err := e.commandContext(p, msg)
	if err != nil {
		t.Fatal(err)
	}
	if two == one || two.ServiceTier(two.GetOrCreateActive(msg.SessionKey)) != "" || e.sessions.ServiceTier(e.sessions.GetOrCreateActive(msg.SessionKey)) != "" {
		t.Fatal("speed leaked across workspace stores")
	}
}
