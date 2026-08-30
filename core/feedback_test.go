package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

func newFeedbackTestEngine(t *testing.T) (*Engine, *restartNotifyStub) {
	t.Helper()
	platform := &restartNotifyStub{name: "feishu"}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	engine.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
	return engine, platform
}

func feedbackTestMsg() *Message {
	return &Message{SessionKey: "feishu:oc_chat:ou_user", Platform: "feishu", UserID: "ou_user"}
}

func captureFeedbackSubmissions(engine *Engine) <-chan appfeatures.FeedbackReport {
	submitted := make(chan appfeatures.FeedbackReport, 8)
	engine.feedbackSubmitFn = func(_ context.Context, draft appfeatures.FeedbackDraft, approved bool) (appfeatures.FeedbackReceipt, error) {
		if !approved {
			panic("core submitted without explicit approval")
		}
		submitted <- draft.Report()
		return appfeatures.FeedbackReceipt{ReferenceURL: "https://github.com/timmyagentic/cc-connect-next/issues/99"}, nil
	}
	return submitted
}

func TestRedactFeedbackText_ScrubsSecretsIDsAndPaths(t *testing.T) {
	input := `my app_secret = sk1234567890abcdef long token: abcdefghijklmnopqrstuvwxyz012345
user ou_1234567890abcdef in chat oc_fedcba0987654321 config at /Users/someone/project/config.toml`
	output := redactFeedbackText(input)

	for _, leaked := range []string{"sk1234567890abcdef", "abcdefghijklmnopqrstuvwxyz012345", "ou_1234567890abcdef", "oc_fedcba0987654321", "/Users/someone"} {
		if strings.Contains(output, leaked) {
			t.Errorf("redacted text still contains %q: %q", leaked, output)
		}
	}
	if !strings.Contains(output, "[REDACTED") {
		t.Errorf("expected redaction markers, got %q", output)
	}
}

func TestCmdFeedback_PreviewsBeforeExplicitApproval(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "I need per-project webhook retries")
	select {
	case <-submitted:
		t.Fatal("building and rendering a preview must not submit")
	default:
	}
	preview := strings.Join(platform.sentTexts(), "\n")
	for _, want := range []string{"I need per-project webhook retries", "cc-connect-next", "OS/Arch", "Agent"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview missing %q: %s", want, preview)
		}
	}

	engine.cmdFeedback(platform, feedbackTestMsg(), "confirm")
	select {
	case report := <-submitted:
		if report.Description != "I need per-project webhook retries" {
			t.Fatalf("submitted description = %q", report.Description)
		}
	case <-time.After(time.Second):
		t.Fatal("a separate confirm action must submit the exact pending preview")
	}
	if sent := platform.sentTexts(); !strings.Contains(sent[len(sent)-1], "issues/99") {
		t.Fatalf("success reply missing reference URL: %v", sent)
	}
}

func TestCmdFeedback_ConfirmUsesExactPendingDraft(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.SetFeedbackCapabilityGaps([]string{"first.gap"})
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "one problem")
	engine.SetFeedbackCapabilityGaps([]string{"later.gap"})
	engine.recordFeedbackError(feedbackTestMsg().SessionKey, "later error")
	engine.cmdFeedback(platform, feedbackTestMsg(), "confirm")

	report := <-submitted
	if len(report.CapabilityGaps) != 1 || report.CapabilityGaps[0] != "first.gap" || report.RecentError != nil {
		t.Fatalf("submission drifted from the rendered draft: %#v", report)
	}
}

func TestCmdFeedback_ControlWordsWithoutContextNeverSubmit(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	for _, word := range []string{"", "error", "config", "confirm", "cancel"} {
		engine.cmdFeedback(platform, feedbackTestMsg(), word)
	}
	select {
	case <-submitted:
		t.Fatal("control words without a pending/reportable draft must not submit")
	default:
	}
	sent := strings.Join(platform.sentTexts(), "\n")
	for _, want := range []string{"/feedback <", "no current feedback preview", "Nothing was submitted"} {
		if !strings.Contains(sent, want) {
			t.Fatalf("control-word guidance missing %q: %s", want, sent)
		}
	}
}

func TestCmdFeedback_DisabledChannel(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.SetFeedbackConfig(false, "https://relay.example/v1/feedback")
	engine.cmdFeedback(platform, feedbackTestMsg(), "anything")
	if sent := platform.sentTexts(); len(sent) != 1 || !strings.Contains(sent[0], "disabled") {
		t.Fatalf("expected disabled reply, got %v", sent)
	}
}

func TestCmdFeedback_BarePreviewsGapKeysThenConfirms(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.SetFeedbackCapabilityGaps([]string{"display.sparkles", "feedbak.enabled"})
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "")
	select {
	case <-submitted:
		t.Fatal("bare /feedback must preview gaps before submission")
	default:
	}
	preview := strings.Join(platform.sentTexts(), "\n")
	for _, want := range []string{"display.sparkles", "feedbak.enabled"} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview missing %q: %s", want, preview)
		}
	}
	engine.cmdFeedback(platform, feedbackTestMsg(), "confirm")
	report := <-submitted
	if len(report.CapabilityGaps) != 2 {
		t.Fatalf("submitted gaps = %v", report.CapabilityGaps)
	}
}

func TestNotifyCapabilityGap_DeliversCompletePreviewToRecentSession(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	touchSession(engine, "feishu:oc_chat:ou_user")

	if !engine.NotifyCapabilityGap([]string{"display.sparkles"}) {
		t.Fatal("expected delivery")
	}
	sent := strings.Join(platform.sentTexts(), "\n")
	for _, want := range []string{"display.sparkles", "cc-connect-next", "/feedback confirm"} {
		if !strings.Contains(sent, want) {
			t.Fatalf("gap preview missing %q: %s", want, sent)
		}
	}
}

func TestFeedbackNotifier_OncePerFingerprintAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()
	engine, platform := newFeedbackTestEngine(t)
	engine.SetFeedbackCapabilityGaps([]string{"display.sparkles"})
	touchSession(engine, "feishu:oc_chat:ou_user")

	notifier := NewFeedbackNotifier(dataDir)
	notifier.RegisterEngine("demo", engine)
	notifier.CheckOnce()
	notifier.CheckOnce()
	if got := platform.sentTexts(); len(got) != 1 {
		t.Fatalf("notice must be delivered exactly once, got %v", got)
	}

	next := NewFeedbackNotifier(dataDir)
	next.RegisterEngine("demo", engine)
	next.CheckOnce()
	if got := platform.sentTexts(); len(got) != 1 {
		t.Fatalf("restart must not re-announce the same keys, got %v", got)
	}

	engine.SetFeedbackCapabilityGaps([]string{"display.sparkles", "another.key"})
	next.CheckOnce()
	if got := platform.sentTexts(); len(got) != 2 {
		t.Fatalf("changed key set must be announced, got %v", got)
	}
}

func TestFeedbackNotifier_RetriesUntilSessionExists(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.SetFeedbackCapabilityGaps([]string{"display.sparkles"})

	notifier := NewFeedbackNotifier(t.TempDir())
	notifier.RegisterEngine("demo", engine)
	notifier.CheckOnce()
	if got := platform.sentTexts(); len(got) != 0 {
		t.Fatalf("no session yet, nothing must be sent, got %v", got)
	}

	touchSession(engine, "feishu:oc_chat:ou_user")
	notifier.CheckOnce()
	if got := platform.sentTexts(); len(got) != 1 {
		t.Fatalf("notice must be delivered once a session exists, got %v", got)
	}
}

func TestCmdFeedback_PreviewAndSubmissionAttachRecentErrorAndGaps(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.SetFeedbackCapabilityGaps([]string{"display.sparkles"})
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "codex app-server turn/start: boom")
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "sending fails")
	preview := strings.Join(platform.sentTexts(), "\n")
	for _, want := range []string{"sending fails", "turn/start: boom", "display.sparkles"} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview missing %q: %s", want, preview)
		}
	}
	engine.cmdFeedback(platform, feedbackTestMsg(), "confirm")
	report := <-submitted
	if report.Description != "sending fails" || report.RecentError == nil || len(report.CapabilityGaps) != 1 {
		t.Fatalf("submitted report = %#v", report)
	}
}

func TestCmdFeedback_StaleErrorIsNotAttached(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "ancient failure")
	engine.feedbackMu.Lock()
	engine.feedbackErrors["feishu:oc_chat:ou_user"].At = time.Now().Add(-time.Hour)
	engine.feedbackMu.Unlock()

	engine.cmdFeedback(platform, feedbackTestMsg(), "unrelated wish")
	if preview := strings.Join(platform.sentTexts(), "\n"); strings.Contains(preview, "ancient failure") {
		t.Fatalf("stale error must not be previewed: %s", preview)
	}
}

func TestFeedbackErrorPreview_ThrottledPerSession(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "boom")
	engine.maybeSendFeedbackErrorHint(platform, "rctx", "feishu:oc_chat:ou_user")
	engine.maybeSendFeedbackErrorHint(platform, "rctx", "feishu:oc_chat:ou_user")
	if sent := platform.sentTexts(); len(sent) != 1 || !strings.Contains(sent[0], "boom") {
		t.Fatalf("preview must be sent exactly once within the cooldown, got %v", sent)
	}

	engine.recordFeedbackError("feishu:oc_other:ou_user", "other boom")
	engine.maybeSendFeedbackErrorHint(platform, "rctx2", "feishu:oc_other:ou_user")
	if sent := platform.sentTexts(); len(sent) != 2 {
		t.Fatalf("second session must get its own preview, got %v", sent)
	}
}

func newFeedbackCardEngine(t *testing.T) (*Engine, *stubCardPlatform) {
	t.Helper()
	platform := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	engine.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
	return engine, platform
}

func feedbackAskButtons(t *testing.T, card *Card) []CardButton {
	t.Helper()
	for _, element := range card.Elements {
		if actions, ok := element.(CardActions); ok {
			return actions.Buttons
		}
	}
	t.Fatalf("card has no button row: %#v", card)
	return nil
}

func TestFeedbackErrorPreview_CardCarriesEveryFieldAndButtons(t *testing.T) {
	engine, platform := newFeedbackCardEngine(t)
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "codex app-server turn/start: boom")
	engine.maybeSendFeedbackErrorHint(platform, "rctx", "feishu:oc_chat:ou_user")

	platform.mu.Lock()
	cards := append([]*Card(nil), platform.sentCards...)
	platform.mu.Unlock()
	if len(cards) != 1 {
		t.Fatalf("expected one preview card, got %d", len(cards))
	}
	body := cardMarkdown(cards[0])
	for _, want := range []string{"turn/start: boom", "Description", "Recent error", "Capability gaps", "Environment", "cc-connect-next"} {
		if !strings.Contains(body, want) {
			t.Errorf("preview card missing %q: %s", want, body)
		}
	}
	buttons := feedbackAskButtons(t, cards[0])
	if len(buttons) != 2 || !strings.HasPrefix(buttons[0].Value, "cmd:/feedback confirm ") || !strings.HasPrefix(buttons[1].Value, "cmd:/feedback cancel ") || strings.TrimPrefix(buttons[0].Value, "cmd:/feedback confirm ") != strings.TrimPrefix(buttons[1].Value, "cmd:/feedback cancel ") {
		t.Errorf("expected submit/dismiss buttons, got %#v", buttons)
	}
}

func TestFeedbackErrorPreview_TextFallbackCarriesCompleteReport(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "boom")
	engine.maybeSendFeedbackErrorHint(platform, "rctx", "feishu:oc_chat:ou_user")
	if sent := strings.Join(platform.sentTexts(), "\n"); !strings.Contains(sent, "boom") || !strings.Contains(sent, "/feedback confirm") {
		t.Fatalf("text platform must receive the complete preview and approval route, got %s", sent)
	}
}

func TestNotifyCapabilityGap_CardPreviewOnCardPlatforms(t *testing.T) {
	engine, platform := newFeedbackCardEngine(t)
	touchSession(engine, "feishu:oc_chat:ou_user")
	engine.SetFeedbackCapabilityGaps([]string{"display.sparkles"})

	if !engine.NotifyCapabilityGap([]string{"display.sparkles"}) {
		t.Fatal("expected delivery")
	}
	platform.mu.Lock()
	cards := append([]*Card(nil), platform.sentCards...)
	platform.mu.Unlock()
	if len(cards) != 1 || !strings.Contains(cardMarkdown(cards[0]), "display.sparkles") {
		t.Fatalf("expected the complete gap preview as a card, got %#v", cards)
	}
}

func TestFeedbackTokenCommandsSubmitOnlyStoredPreview(t *testing.T) {
	engine, platform := newFeedbackCardEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	draft, err := engine.buildFeedbackDraft("feishu:oc_chat:ou_user", "", []string{"display.sparkles"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := engine.rememberPendingFeedback("feishu:oc_chat:ou_user", "ou_user", draft)
	if err != nil {
		t.Fatal(err)
	}

	engine.cmdFeedback(platform, feedbackTestMsg(), "confirm "+token)
	select {
	case report := <-submitted:
		if len(report.CapabilityGaps) != 1 || report.CapabilityGaps[0] != "display.sparkles" {
			t.Fatalf("submission must carry the previewed gap: %#v", report)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("button agreement must submit the pending preview")
	}

	token, err = engine.rememberPendingFeedback("feishu:oc_chat:ou_user", "ou_user", draft)
	if err != nil {
		t.Fatal(err)
	}
	engine.cmdFeedback(platform, feedbackTestMsg(), "cancel "+token)
	engine.cmdFeedback(platform, feedbackTestMsg(), "confirm "+token)
	select {
	case <-submitted:
		t.Fatal("dismissed preview must never submit")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestFeedbackCardApprovalBindsExactDraftAndInitiatingUser(t *testing.T) {
	engine, platform := newFeedbackCardEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	message := feedbackTestMsg()

	engine.cmdFeedback(platform, message, "first problem")
	engine.cmdFeedback(platform, message, "second problem")
	platform.mu.Lock()
	firstCard := platform.repliedCards[0]
	secondCard := platform.repliedCards[1]
	platform.mu.Unlock()
	firstAction := feedbackAskButtons(t, firstCard)[0].Value
	secondAction := feedbackAskButtons(t, secondCard)[0].Value
	if firstAction == secondAction {
		t.Fatalf("two previews share one approval action: %q", firstAction)
	}

	engine.handleCommand(platform, message, strings.TrimPrefix(firstAction, "cmd:"))
	select {
	case report := <-submitted:
		if report.Description != "first problem" {
			t.Fatalf("old card submitted replacement Draft: %#v", report)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first preview approval was not processed")
	}

	engine.cmdFeedback(platform, message, "owner-only problem")
	platform.mu.Lock()
	ownerAction := feedbackAskButtons(t, platform.repliedCards[len(platform.repliedCards)-1])[0].Value
	platform.mu.Unlock()
	other := *message
	other.UserID = "ou_other"
	engine.handleCommand(platform, &other, strings.TrimPrefix(ownerAction, "cmd:"))
	select {
	case report := <-submitted:
		t.Fatalf("another group user submitted the owner's Draft: %#v", report)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestFeedbackTypedConfirmationRejectsAmbiguousPreviews(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	engine.cmdFeedback(platform, feedbackTestMsg(), "first problem")
	engine.cmdFeedback(platform, feedbackTestMsg(), "second problem")
	engine.cmdFeedback(platform, feedbackTestMsg(), "confirm")
	select {
	case report := <-submitted:
		t.Fatalf("ambiguous typed confirmation submitted a Draft: %#v", report)
	default:
	}
}

func TestCmdFeedback_ExpiredPreviewCannotSubmit(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	engine.cmdFeedback(platform, feedbackTestMsg(), "one problem")
	engine.feedbackMu.Lock()
	var token string
	var pending pendingFeedback
	for candidate, value := range engine.feedbackPending {
		token, pending = candidate, value
		break
	}
	pending.At = time.Now().Add(-feedbackPendingTTL - time.Second)
	engine.feedbackPending[token] = pending
	engine.feedbackMu.Unlock()

	engine.cmdFeedback(platform, feedbackTestMsg(), "confirm")
	select {
	case <-submitted:
		t.Fatal("expired preview must never submit")
	default:
	}
}
