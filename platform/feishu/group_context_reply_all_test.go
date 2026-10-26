package feishu

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestGroupContextAcceptsConfiguredReplyTriggers(t *testing.T) {
	for _, options := range []map[string]any{
		{"group_reply_all": true},
		{"group_reply_all_chats": []string{"oc_group"}},
		{"group_reply_all_chats": "oc_group"},
		{"require_mention": false},
		{"respond_to_at_everyone_and_here": true},
	} {
		for _, name := range []string{"feishu", "lark"} {
			opts := map[string]any{"app_id": "cli_test", "app_secret": "synthetic-secret", "group_context": true}
			for key, value := range options {
				opts[key] = value
			}
			if err := validatePlatformOptions(name)(opts); err != nil {
				t.Errorf("%s rejected compatible reply trigger %v: %v", name, options, err)
			}
			if _, err := newPlatform(name, lark.FeishuBaseUrl, opts); err != nil {
				t.Errorf("%s could not start with reply trigger %v: %v", name, options, err)
			}
		}
	}
}

func TestGroupContextPreservesReplyAllScopeAndEveryoneOptIn(t *testing.T) {
	p := &Platform{platformName: "feishu", botOpenID: "ou_bot", groupContext: groupContextConfig{enabled: true},
		groupReplyAll: true, groupReplyAllChats: map[string]struct{}{"oc_selected": {}},
	}
	chatType := "group"
	msg := &larkim.EventMessage{ChatType: &chatType}
	for _, kind := range []string{"text", "image", "file", "audio"} {
		if !p.shouldDispatchGroupMessage(msg, kind, "oc_selected", "selected", p.groupReplyAllForChat("oc_selected")) {
			t.Errorf("context suppressed configured reply-all %s", kind)
		}
		if p.shouldDispatchGroupMessage(msg, kind, "oc_other", "other", p.groupReplyAllForChat("oc_other")) {
			t.Errorf("reply-all escaped the selected chats for %s", kind)
		}
	}
	body := `{"text":"@_all summarize this"}`
	msg.Content = &body
	if p.shouldDispatchGroupMessage(msg, "text", "oc_other", "other", false) {
		t.Fatal("@all triggered a reply without opt-in")
	}
	p.respondToAtEveryoneAndHere = true
	if !p.shouldDispatchGroupMessage(msg, "text", "oc_other", "other", false) {
		t.Fatal("context suppressed configured @all replies")
	}
	// Reply-all does not rely on bot mention resolution; mention-gated chats do.
	p.botOpenID = ""
	if !p.shouldDispatchGroupMessage(msg, "text", "oc_selected", "selected", true) {
		t.Fatal("reply-all incorrectly required a resolved bot mention identity")
	}
	if p.shouldDispatchGroupMessage(msg, "text", "oc_other", "other", false) {
		t.Fatal("unresolved bot identity bypassed the mention gate")
	}
}

func TestGroupContextReplyAllEventReachesLoaderAndPreservesChatAccess(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("container_id") != "oc_group" {
			t.Errorf("read outside the triggering chat: %s", r.URL)
		}
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{
			contextAPIMessage("earlier", "oc_group", "", "ou_bob", "Bob", "Keep the chosen release date", now.Add(-time.Minute)),
		}, "has_more": false}})
	})
	p.dedup = &core.MessageDedup{}
	p.groupReplyAllChats = map[string]struct{}{"oc_group": {}, "oc_denied": {}}
	p.threadMode = threadIsolationTopicsOnly
	p.defaultGroupSharing = true
	p.allowFrom = "ou_bob"
	p.userNameCache.Store("ou_bob", "Bob")
	p.chatNameCache.Store("oc_group", "Group")
	wrapper := &interactivePlatform{Platform: p}
	p.self = wrapper
	received := make(chan *core.Message, 2)
	p.handler = func(_ core.Platform, msg *core.Message) { received <- msg }
	chat, chatType, user, kind, senderType := "oc_denied", "group", "ou_bob", "text", "user"
	body, id := `{"text":"Continue the plan"}`, "denied"
	created := strconv.FormatInt(now.UnixMilli(), 10)
	msg := &larkim.EventMessage{ChatId: &chat, ChatType: &chatType, MessageId: &id, MessageType: &kind, Content: &body, CreateTime: &created}
	event := &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &user}, SenderType: &senderType}, Message: msg}}
	if err := p.onMessage(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	select {
	case <-received:
		t.Fatal("reply-all bypassed allow_chat")
	default:
	}
	chat, id = "oc_group", "accepted"
	if err := p.onMessage(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if got.Content != "Continue the plan" || calls.Load() != 0 {
			t.Fatalf("unexpected inbound behavior: %q, reads=%d", got.Content, calls.Load())
		}
		if !wrapper.MessageContextEnabled(got.ReplyCtx) {
			t.Fatal("accepted reply-all event lost context eligibility")
		}
		background, err := wrapper.LoadMessageContext(context.Background(), got.ReplyCtx)
		if err != nil || len(background.Messages) != 1 || background.Messages[0].Text != "Keep the chosen release date" {
			t.Fatalf("reply-all context unavailable: %+v, %v", background, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("unmentioned reply-all event never reached core")
	}
}
