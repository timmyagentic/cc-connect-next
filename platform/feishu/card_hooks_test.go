package feishu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	"github.com/timmyagentic/cc-connect-next/core"
)

func namedHookCardEvent() *callback.CardActionTriggerEvent {
	event := receiptEvent("")
	action := event.Event.Action
	action.Name = "confirmed"
	action.Value = map[string]any{
		"action":                "hook:reply_confirmation",
		cardActionProjectKey:    "example",
		cardActionDirectUserKey: "ou_allowed",
		"cc_card_schema":        "2.0",
		"data":                  map[string]any{"request_id": "opaque-request", "choice": "text"},
	}
	action.FormValue = map[string]any{"reply_text": "User edited **text**\n$(touch ignored)"}
	return event
}

func TestCardHookRoutesNamedHandlerWithSubmittedValues(t *testing.T) {
	p, messages := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	received := make(chan core.HookEvent, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event core.HookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Error(err)
		}
		received <- event
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	manager := core.NewHookManager("example", []core.HookConfig{
		{Name: "reply_confirmation", Project: "example", Event: "card.action", Type: "http", URL: server.URL},
	}, "", "", "")
	p.SetCardActionHandler(manager.DispatchNamed)
	event := namedHookCardEvent()
	response, err := p.onCardAction(event)
	if err != nil || response == nil || response.Toast == nil || response.Toast.Type != "info" || response.Card != nil {
		t.Fatalf("callback response = %#v, %v", response, err)
	}
	select {
	case got := <-received:
		if got.Event != core.HookEventCardAction || got.Project != "example" || got.Platform != "feishu" ||
			got.UserID != "ou_allowed" || got.Extra["hook"] != "reply_confirmation" || got.Extra["action"] != "confirmed" ||
			got.Extra["message_id"] != "om_first" || got.Extra["chat_id"] != "oc_fixture" {
			t.Fatalf("wrong event routing: %#v", got)
		}
		form := got.Extra["form_values"].(map[string]any)
		if form["reply_text"] != event.Event.Action.FormValue["reply_text"] {
			t.Fatalf("submitted value changed: %#v", form)
		}
		if got.Extra["data"].(map[string]any)["request_id"] != "opaque-request" {
			t.Fatalf("opaque data changed: %#v", got.Extra)
		}
	case <-time.After(time.Second):
		t.Fatal("named hook did not receive callback")
	}
	event.Event.Action.FormValue = nil
	event.Event.Action.Value["data"] = map[string]any{"request_id": "opaque-request", "choice": "reaction"}
	if _, err := p.onCardAction(event); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		form, ok := got.Extra["form_values"].(map[string]any)
		if !ok || len(form) != 0 {
			t.Fatalf("ordinary button form values = %#v, want empty object", got.Extra["form_values"])
		}
	case <-time.After(time.Second):
		t.Fatal("ordinary button did not dispatch")
	}
	waitNoMessage(t, messages, 20*time.Millisecond, "named card hook")
}

func TestCardHookRejectsInvalidOrUnboundCallbacks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*callback.CardActionTriggerEvent)
	}{
		{"wrong operator", func(e *callback.CardActionTriggerEvent) { e.Event.Operator.OpenID = "ou_other" }},
		{"wrong project", func(e *callback.CardActionTriggerEvent) { e.Event.Action.Value[cardActionProjectKey] = "other" }},
		{"missing project", func(e *callback.CardActionTriggerEvent) { delete(e.Event.Action.Value, cardActionProjectKey) }},
		{"mismatched direct user", func(e *callback.CardActionTriggerEvent) { e.Event.Action.Value[cardActionDirectUserKey] = "ou_other" }},
		{"bad chat", func(e *callback.CardActionTriggerEvent) { e.Event.Context.OpenChatID = "" }},
		{"bad message", func(e *callback.CardActionTriggerEvent) { e.Event.Context.OpenMessageID = "bad" }},
		{"bad schema", func(e *callback.CardActionTriggerEvent) { e.Event.Action.Value["cc_card_schema"] = "unknown" }},
		{"missing name", func(e *callback.CardActionTriggerEvent) { e.Event.Action.Name = "" }},
		{"input event", func(e *callback.CardActionTriggerEvent) { e.Event.Action.Tag = "input" }},
		{"invalid hook name", func(e *callback.CardActionTriggerEvent) { e.Event.Action.Value["action"] = "hook:foo; echo bar" }},
		{"nonobject data", func(e *callback.CardActionTriggerEvent) { e.Event.Action.Value["data"] = "command" }},
		{"oversized input", func(e *callback.CardActionTriggerEvent) {
			e.Event.Action.FormValue["reply_text"] = strings.Repeat("x", maxCardHookPayload)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, messages := newAllowFromTestPlatform(t, "ou_allowed")
			p.BindProject("example")
			called := false
			p.SetCardActionHandler(func(string, core.HookEvent) error { called = true; return nil })
			event := namedHookCardEvent()
			tc.change(event)
			response, err := p.onCardAction(event)
			if err != nil || called || (response != nil && response.Toast != nil && response.Toast.Type == "info") {
				t.Fatalf("invalid callback accepted: called=%v response=%#v err=%v", called, response, err)
			}
			waitNoMessage(t, messages, time.Millisecond, "invalid named hook")
		})
	}
}

func TestCardHookUnknownTargetDoesNotAcknowledgeAcceptance(t *testing.T) {
	p, _ := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	manager := core.NewHookManager("example", nil, "", "", "")
	p.SetCardActionHandler(manager.DispatchNamed)
	response, err := p.onCardAction(namedHookCardEvent())
	if err != nil || response == nil || response.Toast == nil || response.Toast.Type != "error" {
		t.Fatalf("unknown hook response = %#v, %v", response, err)
	}
}

func TestCardHookSlowOperationDoesNotDelayCallback(t *testing.T) {
	p, _ := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	defer close(release)
	synchronous := false
	manager := core.NewHookManager("example", []core.HookConfig{
		{Name: "reply_confirmation", Project: "example", Event: "card.action", Type: "http", URL: server.URL, Async: &synchronous},
	}, "", "", "")
	p.SetCardActionHandler(manager.DispatchNamed)
	done := make(chan struct{})
	go func() {
		response, err := p.onCardAction(namedHookCardEvent())
		if err != nil || response == nil || response.Toast.Type != "info" {
			t.Errorf("response = %#v, %v", response, err)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("operation blocked callback response")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("operation was not started")
	}
}

func TestCardHookRequiresInteractiveAdapter(t *testing.T) {
	platform, err := New(map[string]any{"app_id": "fixture", "app_secret": "fixture", "enable_feishu_card": false})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := platform.(core.CardActionHookSource); ok {
		t.Fatal("plain adapter advertised named card hooks")
	}
}
