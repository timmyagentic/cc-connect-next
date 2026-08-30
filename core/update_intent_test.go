package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var errCardSendFailed = errors.New("card send failed")

func TestMatchUpdateIntent(t *testing.T) {
	cases := []struct {
		in   string
		want updateIntent
	}{
		// Bare update verbs — consent only inside an update conversation.
		{"更新", updateIntentBare},
		{"升级", updateIntentBare},
		{"update", updateIntentBare},
		{"Upgrade", updateIntentBare},
		{"帮我更新一下", updateIntentBare},
		{"更新吧", updateIntentBare},
		{"升级一下呗", updateIntentBare},
		{"update now", updateIntentBare},
		{"请更新。", updateIntentBare},

		// Strong: explicit latest/version/self reference — any context.
		{"更新到最新版", updateIntentStrong},
		{"升级到最新版本", updateIntentStrong},
		{"帮我更新到最新版吧", updateIntentStrong},
		{"更新到 v0.2.0", updateIntentStrong},
		{"upgrade to latest", updateIntentStrong},
		{"update to latest version", updateIntentStrong},
		{"update yourself", updateIntentStrong},
		{"更新你自己", updateIntentStrong},
		{"把你自己升级一下", updateIntentStrong},
		{"升级 cc-connect-next", updateIntentStrong},

		// Confirm words — only after an upgrade prompt.
		{"确认", updateIntentConfirm},
		{"確認", updateIntentConfirm},
		{"確定", updateIntentConfirm},
		{"好的", updateIntentConfirm},
		{"yes", updateIntentConfirm},
		{"ok", updateIntentConfirm},
		{"安装", updateIntentConfirm},

		// Must fall through to the agent untouched.
		{"更新 README", updateIntentNone},
		{"更新一下依赖版本", updateIntentNone},
		{"帮我更新数据库 schema", updateIntentNone},
		{"update the docs", updateIntentNone},
		{"update package.json to latest deps", updateIntentNone},
		{"看看更新日志里有什么", updateIntentNone},
		{"这个功能什么时候更新", updateIntentNone},
		{"升级方案写一下", updateIntentNone},
		{"", updateIntentNone},
		{strings.Repeat("更新", 20), updateIntentNone}, // over length cap
	}
	for _, c := range cases {
		if got := matchUpdateIntent(c.in); got != c.want {
			t.Errorf("matchUpdateIntent(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// updateIntentStubPlatform records replies and can reconstruct a reply
// context, so proactive notices can target it.
type updateIntentStubPlatform struct {
	stubPlatformEngine
}

func (p *updateIntentStubPlatform) ReconstructReplyCtx(sessionKey string) (any, error) {
	return "rc:" + sessionKey, nil
}

// upgradeIntentHarness wires an engine whose update check/install are fakes
// and whose platform records replies, so the full natural-language path can
// run without network. The requesting user is an authorized admin; the
// privileged-gate test overrides that.
func upgradeIntentHarness(t *testing.T) (*Engine, *updateIntentStubPlatform) {
	t.Helper()
	drainTestRestartRequests()
	t.Cleanup(drainTestRestartRequests)
	p := &updateIntentStubPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetAdminFrom("user1")
	oldVersion := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = oldVersion })
	e.SetUpdateService(&stubCoreUpdateService{
		plan: PreparedUpdate{
			Release:      ReleaseInfo{TagName: "v9.9.9", Body: "notes"},
			ArchiveAsset: "cc-connect-next-v9.9.9-test-arch.tar.gz",
			Available:    true,
			token:        "exact-plan",
		},
		result: UpdateResult{Release: ReleaseInfo{TagName: "v9.9.9"}, Updated: true},
	})
	return e, p
}

func drainTestRestartRequests() {
	for {
		select {
		case <-RestartCh:
		default:
			return
		}
	}
}

func updateIntentMsg(content string) *Message {
	return &Message{
		SessionKey: "test:user1",
		Platform:   "test",
		UserID:     "user1",
		Content:    content,
		ReplyCtx:   "rc",
	}
}

func TestUpdateIntent_BareOutsideContextFallsThrough(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	msg := updateIntentMsg("更新")
	if e.maybeHandleUpdateIntent(p, msg, "更新") {
		t.Fatal("bare intent outside an update conversation must not be consumed")
	}
	if got := p.getSent(); len(got) != 0 {
		t.Fatalf("no reply expected, got %v", got)
	}
}

func TestUpdateIntent_BareAfterNoticePreparesBeforeInstall(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	e.updateIntents.recordNotice("test:user1")

	msg := updateIntentMsg("更新")
	if !e.maybeHandleUpdateIntent(p, msg, "更新") {
		t.Fatal("bare intent after a notice must be consumed")
	}
	sent := strings.Join(p.getSent(), "\n")
	if strings.Contains(sent, "Downloading") || !strings.Contains(sent, "New version available") {
		t.Fatalf("a discovery notice must lead to exact-plan review before install:\n%s", sent)
	}
	if !strings.Contains(sent, "v9.9.9") {
		t.Fatalf("expected target version in replies, got:\n%s", sent)
	}
	p.clearSent()
	if !e.maybeHandleUpdateIntent(p, updateIntentMsg("确认"), "确认") {
		t.Fatal("confirm after exact-plan review must be consumed")
	}
	if sent := strings.Join(p.getSent(), "\n"); !strings.Contains(sent, "Downloading") {
		t.Fatalf("expected install after plan confirmation, got:\n%s", sent)
	}
}

func TestUpdateIntent_StrongWithoutContextAsksFirst(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	msg := updateIntentMsg("更新到最新版")
	if !e.maybeHandleUpdateIntent(p, msg, "更新到最新版") {
		t.Fatal("strong intent must be consumed")
	}
	sent := strings.Join(p.getSent(), "\n")
	if strings.Contains(sent, "Downloading") {
		t.Fatalf("strong intent outside a conversation must ask, not install:\n%s", sent)
	}
	if !strings.Contains(sent, "New version available") {
		t.Fatalf("expected the upgrade prompt, got:\n%s", sent)
	}

	// The prompt opened a consent window: a bare confirm now installs.
	p.clearSent()
	if !e.maybeHandleUpdateIntent(p, updateIntentMsg("确认"), "确认") {
		t.Fatal("confirm after the prompt must be consumed")
	}
	if sent := strings.Join(p.getSent(), "\n"); !strings.Contains(sent, "Downloading") {
		t.Fatalf("expected install after confirm, got:\n%s", sent)
	}
}

func TestUpdateIntent_ConfirmWithoutAskFallsThrough(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	if e.maybeHandleUpdateIntent(p, updateIntentMsg("确认"), "确认") {
		t.Fatal("generic assent without a pending upgrade prompt must fall through")
	}
	if got := p.getSent(); len(got) != 0 {
		t.Fatalf("no reply expected, got %v", got)
	}
}

func TestUpdateIntent_ExpiredAskWindowFallsThrough(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	e.updateIntents.recordAsk("test:user1")
	e.updateIntents.mu.Lock()
	e.updateIntents.askAt["test:user1"] = time.Now().Add(-updateAskConsentTTL - time.Minute)
	e.updateIntents.mu.Unlock()

	if e.maybeHandleUpdateIntent(p, updateIntentMsg("确认"), "确认") {
		t.Fatal("assent after the window expired must fall through")
	}
}

func TestUpdateIntent_AttachmentsNeverIntercepted(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	e.updateIntents.recordNotice("test:user1")
	msg := updateIntentMsg("更新")
	msg.Images = []ImageAttachment{{Data: []byte{1}}}
	if e.maybeHandleUpdateIntent(p, msg, "更新") {
		t.Fatal("messages with attachments must reach the agent")
	}
}

func TestUpdateIntent_RespectsPrivilegedGate(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	e.SetAdminFrom("boss") // requester "user1" is not an admin
	e.updateIntents.recordNotice("test:user1")

	if !e.maybeHandleUpdateIntent(p, updateIntentMsg("更新"), "更新") {
		t.Fatal("intent should still be consumed (with a refusal reply)")
	}
	sent := strings.Join(p.getSent(), "\n")
	if strings.Contains(sent, "Downloading") {
		t.Fatalf("non-admin must not trigger an install:\n%s", sent)
	}
	if !strings.Contains(strings.ToLower(sent), "admin") {
		t.Fatalf("expected the admin-required refusal, got:\n%s", sent)
	}
}

func TestNotifyUpdateAvailable_RecordsConsentWindow(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	// Give the engine a session so notifyMostRecentSessionFn finds a target.
	s := e.sessions.GetOrCreateActive("test:user1")
	_ = s

	ok := e.NotifyUpdateAvailable(&ReleaseInfo{TagName: "v9.9.9"})
	if !ok {
		t.Fatal("notice delivery failed")
	}
	if !e.updateIntents.noticeActive("test:user1") {
		t.Fatal("delivered notice must open the consent window for that session")
	}
	sent := strings.Join(p.getSent(), "\n")
	if !strings.Contains(sent, "v9.9.9") {
		t.Fatalf("notice content missing, got:\n%s", sent)
	}
	if strings.Contains(sent, "/upgrade") {
		t.Fatalf("notice must not demand command syntax, got:\n%s", sent)
	}
}

// cardMarkdown concatenates the markdown text of a card, for asserting on
// what the user actually reads.
func cardMarkdown(c *Card) string {
	var b strings.Builder
	for _, el := range c.Elements {
		if md, ok := el.(CardMarkdown); ok {
			b.WriteString(md.Content)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func cardNotes(c *Card) string {
	var b strings.Builder
	for _, el := range c.Elements {
		if n, ok := el.(CardNote); ok {
			b.WriteString(n.Text)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func cardButtonLabels(c *Card) []string {
	var out []string
	for _, el := range c.Elements {
		if a, ok := el.(CardActions); ok {
			for _, btn := range a.Buttons {
				out = append(out, btn.Text)
			}
		}
	}
	return out
}

func TestPreviewReleaseBodyKeeps3000Runes(t *testing.T) {
	want := strings.Repeat("界", updateReleaseBodyPreviewRunes)
	if got := previewReleaseBody(want); got != want {
		t.Fatalf("exact-limit preview changed: got %d runes, want %d", len([]rune(got)), len([]rune(want)))
	}
	if got := previewReleaseBody(want + "尾"); got != want+"…" {
		t.Fatalf("over-limit preview = %q", got)
	}
}

func TestReleaseBodyForLanguageSelectsBilingualSection(t *testing.T) {
	body := "# release\n\n## 中文\n\n中文说明\n\n## English\n\nEnglish notes\n"
	tests := []struct {
		lang Language
		want string
	}{
		{LangChinese, "中文说明"},
		{LangTraditionalChinese, "中文说明"},
		{LangEnglish, "English notes"},
		{LangJapanese, "English notes"},
		{LangSpanish, "English notes"},
	}
	for _, tt := range tests {
		if got := releaseBodyForLanguage(body, tt.lang); got != tt.want {
			t.Errorf("releaseBodyForLanguage(%q) = %q, want %q", tt.lang, got, tt.want)
		}
	}
}

func TestReleaseBodyForLanguageFallsBackToOriginal(t *testing.T) {
	const body = "single-language release notes"
	if got := releaseBodyForLanguage(body, LangChinese); got != body {
		t.Fatalf("fallback = %q, want original body", got)
	}
}

// A message must carry exactly one call to action: when a button is present
// the copy must not also instruct the user to type a reply, or they cannot
// tell which one is expected.
func TestUpdateNotice_CardCopyHasNoTypedReplyInstruction(t *testing.T) {
	p := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
	oldVersion := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = oldVersion })
	e.sessions.GetOrCreateActive("feishu:user1")

	if !e.NotifyUpdateAvailable(&ReleaseInfo{TagName: "v9.9.9"}) {
		t.Fatal("notice delivery failed")
	}
	p.mu.Lock()
	cards := append([]*Card(nil), p.sentCards...)
	p.mu.Unlock()
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}

	body := cardMarkdown(cards[0])
	for _, banned := range []string{"回复", "确认", "/upgrade"} {
		if strings.Contains(body, banned) {
			t.Fatalf("card copy must not instruct a typed reply (found %q):\n%s", banned, body)
		}
	}
	if !strings.Contains(body, "v9.9.9") {
		t.Fatalf("card copy lost the version: %s", body)
	}
	if labels := cardButtonLabels(cards[0]); len(labels) != 2 {
		t.Fatalf("card buttons = %v, want [查看并更新 查看变更]", labels)
	}
	// The natural-language route stays discoverable, but as a footnote:
	// subordinate to the button, never a second competing instruction.
	note := cardNotes(cards[0])
	if !strings.Contains(note, "回复") || !strings.Contains(note, "更新") {
		t.Fatalf("notice card must hint the natural-language reply in a note, got %q", note)
	}
}

func TestUpdateNotice_TextOnlyCopyKeepsReplyInstruction(t *testing.T) {
	p := &updateIntentStubPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
	oldVersion := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = oldVersion })
	e.sessions.GetOrCreateActive("test:user1")

	if !e.NotifyUpdateAvailable(&ReleaseInfo{TagName: "v9.9.9"}) {
		t.Fatal("notice delivery failed")
	}
	sent := strings.Join(p.getSent(), "\n")
	if !strings.Contains(sent, "回复") {
		t.Fatalf("a button-less platform must say how to reply:\n%s", sent)
	}
}

// A card platform whose send fails must fall back to the copy that explains
// the typed reply — never to button copy with no button.
func TestUpdateNotice_CardFailureFallsBackToInstructiveCopy(t *testing.T) {
	p := &stubCardPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		cardErr:            errCardSendFailed,
	}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
	oldVersion := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = oldVersion })
	e.sessions.GetOrCreateActive("feishu:user1")

	if !e.NotifyUpdateAvailable(&ReleaseInfo{TagName: "v9.9.9"}) {
		t.Fatal("notice delivery failed")
	}
	sent := strings.Join(p.getSent(), "\n")
	if !strings.Contains(sent, "回复") {
		t.Fatalf("card-send failure must fall back to instructive copy:\n%s", sent)
	}
}

func TestUpgradePrompt_CardCopyHasNoTypedReplyInstruction(t *testing.T) {
	p := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
	e.SetAdminFrom("user1")
	oldVersion := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = oldVersion })
	e.SetUpdateService(&stubCoreUpdateService{
		plan: PreparedUpdate{Release: ReleaseInfo{TagName: "v9.9.9", Body: "notes"}, ArchiveAsset: "asset.tar.gz", Available: true, token: "plan"},
	})

	msg := &Message{SessionKey: "feishu:user1", Platform: "feishu", UserID: "user1", ReplyCtx: "rc"}
	e.cmdUpgrade(p, msg, nil)

	p.mu.Lock()
	cards := append([]*Card(nil), p.repliedCards...)
	p.mu.Unlock()
	if len(cards) != 1 {
		t.Fatalf("got %d reply cards, want 1", len(cards))
	}
	body := cardMarkdown(cards[0])
	for _, banned := range []string{"回复", "确认"} {
		if strings.Contains(body, banned) {
			t.Fatalf("prompt card must not instruct a typed reply (found %q):\n%s", banned, body)
		}
	}
	if !strings.Contains(body, "v9.9.9") || !strings.Contains(body, "notes") {
		t.Fatalf("prompt card lost release details:\n%s", body)
	}
	if labels := cardButtonLabels(cards[0]); len(labels) != 1 {
		t.Fatalf("prompt card buttons = %v, want exactly [查看并更新]", labels)
	}
	note := cardNotes(cards[0])
	if !strings.Contains(note, "回复") || !strings.Contains(note, "确认") {
		t.Fatalf("prompt card must hint the natural-language reply in a note, got %q", note)
	}
}

func TestUpgradePrompt_CardUsesConfiguredReleaseLanguage(t *testing.T) {
	p := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetAdminFrom("user1")
	oldVersion := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = oldVersion })
	e.SetUpdateService(&stubCoreUpdateService{
		plan: PreparedUpdate{
			Release: ReleaseInfo{
				TagName: "v9.9.9",
				Body:    "# release\n\n## 中文\n\n中文说明\n\n## English\n\nEnglish notes",
			},
			ArchiveAsset: "asset.tar.gz",
			Available:    true,
			token:        "plan",
		},
	})

	e.cmdUpgrade(p, &Message{SessionKey: "feishu:user1", Platform: "feishu", UserID: "user1", ReplyCtx: "rc"}, nil)
	p.mu.Lock()
	cards := append([]*Card(nil), p.repliedCards...)
	p.mu.Unlock()
	if len(cards) != 1 {
		t.Fatalf("got %d reply cards, want 1", len(cards))
	}
	body := cardMarkdown(cards[0])
	if !strings.Contains(body, "English notes") || strings.Contains(body, "中文说明") {
		t.Fatalf("English card used the wrong release section:\n%s", body)
	}
}

func TestUpgradePrompt_TextOnlyCopyKeepsReplyInstruction(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	e.i18n = NewI18n(LangChinese)
	msg := &Message{SessionKey: "test:user1", Platform: "test", UserID: "user1", ReplyCtx: "rc"}
	e.cmdUpgrade(p, msg, nil)

	sent := strings.Join(p.getSent(), "\n")
	if !strings.Contains(sent, "确认") {
		t.Fatalf("a button-less platform must say what to reply:\n%s", sent)
	}
}
