package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestPrivateSettingsCardsBindOwnerInEmittedPayload(t *testing.T) {
	for _, tc := range []struct {
		name   string
		send   bool
		thread bool
	}{
		{name: "ReplyCard"},
		{name: "SendCard", send: true},
		{name: "ReplyCardPrivateThread", thread: true},
		{name: "SendCardPrivateThread", send: true, thread: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc := replyContext{
				messageID: "om_trigger", chatID: "oc_test_chat",
				sessionKey: "feishu:oc_test_chat:ou_allowed", directUserID: "ou_allowed",
			}
			if tc.thread {
				rc.threadID = "omt_private"
				rc.sessionKey = "feishu:oc_test_chat:thread:omt_private"
			}
			wantPath := "/open-apis/im/v1/messages/om_trigger/reply"
			if tc.send && !tc.thread {
				wantPath = "/open-apis/im/v1/messages"
			}
			emitted := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if serveResourceTestToken(t, w, r) {
					return
				}
				if r.Method != http.MethodPost || r.URL.Path != wantPath {
					t.Errorf("unexpected card request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				var body struct {
					Content       string `json:"content"`
					MsgType       string `json:"msg_type"`
					ReceiveID     string `json:"receive_id"`
					ReplyInThread bool   `json:"reply_in_thread"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode card request: %v", err)
					http.Error(w, "invalid JSON", http.StatusBadRequest)
					return
				}
				if body.MsgType != "interactive" || body.ReplyInThread != tc.thread {
					t.Errorf("card request lost type/thread routing: %+v", body)
				}
				if tc.send && !tc.thread && (body.ReceiveID != rc.chatID || r.URL.Query().Get("receive_id_type") != "chat_id") {
					t.Errorf("SendCard changed its chat destination: %+v, query %s", body, r.URL.RawQuery)
				}
				emitted <- body.Content
				writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"message_id": "om_settings"}})
			}))
			t.Cleanup(server.Close)

			p, received := newAllowFromTestPlatform(t, "*")
			p.BindProject("private-settings")
			p.domain = server.URL
			p.client = resourceTestPlatform(server).client
			card := core.NewCard().
				Select("Chat model", []core.CardSelectOption{{Text: "Model A", Value: "cmd:/model model-A"}}, "").
				Select("Chat effort", []core.CardSelectOption{{Text: "High", Value: "cmd:/reasoning chat high"}}, "").
				Build()
			send := p.ReplyCard
			if tc.send {
				send = p.SendCard
			}
			if err := send(context.Background(), rc, card); err != nil {
				t.Fatal(err)
			}

			var payload struct {
				Elements []struct {
					Actions []struct {
						Tag     string         `json:"tag"`
						Value   map[string]any `json:"value"`
						Options []struct {
							Value string `json:"value"`
						} `json:"options"`
					} `json:"actions"`
				} `json:"elements"`
			}
			if err := json.Unmarshal([]byte(<-emitted), &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Elements) != 2 {
				t.Fatalf("emitted card has %d elements, want two settings selectors", len(payload.Elements))
			}
			for _, element := range payload.Elements {
				if len(element.Actions) != 1 {
					t.Fatalf("settings element has %d actions, want one", len(element.Actions))
				}
				action := element.Actions[0]
				if action.Tag != "select_static" || len(action.Options) != 1 {
					t.Fatalf("unexpected settings control: %+v", action)
				}
				if action.Value[cardActionDirectUserKey] != rc.directUserID || action.Value[cardActionProjectKey] != "private-settings" || action.Value["session_key"] != rc.sessionKey {
					t.Fatalf("emitted settings callback lost owner/project/session: %+v", action.Value)
				}

				// Replay the actual selector metadata and option, not a hand-bound fixture.
				p.allowChat = "oc_allowed_group"
				owner := cardActionEvent(rc.directUserID, "")
				owner.Event.Context.OpenMessageID = "om_settings"
				owner.Event.Action.Value = action.Value
				owner.Event.Action.Option = action.Options[0].Value
				if _, err := p.onCardAction(owner); err != nil {
					t.Fatal(err)
				}
				msg := waitAnyMessage(t, received, 2*time.Second, "owner with group-only allow_chat")
				if !msg.IsDirect || !msg.IsCardAction || msg.UserID != rc.directUserID || msg.SessionKey != rc.sessionKey || msg.GroupID != "" || msg.Content != strings.TrimPrefix(action.Options[0].Value, "cmd:") {
					t.Fatalf("settings callback lost private command identity: %+v", msg)
				}

				p.allowChat = "*"
				other := cardActionEvent("ou_other", "")
				other.Event.Context.OpenMessageID = "om_settings"
				other.Event.Action.Value = action.Value
				other.Event.Action.Option = action.Options[0].Value
				if _, err := p.onCardAction(other); err != nil {
					t.Fatal(err)
				}
				waitNoMessage(t, received, 100*time.Millisecond, "other operator with wildcard allow_chat")
			}
		})
	}
}
