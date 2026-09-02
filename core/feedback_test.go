package core

import (
	"context"
	"errors"
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

func feedbackSubmitTokenFromText(t *testing.T, text string) string {
	t.Helper()
	const marker = "/feedback submit-token "
	index := strings.Index(text, marker)
	if index < 0 {
		t.Fatalf("feedback offer has no submit action: %s", text)
	}
	fields := strings.Fields(text[index+len(marker):])
	if len(fields) == 0 {
		t.Fatalf("feedback offer has an empty submit token: %s", text)
	}
	return strings.Trim(fields[0], "`.,")
}

func feedbackSubmitTokenFromButton(t *testing.T, button CardButton) string {
	t.Helper()
	const prefix = "cmd:/feedback submit-token "
	if !strings.HasPrefix(button.Value, prefix) {
		t.Fatalf("feedback button action = %q, want %q prefix", button.Value, prefix)
	}
	return strings.TrimSpace(strings.TrimPrefix(button.Value, prefix))
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

func TestCmdFeedback_ExplicitTriggerSubmitsDirectlyWithoutPreview(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "I need per-project webhook retries")
	select {
	case report := <-submitted:
		if report.Description != "I need per-project webhook retries" {
			t.Fatalf("submitted description = %q", report.Description)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("an explicit Feedback trigger did not submit immediately")
	}

	sent := strings.Join(platform.sentTexts(), "\n")
	if sent != "Submission succeeded" {
		t.Fatalf("visible result = %q, want only the link-free success status", sent)
	}
	if strings.Contains(strings.ToLower(sent), "preview") || strings.Contains(sent, "issues/") {
		t.Fatalf("direct Feedback exposed a preview or Issue link: %s", sent)
	}
}

func TestCmdFeedback_LegacyControlWordInDescriptionStillSubmits(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)

	for _, description := range []string{"confirm broken", "cancel flow is broken"} {
		engine.cmdFeedback(platform, feedbackTestMsg(), description)
		if report := <-submitted; report.Description != description {
			t.Fatalf("description %q submitted as %#v", description, report)
		}
	}
}

func TestFeedbackDirectSubmissionIncludesOnlyRecentRedactedAdjacentContext(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	key := feedbackTestMsg().SessionKey
	session := engine.sessions.GetOrCreateActive(key)
	session.AddHistory("user", "更新通道为什么把 beta 当 stable？ token=must-not-leak")
	session.AddHistory("assistant", "诊断：当前只有 LatestStable。日志在 /Users/private/project/run.log")
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "请反馈更新通道问题")
	report := <-submitted
	for _, want := range []string{"请反馈更新通道问题", "Related diagnostic context", "Previous user message", "Previous assistant response", "LatestStable", "[REDACTED"} {
		if !strings.Contains(report.Description, want) {
			t.Fatalf("submitted Draft missing %q: %s", want, report.Description)
		}
	}
	for _, leaked := range []string{"must-not-leak", "/Users/private"} {
		if strings.Contains(report.Description, leaked) {
			t.Fatalf("submitted Draft leaked %q: %s", leaked, report.Description)
		}
	}
	if sent := strings.Join(platform.sentTexts(), "\n"); sent != "Submission succeeded" {
		t.Fatalf("chat exposed Draft contents instead of only the result: %s", sent)
	}
}

func TestFeedbackContextNeverPairsAnUnansweredUserMessageWithAnOlderAssistant(t *testing.T) {
	engine, _ := newFeedbackTestEngine(t)
	key := feedbackTestMsg().SessionKey
	session := engine.sessions.GetOrCreateActive(key)
	session.AddHistory("user", "old question")
	session.AddHistory("assistant", "old unrelated diagnosis")
	session.AddHistory("user", "new unanswered observation")
	draft, err := engine.buildFeedbackDraft(key, "report it", nil)
	if err != nil {
		t.Fatal(err)
	}
	description := draft.Report().Description
	if !strings.Contains(description, "new unanswered observation") || strings.Contains(description, "old unrelated diagnosis") {
		t.Fatalf("mismatched adjacent context: %s", description)
	}
}

type feedbackResultCardPlatform struct {
	stubCardPlatform
	updatedCards []*Card
}

func (platform *feedbackResultCardPlatform) UpdateCard(_ context.Context, _ any, card *Card) error {
	platform.mu.Lock()
	defer platform.mu.Unlock()
	platform.updatedCards = append(platform.updatedCards, card)
	return nil
}

func TestFeedbackCardActionDirectlySubmitsAndUpdatesOnlyOriginalCard(t *testing.T) {
	for _, test := range []struct {
		name          string
		submitErr     error
		disableBefore bool
		wantTitle     string
		wantCalls     int
	}{
		{name: "success", wantTitle: "提交成功", wantCalls: 1},
		{name: "failure", submitErr: errors.New("relay unavailable"), wantTitle: "提交失败", wantCalls: 1},
		{name: "disabled", disableBefore: true, wantTitle: "提交失败"},
	} {
		t.Run(test.name, func(t *testing.T) {
			platform := &feedbackResultCardPlatform{stubCardPlatform: stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}}
			engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangChinese)
			engine.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
			calls := 0
			engine.feedbackSubmitFn = func(_ context.Context, _ appfeatures.FeedbackDraft, approved bool) (appfeatures.FeedbackReceipt, error) {
				calls++
				if !approved {
					t.Fatal("explicit card action was not treated as approval")
				}
				return appfeatures.FeedbackReceipt{ReferenceURL: "https://github.com/timmyagentic/cc-connect-next/issues/99"}, test.submitErr
			}

			message := feedbackTestMsg()
			message.ReplyCtx = "original-card"
			message.IsCardAction = true
			if test.disableBefore {
				engine.SetFeedbackConfig(false, "https://relay.example/v1/feedback")
			}
			engine.cmdFeedback(platform, message, "card result")

			platform.mu.Lock()
			updated := append([]*Card(nil), platform.updatedCards...)
			platform.mu.Unlock()
			if calls != test.wantCalls {
				t.Fatalf("Relay calls = %d, want %d", calls, test.wantCalls)
			}
			if len(updated) != 1 || updated[0].Header == nil || updated[0].Header.Title != test.wantTitle {
				t.Fatalf("updated cards = %#v, want one %q result", updated, test.wantTitle)
			}
			if len(updated[0].Elements) != 0 {
				t.Fatalf("result card has unexpected body/actions: %#v", updated[0].Elements)
			}
			if rendered := updated[0].RenderText(); strings.Contains(rendered, "http") || strings.Contains(rendered, "issues/") {
				t.Fatalf("result card leaked reference URL: %s", rendered)
			}
			if sent := platform.getSent(); len(sent) != 0 {
				t.Fatalf("card action sent an extra message: %v", sent)
			}
		})
	}
}

func TestCmdFeedback_NoReportableContextNeverSubmits(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	for _, value := range []string{"", "confirm", "cancel"} {
		engine.cmdFeedback(platform, feedbackTestMsg(), value)
	}
	select {
	case report := <-submitted:
		t.Fatalf("empty or legacy control submitted a Draft: %#v", report)
	default:
	}
	if sent := strings.Join(platform.sentTexts(), "\n"); !strings.Contains(sent, "/feedback <") || !strings.Contains(sent, "submits immediately") {
		t.Fatalf("direct-submit usage guidance missing: %s", sent)
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

func TestCmdFeedback_BareTriggerDirectlySubmitsCapabilityGaps(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.SetFeedbackCapabilityGaps([]string{"display.sparkles", "feedbak.enabled"})
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "")
	report := <-submitted
	if len(report.CapabilityGaps) != 2 {
		t.Fatalf("submitted gaps = %v", report.CapabilityGaps)
	}
	if sent := strings.Join(platform.sentTexts(), "\n"); sent != "Submission succeeded" {
		t.Fatalf("bare explicit trigger exposed more than the result: %s", sent)
	}
}

func TestNotifyCapabilityGap_OffersExactDraftWithZeroNetworkUntilUserActs(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	touchSession(engine, "feishu:oc_chat:ou_user")
	submitted := captureFeedbackSubmissions(engine)

	if !engine.NotifyCapabilityGap([]string{"display.sparkles"}) {
		t.Fatal("expected delivery")
	}
	select {
	case <-submitted:
		t.Fatal("automatic capability-gap offer made a Relay request")
	default:
	}
	offer := strings.Join(platform.sentTexts(), "\n")
	if strings.Contains(offer, "display.sparkles") || strings.Contains(strings.ToLower(offer), "preview") {
		t.Fatalf("automatic offer exposed a Draft preview: %s", offer)
	}
	token := feedbackSubmitTokenFromText(t, offer)
	engine.SetFeedbackCapabilityGaps([]string{"later.gap"})
	engine.cmdFeedback(platform, feedbackTestMsg(), "submit-token "+token)
	report := <-submitted
	if len(report.CapabilityGaps) != 1 || report.CapabilityGaps[0] != "display.sparkles" {
		t.Fatalf("offer did not submit its exact prepared Draft: %#v", report)
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

func TestCmdFeedback_DirectSubmissionAttachesRecentErrorAndGaps(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.SetFeedbackCapabilityGaps([]string{"display.sparkles"})
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "codex app-server turn/start: boom")
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "sending fails")
	report := <-submitted
	if report.Description != "sending fails" || report.RecentError == nil || report.RecentError.Text != "codex app-server turn/start: boom" || len(report.CapabilityGaps) != 1 {
		t.Fatalf("submitted report = %#v", report)
	}
}

func TestCmdFeedback_StaleErrorIsNotAttached(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "ancient failure")
	engine.feedbackMu.Lock()
	engine.feedbackErrors["feishu:oc_chat:ou_user"].At = time.Now().Add(-time.Hour)
	engine.feedbackMu.Unlock()
	submitted := captureFeedbackSubmissions(engine)

	engine.cmdFeedback(platform, feedbackTestMsg(), "unrelated wish")
	if report := <-submitted; report.RecentError != nil {
		t.Fatalf("stale error was attached: %#v", report.RecentError)
	}
}

func TestFeedbackErrorOffer_ThrottledPerSessionAndZeroNetwork(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "boom")
	engine.maybeSendFeedbackErrorHint(platform, "rctx", "feishu:oc_chat:ou_user")
	engine.maybeSendFeedbackErrorHint(platform, "rctx", "feishu:oc_chat:ou_user")
	if sent := platform.sentTexts(); len(sent) != 1 || strings.Contains(sent[0], "boom") || !strings.Contains(sent[0], "submit-token") {
		t.Fatalf("offer must be generic and sent once within the cooldown, got %v", sent)
	}
	select {
	case <-submitted:
		t.Fatal("automatic error offer submitted without a user action")
	default:
	}

	engine.recordFeedbackError("feishu:oc_other:ou_user", "other boom")
	engine.maybeSendFeedbackErrorHint(platform, "rctx2", "feishu:oc_other:ou_user")
	if sent := platform.sentTexts(); len(sent) != 2 {
		t.Fatalf("second session must get its own offer, got %v", sent)
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

func TestFeedbackErrorOffer_CardHasOneDirectSubmitActionAndNoPreview(t *testing.T) {
	engine, platform := newFeedbackCardEngine(t)
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "codex app-server turn/start: boom")
	submitted := captureFeedbackSubmissions(engine)
	engine.maybeSendFeedbackErrorHint(platform, "rctx", "feishu:oc_chat:ou_user")

	platform.mu.Lock()
	cards := append([]*Card(nil), platform.sentCards...)
	platform.mu.Unlock()
	if len(cards) != 1 {
		t.Fatalf("expected one feedback offer card, got %d", len(cards))
	}
	body := cardMarkdown(cards[0])
	for _, forbidden := range []string{"turn/start: boom", "Description", "Recent error", "Capability gaps", "Environment"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("offer card exposed Draft field %q: %s", forbidden, body)
		}
	}
	buttons := feedbackAskButtons(t, cards[0])
	if len(buttons) != 1 {
		t.Fatalf("offer buttons = %#v, want one direct submit action", buttons)
	}
	token := feedbackSubmitTokenFromButton(t, buttons[0])
	select {
	case <-submitted:
		t.Fatal("rendering the offer submitted without a click")
	default:
	}

	engine.cmdFeedback(platform, feedbackTestMsg(), "submit-token "+token)
	report := <-submitted
	if report.RecentError == nil || report.RecentError.Text != "codex app-server turn/start: boom" {
		t.Fatalf("clicked offer submitted the wrong Draft: %#v", report)
	}
}

func TestFeedbackOfferCardClickSubmitsOnceAndBecomesLinkFreeResult(t *testing.T) {
	platform := &feedbackResultCardPlatform{stubCardPlatform: stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangChinese)
	engine.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "one bounded failure")
	calls := 0
	engine.feedbackSubmitFn = func(_ context.Context, draft appfeatures.FeedbackDraft, approved bool) (appfeatures.FeedbackReceipt, error) {
		calls++
		if !approved || draft.Report().RecentError == nil {
			t.Fatalf("clicked offer did not approve its prepared Draft: %#v", draft.Report())
		}
		return appfeatures.FeedbackReceipt{ReferenceURL: "https://github.com/timmyagentic/cc-connect-next/issues/99"}, nil
	}
	engine.maybeSendFeedbackErrorHint(platform, "original-card", "feishu:oc_chat:ou_user")

	platform.mu.Lock()
	if len(platform.sentCards) != 1 {
		platform.mu.Unlock()
		t.Fatalf("feedback offers = %d, want 1", len(platform.sentCards))
	}
	buttons := feedbackAskButtons(t, platform.sentCards[0])
	platform.mu.Unlock()
	token := feedbackSubmitTokenFromButton(t, buttons[0])

	message := feedbackTestMsg()
	message.ReplyCtx = "original-card"
	message.IsCardAction = true
	engine.cmdFeedback(platform, message, "submit-token "+token)

	platform.mu.Lock()
	updated := append([]*Card(nil), platform.updatedCards...)
	platform.mu.Unlock()
	if calls != 1 || len(updated) != 1 || updated[0].Header == nil || updated[0].Header.Title != "提交成功" || len(updated[0].Elements) != 0 {
		t.Fatalf("click result: calls=%d cards=%#v", calls, updated)
	}
	if rendered := updated[0].RenderText(); strings.Contains(rendered, "http") || strings.Contains(rendered, "issues/") {
		t.Fatalf("clicked result card leaked the Relay reference: %s", rendered)
	}
	if sent := platform.getSent(); len(sent) != 0 {
		t.Fatalf("clicked offer sent an extra result message: %v", sent)
	}
}

func TestFeedbackErrorOffer_TextFallbackSubmitsPreparedDraftInOneCommand(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	engine.recordFeedbackError("feishu:oc_chat:ou_user", "boom")
	submitted := captureFeedbackSubmissions(engine)
	engine.maybeSendFeedbackErrorHint(platform, "rctx", "feishu:oc_chat:ou_user")
	offer := strings.Join(platform.sentTexts(), "\n")
	if strings.Contains(offer, "boom") || strings.Contains(strings.ToLower(offer), "preview") {
		t.Fatalf("text offer exposed a Draft preview: %s", offer)
	}
	token := feedbackSubmitTokenFromText(t, offer)
	engine.cmdFeedback(platform, feedbackTestMsg(), "submit-token "+token)
	if report := <-submitted; report.RecentError == nil || report.RecentError.Text != "boom" {
		t.Fatalf("text offer submitted the wrong Draft: %#v", report)
	}
}

func TestNotifyCapabilityGap_CardOfferSubmitsExactPreparedKeys(t *testing.T) {
	engine, platform := newFeedbackCardEngine(t)
	touchSession(engine, "feishu:oc_chat:ou_user")
	submitted := captureFeedbackSubmissions(engine)

	if !engine.NotifyCapabilityGap([]string{"display.sparkles"}) {
		t.Fatal("expected delivery")
	}
	platform.mu.Lock()
	cards := append([]*Card(nil), platform.sentCards...)
	platform.mu.Unlock()
	if len(cards) != 1 || strings.Contains(cardMarkdown(cards[0]), "display.sparkles") {
		t.Fatalf("expected a generic offer rather than a gap preview, got %#v", cards)
	}
	buttons := feedbackAskButtons(t, cards[0])
	token := feedbackSubmitTokenFromButton(t, buttons[0])
	engine.SetFeedbackCapabilityGaps([]string{"later.gap"})
	engine.cmdFeedback(platform, feedbackTestMsg(), "submit-token "+token)
	report := <-submitted
	if len(report.CapabilityGaps) != 1 || report.CapabilityGaps[0] != "display.sparkles" {
		t.Fatalf("clicked offer did not submit exact prepared keys: %#v", report)
	}
}

func TestFeedbackOfferTokenBindsDraftAndInitiatingUserAndCannotReplay(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	draft, err := engine.buildFeedbackDraft("feishu:oc_chat:ou_user", "owner-only problem", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := engine.rememberPendingFeedback("feishu:oc_chat:ou_user", "ou_user", draft)
	if err != nil {
		t.Fatal(err)
	}

	other := *feedbackTestMsg()
	other.UserID = "ou_other"
	engine.cmdFeedback(platform, &other, "submit-token "+token)
	select {
	case report := <-submitted:
		t.Fatalf("another group user submitted the owner's Draft: %#v", report)
	default:
	}

	engine.cmdFeedback(platform, feedbackTestMsg(), "submit-token "+token)
	if report := <-submitted; report.Description != "owner-only problem" {
		t.Fatalf("owner action submitted the wrong Draft: %#v", report)
	}
	engine.cmdFeedback(platform, feedbackTestMsg(), "submit-token "+token)
	select {
	case report := <-submitted:
		t.Fatalf("one-time offer token replayed: %#v", report)
	default:
	}
}

func TestCmdFeedback_ExpiredOfferCannotSubmit(t *testing.T) {
	engine, platform := newFeedbackTestEngine(t)
	submitted := captureFeedbackSubmissions(engine)
	draft, err := engine.buildFeedbackDraft("feishu:oc_chat:ou_user", "one problem", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := engine.rememberPendingFeedback("feishu:oc_chat:ou_user", "ou_user", draft)
	if err != nil {
		t.Fatal(err)
	}
	engine.feedbackMu.Lock()
	pending := engine.feedbackPending[token]
	pending.At = time.Now().Add(-feedbackPendingTTL - time.Second)
	engine.feedbackPending[token] = pending
	engine.feedbackMu.Unlock()

	engine.cmdFeedback(platform, feedbackTestMsg(), "submit-token "+token)
	select {
	case report := <-submitted:
		t.Fatalf("expired offer submitted a Draft: %#v", report)
	default:
	}
}
