package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestGroupContextConfigurationBoundsAndConflicts(t *testing.T) {
	cfg, err := parseGroupContextConfig(nil)
	if err != nil || cfg.enabled || cfg.maxMessages != 20 || cfg.minutes != 30 || cfg.maxChars != 8000 {
		t.Fatalf("omitted defaults = %+v, %v", cfg, err)
	}
	for _, bad := range []map[string]any{
		{"group_context": "true"}, {"group_context_max_messages": 0}, {"group_context_max_messages": 101},
		{"group_context_max_messages": 1.5}, {"group_context_window_minutes": 1441},
		{"group_context_max_chars": 511}, {"group_context_max_chars": 32001},
		{"group_context_max_chars": "8000"},
		{"group_context": true, "group_reply_all": true},
		{"group_context": true, "group_reply_all_chats": "oc_somewhere"},
		{"group_context": true, "require_mention": false},
		{"group_context": true, "respond_to_at_everyone_and_here": true},
	} {
		for _, name := range []string{"feishu", "lark"} {
			opts := map[string]any{"app_id": "cli_test", "app_secret": "synthetic-secret"}
			for key, value := range bad {
				opts[key] = value
			}
			if err := validatePlatformOptions(name)(opts); err == nil {
				t.Errorf("side-effect-free validation accepted %v", bad)
			}
			if _, err := newPlatform(name, lark.FeishuBaseUrl, opts); err == nil {
				t.Errorf("runtime accepted %v", bad)
			}
		}
	}
	for _, option := range feishuConfigOptions(lark.FeishuBaseUrl) {
		if !strings.HasPrefix(option.Key, "group_context") {
			continue
		}
		if option.DefaultSource != core.ConfigDefaultBuiltin || option.ApplyMode != core.ConfigApplyRestart || option.Example == "" {
			t.Fatalf("incomplete config contract: %+v", option)
		}
		if option.Key != "group_context" && (option.Minimum == nil || option.Maximum == nil || option.Type != "integer") {
			t.Fatalf("missing numeric bounds: %+v", option)
		}
	}
}

func contextAPIPlatform(t *testing.T, handler http.HandlerFunc) (*Platform, *atomic.Int64) {
	t.Helper()
	requests := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/auth/") {
			writeJSON(t, w, map[string]any{"code": 0, "expire": 7200, "tenant_access_token": "synthetic-context-token"})
			return
		}
		requests.Add(1)
		if r.URL.Path != "/open-apis/im/v1/messages" {
			t.Errorf("context must not read contacts, other chats or attachments: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	cfg, _ := parseGroupContextConfig(map[string]any{"group_context": true})
	p := &Platform{platformName: "feishu", appID: "cli_context", groupContext: cfg, botOpenID: "ou_bot", allowChat: "oc_group",
		client: lark.NewClient("cli_context", "synthetic-secret", lark.WithOpenBaseUrl(server.URL), lark.WithHttpClient(server.Client()), lark.WithEnableTokenCache(false)),
	}
	return p, requests
}

func contextAPIMessage(id, chat, thread, user, name, text string, at time.Time) map[string]any {
	body, _ := json.Marshal(map[string]string{"text": text})
	return map[string]any{"message_id": id, "chat_id": chat, "thread_id": thread, "msg_type": "text", "create_time": strconv.FormatInt(at.UnixMilli(), 10),
		"sender": map[string]string{"id": user, "sender_type": "user", "sender_name": name}, "body": map[string]string{"content": string(body)}}
}

func TestGroupContextReadsOnlyMatchingChatOrRealTopic(t *testing.T) {
	for _, thread := range []string{"", "omt_topic"} {
		t.Run("thread="+thread, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Millisecond)
			a := contextAPIMessage("om_a", "oc_group", thread, "ou_a", "Alice", "The old budget was 10", now.Add(-2*time.Minute))
			b := contextAPIMessage("om_b", "oc_group", thread, "ou_b", "Bob", "The approved budget is now 20", now.Add(-time.Minute))
			b["parent_id"] = "om_a"
			deleted := contextAPIMessage("om_deleted", "oc_group", thread, "ou_b", "Bob", "withdrawn", now.Add(-time.Second))
			deleted["deleted"] = true
			bot := contextAPIMessage("om_bot", "oc_group", thread, "cli_bot", "Bot", "already known answer", now.Add(-time.Minute))
			bot["sender"] = map[string]string{"id": "cli_bot", "sender_type": "app"}
			items := []any{b, a, deleted, bot,
				contextAPIMessage("om_current", "oc_group", thread, "ou_b", "Bob", "what do you think?", now),
				contextAPIMessage("om_other_chat", "oc_other", thread, "ou_a", "Alice", "private other group", now.Add(-time.Minute)),
				contextAPIMessage("om_other_topic", "oc_group", "omt_other", "ou_a", "Alice", "private other topic", now.Add(-time.Minute)),
				contextAPIMessage("om_old", "oc_group", thread, "ou_a", "Alice", "too old", now.Add(-time.Hour)),
				contextAPIMessage("om_future", "oc_group", thread, "ou_a", "Alice", "after trigger", now.Add(time.Second)),
			}
			p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				wantType, wantID := "chat", "oc_group"
				if thread != "" {
					wantType, wantID = "thread", thread
				}
				if q.Get("container_id_type") != wantType || q.Get("container_id") != wantID || q.Get("sort_type") != "ByCreateTimeDesc" || q.Get("with_sender_name") != "true" {
					t.Errorf("wrong history request: %s", r.URL)
				}
				if (thread != "") != (q.Get("start_time") == "" && q.Get("end_time") == "") {
					t.Errorf("thread time filters must be local: %s", r.URL)
				}
				writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": items, "has_more": false}})
			})
			rctx := replyContext{chatID: "oc_group", threadID: thread, messageID: "om_current", contextEligible: true, contextTimeMs: now.UnixMilli()}
			got, err := p.LoadMessageContext(context.Background(), rctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Messages) != 2 || got.Messages[0].ID != "om_a" || got.Messages[1].ID != "om_b" || got.Messages[1].SenderName != "Bob" || got.Messages[1].ReplyTo != "om_a" {
				t.Fatalf("wrong attribution or scope: %+v", got)
			}
			if got.Messages[0].Time != now.Add(-2*time.Minute) || calls.Load() != 1 {
				t.Fatalf("unexpected timestamps/reads: %+v", got)
			}
			for _, target := range []any{replyContext{chatID: "oc_group"}, replyContext{chatID: "oc_other", contextEligible: true}, "synthetic target"} {
				before := calls.Load()
				if c, err := p.LoadMessageContext(context.Background(), target); err != nil || len(c.Messages) != 0 || calls.Load() != before {
					t.Fatal("non-mention/private/unauthorized target caused a read")
				}
			}
		})
	}
}

func TestGroupContextBoundsPagingAndDisclosesUnavailableText(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page_token")
		items := []any{}
		if page == "" {
			file := contextAPIMessage("om_file", "oc_group", "", "ou_a", "Alice", "ignored", now.Add(-time.Second))
			file["msg_type"] = "file"
			items = append(items, file)
		} else {
			for i := 0; i < 4; i++ {
				items = append(items, contextAPIMessage(fmt.Sprintf("om_%d", i), "oc_group", "", "ou_a", "Alice", fmt.Sprintf("fact %d", i), now.Add(-time.Duration(i+1)*time.Minute)))
			}
		}
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": items, "has_more": page == "", "page_token": "page-2"}})
	})
	p.groupContext.maxMessages = 2
	got, err := p.LoadMessageContext(context.Background(), replyContext{chatID: "oc_group", messageID: "trigger", contextEligible: true, contextTimeMs: now.UnixMilli()})
	if err != nil || !got.Incomplete || len(got.Messages) != 2 || got.Messages[0].ID != "om_1" || got.Messages[1].ID != "om_0" || calls.Load() != 2 {
		t.Fatalf("bounded newest context = %+v, calls=%d, err=%v", got, calls.Load(), err)
	}
}

func TestGroupContextHistoryFailureAndPagingBudget(t *testing.T) {
	for _, mode := range []string{"permission", "endless", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if mode == "permission" {
					writeJSON(t, w, map[string]any{"code": 99991672, "msg": "missing permission"})
					return
				}
				if mode == "cancelled" {
					<-r.Context().Done()
					return
				}
				writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{}, "has_more": true, "page_token": r.URL.Query().Get("page_token") + "x"}})
			})
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			got, err := p.LoadMessageContext(ctx, replyContext{chatID: "oc_group", messageID: "trigger", contextEligible: true, contextTimeMs: time.Now().UnixMilli()})
			if mode == "endless" {
				if err != nil || !got.Incomplete || calls.Load() != groupContextMaxPages {
					t.Fatalf("unbounded paging: %+v, %v, %d", got, err, calls.Load())
				}
			} else if err == nil {
				t.Fatal("unreadable history was reported complete")
			}
		})
	}
}

func TestGroupContextRequiresExplicitMentionIncludingAttachments(t *testing.T) {
	p := &Platform{platformName: "feishu", botOpenID: "ou_bot", threadMode: threadIsolationTopicsOnly, groupContext: groupContextConfig{enabled: true}}
	chatType := "group"
	p.activeThreadSessions.Store("feishu:oc_group:root:om_root", time.Now())
	for _, kind := range []string{"text", "image", "file", "audio"} {
		msg := &larkim.EventMessage{ChatType: &chatType}
		if p.shouldDispatchGroupMessage(msg, kind, "oc_group", "feishu:oc_group:root:om_root", false) {
			t.Fatalf("unmentioned %s triggered a reply", kind)
		}
		bot := "ou_bot"
		msg.Mentions = []*larkim.MentionEvent{{Id: &larkim.UserId{OpenId: &bot}}}
		if !p.shouldDispatchGroupMessage(msg, kind, "oc_group", "feishu:oc_group:root:om_root", false) {
			t.Fatalf("mentioned %s ignored", kind)
		}
	}
	p.botOpenID = ""
	if p.shouldDispatchGroupMessage(&larkim.EventMessage{ChatType: &chatType}, "text", "oc_group", "key", false) {
		t.Fatal("unknown bot identity bypassed mention verification")
	}
}

func TestGroupContextCapabilityReportsConfigurationAndTarget(t *testing.T) {
	p := &Platform{}
	if got := p.RuntimeCapabilityAvailability("", nil)["recent_conversation_context"]; got.State != core.CapabilityUnavailable {
		t.Fatal(got)
	}
	p.groupContext.enabled = true
	if got := p.RuntimeCapabilityAvailability("", nil)["recent_conversation_context"]; got.State != core.CapabilityConditional {
		t.Fatal(got)
	}
	if got := p.RuntimeCapabilityAvailability("", replyContext{directUserID: "ou_user"})["recent_conversation_context"]; got.State != core.CapabilityUnavailable {
		t.Fatal(got)
	}
	if reflect.TypeOf(&interactivePlatform{Platform: p}).Implements(reflect.TypeOf((*core.MessageContextProvider)(nil)).Elem()) != true {
		t.Fatal("card wrapper lost context capability")
	}
}

func TestGroupContextPartialFailurePreservesTimeOrder(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page_token") != "" {
			writeJSON(t, w, map[string]any{"code": 99991672})
			return
		}
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{
			contextAPIMessage("new", "oc_group", "", "ou_b", "Bob", "newer", now.Add(-time.Minute)),
			contextAPIMessage("old", "oc_group", "", "ou_a", "Alice", "older", now.Add(-2*time.Minute)),
		}, "has_more": true, "page_token": "next"}})
	})
	got, err := p.LoadMessageContext(context.Background(), replyContext{chatID: "oc_group", contextEligible: true, contextTimeMs: now.UnixMilli()})
	if err == nil || !got.Incomplete || calls.Load() != 2 || len(got.Messages) != 2 || got.Messages[0].ID != "old" || got.Messages[1].ID != "new" {
		t.Fatalf("partial context lost order/bounds: %+v err=%v", got, err)
	}
}

func TestGroupContextMentionEventReachesLoaderThroughCardWrapper(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("container_id_type") != "thread" || r.URL.Query().Get("container_id") != "omt_topic" {
			t.Errorf("lost actual thread: %s", r.URL)
		}
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{contextAPIMessage("discussion", "oc_group", "omt_topic", "ou_alice", "Alice", "unmentioned fact", now.Add(-time.Minute))}, "has_more": false}})
	})
	p.dedup = &core.MessageDedup{}
	p.threadMode = threadIsolationTopicsOnly
	p.defaultGroupSharing = true
	p.userNameCache.Store("ou_bob", "Bob")
	p.chatNameCache.Store("oc_group", "Group")
	wrapper := &interactivePlatform{Platform: p}
	p.self = wrapper
	received := make(chan *core.Message, 1)
	p.handler = func(platform core.Platform, msg *core.Message) {
		if _, ok := platform.(core.MessageContextProvider); !ok {
			t.Error("wrapper dropped the context interface")
		}
		received <- msg
	}
	chat, chatType, thread, root, user, kind, senderType := "oc_group", "group", "omt_topic", "om_root", "ou_bob", "text", "user"
	body := `{"text":"@_user_1 what do you think?"}`
	created := strconv.FormatInt(now.UnixMilli(), 10)
	bot, key := p.botOpenID, "@_user_1"
	msg := &larkim.EventMessage{ChatId: &chat, ChatType: &chatType, ThreadId: &thread, RootId: &root, ParentId: &root, MessageType: &kind, Content: &body, CreateTime: &created}
	event := &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &user}, SenderType: &senderType}, Message: msg}}
	for _, id := range []string{"quiet-a", "quiet-b"} {
		msg.MessageId = &id
		if err := p.onMessage(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-received:
		t.Fatal("unmentioned discussion triggered an answer")
	default:
	}
	if calls.Load() != 0 {
		t.Fatal("unmentioned messages triggered history polling")
	}
	id := "mentioned"
	msg.MessageId = &id
	msg.Mentions = []*larkim.MentionEvent{{Key: &key, Id: &larkim.UserId{OpenId: &bot}}}
	if err := p.onMessage(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if got.Content != "what do you think?" || got.ExtraContent != "" || calls.Load() != 0 {
			t.Fatalf("event fetched unbounded quote context early: %+v calls=%d", got, calls.Load())
		}
		provider := core.MessageContextProvider(wrapper)
		if !provider.MessageContextEnabled(got.ReplyCtx) {
			t.Fatal("authorized event lost context eligibility")
		}
		background, err := provider.LoadMessageContext(context.Background(), got.ReplyCtx)
		if err != nil || len(background.Messages) != 1 || background.Messages[0].Text != "unmentioned fact" {
			t.Fatalf("event context unavailable: %+v %v", background, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mentioned event did not reach core")
	}
}
