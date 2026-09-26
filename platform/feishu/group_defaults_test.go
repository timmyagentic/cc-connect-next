package feishu

import (
	"context"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestGroupDefaultsDispatchDifferentUsersWithTheirOwnIdentity(t *testing.T) {
	created, err := New(map[string]any{"app_id": "cli_test", "app_secret": "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	p := created.(*interactivePlatform)
	botID, mentionKey := "ou_bot", "@_user_1"
	p.botOpenID = botID
	for _, user := range []string{"ou_alice", "ou_bob"} {
		p.userNameCache.Store(user, user)
	}
	p.chatNameCache.Store("oc_group", "Group")
	p.chatNameCache.Store("oc_other", "Other group")
	received := make(chan *core.Message, 1)
	p.handler = func(_ core.Platform, msg *core.Message) { received <- msg }
	for i, input := range []struct{ chat, user, key string }{
		{"oc_group", "ou_alice", "feishu:oc_group"},
		{"oc_group", "ou_bob", "feishu:oc_group"},
		{"oc_other", "ou_bob", "feishu:oc_other"},
	} {
		messageID := []string{"om_first", "om_second", "om_third"}[i]
		group, text, sender, content := "group", "text", "user", `{"text":"@_user_1 continue the discussion"}`
		err := p.onMessage(context.Background(), &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &input.user}, SenderType: &sender},
			Message: &larkim.EventMessage{
				MessageId: &messageID, ChatId: &input.chat, ChatType: &group, MessageType: &text, Content: &content,
				Mentions: []*larkim.MentionEvent{{Key: &mentionKey, Id: &larkim.UserId{OpenId: &botID}}},
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case msg := <-received:
			if msg.SessionKey != input.key || msg.UserID != input.user || msg.ReplyCtx.(replyContext).sessionKey != input.key {
				t.Fatalf("dispatch crossed conversation/user identity: %#v", msg)
			}
		case <-time.After(time.Second):
			t.Fatal("mentioned group message was not dispatched")
		}
	}
}

func TestGroupDefaultsShareUsersAndIsolateTopics(t *testing.T) {
	for _, name := range []string{"feishu", "lark"} {
		t.Run(name, func(t *testing.T) {
			newPlatformForTest := func(overrides map[string]any) *Platform {
				opts := map[string]any{"app_id": "cli_test", "app_secret": "test-secret"}
				for key, value := range overrides {
					opts[key] = value
				}
				created, err := newPlatform(name, lark.FeishuBaseUrl, opts)
				if err != nil {
					t.Fatal(err)
				}
				return created.(*interactivePlatform).Platform
			}
			group, private := "group", "p2p"
			root, thread, otherRoot, otherThread := "om_root", "omt_topic", "om_other", "omt_other"
			plain := &larkim.EventMessage{ChatType: &group}
			topic := &larkim.EventMessage{ChatType: &group, RootId: &root, ThreadId: &thread}
			otherTopic := &larkim.EventMessage{ChatType: &group, RootId: &otherRoot, ThreadId: &otherThread}
			p := newPlatformForTest(nil)
			for _, user := range []string{"ou_alice", "ou_bob"} {
				if got := p.makeSessionKey(plain, "oc_group", user); got != name+":oc_group" {
					t.Errorf("ordinary group for %s = %q, want shared group", user, got)
				}
				if got := p.makeSessionKey(topic, "oc_group", user); got != name+":oc_group:root:om_root" {
					t.Errorf("topic for %s = %q, want shared isolated topic", user, got)
				}
			}
			if p.makeSessionKey(plain, "oc_group", "ou_alice") == p.makeSessionKey(plain, "oc_other", "ou_alice") {
				t.Error("different groups shared a session")
			}
			if p.makeSessionKey(topic, "oc_group", "ou_alice") == p.makeSessionKey(otherTopic, "oc_group", "ou_alice") {
				t.Error("different topics shared a session")
			}
			if got := p.makeSessionKey(&larkim.EventMessage{ChatType: &private}, "oc_private", "ou_alice"); got != name+":oc_private:ou_alice" {
				t.Errorf("omitted sharing changed ordinary private history binding: %q", got)
			}
			for _, enabled := range []bool{false, true} {
				explicit := newPlatformForTest(map[string]any{"share_session_in_channel": enabled})
				alice := explicit.makeSessionKey(plain, "oc_group", "ou_alice")
				bob := explicit.makeSessionKey(plain, "oc_group", "ou_bob")
				if (alice == bob) != enabled {
					t.Errorf("explicit sharing=%t: Alice=%q Bob=%q", enabled, alice, bob)
				}
				if got := explicit.makeSessionKey(topic, "oc_group", "ou_bob"); got != name+":oc_group:root:om_root" {
					t.Errorf("explicit sharing=%t changed topic boundary: %q", enabled, got)
				}
				wantPrivate := name + ":oc_private:ou_alice"
				if enabled {
					wantPrivate = name + ":oc_private"
				}
				if got := explicit.makeSessionKey(&larkim.EventMessage{ChatType: &private}, "oc_private", "ou_alice"); got != wantPrivate {
					t.Errorf("explicit sharing=%t changed legacy private binding: %q", enabled, got)
				}
			}
			for _, off := range []any{false, "off"} {
				explicit := newPlatformForTest(map[string]any{"thread_isolation": off})
				if got := explicit.makeSessionKey(topic, "oc_group", "ou_bob"); got != name+":oc_group" {
					t.Errorf("explicit thread opt-out %v ignored: %q", off, got)
				}
			}
		})
	}
}

func TestGroupDefaultsCardActionsKeepBoundSession(t *testing.T) {
	created, err := New(map[string]any{"app_id": "cli_test", "app_secret": "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	p := created.(*interactivePlatform)
	if got := p.sessionKeyFromCardAction("oc_group", "ou_bob", nil); got != "feishu:oc_group" {
		t.Fatalf("unbound group action = %q, want default shared group", got)
	}
	for _, key := range []string{"feishu:oc_group:ou_alice", "feishu:oc_private:ou_alice", "feishu:oc_group:root:om_root"} {
		if got := p.sessionKeyFromCardAction("oc_group", "ou_alice", map[string]any{"session_key": key}); got != key {
			t.Errorf("bound action moved from %q to %q", key, got)
		}
	}
	if got := p.sessionKeyFromCardAction("oc_private", "ou_alice", map[string]any{cardActionDirectUserKey: "ou_alice"}); got != "feishu:oc_private:ou_alice" {
		t.Errorf("direct action fallback = %q, want private session", got)
	}
}
