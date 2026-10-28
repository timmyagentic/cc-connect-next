package feishu

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGroupContextNewGroupSystemEventsHaveNoIncompleteWarning(t *testing.T) {
	for _, sender := range []string{"absent", "empty"} {
		t.Run(sender, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Millisecond)
			var items []any
			for i, template := range []string{
				`{"template":"{botName} started the group chat.","from_user":[],"to_chatters":[]}`,
				`{"template":"{GroupOwner} added {NewGroupAdministrators} to group administrators."}`,
				`{"template":"{from_user} invited {to_chatters} to the group.","from_user":["Alice"],"to_chatters":["Bob"]}`,
				`{"template":"{from_user} invited {to_chatters} to the group.","from_user":["Alice"],"to_chatters":["Helper"]}`,
			} {
				m := contextTypedMessage(string(rune('a'+i)), "system", template, now.Add(-time.Minute))
				if sender == "absent" {
					delete(m, "sender")
				} else {
					m["sender"] = map[string]string{"id": "", "sender_type": "", "id_type": ""}
				}
				items = append(items, m)
			}
			current := contextTypedMessage("trigger", "text", `{"text":"first question"}`, now)
			welcome := contextTypedMessage("welcome", "text", `{"text":"Welcome"}`, now.Add(-time.Second))
			welcome["sender"] = map[string]string{"id": "cli_context", "sender_type": "app"}
			items = append(items, current, welcome)
			p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": items, "has_more": false}})
			})
			got, err := p.LoadMessageContext(context.Background(), replyContext{
				chatID: "oc_group", messageID: "trigger", contextEligible: true, contextTimeMs: now.UnixMilli(),
			}, time.Time{})
			if err != nil || got.Incomplete || got.Unreadable || len(got.Messages) != 4 || calls.Load() != 1 {
				t.Fatalf("fresh group history misclassified: %+v err=%v calls=%d", got, err, calls.Load())
			}
			for _, entry := range got.Messages {
				if entry.Incomplete || entry.SenderType != "system" || entry.SenderID != "system" || entry.SenderName != "System" {
					t.Fatalf("system event lacks safe attribution: %+v", entry)
				}
			}
			if !strings.Contains(got.Messages[2].Text, "Alice invited Bob") {
				t.Fatal("supplied system-event names not preserved")
			}
		})
	}
}

func TestGroupContextEmptyHistoryIsComplete(t *testing.T) {
	p, _ := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{}, "has_more": false}})
	})
	got, err := p.LoadMessageContext(context.Background(), contextMediaTarget(time.Now()), time.Time{})
	if err != nil || got.Incomplete || got.Unreadable || len(got.Messages) != 0 {
		t.Fatalf("empty history misclassified: %+v %v", got, err)
	}
}

func TestGroupContextSystemSenderExceptionPreservesScopeAndUserValidation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, variant := range []string{"other-chat", "other-topic", "old", "deleted", "malformed-user"} {
		t.Run(variant, func(t *testing.T) {
			m := contextTypedMessage("event", "system", `{"template":"group created"}`, now.Add(-time.Minute))
			delete(m, "sender")
			switch variant {
			case "other-chat":
				m["chat_id"] = "oc_other"
			case "other-topic":
				m["thread_id"] = "omt_other"
			case "old":
				m["create_time"] = "1"
			case "deleted":
				m["deleted"] = true
			case "malformed-user":
				m["msg_type"] = "text"
			}
			p, _ := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{m}, "has_more": false}})
			})
			got, err := p.LoadMessageContext(context.Background(), contextMediaTarget(now), time.Time{})
			if err != nil || len(got.Messages) != 0 {
				t.Fatalf("invalid record admitted by system exception: %+v %v", got, err)
			}
			if variant == "malformed-user" && !got.Unreadable {
				t.Fatal("missing human sender was silently accepted")
			}
		})
	}
}
