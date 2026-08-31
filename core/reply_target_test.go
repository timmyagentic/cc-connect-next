package core

import (
	"encoding/json"
	"errors"
	"testing"
)

type replyTargetSnapshotPlatform struct {
	stubPlatformEngine
	reconstructCalls int
	requireSnapshot  bool
}

func (p *replyTargetSnapshotPlatform) SnapshotReplyCtx(replyCtx any) (json.RawMessage, error) {
	return json.Marshal(replyCtx)
}

func (p *replyTargetSnapshotPlatform) RestoreReplyCtx(snapshot json.RawMessage) (any, error) {
	var value string
	if err := json.Unmarshal(snapshot, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (p *replyTargetSnapshotPlatform) ReconstructReplyCtx(sessionKey string) (any, error) {
	p.reconstructCalls++
	return "fallback:" + sessionKey, nil
}

func (p *replyTargetSnapshotPlatform) RequiresPersistentReplyTarget(string) bool {
	return p.requireSnapshot
}

func TestEngine_PersistentReplyTargetPreferredAfterRestart(t *testing.T) {
	storePath := t.TempDir() + "/sessions.json"
	key := "snap:chat:thread:topic"
	p := &replyTargetSnapshotPlatform{stubPlatformEngine: stubPlatformEngine{n: "snap"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, storePath, LangEnglish)
	e.rememberReplyTarget(p, &Message{SessionKey: key, ReplyCtx: "live-target"})

	p2 := &replyTargetSnapshotPlatform{stubPlatformEngine: stubPlatformEngine{n: "snap"}}
	restarted := NewEngine("test", &stubAgent{}, []Platform{p2}, storePath, LangEnglish)
	got, err := restarted.reconstructReplyContext(p2, key)
	if err != nil {
		t.Fatalf("reconstructReplyContext() error = %v", err)
	}
	if got != "live-target" || p2.reconstructCalls != 0 {
		t.Fatalf("restored target = %#v, fallback calls = %d", got, p2.reconstructCalls)
	}
}

func TestEngine_ReplyTargetFallsBackWhenNoSnapshotExists(t *testing.T) {
	p := &replyTargetSnapshotPlatform{stubPlatformEngine: stubPlatformEngine{n: "snap"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	got, err := e.reconstructReplyContext(p, "snap:chat:user")
	if err != nil || got != "fallback:snap:chat:user" || p.reconstructCalls != 1 {
		t.Fatalf("fallback result = %#v, calls = %d, err = %v", got, p.reconstructCalls, err)
	}
}

func TestEngine_PersistentTargetRequirementFailsClosedUntilSnapshotExists(t *testing.T) {
	key := "snap:chat:thread:topic"
	p := &replyTargetSnapshotPlatform{stubPlatformEngine: stubPlatformEngine{n: "snap"}, requireSnapshot: true}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	if err := e.ValidatePersistentProactiveTarget(key); !errors.Is(err, ErrPersistentProactiveDeliveryUnsupported) {
		t.Fatalf("ValidatePersistentProactiveTarget() error = %v", err)
	}

	e.rememberReplyTarget(p, &Message{SessionKey: key, ReplyCtx: "live-target"})
	if err := e.ValidatePersistentProactiveTarget(key); err != nil {
		t.Fatalf("ValidatePersistentProactiveTarget() with snapshot error = %v", err)
	}
}

func TestAgentCapabilityManifest_ReportsReplyTargetSnapshotAndDurability(t *testing.T) {
	key := "snap:chat:thread:topic"
	p := &replyTargetSnapshotPlatform{stubPlatformEngine: stubPlatformEngine{n: "snap"}, requireSnapshot: true}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.platformReady[p] = true
	e.interactiveStates[key] = &interactiveState{platform: p, replyCtx: "live-target", agentSession: &stubAgentSession{}}

	availability := func(manifest AgentCapabilityManifest, id string) CapabilityAvailabilityState {
		for _, adapter := range manifest.Runtime {
			if adapter.Kind != "platform" || adapter.Name != "snap" {
				continue
			}
			for _, capability := range adapter.Capabilities {
				if capability.ID == id {
					return capability.Availability.State
				}
			}
		}
		return ""
	}

	before := e.QueryAgentCapabilityManifest(key, "", false)
	if got := availability(before, "reply_target_snapshot"); got != CapabilityAvailable {
		t.Fatalf("reply_target_snapshot availability = %q", got)
	}
	if got := availability(before, "persistent_proactive_delivery"); got != CapabilityUnavailable {
		t.Fatalf("persistent delivery without snapshot = %q", got)
	}

	e.rememberReplyTarget(p, &Message{SessionKey: key, ReplyCtx: "live-target"})
	after := e.QueryAgentCapabilityManifest(key, "", false)
	if got := availability(after, "persistent_proactive_delivery"); got != CapabilityAvailable {
		t.Fatalf("persistent delivery with snapshot = %q", got)
	}
}
