package core

import "testing"

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
