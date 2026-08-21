package core

import (
	"strings"
	"testing"
	"time"
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
		t.Fatal("submission must happen on the first command")
	}
	if posted.Schema != 1 || posted.InstallID != "install-test" {
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

func TestCmdFeedback_ReservedWordsWithNoContextShowUsage(t *testing.T) {
	// error/config/confirm/cancel are earlier-iteration spellings, not
	// descriptions; with nothing to attach they must not file anything.
	e, plat := newFeedbackTestEngine(t)
	submitted := false
	e.feedbackPostFn = func(string, FeedbackSubmission) (string, error) {
		submitted = true
		return "https://example/1", nil
	}
	for _, w := range []string{"", "error", "config", "confirm", "cancel"} {
		e.cmdFeedback(plat, feedbackTestMsg(), w)
	}
	if submitted {
		t.Fatal("reserved words with no attachable context must not submit")
	}
	if sent := plat.sentTexts(); len(sent) != 5 || !strings.Contains(sent[0], "/feedback <") {
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

func TestCmdFeedback_BareSubmitsGapKeys(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles", "feedbak.enabled"})
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(_ string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		return "https://example/2", nil
	}

	e.cmdFeedback(plat, feedbackTestMsg(), "")
	if posted == nil {
		t.Fatal("bare /feedback with gap keys must submit")
	}
	if !strings.Contains(posted.Title, "unsupported config") || !strings.Contains(posted.Body, "display.sparkles") || !strings.Contains(posted.Body, "feedbak.enabled") {
		t.Errorf("submission must carry the keys: title=%q body=%q", posted.Title, posted.Body)
	}
	_ = plat
}

func TestNotifyCapabilityGap_DeliversToRecentSession(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	touchSession(e, "feishu:oc_chat:ou_user")

	if !e.NotifyCapabilityGap([]string{"display.sparkles"}) {
		t.Fatal("expected delivery")
	}
	sent := plat.sentTexts()
	if len(sent) != 1 || !strings.Contains(sent[0], "display.sparkles") || !strings.Contains(sent[0], "/feedback") {
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

func TestCmdFeedback_AttachesRecentErrorAndGaps(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles"})
	e.recordFeedbackError("feishu:oc_chat:ou_user", "codex app-server turn/start: boom")
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(_ string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		return "https://example/3", nil
	}

	// One report carries the description plus every piece of context on hand.
	e.cmdFeedback(plat, feedbackTestMsg(), "sending fails")
	if posted == nil {
		t.Fatal("expected submission")
	}
	if !strings.Contains(posted.Title, "[feedback] sending fails") {
		t.Errorf("title must come from the description, got %q", posted.Title)
	}
	for _, want := range []string{"sending fails", "turn/start: boom", "display.sparkles"} {
		if !strings.Contains(posted.Body, want) {
			t.Errorf("body missing %q: %q", want, posted.Body)
		}
	}
}

func TestCmdFeedback_BareSubmitsRecentError(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.recordFeedbackError("feishu:oc_chat:ou_user", "codex app-server turn/start: boom")
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(_ string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		return "https://example/4", nil
	}

	e.cmdFeedback(plat, feedbackTestMsg(), "error")
	if posted == nil {
		t.Fatal("legacy /feedback error spelling must still report the recent error")
	}
	if !strings.Contains(posted.Title, "[feedback] error: codex app-server turn/start: boom") {
		t.Errorf("title = %q", posted.Title)
	}
}

func TestCmdFeedback_StaleErrorIsNotAttached(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.recordFeedbackError("feishu:oc_chat:ou_user", "ancient failure")
	e.feedbackMu.Lock()
	e.feedbackErrors["feishu:oc_chat:ou_user"].At = time.Now().Add(-time.Hour)
	e.feedbackMu.Unlock()
	var posted *FeedbackSubmission
	e.feedbackPostFn = func(_ string, sub FeedbackSubmission) (string, error) {
		posted = &sub
		return "https://example/5", nil
	}

	e.cmdFeedback(plat, feedbackTestMsg(), "unrelated wish")
	if posted == nil {
		t.Fatal("expected submission")
	}
	if strings.Contains(posted.Body, "ancient failure") {
		t.Errorf("stale error must not be attached: %q", posted.Body)
	}
}

func TestFeedbackErrorHint_ThrottledPerSession(t *testing.T) {
	e, plat := newFeedbackTestEngine(t)
	e.maybeSendFeedbackErrorHint(plat, "rctx", "feishu:oc_chat:ou_user")
	e.maybeSendFeedbackErrorHint(plat, "rctx", "feishu:oc_chat:ou_user")
	if sent := plat.sentTexts(); len(sent) != 1 || !strings.Contains(sent[0], "/feedback") {
		t.Fatalf("hint must be sent exactly once within the cooldown, got %v", sent)
	}
	// A different session has its own budget.
	e.maybeSendFeedbackErrorHint(plat, "rctx2", "feishu:oc_other:ou_user")
	if sent := plat.sentTexts(); len(sent) != 2 {
		t.Fatalf("second session must get its own hint, got %v", sent)
	}
}

func newFeedbackCardEngine(t *testing.T) (*Engine, *stubCardPlatform) {
	t.Helper()
	plat := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{plat}, "", LangEnglish)
	e.SetFeedbackConfig(true, "https://relay.example/v1/feedback", "install-test")
	return e, plat
}

func feedbackAskButtons(t *testing.T, card *Card) []CardButton {
	t.Helper()
	for _, el := range card.Elements {
		if actions, ok := el.(CardActions); ok {
			return actions.Buttons
		}
	}
	t.Fatalf("card has no button row: %#v", card)
	return nil
}

// The ask replaces the typed-command hint on card platforms: summarized
// problem + agree/ignore buttons, no command for the user to learn.
func TestFeedbackErrorAsk_CardCarriesSummaryAndButtons(t *testing.T) {
	e, plat := newFeedbackCardEngine(t)
	e.recordFeedbackError("feishu:oc_chat:ou_user", "codex app-server turn/start: boom")

	e.maybeSendFeedbackErrorHint(plat, "rctx", "feishu:oc_chat:ou_user")

	plat.mu.Lock()
	cards := append([]*Card(nil), plat.sentCards...)
	plat.mu.Unlock()
	if len(cards) != 1 {
		t.Fatalf("expected one ask card, got %d", len(cards))
	}
	var body string
	for _, el := range cards[0].Elements {
		if md, ok := el.(CardMarkdown); ok {
			body += md.Content
		}
	}
	if !strings.Contains(body, "turn/start: boom") {
		t.Errorf("ask card must summarize the problem, got %q", body)
	}
	buttons := feedbackAskButtons(t, cards[0])
	if len(buttons) != 2 || buttons[0].Value != "act:/feedback submit" || buttons[1].Value != "act:/feedback dismiss" {
		t.Errorf("expected submit/dismiss buttons, got %#v", buttons)
	}
}

func TestFeedbackErrorAsk_TextFallbackWithoutCards(t *testing.T) {
	e, plat := newFeedbackTestEngine(t) // restartNotifyStub: no CardSender
	e.recordFeedbackError("feishu:oc_chat:ou_user", "boom")
	e.maybeSendFeedbackErrorHint(plat, "rctx", "feishu:oc_chat:ou_user")
	if sent := plat.sentTexts(); len(sent) != 1 || !strings.Contains(sent[0], "/feedback") {
		t.Fatalf("non-card platforms keep the /feedback text pointer, got %v", sent)
	}
}

func TestNotifyCapabilityGap_CardAskOnCardPlatforms(t *testing.T) {
	e, plat := newFeedbackCardEngine(t)
	touchSession(e, "feishu:oc_chat:ou_user")
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles"})

	if !e.NotifyCapabilityGap([]string{"display.sparkles"}) {
		t.Fatal("expected delivery")
	}
	plat.mu.Lock()
	cards := append([]*Card(nil), plat.sentCards...)
	plat.mu.Unlock()
	if len(cards) != 1 {
		t.Fatalf("expected the gap ask as a card, got %d cards and texts %v", len(cards), plat.getSent())
	}
}

func TestHandleFeedbackCardAction_SubmitFilesReport(t *testing.T) {
	e, _ := newFeedbackCardEngine(t)
	e.SetFeedbackCapabilityGaps([]string{"display.sparkles"})
	posted := make(chan FeedbackSubmission, 1)
	e.feedbackPostFn = func(_ string, sub FeedbackSubmission) (string, error) {
		posted <- sub
		return "https://example/9", nil
	}

	e.handleFeedbackCardAction("submit", "feishu:oc_chat:ou_user")
	select {
	case sub := <-posted:
		if !strings.Contains(sub.Body, "display.sparkles") {
			t.Errorf("submission must carry the gap context, got %q", sub.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("button agreement must file the report")
	}

	// dismiss must never submit.
	e.handleFeedbackCardAction("dismiss", "feishu:oc_chat:ou_user")
	select {
	case <-posted:
		t.Fatal("dismiss must not submit")
	case <-time.After(100 * time.Millisecond):
	}
}
