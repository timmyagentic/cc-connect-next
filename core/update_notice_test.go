package core

// Tests for the daemon-side update notice: a user running an old version gets
// a localized private reminder delivered to every explicit admin_from user;
// the project/version completes only after one full pass succeeds.

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
)

type updateNoticeDirectCall struct {
	userID  string
	content string
}

type updateNoticeDirectStub struct {
	restartNotifyStub
	directMu    sync.Mutex
	directCalls []updateNoticeDirectCall
	failUsers   map[string]error
}

func newUpdateNoticeDirectStub(name string) *updateNoticeDirectStub {
	return &updateNoticeDirectStub{restartNotifyStub: restartNotifyStub{name: name}}
}

func (p *updateNoticeDirectStub) SendDirectUser(_ context.Context, userID, content string) error {
	p.directMu.Lock()
	defer p.directMu.Unlock()
	p.directCalls = append(p.directCalls, updateNoticeDirectCall{userID: userID, content: content})
	return p.failUsers[userID]
}

func (p *updateNoticeDirectStub) directSnapshot() []updateNoticeDirectCall {
	p.directMu.Lock()
	defer p.directMu.Unlock()
	return append([]updateNoticeDirectCall(nil), p.directCalls...)
}

func (p *updateNoticeDirectStub) sentTexts() []string {
	calls := p.directSnapshot()
	texts := make([]string, 0, len(calls))
	for _, call := range calls {
		texts = append(texts, call.content)
	}
	return texts
}

func (p *updateNoticeDirectStub) clearDirect() {
	p.directMu.Lock()
	p.directCalls = nil
	p.directMu.Unlock()
}

func TestUpdateNotice_SendsOnlyToExplicitAdminPrivateUsers(t *testing.T) {
	withCurrentVersion(t, "v0.1.0")
	platform := newUpdateNoticeDirectStub("feishu")
	e := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	e.SetAdminFrom("ou_admin_b, ou_admin_a, OU_ADMIN_A")
	// Recent group/topic activity must never influence the target.
	touchSession(e, "feishu:oc_recent_group:ou_member:root:om_topic")
	touchSession(e, "feishu:oc_recent_group:ou_admin_a")

	if !e.NotifyUpdateAvailable(&ReleaseInfo{TagName: "v0.1.2"}) {
		t.Fatal("expected every explicit admin direct notice to succeed")
	}
	calls := platform.directSnapshot()
	gotUsers := make([]string, 0, len(calls))
	for _, call := range calls {
		gotUsers = append(gotUsers, call.userID)
		if !strings.Contains(call.content, "v0.1.2") || !strings.Contains(call.content, "v0.1.0") {
			t.Fatalf("direct notice content = %q", call.content)
		}
	}
	if !slices.Equal(gotUsers, []string{"ou_admin_a", "ou_admin_b"}) {
		t.Fatalf("direct notice users = %v", gotUsers)
	}
	if regular := platform.restartNotifyStub.sentTexts(); len(regular) != 0 {
		t.Fatalf("update notice leaked to recent session: %v", regular)
	}
}

func TestUpdateNotice_EmptyOrWildcardAdminNeverFallsBackToRecentSession(t *testing.T) {
	for _, adminFrom := range []string{"", "*", " , * , ", "ou_admin,*"} {
		t.Run(strings.ReplaceAll(adminFrom, " ", "_"), func(t *testing.T) {
			platform := newUpdateNoticeDirectStub("feishu")
			e := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
			e.SetAdminFrom(adminFrom)
			touchSession(e, "feishu:oc_recent_group:ou_member")
			if e.NotifyUpdateAvailable(&ReleaseInfo{TagName: "v0.1.2"}) {
				t.Fatal("non-enumerable admin_from must not report delivery")
			}
			if len(platform.directSnapshot()) != 0 || len(platform.sentTexts()) != 0 {
				t.Fatalf("admin_from %q produced a notice", adminFrom)
			}
		})
	}
}

func TestUpdateNotice_MultipleDirectPlatformsAreAmbiguous(t *testing.T) {
	p1 := newUpdateNoticeDirectStub("feishu-a")
	p2 := newUpdateNoticeDirectStub("feishu-b")
	e := NewEngine("test", &stubAgent{}, []Platform{p1, p2}, "", LangEnglish)
	e.SetAdminFrom("ou_admin")
	if e.NotifyUpdateAvailable(&ReleaseInfo{TagName: "v0.1.2"}) {
		t.Fatal("ambiguous direct-user platform selection must fail closed")
	}
	if len(p1.directSnapshot()) != 0 || len(p2.directSnapshot()) != 0 {
		t.Fatal("ambiguous platforms must not receive any notice")
	}
}

func TestUpdateNotice_UnsupportedDirectPlatformNeverFallsBackToRecentSession(t *testing.T) {
	platform := &restartNotifyStub{name: "legacy"}
	e := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	e.SetAdminFrom("ou_admin")
	touchSession(e, "legacy:recent-group:ou_admin")
	if e.NotifyUpdateAvailable(&ReleaseInfo{TagName: "v0.1.2"}) {
		t.Fatal("platform without DirectUserSender must not report delivery")
	}
	if sent := platform.sentTexts(); len(sent) != 0 {
		t.Fatalf("unsupported direct platform fell back to recent session: %v", sent)
	}
}

func TestUpdateNotice_PartialAdminFailureRetriesAllBeforeMarkingVersion(t *testing.T) {
	withCurrentVersion(t, "v0.1.0")
	platform := newUpdateNoticeDirectStub("feishu")
	platform.failUsers = map[string]error{"ou_admin_b": errors.New("temporary failure")}
	e := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	e.SetAdminFrom("ou_admin_a,ou_admin_b")
	n, _ := newTestNotifier(t, &ReleaseInfo{TagName: "v0.1.2"})
	n.RegisterEngine("demo", e)

	n.CheckOnce()
	if got := len(platform.directSnapshot()); got != 2 {
		t.Fatalf("first attempt calls = %d, want 2", got)
	}
	platform.directMu.Lock()
	delete(platform.failUsers, "ou_admin_b")
	platform.directMu.Unlock()
	platform.clearDirect()
	n.CheckOnce()
	if calls := platform.directSnapshot(); len(calls) != 2 ||
		calls[0].userID != "ou_admin_a" || calls[1].userID != "ou_admin_b" {
		t.Fatalf("partial failure should retry every explicit admin; second calls = %+v", calls)
	}
	platform.clearDirect()
	n.CheckOnce()
	if got := len(platform.directSnapshot()); got != 0 {
		t.Fatalf("fully delivered version should not repeat; calls = %d", got)
	}
}

func TestUpdateNotice_PartialFailureRetriesAllAcrossNotifierRestart(t *testing.T) {
	withCurrentVersion(t, "v0.1.0")
	dataDir := t.TempDir()
	release := &ReleaseInfo{TagName: "v0.1.2"}

	firstPlatform := newUpdateNoticeDirectStub("feishu")
	firstPlatform.failUsers = map[string]error{"ou_admin_b": errors.New("temporary failure")}
	firstEngine := NewEngine("test", &stubAgent{}, []Platform{firstPlatform}, "", LangEnglish)
	firstEngine.SetAdminFrom("ou_admin_a,ou_admin_b")
	firstNotifier := NewUpdateNotifier(dataDir)
	firstNotifier.checkFn = func(string) (*ReleaseInfo, error) { return release, nil }
	firstNotifier.RegisterEngine("demo", firstEngine)
	firstNotifier.CheckOnce()
	stateBytes, err := os.ReadFile(firstNotifier.statePath())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.Contains(string(stateBytes), "recipient:") || strings.Contains(string(stateBytes), "ou_admin") {
		t.Fatalf("partial failure persisted recipient-specific state: %s", stateBytes)
	}

	secondPlatform := newUpdateNoticeDirectStub("feishu")
	secondEngine := NewEngine("test", &stubAgent{}, []Platform{secondPlatform}, "", LangEnglish)
	secondEngine.SetAdminFrom("ou_admin_a,ou_admin_b")
	secondNotifier := NewUpdateNotifier(dataDir)
	secondNotifier.checkFn = func(string) (*ReleaseInfo, error) { return release, nil }
	secondNotifier.RegisterEngine("demo", secondEngine)
	secondNotifier.CheckOnce()
	if calls := secondPlatform.directSnapshot(); len(calls) != 2 ||
		calls[0].userID != "ou_admin_a" || calls[1].userID != "ou_admin_b" {
		t.Fatalf("restarted notifier should retry every explicit admin after partial failure: %+v", calls)
	}
}

func TestUpdateNotice_DirectConsentWindowDoesNotApplyInGroup(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, nil, "", LangEnglish)
	e.updateIntents.recordDirectNotice("feishu", "ou_admin")
	direct := &Message{Platform: "feishu", UserID: "ou_admin", SessionKey: "feishu:oc_dm:ou_admin", IsDirect: true}
	group := &Message{Platform: "feishu", UserID: "ou_admin", SessionKey: "feishu:oc_group:ou_admin"}
	if !e.updateNoticeActiveForMessage(direct) {
		t.Fatal("direct admin reply did not inherit notice consent window")
	}
	if e.updateNoticeActiveForMessage(group) {
		t.Fatal("group message inherited private update notice consent window")
	}
}

func TestAgentCapabilityManifestReportsDirectUserNoticeCapabilities(t *testing.T) {
	platform := newUpdateNoticeDirectStub("feishu")
	e := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	e.OnPlatformReady(platform)
	manifest := e.QueryAgentCapabilityManifest("", "", false)
	adapter := findRuntimeAdapter(t, manifest.Runtime, "platform", "feishu")
	if got := findRuntimeFeature(t, adapter.Capabilities, "direct_user_messages").Availability.State; got != CapabilityAvailable {
		t.Fatalf("direct_user_messages = %s, want available", got)
	}
	card := findRuntimeFeature(t, adapter.Capabilities, "direct_user_cards")
	if card.Availability.State != CapabilityUnavailable || card.Fallback.Mode != "direct-text" {
		t.Fatalf("direct_user_cards = %#v", card)
	}
}

func newUpdateNoticeTestEngine(t *testing.T, platformName string) (*Engine, *updateNoticeDirectStub) {
	t.Helper()
	plat := newUpdateNoticeDirectStub(platformName)
	e := NewEngine("test", &stubAgent{}, []Platform{plat}, "", LangEnglish)
	e.SetAdminFrom("ou_admin")
	return e, plat
}

func withCurrentVersion(t *testing.T, v string) {
	t.Helper()
	prev := CurrentVersion
	CurrentVersion = v
	t.Cleanup(func() { CurrentVersion = prev })
}

func touchSession(e *Engine, key string) {
	s := e.sessions.GetOrCreateActive(key)
	s.TouchUserActivity()
}

func newTestNotifier(t *testing.T, release *ReleaseInfo) (*UpdateNotifier, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	n := NewUpdateNotifier(t.TempDir())
	n.checkFn = func(string) (*ReleaseInfo, error) {
		calls.Add(1)
		return release, nil
	}
	return n, calls
}

func TestUpdateNotice_DeliversOncePerVersion(t *testing.T) {
	withCurrentVersion(t, "v0.1.0")
	e, plat := newUpdateNoticeTestEngine(t, "feishu")
	touchSession(e, "feishu:oc_chat:ou_user")

	n, _ := newTestNotifier(t, &ReleaseInfo{TagName: "v0.1.2"})
	n.RegisterEngine("demo", e)

	n.CheckOnce()
	sent := plat.sentTexts()
	if len(sent) != 1 {
		t.Fatalf("expected exactly one notice, got %v", sent)
	}
	if !strings.Contains(sent[0], "v0.1.2") || !strings.Contains(sent[0], "v0.1.0") {
		t.Fatalf("notice must mention both versions, got %q", sent[0])
	}
	if strings.Contains(sent[0], "/upgrade") {
		t.Fatalf("notice must not demand command syntax, got %q", sent[0])
	}
	if !strings.Contains(strings.ToLower(sent[0]), "update") {
		t.Fatalf("notice must invite a natural-language reply, got %q", sent[0])
	}

	// Same version discovered again (restart of the loop): no second notice.
	n.CheckOnce()
	if got := plat.sentTexts(); len(got) != 1 {
		t.Fatalf("same version must not be announced twice, got %v", got)
	}
}

func TestUpdateNotice_NewVersionAnnouncedAgain(t *testing.T) {
	withCurrentVersion(t, "v0.1.0")
	e, plat := newUpdateNoticeTestEngine(t, "feishu")
	touchSession(e, "feishu:oc_chat:ou_user")

	release := &ReleaseInfo{TagName: "v0.1.2"}
	n := NewUpdateNotifier(t.TempDir())
	n.checkFn = func(string) (*ReleaseInfo, error) { return release, nil }
	n.RegisterEngine("demo", e)

	n.CheckOnce()
	release.TagName = "v0.1.3"
	n.CheckOnce()

	sent := plat.sentTexts()
	if len(sent) != 2 || !strings.Contains(sent[1], "v0.1.3") {
		t.Fatalf("a newer version must produce a fresh notice, got %v", sent)
	}
}

func TestUpdateNoticeBetaChannelNamesAndPersistsTheExactChannel(t *testing.T) {
	withCurrentVersion(t, "v0.3.0-beta.1")
	e, platform := newUpdateNoticeTestEngine(t, "feishu")
	n := NewUpdateNotifier(t.TempDir(), string(updatechannel.Beta))
	n.checkFn = func(string) (*ReleaseInfo, error) {
		return &ReleaseInfo{TagName: "v0.3.0-beta.2", Prerelease: true, Channel: string(updatechannel.Beta)}, nil
	}
	n.RegisterEngine("demo", e)
	n.CheckOnce()
	texts := platform.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "beta channel") || strings.Contains(texts[0], "stable release") {
		t.Fatalf("beta notice copy = %v", texts)
	}
	state, err := os.ReadFile(n.statePath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), "beta:v0.3.0-beta.2") {
		t.Fatalf("beta notice state = %s", state)
	}
}

func TestUpdateNotice_StatePersistsAcrossRestarts(t *testing.T) {
	withCurrentVersion(t, "v0.1.0")
	dataDir := t.TempDir()
	release := &ReleaseInfo{TagName: "v0.1.2"}

	e1, plat1 := newUpdateNoticeTestEngine(t, "feishu")
	touchSession(e1, "feishu:oc_chat:ou_user")
	n1 := NewUpdateNotifier(dataDir)
	n1.checkFn = func(string) (*ReleaseInfo, error) { return release, nil }
	n1.RegisterEngine("demo", e1)
	n1.CheckOnce()
	if got := plat1.sentTexts(); len(got) != 1 {
		t.Fatalf("first process must deliver the notice, got %v", got)
	}

	// Simulated restart: a new notifier over the same data dir must not
	// re-announce the same version.
	e2, plat2 := newUpdateNoticeTestEngine(t, "feishu")
	touchSession(e2, "feishu:oc_chat:ou_user")
	n2 := NewUpdateNotifier(dataDir)
	n2.checkFn = func(string) (*ReleaseInfo, error) { return release, nil }
	n2.RegisterEngine("demo", e2)
	n2.CheckOnce()
	if got := plat2.sentTexts(); len(got) != 0 {
		t.Fatalf("restart must not re-announce the same version, got %v", got)
	}
}

func TestUpdateNotice_RetriesWhenDirectDeliveryFails(t *testing.T) {
	withCurrentVersion(t, "v0.1.0")
	e, plat := newUpdateNoticeTestEngine(t, "feishu")
	plat.failUsers = map[string]error{"ou_admin": errors.New("temporarily unavailable")}

	n, _ := newTestNotifier(t, &ReleaseInfo{TagName: "v0.1.2"})
	n.RegisterEngine("demo", e)
	n.CheckOnce()
	if got := len(plat.directSnapshot()); got != 1 {
		t.Fatalf("failed direct attempt count = %d, want 1", got)
	}

	plat.directMu.Lock()
	delete(plat.failUsers, "ou_admin")
	plat.directMu.Unlock()
	plat.clearDirect()
	n.CheckOnce()
	if got := plat.sentTexts(); len(got) != 1 {
		t.Fatalf("notice must be retried after direct send recovers, got %v", got)
	}
}

func TestUpdateNotice_SkipsDevBuilds(t *testing.T) {
	withCurrentVersion(t, "dev")
	e, plat := newUpdateNoticeTestEngine(t, "feishu")
	touchSession(e, "feishu:oc_chat:ou_user")

	n, calls := newTestNotifier(t, &ReleaseInfo{TagName: "v9.9.9"})
	n.RegisterEngine("demo", e)
	n.CheckOnce()
	if calls.Load() != 0 {
		t.Fatal("dev builds must not hit the release API")
	}
	if got := plat.sentTexts(); len(got) != 0 {
		t.Fatalf("dev builds must not notify, got %v", got)
	}
}

func TestUpdateNotice_NoNewerRelease(t *testing.T) {
	withCurrentVersion(t, "v0.1.2")
	e, plat := newUpdateNoticeTestEngine(t, "feishu")
	touchSession(e, "feishu:oc_chat:ou_user")

	n, _ := newTestNotifier(t, nil) // CheckForUpdate returns nil when up to date
	n.RegisterEngine("demo", e)
	n.CheckOnce()
	if got := plat.sentTexts(); len(got) != 0 {
		t.Fatalf("up-to-date daemon must stay silent, got %v", got)
	}
}
