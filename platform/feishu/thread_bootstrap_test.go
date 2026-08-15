package feishu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestMarkThreadSessionActiveReportsFirstActivation(t *testing.T) {
	p := &Platform{threadIsolation: true}
	const key = "feishu:oc_chat:root:om_root"
	if !p.markThreadSessionActive(key) {
		t.Fatal("first activation must request thread bootstrap")
	}
	if p.markThreadSessionActive(key) {
		t.Fatal("subsequent activation must not bootstrap again")
	}
}

func TestOnMessageThreadIsolationBootstrapsExistingRoot(t *testing.T) {
	const (
		appID        = "cli_thread_bootstrap"
		appSecret    = "secret-thread-bootstrap"
		botOpenID    = "ou_bot"
		userOpenID   = "ou_user"
		chatID       = "oc_chat"
		rootMsgID    = "om_root"
		triggerMsgID = "om_trigger"
	)

	got := make(chan *core.Message, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			writeJSON(t, w, map[string]any{
				"code": 0, "msg": "success", "expire": 7200,
				"tenant_access_token": "tenant-token",
			})
		case r.URL.Path == "/open-apis/im/v1/messages/"+rootMsgID:
			writeJSON(t, w, map[string]any{
				"code": 0, "msg": "success",
				"data": map[string]any{"items": []map[string]any{{
					"msg_type": "post", "parent_id": "",
					"sender": map[string]any{"id": "ou_root_author", "sender_type": "user"},
					"body":   map[string]any{"content": `{"title":"环境信息","content":[[{"tag":"text","text":"根消息上下文"}]]}`},
				}}},
			})
		case strings.HasPrefix(r.URL.Path, "/open-apis/contact/v3/users/"):
			writeJSON(t, w, map[string]any{"code": 0, "msg": "success"})
		case strings.HasPrefix(r.URL.Path, "/open-apis/im/v1/chats/"):
			writeJSON(t, w, map[string]any{"code": 0, "msg": "success"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := &Platform{
		platformName:    "feishu",
		domain:          srv.URL,
		appID:           appID,
		appSecret:       appSecret,
		botOpenID:       botOpenID,
		threadIsolation: true,
		dedup:           &core.MessageDedup{},
		client: lark.NewClient(appID, appSecret,
			lark.WithOpenBaseUrl(srv.URL),
			lark.WithHttpClient(srv.Client()),
		),
		handler: func(_ core.Platform, msg *core.Message) { got <- msg },
	}

	chatType := "group"
	senderType := "user"
	msgType := "text"
	content := `{"text":"@_user_1 看看这个"}`
	createTime := strconv.FormatInt(time.Now().Add(time.Second).UnixMilli(), 10)
	threadID := "omt_thread"
	if err := p.onMessage(context.Background(), &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: strPtr(userOpenID)},
				SenderType: &senderType,
			},
			Message: &larkim.EventMessage{
				MessageId: strPtr(triggerMsgID), RootId: strPtr(rootMsgID), ThreadId: &threadID,
				ChatId: strPtr(chatID), ChatType: &chatType, MessageType: &msgType,
				Content: &content, CreateTime: &createTime,
				Mentions: []*larkim.MentionEvent{{
					Key: strPtr("@_user_1"), Id: &larkim.UserId{OpenId: strPtr(botOpenID)}, Name: strPtr("Bot"),
				}},
			},
		},
	}); err != nil {
		t.Fatalf("onMessage() error = %v", err)
	}

	select {
	case msg := <-got:
		if msg.SessionKey != "feishu:"+chatID+":root:"+rootMsgID {
			t.Fatalf("SessionKey = %q, want root-scoped session", msg.SessionKey)
		}
		if msg.Content != "看看这个" {
			t.Fatalf("Content = %q, want trigger text", msg.Content)
		}
		if !strings.Contains(msg.ExtraContent, "根消息上下文") {
			t.Fatalf("ExtraContent = %q, want root context", msg.ExtraContent)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for bootstrapped message")
	}
}
