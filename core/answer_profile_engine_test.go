package core

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type answerProfileTurnCall struct {
	prompt  string
	options TurnOptions
}

type answerProfileTestSession struct {
	mu            sync.Mutex
	calls         []answerProfileTurnCall
	events        chan Event
	releaseFirst  chan struct{}
	releaseSecond chan struct{}
	steerStarted  chan struct{}
	releaseSteer  chan struct{}
	steerCalls    int
	closed        bool
}

func newAnswerProfileTestSession() *answerProfileTestSession {
	return &answerProfileTestSession{events: make(chan Event, 16)}
}

func (s *answerProfileTestSession) Send(prompt string, _ []ImageAttachment, _ []FileAttachment) error {
	return s.SendWithTurnOptions(prompt, nil, nil, TurnOptions{})
}

func (s *answerProfileTestSession) SendWithTurnOptions(prompt string, _ []ImageAttachment, _ []FileAttachment, options TurnOptions) error {
	s.mu.Lock()
	callIndex := len(s.calls)
	s.calls = append(s.calls, answerProfileTurnCall{prompt: prompt, options: options})
	releaseFirst := s.releaseFirst
	releaseSecond := s.releaseSecond
	s.mu.Unlock()

	go func() {
		if callIndex == 0 && releaseFirst != nil {
			<-releaseFirst
		}
		if callIndex == 1 && releaseSecond != nil {
			<-releaseSecond
		}
		s.events <- Event{Type: EventResult, Content: "ok", Done: true}
	}()
	return nil
}

func (s *answerProfileTestSession) Steer(_ string, _ []ImageAttachment, _ []FileAttachment) error {
	s.mu.Lock()
	s.steerCalls++
	steerStarted := s.steerStarted
	releaseSteer := s.releaseSteer
	s.mu.Unlock()
	if steerStarted != nil {
		select {
		case steerStarted <- struct{}{}:
		default:
		}
	}
	if releaseSteer != nil {
		<-releaseSteer
	}
	return nil
}

func (s *answerProfileTestSession) RespondPermission(string, PermissionResult) error { return nil }
func (s *answerProfileTestSession) Events() <-chan Event                             { return s.events }
func (s *answerProfileTestSession) CurrentSessionID() string                         { return "answer-profile-session" }
func (s *answerProfileTestSession) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed
}
func (s *answerProfileTestSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *answerProfileTestSession) snapshot() ([]answerProfileTurnCall, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	calls := append([]answerProfileTurnCall(nil), s.calls...)
	return calls, s.steerCalls
}

type answerProfileTestAgent struct {
	session *answerProfileTestSession
}

func (a *answerProfileTestAgent) Name() string { return "answer-profile-test" }
func (a *answerProfileTestAgent) StartSession(context.Context, string) (AgentSession, error) {
	return a.session, nil
}
func (a *answerProfileTestAgent) ListSessions(context.Context) ([]AgentSessionInfo, error) {
	return nil, nil
}
func (a *answerProfileTestAgent) Stop() error                { return nil }
func (a *answerProfileTestAgent) GetModel() string           { return "balanced-model" }
func (a *answerProfileTestAgent) GetReasoningEffort() string { return "medium" }
func (a *answerProfileTestAgent) GetServiceTier() string     { return "default" }

func TestEngineAnswerProfileSequenceRestoresDefault(t *testing.T) {
	session := newAnswerProfileTestSession()
	agent := &answerProfileTestAgent{session: session}
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", agent, []Platform{platform}, "", LangEnglish)
	engine.SetAnswerProfiles(AnswerProfiles{
		Fast:    &AnswerProfileOptions{Model: "fast-model", ReasoningEffort: "low", ServiceTier: "fast"},
		Quality: &AnswerProfileOptions{Model: "quality-model", ReasoningEffort: "max", ServiceTier: "default"},
	})
	t.Cleanup(func() { _ = engine.Stop() })

	sendAndWaitForCall(t, engine, platform, 1, "m1", "ordinary task")
	sendAndWaitForCall(t, engine, platform, 2, "m2", "/quality inspect deeply")
	sendAndWaitForCall(t, engine, platform, 3, "m3", "ordinary again")

	calls, _ := session.snapshot()
	if got := calls[0].options; got.AnswerProfile != "" || got.Model != "balanced-model" || got.ReasoningEffort != "medium" || got.ServiceTier != "default" {
		t.Fatalf("ordinary turn options = %+v", got)
	}
	if got := calls[1].options; got.AnswerProfile != AnswerProfileQuality || got.Model != "quality-model" || got.ReasoningEffort != "max" || got.ServiceTier != "default" {
		t.Fatalf("quality turn options = %+v", got)
	}
	if got := calls[1].prompt; got != "inspect deeply" {
		t.Fatalf("quality prompt = %q, want directive stripped", got)
	}
	if got := calls[2].options; got.AnswerProfile != "" || got.Model != "balanced-model" || got.ReasoningEffort != "medium" || got.ServiceTier != "default" {
		t.Fatalf("ordinary turn after quality = %+v, want defaults restored", got)
	}
}

func TestEngineBusyProfileQueuesInsteadOfSteering(t *testing.T) {
	session := newAnswerProfileTestSession()
	session.releaseFirst = make(chan struct{})
	agent := &answerProfileTestAgent{session: session}
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", agent, []Platform{platform}, "", LangEnglish)
	engine.SetBusyMessageMode(BusyMessageModeSteer)
	engine.SetAnswerProfiles(AnswerProfiles{
		Fast: &AnswerProfileOptions{Model: "fast-model", ReasoningEffort: "low", ServiceTier: "fast"},
	})
	t.Cleanup(func() { _ = engine.Stop() })

	engine.ReceiveMessage(platform, answerProfileMessage("m1", "ordinary task"))
	waitAnswerProfileTest(t, "first turn starts", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 1
	})
	engine.ReceiveMessage(platform, answerProfileMessage("m2", "/fast quick follow-up"))
	waitAnswerProfileTest(t, "profiled message queues", func() bool {
		return len(platform.getSent()) >= 1
	})
	if _, steerCalls := session.snapshot(); steerCalls != 0 {
		t.Fatalf("steer calls = %d, want 0 for explicit answer profile", steerCalls)
	}

	close(session.releaseFirst)
	waitAnswerProfileTest(t, "queued profiled turn starts", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 2
	})
	calls, steerCalls := session.snapshot()
	if steerCalls != 0 {
		t.Fatalf("steer calls after drain = %d, want 0", steerCalls)
	}
	if got := calls[1].options; got.AnswerProfile != AnswerProfileFast || got.Model != "fast-model" || got.ReasoningEffort != "low" || got.ServiceTier != "fast" {
		t.Fatalf("queued fast turn options = %+v", got)
	}
}

func TestEngineBusyOrdinaryQueuesWhileProfiledTurnActive(t *testing.T) {
	session := newAnswerProfileTestSession()
	session.releaseFirst = make(chan struct{})
	released := false

	agent := &answerProfileTestAgent{session: session}
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", agent, []Platform{platform}, "", LangEnglish)
	engine.SetBusyMessageMode(BusyMessageModeSteer)
	engine.SetAnswerProfiles(AnswerProfiles{
		Fast: &AnswerProfileOptions{Model: "fast-model", ReasoningEffort: "low", ServiceTier: "fast"},
	})
	t.Cleanup(func() { _ = engine.Stop() })
	t.Cleanup(func() {
		if !released {
			close(session.releaseFirst)
		}
	})

	engine.ReceiveMessage(platform, answerProfileMessage("m1", "/fast profiled task"))
	waitAnswerProfileTest(t, "profiled turn starts", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 1 && calls[0].options.AnswerProfile == AnswerProfileFast
	})

	engine.ReceiveMessage(platform, answerProfileMessage("m2", "ordinary follow-up"))
	waitAnswerProfileTest(t, "ordinary busy message is routed", func() bool {
		_, steerCalls := session.snapshot()
		return steerCalls > 0 || len(platform.getSent()) > 0
	})
	if _, steerCalls := session.snapshot(); steerCalls != 0 {
		t.Fatalf("steer calls = %d, want 0 when ordinary input would change the active profile", steerCalls)
	}

	close(session.releaseFirst)
	released = true
	waitAnswerProfileTest(t, "queued ordinary turn starts", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 2
	})
	calls, steerCalls := session.snapshot()
	if steerCalls != 0 {
		t.Fatalf("steer calls after drain = %d, want 0", steerCalls)
	}
	if got := calls[1].options; got.AnswerProfile != "" || got.Model != "balanced-model" || got.ReasoningEffort != "medium" || got.ServiceTier != "default" {
		t.Fatalf("queued ordinary turn options = %+v, want project defaults", got)
	}
}

func TestEngineQueuedProfileKeepsOrdinaryFollowUpInFIFO(t *testing.T) {
	session := newAnswerProfileTestSession()
	session.releaseFirst = make(chan struct{})
	session.releaseSecond = make(chan struct{})
	releasedFirst := false
	releasedSecond := false

	agent := &answerProfileTestAgent{session: session}
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", agent, []Platform{platform}, "", LangEnglish)
	engine.SetBusyMessageMode(BusyMessageModeSteer)
	engine.SetAnswerProfiles(AnswerProfiles{
		Fast: &AnswerProfileOptions{Model: "fast-model", ReasoningEffort: "low", ServiceTier: "fast"},
	})
	t.Cleanup(func() { _ = engine.Stop() })
	t.Cleanup(func() {
		if !releasedSecond {
			close(session.releaseSecond)
		}
		if !releasedFirst {
			close(session.releaseFirst)
		}
	})

	engine.ReceiveMessage(platform, answerProfileMessage("m1", "ordinary task"))
	waitAnswerProfileTest(t, "ordinary turn starts", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 1
	})
	engine.ReceiveMessage(platform, answerProfileMessage("m2", "/fast queued profile"))
	waitAnswerProfileTest(t, "profiled message queues", func() bool {
		return len(platform.getSent()) > 0
	})

	close(session.releaseFirst)
	releasedFirst = true
	waitAnswerProfileTest(t, "queued profiled turn starts", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 2 && calls[1].options.AnswerProfile == AnswerProfileFast
	})

	sentBeforeOrdinaryFollowUp := len(platform.getSent())
	engine.ReceiveMessage(platform, answerProfileMessage("m3", "ordinary after queued profile"))
	waitAnswerProfileTest(t, "ordinary follow-up queues behind profiled turn", func() bool {
		_, steerCalls := session.snapshot()
		return steerCalls > 0 || len(platform.getSent()) > sentBeforeOrdinaryFollowUp
	})
	if _, steerCalls := session.snapshot(); steerCalls != 0 {
		t.Fatalf("steer calls = %d, want 0 while queued profiled turn is active", steerCalls)
	}

	close(session.releaseSecond)
	releasedSecond = true
	waitAnswerProfileTest(t, "ordinary follow-up starts with defaults", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 3
	})
	calls, steerCalls := session.snapshot()
	if steerCalls != 0 {
		t.Fatalf("steer calls after chained drain = %d, want 0", steerCalls)
	}
	if got := calls[2].options; got.AnswerProfile != "" || got.Model != "balanced-model" || got.ReasoningEffort != "medium" || got.ServiceTier != "default" {
		t.Fatalf("ordinary follow-up options = %+v, want project defaults", got)
	}
}

func TestEngineProfileTransitionWaitsForInFlightSteer(t *testing.T) {
	session := newAnswerProfileTestSession()
	session.releaseFirst = make(chan struct{})
	session.steerStarted = make(chan struct{}, 1)
	session.releaseSteer = make(chan struct{})
	releasedFirst := false
	releasedSteer := false

	agent := &answerProfileTestAgent{session: session}
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", agent, []Platform{platform}, "", LangEnglish)
	engine.SetBusyMessageMode(BusyMessageModeSteer)
	engine.SetAnswerProfiles(AnswerProfiles{
		Fast: &AnswerProfileOptions{Model: "fast-model", ReasoningEffort: "low", ServiceTier: "fast"},
	})
	t.Cleanup(func() { _ = engine.Stop() })
	t.Cleanup(func() {
		if !releasedSteer {
			close(session.releaseSteer)
		}
		if !releasedFirst {
			close(session.releaseFirst)
		}
	})

	engine.ReceiveMessage(platform, answerProfileMessage("m1", "ordinary task"))
	waitAnswerProfileTest(t, "ordinary turn starts", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 1
	})
	engine.ReceiveMessage(platform, answerProfileMessage("m2", "/fast queued profile"))
	waitAnswerProfileTest(t, "profiled message queues", func() bool {
		return len(platform.getSent()) > 0
	})

	steerDone := make(chan struct{})
	go func() {
		engine.ReceiveMessage(platform, answerProfileMessage("m3", "ordinary steer before transition"))
		close(steerDone)
	}()
	select {
	case <-session.steerStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for steer to enter the backend")
	}

	close(session.releaseFirst)
	releasedFirst = true
	transitionedDuringSteer := false
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if calls, _ := session.snapshot(); len(calls) > 1 {
			transitionedDuringSteer = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	close(session.releaseSteer)
	releasedSteer = true
	select {
	case <-steerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for steer routing to finish")
	}
	waitAnswerProfileTest(t, "queued profiled turn starts after steer resolves", func() bool {
		calls, _ := session.snapshot()
		return len(calls) == 2 && calls[1].options.AnswerProfile == AnswerProfileFast && !engine.sessions.GetOrCreateActive("test:user").Busy()
	})
	if transitionedDuringSteer {
		t.Fatal("queued profiled turn started while steer was still resolving")
	}
}

func TestEngineAnswerProfileMustBeConfigured(t *testing.T) {
	session := newAnswerProfileTestSession()
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", &answerProfileTestAgent{session: session}, []Platform{platform}, "", LangEnglish)
	t.Cleanup(func() { _ = engine.Stop() })

	engine.ReceiveMessage(platform, answerProfileMessage("m1", "/fast do it"))
	waitAnswerProfileTest(t, "not-configured reply", func() bool {
		for _, message := range platform.getSent() {
			if strings.Contains(message, "not configured") {
				return true
			}
		}
		return false
	})
	if calls, _ := session.snapshot(); len(calls) != 0 {
		t.Fatalf("agent calls = %d, want 0", len(calls))
	}
}

func TestEngineAnswerProfileFailsWhenSessionDoesNotSupportTurnOptions(t *testing.T) {
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	engine.SetAnswerProfiles(AnswerProfiles{
		Fast: &AnswerProfileOptions{ReasoningEffort: "low"},
	})
	t.Cleanup(func() { _ = engine.Stop() })

	engine.ReceiveMessage(platform, answerProfileMessage("m1", "/fast do it"))
	waitAnswerProfileTest(t, "unsupported-session reply", func() bool {
		for _, message := range platform.getSent() {
			if strings.Contains(message, "does not support one-shot answer profiles") {
				return true
			}
		}
		return false
	})
}

func sendAndWaitForCall(t *testing.T, engine *Engine, platform *stubPlatformEngine, wantCalls int, messageID, content string) {
	t.Helper()
	engine.ReceiveMessage(platform, answerProfileMessage(messageID, content))
	waitAnswerProfileTest(t, "agent turn", func() bool {
		state := engine.sessions.GetOrCreateActive("test:user")
		return state.HistoryLen() >= wantCalls*2
	})
}

func answerProfileMessage(messageID, content string) *Message {
	return &Message{
		SessionKey: "test:user",
		Platform:   "test",
		MessageID:  messageID,
		UserID:     "user",
		UserName:   "user",
		Content:    content,
		ReplyCtx:   messageID,
	}
}

func waitAnswerProfileTest(t *testing.T, reason string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", reason)
}
