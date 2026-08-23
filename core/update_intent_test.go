package core

import (
	"strings"
	"testing"
	"time"
)

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
	p := &updateIntentStubPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetAdminFrom("user1")
	oldVersion := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = oldVersion })
	e.updateCheckFn = func(cur string, useGitee bool) (*ReleaseInfo, error) {
		return &ReleaseInfo{TagName: "v9.9.9", Body: "notes"}, nil
	}
	e.selfUpdateFn = func(tag string, useGitee bool) error { return nil }
	return e, p
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

func TestUpdateIntent_BareAfterNoticeInstalls(t *testing.T) {
	e, p := upgradeIntentHarness(t)
	e.updateIntents.recordNotice("test:user1")

	msg := updateIntentMsg("更新")
	if !e.maybeHandleUpdateIntent(p, msg, "更新") {
		t.Fatal("bare intent after a notice must be consumed")
	}
	sent := strings.Join(p.getSent(), "\n")
	if !strings.Contains(sent, "Downloading") {
		t.Fatalf("expected download to start, got:\n%s", sent)
	}
	if !strings.Contains(sent, "v9.9.9") {
		t.Fatalf("expected target version in replies, got:\n%s", sent)
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
