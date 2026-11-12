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
}

func (p *finalReplyHookPlatform) FinalReplyHookRoute(_ any, handle any) (string, string, string) {
	answer, _ := handle.(string)
	return "om_source", answer, "oc_chat"
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
		name, answer string
		failUpdate   bool
		failFallback bool
	}{
		{name: "updated card", answer: "handle-1"},
		{name: "replacement card", answer: "handle-2", failUpdate: true},
		{name: "failed delivery", failUpdate: true, failFallback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &finalReplyHookPlatform{
				stubRichCardSilentPlatform: &stubRichCardSilentPlatform{stubPlatformEngine: stubPlatformEngine{n: "fixture"}},
				failUpdate:                 tc.failUpdate, failFallback: tc.failFallback,
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
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			e.SetHooks(NewHookManager("test", []HookConfig{{
				Event: string(HookEventReplyCompleted), Type: "http", URL: server.URL, Async: boolPtr(false),
			}}, "", "", ""))
			e.SetDisplayConfig(DisplayCfg{Mode: "compact", CardMode: "rich"})
			sessionKey := "fixture:group"
			session := e.sessions.GetOrCreateActive(sessionKey)
			agent := newControllableSession("reply-completed")
			state := &interactiveState{agentSession: agent, platform: p, replyCtx: "group"}
			e.interactiveStates[sessionKey] = state
			agent.events <- Event{Type: EventText, Content: "Final answer."}
			agent.events <- Event{Type: EventResult, Content: "Final answer.", Done: true}
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
