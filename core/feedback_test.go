package core

import (
	"strings"
	"testing"
)

func newFeedbackTestEngine(t *testing.T) (*Engine, *restartNotifyStub) {
	t.Helper()
	plat := &restartNotifyStub{name: "feishu"}
	e := NewEngine("test", &stubAgent{}, []Platform{plat}, "", LangEnglish)
	e.SetFeedbackConfig(true, "https://relay.example/v1/feedback", "install-test")
	return e, plat
}

func feedbackTestMsg() *Message {
	return &Message{SessionKey: "feishu:oc_chat:ou_user", Platform: "feishu", UserID: "ou_user"}
}

func TestRedactFeedbackText_ScrubsSecretsIDsAndPaths(t *testing.T) {
	in := `my app_secret = sk1234567890abcdef long token: abcdefghijklmnopqrstuvwxyz012345
user ou_1234567890abcdef in chat oc_fedcba0987654321 config at /Users/someone/project/config.toml`
	out := redactFeedbackText(in)

	for _, leaked := range []string{"sk1234567890abcdef", "abcdefghijklmnopqrstuvwxyz012345", "ou_1234567890abcdef", "oc_fedcba0987654321", "/Users/someone"} {
		if strings.Contains(out, leaked) {
			t.Errorf("redacted text still contains %q: %q", leaked, out)
		}
	}
	if !strings.Contains(out, "[REDACTED") {
		t.Errorf("expected redaction markers, got %q", out)
	}
}

func TestCmdFeedback_PreviewThenConfirmSubmits(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(endpoint string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		if endpoint != "https://relay.example/v1/feedback" {
			t.Errorf("endpoint = %q", endpoint)
		}
		return "https://github.com/timmyagentic/cc-connect-next/issues/99", nil
	}

	e.cmdFeedback(plat, feedbackTestMsg(), "I need per-project webhook retries")
	sent := plat.sentTexts()
	if len(sent) != 1 || !strings.Contains(sent[0], "I need per-project webhook retries") {
		t.Fatalf("expected preview containing the description, got %v", sent)
	}
	if !strings.Contains(sent[0], "[feedback] I need per-project webhook retries") {
		t.Fatalf("preview must show the derived title, got %q", sent[0])
	}

	e.cmdFeedback(plat, feedbackTestMsg(), "confirm")
	sent = plat.sentTexts()
	if posted == nil {
		t.Fatal("confirm must POST the draft")
	}
	if posted.Schema != 1 || posted.Trigger != "user" || posted.InstallID != "install-test" {
		t.Errorf("submission envelope wrong: %+v", posted)
	}
	if !strings.Contains(posted.Body, "Environment (auto-generated)") {
		t.Errorf("body missing environment section: %q", posted.Body)
	}
	if !strings.Contains(sent[len(sent)-1], "issues/99") {
		t.Errorf("success reply must contain the issue URL, got %q", sent[len(sent)-1])
	}

	// The draft is consumed: a second confirm has nothing to submit.
	posted = nil
	e.cmdFeedback(plat, feedbackTestMsg(), "confirm")
	if posted != nil {
		t.Error("second confirm must not re-submit")
	}
}

func TestCmdFeedback_ConfirmWithoutDraft(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.cmdFeedback(plat, feedbackTestMsg(), "confirm")
	if sent := plat.sentTexts(); len(sent) != 1 || !strings.Contains(sent[0], "/feedback <") {
		t.Fatalf("expected no-draft reply, got %v", sent)
	}
}

func TestCmdFeedback_CancelDiscardsDraft(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	submitted := false
	e.feedbackPostFn = func(string, FeedbackSubmission) (string, error) {
		submitted = true
		return "https://example/1", nil
	}
	e.cmdFeedback(plat, feedbackTestMsg(), "something")
	e.cmdFeedback(plat, feedbackTestMsg(), "cancel")
	e.cmdFeedback(plat, feedbackTestMsg(), "confirm")
	if submitted {
		t.Error("cancelled draft must never be submitted")
	}
}

func TestCmdFeedback_DisabledChannel(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.SetFeedbackConfig(false, "https://relay.example", "id")
	e.cmdFeedback(plat, feedbackTestMsg(), "anything")
	if sent := plat.sentTexts(); len(sent) != 1 || !strings.Contains(sent[0], "disabled") {
		t.Fatalf("expected disabled reply, got %v", sent)
	}
}

func TestCmdFeedback_ConfigDraftFromGapKeys(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles", "feedbak.enabled"})
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(_ string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		return "https://example/2", nil
	}

	e.cmdFeedback(plat, feedbackTestMsg(), "config")
	sent := plat.sentTexts()
	if len(sent) != 1 || !strings.Contains(sent[0], "display.sparkles") || !strings.Contains(sent[0], "feedbak.enabled") {
		t.Fatalf("config preview must list the unsupported keys, got %v", sent)
	}

	e.cmdFeedback(plat, feedbackTestMsg(), "confirm")
	if posted == nil || posted.Trigger != "config_keys" {
		t.Fatalf("expected config_keys submission, got %+v", posted)
	}
	if !strings.Contains(posted.Title, "display.sparkles") {
		t.Errorf("title must name the keys, got %q", posted.Title)
	}
}

func TestNotifyCapabilityGap_DeliversToRecentSession(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	touchSession(e, "feishu:oc_chat:ou_user")

	if !e.NotifyCapabilityGap([]string{"display.sparkles"}) {
		t.Fatal("expected delivery")
	}
	sent := plat.sentTexts()
	if len(sent) != 1 || !strings.Contains(sent[0], "display.sparkles") || !strings.Contains(sent[0], "/feedback config") {
		t.Fatalf("gap notice must name the key and the reporting command, got %v", sent)
	}
}

func TestFeedbackNotifier_OncePerFingerprintAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()
	e, plat := newFeedbackTestEngine(t)
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles"})
	touchSession(e, "feishu:oc_chat:ou_user")

	n := NewFeedbackNotifier(dataDir)
	n.RegisterEngine("demo", e)
	n.CheckOnce()
	n.CheckOnce()
	if got := plat.sentTexts(); len(got) != 1 {
		t.Fatalf("notice must be delivered exactly once, got %v", got)
	}

	// A fresh notifier over the same dataDir (daemon restart) must not
	// re-announce the same key set…
	n2 := NewFeedbackNotifier(dataDir)
	n2.RegisterEngine("demo", e)
	n2.CheckOnce()
	if got := plat.sentTexts(); len(got) != 1 {
		t.Fatalf("restart must not re-announce the same keys, got %v", got)
	}

	// …but a different key set is news.
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles", "another.key"})
	n2.CheckOnce()
	if got := plat.sentTexts(); len(got) != 2 {
		t.Fatalf("changed key set must be announced, got %v", got)
	}
}

func TestFeedbackNotifier_RetriesUntilSessionExists(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles"})

	n := NewFeedbackNotifier(t.TempDir())
	n.RegisterEngine("demo", e)
	n.CheckOnce()
	if got := plat.sentTexts(); len(got) != 0 {
		t.Fatalf("no session yet, nothing must be sent, got %v", got)
	}

	touchSession(e, "feishu:oc_chat:ou_user")
	n.CheckOnce()
	if got := plat.sentTexts(); len(got) != 1 {
		t.Fatalf("notice must be delivered once a session exists, got %v", got)
	}
}
