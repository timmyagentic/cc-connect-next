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

func TestCmdFeedback_SubmitsImmediately(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(endpoint string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		if endpoint != "https://relay.example/v1/feedback" {
			t.Errorf("endpoint = %q", endpoint)
		}
		return "https://github.com/timmyagentic/cc-connect-next/issues/99", nil
	}

	// Invoking /feedback IS the consent: one command, immediate submission.
	e.cmdFeedback(plat, feedbackTestMsg(), "I need per-project webhook retries")
	if posted == nil {
		t.Fatal("submission must happen on the first command, without confirm")
	}
	if posted.Schema != 1 || posted.Trigger != "user" || posted.InstallID != "install-test" {
		t.Errorf("submission envelope wrong: %+v", posted)
	}
	if !strings.Contains(posted.Title, "[feedback] I need per-project webhook retries") {
		t.Errorf("title = %q", posted.Title)
	}
	if !strings.Contains(posted.Body, "Environment (auto-generated)") {
		t.Errorf("body missing environment section: %q", posted.Body)
	}
	sent := plat.sentTexts()
	if len(sent) != 1 || !strings.Contains(sent[0], "issues/99") {
		t.Fatalf("expected one success reply with the issue URL, got %v", sent)
	}
}

func TestCmdFeedback_ConfirmAndCancelShowUsageNotAnIssue(t *testing.T) {
	// confirm/cancel are leftovers of the removed two-step flow; they must
	// never be submitted as a literal report.
	e, plat := newFeedbackTestEngine(t)
	submitted := false
	e.feedbackPostFn = func(string, FeedbackSubmission) (string, error) {
		submitted = true
		return "https://example/1", nil
	}
	e.cmdFeedback(plat, feedbackTestMsg(), "confirm")
	e.cmdFeedback(plat, feedbackTestMsg(), "cancel")
	if submitted {
		t.Fatal("confirm/cancel must not submit anything")
	}
	if sent := plat.sentTexts(); len(sent) != 2 || !strings.Contains(sent[0], "/feedback <") {
		t.Fatalf("expected usage replies, got %v", sent)
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

func TestCmdFeedback_ConfigSubmitsGapKeys(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles", "feedbak.enabled"})
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(_ string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		return "https://example/2", nil
	}

	e.cmdFeedback(plat, feedbackTestMsg(), "config")
	if posted == nil || posted.Trigger != "config_keys" {
		t.Fatalf("expected immediate config_keys submission, got %+v", posted)
	}
	if !strings.Contains(posted.Title, "display.sparkles") || !strings.Contains(posted.Body, "feedbak.enabled") {
		t.Errorf("submission must name the keys: title=%q", posted.Title)
	}

	// With no gap keys there is nothing to submit.
	posted = nil
	e.SetFeedbackCapabilityGaps(nil)
	e.cmdFeedback(plat, feedbackTestMsg(), "config")
	if posted != nil {
		t.Error("config with no gap keys must not submit")
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

func TestCmdFeedback_ErrorSubmitsRecordedError(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(_ string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		return "https://example/3", nil
	}

	// No error recorded yet.
	e.cmdFeedback(plat, feedbackTestMsg(), "error")
	if posted != nil {
		t.Fatal("no recorded error must mean no submission")
	}
	if sent := plat.sentTexts(); len(sent) != 1 || !strings.Contains(sent[0], "/feedback <") {
		t.Fatalf("expected no-error reply, got %v", sent)
	}

	e.recordFeedbackError("feishu:oc_chat:ou_user", "codex app-server turn/start: boom")
	e.cmdFeedback(plat, feedbackTestMsg(), "error")
	if posted == nil || posted.Trigger != "error" {
		t.Fatalf("expected immediate error submission, got %+v", posted)
	}
	if !strings.Contains(posted.Title, "[feedback] error: codex app-server turn/start: boom") {
		t.Errorf("title = %q", posted.Title)
	}
	if !strings.Contains(posted.Body, "turn/start: boom") {
		t.Errorf("body must include the recorded error, got %q", posted.Body)
	}
}

func TestFeedbackErrorHint_ThrottledPerSession(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.maybeSendFeedbackErrorHint(plat, "rctx", "feishu:oc_chat:ou_user")
	e.maybeSendFeedbackErrorHint(plat, "rctx", "feishu:oc_chat:ou_user")
	if sent := plat.sentTexts(); len(sent) != 1 || !strings.Contains(sent[0], "/feedback error") {
		t.Fatalf("hint must be sent exactly once within the cooldown, got %v", sent)
	}
	// A different session has its own budget.
	e.maybeSendFeedbackErrorHint(plat, "rctx2", "feishu:oc_other:ou_user")
	if sent := plat.sentTexts(); len(sent) != 2 {
		t.Fatalf("second session must get its own hint, got %v", sent)
	}
}
