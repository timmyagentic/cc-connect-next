package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type finalReplyHookPlatform struct {
	*stubRichCardSilentPlatform
	failUpdate   bool
	failFallback bool
	missingRoute string
}

func (p *finalReplyHookPlatform) FinalReplyHookRoute(_ any, handle any) (string, string, string) {
	answer, _ := handle.(string)
	source, chat := "om_source", "oc_chat"
	switch p.missingRoute {
	case "source":
		source = ""
	case "answer":
		answer = ""
	case "chat":
		chat = ""
	}
	return source, answer, chat
}

func (p *finalReplyHookPlatform) UpdateMessage(ctx context.Context, handle any, content string) error {
	if p.failUpdate && strings.Contains(content, "status=done") {
		return errors.New("final update failed")
	}
	return p.stubRichCardSilentPlatform.UpdateMessage(ctx, handle, content)
}

func (p *finalReplyHookPlatform) SendPreviewStart(ctx context.Context, replyCtx any, content string) (any, error) {
	if p.failFallback && strings.Contains(content, "status=done") {
		return nil, errors.New("replacement send failed")
	}
	return p.stubRichCardSilentPlatform.SendPreviewStart(ctx, replyCtx, content)
}

func TestReplyCompletedHookFollowsFinalDelivery(t *testing.T) {
	for _, tc := range []struct {
		name, answer      string
		content, cardMode string
		hookFailure       bool
		missingRoute      string
		failUpdate        bool
		failFallback      bool
	}{
		{name: "updated card", answer: "handle-1"},
		{name: "replacement card", answer: "handle-2", failUpdate: true},
		{name: "failed delivery", failUpdate: true, failFallback: true},
		{name: "silent reply", content: "NO_REPLY"},
		{name: "missing source", missingRoute: "source"},
		{name: "missing answer", missingRoute: "answer"},
		{name: "missing chat", missingRoute: "chat"},
		{name: "legacy reply", cardMode: "legacy"},
		{name: "hook failure is not retried", answer: "handle-1", hookFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &finalReplyHookPlatform{
				stubRichCardSilentPlatform: &stubRichCardSilentPlatform{stubPlatformEngine: stubPlatformEngine{n: "fixture"}},
				failUpdate:                 tc.failUpdate, failFallback: tc.failFallback, missingRoute: tc.missingRoute,
			}
			received := make(chan HookEvent, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var event HookEvent
				if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				received <- event
				if tc.hookFailure {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			e.SetHooks(NewHookManager("test", []HookConfig{{
				Event: string(HookEventReplyCompleted), Type: "http", URL: server.URL, Async: boolPtr(false),
			}}, "", "", ""))
			cardMode := tc.cardMode
			if cardMode == "" {
				cardMode = "rich"
			}
			e.SetDisplayConfig(DisplayCfg{Mode: "compact", CardMode: cardMode})
			sessionKey := "fixture:group"
			session := e.sessions.GetOrCreateActive(sessionKey)
			agent := newControllableSession("reply-completed")
			state := &interactiveState{agentSession: agent, platform: p, replyCtx: "group"}
			e.interactiveStates[sessionKey] = state
			content := tc.content
			if content == "" {
				content = "Final answer."
			}
			agent.events <- Event{Type: EventText, Content: content}
			agent.events <- Event{Type: EventResult, Content: content, Done: true}
			e.processInteractiveEvents(state, session, e.sessions, sessionKey, "om_source", time.Now(), nil, nil, state.replyCtx)
			if tc.answer == "" {
				if len(received) != 0 {
					t.Fatal("failed delivery emitted a completion hook")
				}
				return
			}
			if len(received) != 1 {
				t.Fatalf("got %d completion hooks, want one", len(received))
			}
			event := <-received
			if event.Extra["source_id"] != "om_source" || event.Extra["answer_id"] != tc.answer ||
				event.Extra["chat_id"] != "oc_chat" || event.Extra["completed_at_ms"] == nil || event.Content != "" {
				t.Fatalf("unexpected completion event: %#v", event)
			}
		})
	}
}

func TestReplyCompletedManifest_ContractAndPrivacy(t *testing.T) {
	p := &finalReplyHookPlatform{stubRichCardSilentPlatform: &stubRichCardSilentPlatform{stubPlatformEngine: stubPlatformEngine{n: "fixture"}}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.platformLifecycleMu.Lock()
	e.platformReady[p] = true
	e.platformLifecycleMu.Unlock()
	feature := func() RuntimeFeatureCapability {
		manifest := e.QueryAgentCapabilityManifest("", "", false)
		adapter := findRuntimeAdapter(t, manifest.Runtime, "platform", "fixture")
		return findRuntimeFeature(t, adapter.Capabilities, "reply_completion_hook")
	}
	e.SetDisplayConfig(DisplayCfg{CardMode: "rich"})
	if got := feature(); got.Availability.State != CapabilityUnavailable {
		t.Fatalf("unconfigured hook advertised: %#v", got)
	}
	for _, pattern := range []string{"reply.completed", "*"} {
		e.SetHooks(NewHookManager("test", []HookConfig{{Event: pattern, Type: "command", Command: "DO_NOT_LEAK_REPLY_COMMAND"}}, "sh", "-c", ""))
		got := feature()
		if got.Availability.State != CapabilityConditional || got.Permission != CapabilityPermissionConditional || got.ReadOnly == nil || *got.ReadOnly || !hasManifestEffect(got.SideEffects, "shell_execution") || !hasManifestEffect(got.SideEffects, "network") || got.Fallback.Mode != "skip" {
			t.Fatalf("incomplete reply hook contract: %#v", got)
		}
		encoded, err := json.Marshal(e.QueryAgentCapabilityManifest("", "", false))
		if err != nil || strings.Contains(string(encoded), "DO_NOT_LEAK_REPLY_COMMAND") {
			t.Fatal("manifest exposed hook body")
		}
	}
	e.SetDisplayConfig(DisplayCfg{CardMode: "legacy"})
	if got := feature(); got.Availability.State != CapabilityUnavailable {
		t.Fatalf("legacy mode advertised: %#v", got)
	}
	e.SetHooks(nil)
	if got := feature(); got.Availability.State != CapabilityUnavailable {
		t.Fatalf("removed hook advertised: %#v", got)
	}
	unsupported := &stubPlatformEngine{n: "plain"}
	e2 := NewEngine("plain", &stubAgent{}, []Platform{unsupported}, "", LangEnglish)
	adapter := findRuntimeAdapter(t, e2.QueryAgentCapabilityManifest("", "", false).Runtime, "platform", "plain")
	if got := findRuntimeFeature(t, adapter.Capabilities, "reply_completion_hook"); got.Availability.State != CapabilityUnavailable {
		t.Fatalf("unsupported adapter advertised: %#v", got)
	}
}
