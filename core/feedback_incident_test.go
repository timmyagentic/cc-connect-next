package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

type feedbackCaptureSession struct {
	stubAgentSession
	events  chan Event
	started chan struct{}
	once    sync.Once
}

func TestFeedbackStartFailureRetainsRequestForExistingManualCommand(t *testing.T) {
	p := &restartNotifyStub{name: "test"}
	agent := &controllableAgent{startSessionFn: func(context.Context, string) (AgentSession, error) {
		return nil, errors.New("backend failed before turn/start")
	}}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	e.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
	t.Cleanup(func() { _ = e.Stop() })
	reports := captureFeedbackSubmissions(e)
	e.ReceiveMessage(p, &Message{SessionKey: "test:alice", UserID: "alice", Content: "the request before startup failed"})
	deadline := time.Now().Add(2 * time.Second)
	for len(p.sentTexts()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	e.ReceiveMessage(p, &Message{SessionKey: "test:alice", UserID: "alice", Content: "/feedback"})
	select {
	case report := <-reports:
		d := report.Diagnostic
		if d == nil || d.Request != "the request before startup failed" || d.Phase != "agent_start" || d.Transport.ProcessState != "not_started" || !strings.Contains(d.Error, "before turn/start") {
			t.Fatalf("startup failure lost its context: %#v", d)
		}
	case <-time.After(time.Second):
		t.Fatal("existing manual feedback command did not submit")
	}
}

type feedbackObservedSession struct {
	feedbackCaptureSession
	closed atomic.Bool
}

func (s *feedbackObservedSession) Close() error { s.closed.Store(true); return nil }
func (s *feedbackObservedSession) Alive() bool  { return !s.closed.Load() }
func (s *feedbackObservedSession) DiagnosticSnapshot() AgentDiagnosticSnapshot {
	state, dropped, received, delivered := "eof", 3, true, false
	if s.closed.Load() {
		state = "cancelled"
	}
	return AgentDiagnosticSnapshot{Backend: "observed", ReadState: state, DroppedEvents: &dropped, TerminalReceived: &received, TerminalDelivered: &delivered}
}

func TestFeedbackTerminalFailureFreezesFactsBeforeCleanup(t *testing.T) {
	for _, phase := range []string{"idle_timeout", "event_channel_closed"} {
		t.Run(phase, func(t *testing.T) {
			as := &feedbackObservedSession{feedbackCaptureSession: feedbackCaptureSession{events: make(chan Event, 8), started: make(chan struct{})}}
			p := &restartNotifyStub{name: "test"}
			e := NewEngine("test", &controllableAgent{nextSession: as}, []Platform{p}, "", LangEnglish)
			e.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
			e.SetEventIdleTimeout(30 * time.Millisecond)
			t.Cleanup(func() { _ = e.Stop() })
			reports := captureFeedbackSubmissions(e)
			e.ReceiveMessage(p, &Message{SessionKey: "test:alice", UserID: "alice", Content: "keep this original request"})
			select {
			case <-as.started:
			case <-time.After(2 * time.Second):
				t.Fatal("turn did not start")
			}
			if phase == "event_channel_closed" {
				close(as.events)
			}
			deadline := time.Now().Add(2 * time.Second)
			for !as.closed.Load() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if !as.closed.Load() {
				t.Fatal("failure did not close the session")
			}
			e.ReceiveMessage(p, &Message{SessionKey: "test:alice", UserID: "alice", Content: "/feedback"})
			select {
			case report := <-reports:
				d := report.Diagnostic
				if d == nil || d.Phase != phase || d.Request != "keep this original request" || d.Transport.ReadState != "eof" || d.Transport.DroppedEvents == nil || *d.Transport.DroppedEvents != 3 || !*d.Transport.TerminalReceived || *d.Transport.TerminalDelivered {
					t.Fatalf("cleanup overwrote original facts: %#v", d)
				}
			case <-time.After(time.Second):
				t.Fatal("manual feedback did not submit captured fault")
			}
		})
	}
}

func (s *feedbackCaptureSession) Send(string, []ImageAttachment, []FileAttachment) error {
	s.once.Do(func() { close(s.started) })
	return nil
}
func (s *feedbackCaptureSession) Events() <-chan Event { return s.events }

func TestFeedbackLongTurnKeepsOriginalRequestAfterHistoryWindowExpires(t *testing.T) {
	previousVersion := CurrentVersion
	CurrentVersion = "v0.0.0-feedback-test"
	t.Cleanup(func() { CurrentVersion = previousVersion })
	as := &feedbackCaptureSession{events: make(chan Event, 8), started: make(chan struct{})}
	p := &restartNotifyStub{name: "test"}
	e := NewEngine("test", &controllableAgent{nextSession: as}, []Platform{p}, "", LangEnglish)
	t.Cleanup(func() { _ = e.Stop() })
	e.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
	const key = "test:alice"
	const request = "Find why the overnight export never finishes"
	approvedPayload := make(chan string, 1)
	e.feedbackSubmitFn = func(_ context.Context, draft appfeatures.FeedbackDraft, approved bool) (appfeatures.FeedbackReceipt, error) {
		value, err := draft.Approve(approved)
		if err != nil {
			return appfeatures.FeedbackReceipt{}, err
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return appfeatures.FeedbackReceipt{}, err
		}
		approvedPayload <- string(payload)
		return appfeatures.FeedbackReceipt{ReferenceURL: "https://github.com/owner/repository/issues/1"}, nil
	}
	e.ReceiveMessage(p, &Message{SessionKey: key, UserID: "alice", Platform: "test", MessageID: "original", Content: request, ExtraContent: "INJECTED_CONTEXT_MUST_STAY_PRIVATE"})
	select {
	case <-as.started:
	case <-time.After(2 * time.Second):
		t.Fatal("agent did not receive request")
	}
	// Simulate a long-running task without waiting two hours. The history
	// timestamp is not a reliable lifetime for the still-active request.
	s := e.sessions.GetOrCreateActive(key)
	s.mu.Lock()
	for i := range s.History {
		s.History[i].Timestamp = time.Now().Add(-2 * time.Hour)
	}
	s.mu.Unlock()
	e.feedbackMu.Lock()
	for _, turn := range e.feedbackTurns {
		turn.mu.Lock()
		turn.at = time.Now().Add(-2 * time.Hour)
		turn.diagnostic.StartedAt = turn.at
		turn.mu.Unlock()
	}
	e.feedbackMu.Unlock()
	as.events <- Event{Type: EventError, Error: errors.New("agent turn exceeded max_turn_time (2h0m0s), stopped"), Done: true}
	var offer string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, text := range p.sentTexts() {
			if strings.Contains(text, "/feedback submit-token ") {
				offer = text
				break
			}
		}
		if offer != "" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if offer == "" {
		t.Fatal("error did not offer feedback")
	}
	token := feedbackSubmitTokenFromText(t, offer)
	// Starting a new conversation must not replace an older card's exact
	// owned snapshot. All three user actions use the real routing boundary.
	e.ReceiveMessage(p, &Message{SessionKey: key, UserID: "alice", Content: "/new"})
	e.ReceiveMessage(p, &Message{SessionKey: key, UserID: "alice", Content: "/feedback submit-token " + token})
	select {
	case payload := <-approvedPayload:
		if !strings.Contains(payload, request) {
			t.Fatalf("long-running request was lost from approved feedback: %s", payload)
		}
		if strings.Contains(payload, "INJECTED_CONTEXT_MUST_STAY_PRIVATE") {
			t.Fatal("injected context entered the approved report")
		}
		// The host delivery gate passes this exact user-approved wire payload
		// through the real Relay renderer and a mocked GitHub boundary.
		if path := os.Getenv("CCN_FEEDBACK_CUJ_PAYLOAD"); path != "" {
			if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if !strings.Contains(strings.Join(p.sentTexts(), "\n"), "Submission succeeded") {
			t.Fatal("existing feedback result was not delivered")
		}
	case <-time.After(time.Second):
		t.Fatal("feedback did not submit")
	}
}
