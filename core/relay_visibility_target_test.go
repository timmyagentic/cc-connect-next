package core

import (
	"context"
	"testing"
)

type relayVisibilityTargetStub struct {
	stubPlatformEngine
	key string
	ok  bool
}

func (p *relayVisibilityTargetStub) RelayGroupVisibilityKey(callerSessionKey string) (string, bool) {
	return p.key, p.ok
}

func TestResolveGroupVisibilityKeyUsesPlatformThreadTarget(t *testing.T) {
	rm := NewRelayManager("")
	platform := &relayVisibilityTargetStub{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		key:                "feishu:oc_chat:root:om_root",
		ok:                 true,
	}
	engine := NewEngine("source", &stubAgent{}, []Platform{platform}, "", LangEnglish)

	got := rm.resolveGroupVisibilityKey("feishu", "oc_chat", "feishu:oc_chat:root:om_root", engine)
	if got != "feishu:oc_chat:root:om_root" {
		t.Fatalf("resolveGroupVisibilityKey() = %q, want thread-scoped key", got)
	}
}

func TestResolveGroupVisibilityKeyFallsBackForUnsupportedPlatform(t *testing.T) {
	rm := NewRelayManager("")
	platform := &relayVisibilityTargetStub{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		ok:                 false,
	}
	engine := NewEngine("source", &stubAgent{}, []Platform{platform}, "", LangEnglish)

	got := rm.resolveGroupVisibilityKey("feishu", "oc_chat", "feishu:oc_chat:root:om_root", engine)
	if got != "feishu:oc_chat:relay" {
		t.Fatalf("resolveGroupVisibilityKey() = %q, want legacy fallback", got)
	}
}

type relayVisibilitySenderStub struct {
	stubPlatformEngine
	sessionKey string
}

func (p *relayVisibilitySenderStub) SendRelayGroupVisibility(_ context.Context, sessionKey, content string) error {
	p.mu.Lock()
	p.sessionKey = sessionKey
	p.sent = append(p.sent, content)
	p.mu.Unlock()
	return nil
}

func TestSendToGroupPrefersPlatformVisibilitySender(t *testing.T) {
	rm := NewRelayManager("")
	platform := &relayVisibilitySenderStub{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	engine := NewEngine("source", &stubAgent{}, []Platform{platform}, "", LangEnglish)

	rm.sendToGroup(context.Background(), engine, "feishu", "feishu:oc_chat:root:om_root", "relay visible")

	if platform.sessionKey != "feishu:oc_chat:root:om_root" {
		t.Fatalf("visibility session key = %q, want topic-scoped key", platform.sessionKey)
	}
	if got := platform.getSent(); len(got) != 1 || got[0] != "relay visible" {
		t.Fatalf("visibility send = %#v", got)
	}
}
