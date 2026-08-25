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
	mu           sync.Mutex
	calls        []answerProfileTurnCall
	events       chan Event
	releaseFirst chan struct{}
	steerCalls   int
	closed       bool
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
	s.mu.Unlock()

	go func() {
		if callIndex == 0 && releaseFirst != nil {
			<-releaseFirst
		}
		s.events <- Event{Type: EventResult, Content: "ok", Done: true}
	}()
	return nil
}

func (s *answerProfileTestSession) Steer(_ string, _ []ImageAttachment, _ []FileAttachment) error {
	s.mu.Lock()
	s.steerCalls++
	s.mu.Unlock()
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
