package core

import (
	"strings"
	"testing"
	"time"
)

func TestFeedbackProactiveOfferBindsInitiatingUserInSharedSession(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	const key = "feishu:shared-chat:root:topic"
	engine.recordFeedbackError(key, "the owner's diagnostic context")
	submitted := captureFeedbackSubmissions(engine)
	engine.maybeSendFeedbackErrorHint(platform, "reply", key, "owner-user")
	token := feedbackSubmitTokenFromText(t, strings.Join(platform.sentTexts(), "\n"))
	other := &Message{SessionKey: key, UserID: "other-user"}
	engine.cmdFeedback(platform, other, "submit-token "+token)
	select {
	case <-submitted:
		t.Fatal("another participant submitted the owner's prepared diagnostics")
	default:
	}
	owner := &Message{SessionKey: key, UserID: "owner-user"}
	engine.cmdFeedback(platform, owner, "submit-token "+token)
	select {
	case report := <-submitted:
		if report.RecentError == nil {
			t.Fatal("the owner's click lost the prepared diagnostic report")
		}
	default:
		t.Fatal("the owner's click did not submit")
	}
	engine.cmdFeedback(platform, owner, "submit-token "+token)
	select {
	case <-submitted:
		t.Fatal("prepared diagnostics were submitted twice")
	default:
	}
}

func TestFeedbackContextRejectsUnknownOrFutureHistoryTimestamps(t *testing.T) {
	for _, timestamp := range []time.Time{{}, time.Now().Add(time.Hour)} {
		engine, _ := newFeedbackTestEngine(t)
		key := feedbackTestMsg().SessionKey
		session := engine.sessions.GetOrCreateActive(key)
		session.AddHistory("user", "unverified age of private context")
		session.mu.Lock()
		session.History[0].Timestamp = timestamp
		session.mu.Unlock()
		draft, err := engine.buildFeedbackDraft(key, "explicit feedback", nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(draft.Report().Description, "unverified age") {
			t.Fatal("feedback attached history whose freshness was not established")
		}
	}
}

func TestFeedbackErrorOfferRequiresKnownInitiatingUser(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.recordFeedbackError("test:shared-topic", "private error")
	engine.maybeSendFeedbackErrorHint(platform, "reply", "test:shared-topic", "")
	if len(platform.sentTexts()) != 0 {
		t.Fatal("an ownerless error offer was sent")
	}
}
