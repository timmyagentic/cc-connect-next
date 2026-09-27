package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFeedbackSteerAndQueueKeepParticipantOwnership(t *testing.T) {
	e, p := newFeedbackTestEngine(t)
	const key = "test:shared"
	session := e.sessions.GetOrCreateActive(key)
	turn := e.beginFeedbackTurn(p, &Message{SessionKey: key, UserID: "alice", Content: "alice private request"}, session, e.agent)
	state := &interactiveState{feedbackTurn: turn, currentSessionKey: key, currentUserID: "alice", agent: e.agent}
	state.mu.Lock()
	e.adoptSteerFeedbackLocked(state, steerHandoff{userID: "bob", feedbackText: "bob request"})
	state.currentUserID = "bob"
	state.mu.Unlock()
	e.completeFeedbackTurn(state, "answer includes alice private request")
	bob, err := e.buildFeedbackDraft(key, "bob", "report answer quality", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value := bob.Report().Diagnostic; value == nil || value.Request != "bob request" || value.Response != "" || !strings.Contains(strings.Join(value.Missing, " "), "handoff") {
		t.Fatalf("mixed participants: %#v", value)
	}
	alice, err := e.buildFeedbackDraft(key, "alice", "report my request", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := approvedFeedbackBytes(t, alice); strings.Contains(got, "bob request") {
		t.Fatal("predecessor inherited successor input")
	}
	e.adoptQueuedFeedbackTurn(state, queuedMessage{platform: p, msgSessionKey: key, userID: "carol", feedbackText: "carol queued request"}, session)
	e.recordFeedbackFailure(state, "prompt_send", errors.New("queue transport failed"))
	carol, err := e.buildFeedbackDraft(key, "carol", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := approvedFeedbackBytes(t, carol); !strings.Contains(got, "carol queued request") || strings.Contains(got, "alice private") || strings.Contains(got, "bob request") {
		t.Fatalf("queue ownership lost: %s", got)
	}
}

func TestFeedbackUnownedHistoryAndUnrelatedFailureAreNotAttached(t *testing.T) {
	e, p := newFeedbackTestEngine(t)
	msg := feedbackTestMsg()
	session := e.sessions.GetOrCreateActive(msg.SessionKey)
	session.AddHistory("user", "someone else's original request")
	draft, err := e.buildFeedbackDraft(msg.SessionKey, msg.UserID, "new feature", nil)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Report().Diagnostic != nil {
		t.Fatal("unowned history was trusted")
	}
	msg.Content = "failed operation"
	turn := e.beginFeedbackTurn(p, msg, session, e.agent)
	e.recordFeedbackFailure(&interactiveState{feedbackTurn: turn}, "agent_error", errors.New("old failure"))
	draft, err = e.buildFeedbackDraft(msg.SessionKey, msg.UserID, "please add a theme", nil)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Report().Diagnostic != nil || draft.Report().RecentError != nil {
		t.Fatal("unrelated feature request inherited old failure")
	}
	foreign, err := e.buildFeedbackDraft(msg.SessionKey, "someone-else", "my feedback", nil)
	if err != nil {
		t.Fatal(err)
	}
	if foreign.Report().Diagnostic != nil {
		t.Fatal("direct feedback crossed owner boundary")
	}
}

func TestFeedbackActiveLookupStaysBoundedAndRecoversEvictedFailure(t *testing.T) {
	e, p := newFeedbackTestEngine(t)
	var first *feedbackTurn
	for i := range feedbackTurnLimit + 10 {
		key := fmt.Sprintf("test:%d", i)
		session := e.sessions.GetOrCreateActive(key)
		turn := e.beginFeedbackTurn(p, &Message{SessionKey: key, UserID: "owner", Content: "request"}, session, e.agent)
		if first == nil {
			first = turn
		}
	}
	if len(e.feedbackTurns) != feedbackTurnLimit {
		t.Fatalf("unbounded active lookup: %d", len(e.feedbackTurns))
	}
	e.recordFeedbackFailure(&interactiveState{feedbackTurn: first}, "agent_error", errors.New("late failure"))
	// A freshly frozen incident takes precedence over the oldest still-active
	// lookup; its state pointer kept the original request throughout eviction.
	if first.diagnostic.Request != "request" || first.diagnostic.Error != "late failure" {
		t.Fatal("evicted active state lost context")
	}
	if e.feedbackTurns[first.key] != first {
		t.Fatal("latest failed incident was not retained")
	}
}
