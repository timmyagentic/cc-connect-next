package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestGroupContextPeersRequireExplicitOptInAndExactIdentities(t *testing.T) {
	for _, changes := range []map[string]any{
		{"group_context_include_peer_bots": "true"},
		{"group_context": false},
		{"peer_bots": nil},
		{"peer_bots": map[string]any{}},
		{"peer_bots": map[string]any{"cli_peer": 1}},
		{"peer_bots": map[string]any{"cli_peer": " "}},
		{"peer_bots": map[string]any{"cli_self": "Self"}},
		{"peer_bots": map[string]any{"": "Missing identity"}},
	} {
		opts := map[string]any{"app_id": "cli_self", "app_secret": "synthetic", "group_context": true, "group_context_include_peer_bots": true, "peer_bots": map[string]any{"cli_peer": "Peer"}}
		for k, v := range changes {
			opts[k] = v
		}
		for _, name := range []string{"feishu", "lark"} {
			if err := validatePlatformOptions(name)(opts); err == nil {
				t.Fatalf("validation accepted %+v", changes)
			}
			if _, err := newPlatform(name, lark.FeishuBaseUrl, opts); err == nil {
				t.Fatalf("runtime accepted %+v", changes)
			}
		}
	}
	for _, enabled := range []bool{false, true} {
		opts := map[string]any{"app_id": "cli_self", "app_secret": "synthetic", "enable_feishu_card": false, "group_context": true, "peer_bots": map[string]any{"cli_peer": "Peer"}}
		if enabled {
			opts["group_context_include_peer_bots"] = true
		}
		p, err := newPlatform("feishu", lark.FeishuBaseUrl, opts)
		if err != nil {
			t.Fatal(err)
		}
		if p.(*Platform).groupContext.includePeerBots != enabled {
			t.Fatal("peer aliases silently enabled sharing")
		}
	}
}

func peerContextCard(id, chat, thread, app, status, body string, at time.Time) map[string]any {
	m := contextAPIMessage(id, chat, thread, app, "untrusted alias", "", at)
	m["sender"] = map[string]string{"id": app, "sender_type": "app"}
	m["msg_type"] = "interactive"
	card := map[string]any{"schema": "2.0", "header": map[string]any{"title": map[string]string{"content": status}}, "body": map[string]any{"elements": []any{map[string]string{"tag": "markdown", "content": body}}}}
	content, _ := json.Marshal(card)
	m["body"] = map[string]string{"content": string(content)}
	return m
}

func TestGroupContextPeersReadPublicCardsOnlyInMatchingScope(t *testing.T) {
	for _, thread := range []string{"", "omt_topic"} {
		t.Run("thread="+thread, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Millisecond)
			at := now.Add(-time.Minute)
			good := peerContextCard("peer", "oc_group", thread, "cli_peer", "Completed", "The next step is verify deployment", at)
			deleted := peerContextCard("deleted", "oc_group", thread, "cli_peer", "Completed", "silent removed card", at)
			deleted["deleted"] = true
			items := []any{
				good, deleted,
				contextAPIMessage("human", "oc_group", thread, "ou_user", "User", "Please investigate", at.Add(-time.Second)),
				peerContextCard("self", "oc_group", thread, "cli_context", "Completed", "self", at),
				peerContextCard("unknown", "oc_group", thread, "cli_unknown", "Completed", "unknown", at),
				peerContextCard("missing", "oc_group", thread, "", "Completed", "missing identity", at),
				peerContextCard("other-chat", "oc_other", thread, "cli_peer", "Completed", "other chat", at),
				peerContextCard("other-topic", "oc_group", "omt_other", "cli_peer", "Completed", "other topic", at),
				peerContextCard("too-old", "oc_group", thread, "cli_peer", "Completed", "outside 30 minutes", now.Add(-31*time.Minute)),
				peerContextCard("future", "oc_group", thread, "cli_peer", "Completed", "future", now.Add(time.Second)),
			}
			wantCards := true // human cards need original content too
			p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("card_msg_content_type"); (got == "user_card_content") != wantCards {
					t.Errorf("card format = %q", got)
				}
				writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": items, "has_more": false}})
			})
			p.peerBots = map[string]string{"cli_peer": "Peer B", "cli_context": "Self"}
			target := replyContext{chatID: "oc_group", threadID: thread, contextEligible: true, contextTimeMs: now.UnixMilli()}
			result, err := p.LoadMessageContext(context.Background(), target)
			if err != nil || len(result.Messages) != 1 || result.Messages[0].ID != "human" {
				t.Fatalf("aliases alone shared bot output: %+v %v", result, err)
			}
			p.groupContext.includePeerBots, wantCards = true, true
			result, err = p.LoadMessageContext(context.Background(), target)
			if err != nil || len(result.Messages) != 2 {
				t.Fatalf("wrong shared history: %+v %v", result, err)
			}
			got := result.Messages[1]
			if got.ID != "peer" || got.SenderName != "Peer B" || got.SenderType != "app" || got.Revision == "" || !strings.Contains(got.Text, "Completed\nThe next step") {
				t.Fatalf("lost public identity/status: %+v", got)
			}
			for _, invalid := range []any{replyContext{chatID: "oc_group", directUserID: "ou_private"}, replyContext{chatID: "oc_other", contextEligible: true}, "restored target"} {
				before := calls.Load()
				c, err := p.LoadMessageContext(context.Background(), invalid)
				if err != nil || len(c.Messages) != 0 || calls.Load() != before {
					t.Fatal("private/synthetic/unauthorized target caused read")
				}
			}
		})
	}
}

func TestGroupContextPeerCardRevisionsAndSnapshotTime(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	p := &Platform{appID: "cli_self", peerBots: map[string]string{"cli_peer": "Peer"}, groupContext: groupContextConfig{includePeerBots: true, maxChars: 512}}
	rctx := replyContext{chatID: "oc_group"}
	parse := func(status, body string) core.ContextMessage {
		m := peerContextCard("same-id", "oc_group", "", "cli_peer", status, body, now.Add(-time.Minute))
		data, _ := json.Marshal(m)
		var item larkim.Message
		_ = json.Unmarshal(data, &item)
		got, usable, _ := p.contextMessage(&item, rctx, now.Add(-30*time.Minute), now)
		if !usable {
			t.Fatal("valid snapshot excluded")
		}
		return got
	}
	partial := parse("Working", "partial reply")
	final := parse("Completed", "partial reply")
	if partial.Revision == final.Revision || final.Revision != parse("Completed", "partial reply").Revision {
		t.Fatal("status revision missing or unstable")
	}
	long := strings.Repeat("x", 600)
	if parse("Completed", long+"a").Revision == parse("Completed", long+"b").Revision {
		t.Fatal("revision hashed only truncated prefix")
	}
	for _, state := range []string{"Failed", "Interrupted", "Redirected"} {
		if !strings.HasPrefix(parse(state, "partial reply").Text, state+"\n") {
			t.Fatal("terminal state lost")
		}
	}
	item := &larkim.Message{MessageId: stringPtr("edited"), ChatId: stringPtr("oc_group"), CreateTime: stringPtr("1"), UpdateTime: stringPtr("9999999999999"), Sender: &larkim.Sender{Id: stringPtr("cli_peer"), SenderType: stringPtr("app")}, Body: &larkim.MessageBody{Content: stringPtr(`{"text":"future revision"}`)}, MsgType: stringPtr("text")}
	item.CreateTime = stringPtr(fmtMillis(now.Add(-time.Minute)))
	if _, usable, incomplete := p.contextMessage(item, rctx, now.Add(-30*time.Minute), now); usable || !incomplete {
		t.Fatal("future edit used as an old snapshot")
	}
	item.UpdateTime = nil
	item.MsgType = stringPtr("interactive")
	for _, content := range []string{`{"unsupported":true}`, `{"header":{"title":{"content":"Completed"}},"body":{}}`} {
		item.Body.Content = stringPtr(content)
		if entry, usable, incomplete := p.contextMessage(item, rctx, now.Add(-30*time.Minute), now); !usable || !incomplete || !entry.Incomplete || !strings.Contains(entry.Text, "unavailable") {
			t.Fatal("unreadable/title-only card reported complete")
		}
	}
}

func TestGroupContextPeerImageOnlyEditInvalidatesReceipt(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	p := &Platform{appID: "self", peerBots: map[string]string{"peer": "Peer"}, groupContext: groupContextConfig{includePeerBots: true, maxChars: 8000}}
	parse := func(key string) core.ContextMessage {
		m := contextTypedMessage("same-card", "interactive", `{"body":{"elements":[{"tag":"img","img_key":"`+key+`"}]}}`, now.Add(-time.Minute))
		m["sender"] = map[string]string{"id": "peer", "sender_type": "app"}
		data, _ := json.Marshal(m)
		var item larkim.Message
		_ = json.Unmarshal(data, &item)
		got, usable, _ := p.contextMessage(&item, replyContext{chatID: "oc_group"}, now.Add(-30*time.Minute), now)
		if !usable || len(got.Resources) != 1 {
			t.Fatal("image card missing")
		}
		return got
	}
	old, replacement := parse("image-old"), parse("image-new")
	if old.Text != replacement.Text || old.Revision == replacement.Revision {
		t.Fatal("image-only edit would be mistaken for an already delivered card")
	}
	if replacement.Revision != parse("image-new").Revision {
		t.Fatal("unchanged image would be redelivered")
	}
}

func fmtMillis(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }

func TestGroupContextCardStatusAndOriginalNestedElements(t *testing.T) {
	for _, card := range []string{
		`{"schema":"2.0","config":{"streaming_mode":true},"header":{"title":{"content":"Working"}},"body":{"elements":[{"tag":"column_set","columns":[{"elements":[{"tag":"markdown","content":"partial answer"}]}]}]}}`,
		`{"config":{"streaming_mode":true},"header":{"title":{"content":"Working"}},"elements":[{"tag":"div","text":{"tag":"lark_md","content":"partial answer"}}]}`,
		`{"config":{"streamingMode":true},"header":{"property":{"title":{"property":{"content":"Working"}}}},"body":{"tag":"body","property":{"elements":[{"tag":"markdown","property":{"content":"partial answer"}}]}}}`,
	} {
		for _, wrapped := range []bool{false, true} {
			content := card
			if wrapped {
				b, _ := json.Marshal(map[string]string{"json_card": card})
				content = string(b)
			}
			got := extractGroupContextCardText(content)
			for _, want := range []string{"in-progress", "Working", "partial answer"} {
				if !strings.Contains(got, want) {
					t.Fatalf("lost %q: %s", want, got)
				}
			}
		}
	}
}

func TestGroupContextPeerEventsNeverTriggerBotReplies(t *testing.T) {
	for _, replyAll := range []bool{false, true} {
		p := &Platform{groupContext: groupContextConfig{enabled: true, includePeerBots: true}, groupReplyAll: replyAll, allowFrom: "ou_human", handler: func(core.Platform, *core.Message) { t.Fatal("bot event dispatched") }}
		// No API client/dedup is supplied: reaching either would itself be a bug.
		event := &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{Sender: &larkim.EventSender{SenderType: stringPtr("app")}, Message: &larkim.EventMessage{MessageType: stringPtr("text"), ChatType: stringPtr("group"), Content: stringPtr(`{"text":"@bot continue"}`)}}}
		if err := p.onMessage(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}
