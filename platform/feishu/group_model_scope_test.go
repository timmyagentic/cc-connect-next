package feishu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func newGroupModelScopeTestPlatform(t *testing.T, name string) (*Platform, <-chan *core.Message) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		if strings.Contains(r.URL.Path, "/resources/") {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nimage"))
			return
		}
		// A new topic may bootstrap its root before dispatching the trigger.
		if strings.HasPrefix(r.URL.Path, "/open-apis/im/v1/messages/") {
			writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []map[string]any{{
				"msg_type": "text", "parent_id": "",
				"sender": map[string]any{"id": "ou_alice", "sender_type": "user"},
				"body":   map[string]any{"content": `{"text":"topic root"}`},
			}}}})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	created, err := newPlatform(name, server.URL, map[string]any{
		"app_id": "cli_test", "app_secret": "test-secret", "group_reply_all": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	p := created.(*interactivePlatform).Platform
	p.client = resourceTestPlatform(server).client
	p.botOpenID = "ou_bot"
	p.imageBatchWindow = time.Hour
	for _, user := range []string{"ou_alice", "ou_bob"} {
		p.userNameCache.Store(user, user)
	}
	for _, chat := range []string{"oc_chat", "oc_other"} {
		p.chatNameCache.Store(chat, chat)
	}
	received := make(chan *core.Message, 8)
	// Observe the same handler boundary used by Engine.ReceiveMessage.
	p.handler = func(_ core.Platform, msg *core.Message) { received <- msg }
	t.Cleanup(p.flushImageBatches)
	return p, received
}

func dispatchGroupModelScopeEvent(t *testing.T, p *Platform, msg *larkim.EventMessage, user string) {
	t.Helper()
	if err := p.onMessage(context.Background(), &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{
		Sender:  &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &user}, SenderType: stringPtr("user")},
		Message: msg,
	}}); err != nil {
		t.Fatal(err)
	}
	// Wait for downloads and batch buffering without relying on timer sleeps.
	drained := make(chan struct{})
	p.enqueueInboundDispatch(p.makeSessionKey(msg, stringValue(msg.ChatId), user), func() { close(drained) })
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		t.Fatal("onMessage dispatch did not drain")
	}
}

func TestGroupModelScopeOnMessageDispatch(t *testing.T) {
	for _, platform := range []string{"feishu", "lark"} {
		t.Run(platform, func(t *testing.T) {
			for _, kind := range []string{"text", "image_batch"} {
				t.Run(kind, func(t *testing.T) {
					p, received := newGroupModelScopeTestPlatform(t, platform)
					for _, tc := range []struct {
						name, chat, chatType, user, root, thread string
						group, channel, legacy, session          string
					}{
						{name: "ordinary_group", chat: "oc_chat", chatType: "group", user: "ou_alice", group: "oc_chat", channel: "oc_chat", session: "oc_chat"},
						{name: "other_user", chat: "oc_chat", chatType: "group", user: "ou_bob", group: "oc_chat", channel: "oc_chat", session: "oc_chat"},
						{name: "first_topic", chat: "oc_chat", chatType: "group", user: "ou_alice", root: "om_first", thread: "omt_first", group: "oc_chat", channel: "oc_chat:topic:om_first", legacy: "oc_chat", session: "oc_chat:root:om_first"},
						{name: "second_topic", chat: "oc_chat", chatType: "group", user: "ou_bob", root: "om_second", thread: "omt_second", group: "oc_chat", channel: "oc_chat:topic:om_second", legacy: "oc_chat", session: "oc_chat:root:om_second"},
						{name: "other_group", chat: "oc_other", chatType: "group", user: "ou_alice", group: "oc_other", channel: "oc_other", session: "oc_other"},
						{name: "private", chat: "oc_chat", chatType: "p2p", user: "ou_alice", channel: "oc_chat", session: "oc_chat:ou_alice"},
						{name: "private_thread", chat: "oc_chat", chatType: "p2p", user: "ou_alice", root: "om_private", thread: "omt_private", channel: "oc_chat:topic:omt_private", legacy: "oc_chat", session: "oc_chat:thread:omt_private"},
						{name: "missing_chat_type", chat: "oc_chat", user: "ou_alice", root: "om_missing", thread: "omt_missing", channel: "oc_chat:topic:omt_missing", legacy: "oc_chat", session: "oc_chat:thread:omt_missing"},
						{name: "unknown_chat_type", chat: "oc_chat", chatType: "unknown", user: "ou_alice", root: "om_unknown", thread: "omt_unknown", channel: "oc_chat:topic:omt_unknown", legacy: "oc_chat", session: "oc_chat:thread:omt_unknown"},
					} {
						t.Run(tc.name, func(t *testing.T) {
							event := func(suffix, messageType, content string) *larkim.EventMessage {
								msg := &larkim.EventMessage{
									MessageId: stringPtr("om_" + tc.name + suffix), ChatId: &tc.chat,
									MessageType: &messageType, Content: &content, RootId: &tc.root, ThreadId: &tc.thread,
								}
								if tc.chatType != "" {
									msg.ChatType = &tc.chatType
								}
								return msg
							}
							sessionKey := platform + ":" + tc.session
							wantID := "om_" + tc.name
							if kind == "image_batch" {
								// Bootstrap the topic first so both images take the batch path.
								dispatchGroupModelScopeEvent(t, p, event("_start", "text", `{"text":"start"}`), tc.user)
								waitAnyMessage(t, received, 3*time.Second, "topic bootstrap")
								for i := 0; i < 2; i++ {
									dispatchGroupModelScopeEvent(t, p, event(fmt.Sprintf("_%d", i), "image", `{"image_key":"img_test"}`), tc.user)
								}
								select {
								case msg := <-received:
									t.Fatalf("image bypassed batching: %+v", msg)
								default:
								}
								p.flushImageBatchForSession(sessionKey)
								wantID += "_1"
							} else {
								dispatchGroupModelScopeEvent(t, p, event("", "text", `{"text":"continue"}`), tc.user)
							}
							msg := waitAnyMessage(t, received, 3*time.Second, "group scope dispatch")
							if msg.Platform != platform || msg.SessionKey != sessionKey || msg.UserID != tc.user || msg.MessageID != wantID {
								t.Fatalf("dispatch lost message identity: %+v", msg)
							}
							if msg.GroupID != tc.group || msg.ReplyCtx.(replyContext).groupID != tc.group {
								t.Errorf("GroupID = %q, reply groupID = %q, want %q", msg.GroupID, msg.ReplyCtx.(replyContext).groupID, tc.group)
							}
							if msg.ChannelKey != tc.channel || msg.LegacyChannelKey != tc.legacy {
								t.Errorf("workspace keys = (%q, %q), want (%q, %q)", msg.ChannelKey, msg.LegacyChannelKey, tc.channel, tc.legacy)
							}
							if kind == "image_batch" && len(msg.Images) != 2 {
								t.Errorf("batch contains %d images, want 2", len(msg.Images))
							}
							if kind == "text" && msg.Content != "continue" {
								t.Errorf("Content = %q, want continue", msg.Content)
							}
						})
					}
				})
			}
		})
	}
}

func TestGroupModelScopeCardCallbackWithoutChatTypeStaysEmpty(t *testing.T) {
	for _, name := range []string{"feishu", "lark"} {
		t.Run(name, func(t *testing.T) {
			p, received := newGroupModelScopeTestPlatform(t, name)
			for _, suffix := range []string{"", ":root:om_topic", ":thread:omt_private", ":ou_alice"} {
				sessionKey := name + ":oc_chat" + suffix
				event := cardActionEvent("ou_alice", "cmd:/model")
				event.Event.Context.OpenChatID = "oc_chat"
				event.Event.Action.Value["session_key"] = sessionKey
				if _, err := p.onCardAction(event); err != nil {
					t.Fatal(err)
				}
				msg := waitAnyMessage(t, received, 3*time.Second, "model card callback")
				if msg.SessionKey != sessionKey || !msg.IsCardAction || msg.Content != "/model" {
					t.Fatalf("callback changed command/session: %+v", msg)
				}
				if msg.GroupID != "" || msg.ReplyCtx.(replyContext).groupID != "" {
					t.Fatalf("callback inferred group from chat/session: %+v", msg)
				}
			}
		})
	}
}

func TestGroupModelNavigationUsesTopicAwareCommandPath(t *testing.T) {
	for _, name := range []string{"feishu", "lark"} {
		t.Run(name, func(t *testing.T) {
			p, received := newGroupModelScopeTestPlatform(t, name)
			p.cardNavHandler = func(string, string) *core.Card { t.Error("model navigation lost message context"); return nil }
			event := cardActionEvent("ou_alice", "nav:/model")
			event.Event.Context.OpenChatID = "oc_chat"
			event.Event.Action.Value["session_key"] = name + ":oc_chat:root:om_topic"
			if _, err := p.onCardAction(event); err != nil {
				t.Fatal(err)
			}
			msg := waitAnyMessage(t, received, 3*time.Second, "model navigation command")
			if msg.Content != "/model" || !msg.IsCardAction || msg.ChannelKey != "oc_chat:topic:om_topic" || msg.UserID != "ou_alice" {
				t.Fatalf("model navigation lost scope: %+v", msg)
			}
		})
	}
}
