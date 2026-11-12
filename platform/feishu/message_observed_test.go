package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func observationPlatform() *Platform {
	return &Platform{platformName: "feishu", allowFrom: "*", allowChat: "*",
		botOpenID: "ou_bot", dedup: &core.MessageDedup{}}
}

func observationEvent(kind string) *larkim.P2MessageReceiveV1 {
	return &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{
		Sender: &larkim.EventSender{SenderType: stringPtr("user"), SenderId: &larkim.UserId{OpenId: stringPtr("ou_sender")}},
		Message: &larkim.EventMessage{
			MessageId: stringPtr("om_source"), ChatId: stringPtr("oc_source"), ChatType: stringPtr("group"),
			MessageType: stringPtr(kind), CreateTime: stringPtr(strconv.FormatInt(time.Now().UnixMilli(), 10)),
			Content:  stringPtr(`{"text":"@_user_1 private body"}`),
			Mentions: []*larkim.MentionEvent{{Id: &larkim.UserId{OpenId: stringPtr("ou_recipient")}}},
		},
	}}
}

func TestMessageObserved_PersonMentionDoesNotStartAgent(t *testing.T) {
	for _, kind := range []string{"text", "post"} {
		t.Run(kind, func(t *testing.T) {
			p := observationPlatform()
			var agentCalls atomic.Int32
			p.handler = func(core.Platform, *core.Message) { agentCalls.Add(1) }
			event := observationEvent(kind)
			msg := event.Event.Message
			msg.ThreadId, msg.RootId, msg.ParentId = stringPtr("omt_topic"), stringPtr("om_root"), stringPtr("om_parent")
			if kind == "post" {
				msg.Content = stringPtr(`{"zh_cn":{"title":"private title","content":[[{"tag":"at","user_id":"ou_recipient"}]]}}`)
			}
			var got []core.HookEvent
			p.SetMessageObserver(func(e core.HookEvent) error { got = append(got, e); return nil })
			if err := p.onMessage(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || agentCalls.Load() != 0 {
				t.Fatalf("observations=%d agent calls=%d", len(got), agentCalls.Load())
			}
			created, _ := strconv.ParseInt(*msg.CreateTime, 10, 64)
			want := map[string]any{"message_id": "om_source", "chat_id": "oc_source", "chat_type": "group",
				"created_at_ms": created, "thread_id": "omt_topic", "root_id": "om_root", "parent_id": "om_parent",
				"sender_type": "user", "mentioned_user_ids": []string{"ou_recipient"}}
			if got[0].Event != core.HookEventMessageObserved || got[0].Platform != "feishu" ||
				got[0].UserID != "ou_sender" || got[0].Content != "" || got[0].UserName != "" || !reflect.DeepEqual(want, got[0].Extra) {
				t.Fatalf("observation must contain only exact routing metadata: %#v", got[0])
			}
		})
	}
}

func TestMessageObserved_OnlyExplicitHumanMentionsWithinAccess(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Platform, *larkim.P2MessageReceiveV1)
	}{
		{"bot sender", func(_ *Platform, e *larkim.P2MessageReceiveV1) { e.Event.Sender.SenderType = stringPtr("app") }},
		{"at all", func(_ *Platform, e *larkim.P2MessageReceiveV1) {
			e.Event.Message.Mentions = nil
			e.Event.Message.Content = stringPtr(`{"text":"@_all"}`)
		}},
		{"sender denied", func(p *Platform, _ *larkim.P2MessageReceiveV1) { p.allowFrom = "ou_other" }},
		{"chat denied", func(p *Platform, _ *larkim.P2MessageReceiveV1) { p.allowChat = "oc_other" }},
		{"group only", func(p *Platform, e *larkim.P2MessageReceiveV1) {
			p.groupOnly = true
			e.Event.Message.ChatType = stringPtr("p2p")
			// Preserve the legacy old-message gate; this test exercises only the observer's scope.
			e.Event.Message.CreateTime = stringPtr(strconv.FormatInt(core.StartTime.Add(-time.Hour).UnixMilli(), 10))
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p, event := observationPlatform(), observationEvent("text")
			tt.change(p, event)
			calls := 0
			p.SetMessageObserver(func(core.HookEvent) error { calls++; return nil })
			if err := p.onMessage(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			if calls != 0 {
				t.Fatalf("out-of-scope message observed %d times", calls)
			}
		})
	}
}

func TestMessageObserved_FailedPersistenceCanRetryBeforeAgentDedup(t *testing.T) {
	p, event := observationPlatform(), observationEvent("text")
	failure := errors.New("could not persist pending message")
	calls := 0
	p.SetMessageObserver(func(core.HookEvent) error {
		calls++
		if calls == 1 {
			return failure
		}
		return nil
	})
	if err := p.onMessage(context.Background(), event); !errors.Is(err, failure) {
		t.Fatalf("first attempt must fail for transport retry: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := p.onMessage(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatalf("Agent dedup swallowed an observer retry: %d observations", calls)
	}
}

func TestMessageObserved_RestartReplayStillReachesDurableConsumer(t *testing.T) {
	p, event := observationPlatform(), observationEvent("text")
	event.Event.Message.CreateTime = stringPtr(strconv.FormatInt(core.StartTime.Add(-time.Hour).UnixMilli(), 10))
	event.Event.Message.Mentions = append(event.Event.Message.Mentions, &larkim.MentionEvent{Id: &larkim.UserId{OpenId: stringPtr("ou_bot")}})
	observed := 0
	var agentCalls atomic.Int32
	p.handler = func(core.Platform, *core.Message) { agentCalls.Add(1) }
	p.SetMessageObserver(func(core.HookEvent) error { observed++; return nil })
	if err := p.onMessage(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if observed != 1 || agentCalls.Load() != 0 {
		t.Fatalf("old message: observations=%d Agent calls=%d", observed, agentCalls.Load())
	}
}

func TestMessageObserved_SharedFanoutReturnsFailureAndRetriesEveryProject(t *testing.T) {
	first, second := observationPlatform(), observationPlatform()
	first.sharedGroup = &sharedWSGroup{platforms: []*Platform{first, second}}
	event := observationEvent("text")
	failure := errors.New("pending store unavailable")
	firstCalls, secondCalls := 0, 0
	first.SetMessageObserver(func(core.HookEvent) error { firstCalls++; return nil })
	second.SetMessageObserver(func(core.HookEvent) error {
		secondCalls++
		if secondCalls == 1 {
			return failure
		}
		return nil
	})
	payload, err := json.Marshal(map[string]any{"schema": "2.0", "header": map[string]string{"event_type": "im.message.receive_v1"}, "event": event.Event})
	if err != nil {
		t.Fatal(err)
	}
	transport := dispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(first.onSharedMessage)
	if _, err := transport.Do(context.Background(), payload); !errors.Is(err, failure) {
		t.Fatalf("shared dispatcher must return hook failure to SDK: %v", err)
	}
	if _, err := transport.Do(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if firstCalls != 2 || secondCalls != 2 {
		t.Fatalf("fanout retries: first=%d second=%d", firstCalls, secondCalls)
	}
}

func TestMessageObserved_NoObserverKeepsReplyGateAndSkipsMalformedMetadata(t *testing.T) {
	p, event := observationPlatform(), observationEvent("text")
	event.Event.Message.CreateTime = stringPtr("not a timestamp")
	if err := p.onMessage(context.Background(), event); err != nil {
		t.Fatalf("unconfigured observer changed existing handling: %v", err)
	}
	p.SetMessageObserver(func(core.HookEvent) error { t.Fatal("invalid mentions observed"); return nil })
	event.Event.Message.Mentions = []*larkim.MentionEvent{nil, {}, {Id: &larkim.UserId{}}, {Id: &larkim.UserId{OpenId: stringPtr("all")}}}
	if err := p.observeMessage(event.Event.Message, event.Event.Sender); err != nil {
		t.Fatal(err)
	}
}
