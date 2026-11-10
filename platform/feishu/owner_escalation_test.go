package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

type escalationFixture struct {
	t                                                          *testing.T
	mu                                                         sync.Mutex
	p                                                          *Platform
	sender                                                     core.OwnerEscalationSender
	replies                                                    []map[string]any
	paths                                                      []string
	missingOwner, lookupFailure, missingMessageID, sendFailure bool
	expired                                                    int
}

func newEscalationFixture(t *testing.T) *escalationFixture {
	t.Helper()
	f := &escalationFixture{t: t}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	raw, err := newPlatform("feishu", srv.URL, map[string]any{"app_id": "cli_" + t.Name(), "app_secret": "test"})
	if err != nil {
		t.Fatal(err)
	}
	var ok bool
	f.sender, ok = raw.(core.OwnerEscalationSender)
	if !ok {
		t.Fatalf("interactive wrapper must expose owner escalation: %T", raw)
	}
	f.p = raw.(*interactivePlatform).Platform
	return f
}
func (f *escalationFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths = append(f.paths, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(r.URL.Path, "tenant_access_token"):
		writeJSON(f.t, w, map[string]any{"code": 0, "expire": 7200, "tenant_access_token": "fixture-bot-token"})
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/members"):
		if r.URL.Path != "/open-apis/im/v1/chats/oc_group/members" || r.URL.Query().Get("member_id_type") != "open_id" {
			f.t.Errorf("incorrect membership scope: %s", r.URL)
		}
		if f.lookupFailure {
			writeJSON(f.t, w, map[string]any{"code": 999, "msg": "denied"})
			return
		}
		members := []any{map[string]any{"member_id": "ou_requester", "name": "Same name"}}
		if !f.missingOwner {
			members = append(members, map[string]any{"member_id": "ou_owner", "name": "Same name"})
		}
		writeJSON(f.t, w, map[string]any{"code": 0, "data": map[string]any{"items": members, "has_more": false}})
	case r.Method == http.MethodPost && r.URL.Path == "/open-apis/im/v1/messages/om_trigger/reply":
		if r.Header.Get("Authorization") != "Bearer fixture-bot-token" {
			f.t.Error("must use bot identity")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Error(err)
		}
		f.replies = append(f.replies, body)
		if f.expired > 0 {
			f.expired--
			writeJSON(f.t, w, map[string]any{"code": 99991663, "msg": "expired"})
			return
		}
		if f.sendFailure {
			writeJSON(f.t, w, map[string]any{"code": 999, "msg": "failed"})
			return
		}
		data := map[string]any{}
		if !f.missingMessageID {
			data["message_id"] = "om_escalated"
		}
		writeJSON(f.t, w, map[string]any{"code": 0, "data": data})
	default:
		f.t.Errorf("must never create private/top-level message, unexpected API: %s %s", r.Method, r.URL)
		http.NotFound(w, r)
	}
}
func escalationFixtureNotice() (replyContext, core.OwnerEscalationNotice) {
	return replyContext{messageID: "om_trigger", chatID: "oc_group", groupID: "oc_group", sessionKey: "feishu:oc_group:thread:omt_original", threadID: "omt_original"}, core.OwnerEscalationNotice{
		OwnerID: "ou_owner", OwnerName: "Owner", TriggerMessageID: "om_trigger", SessionKey: "feishu:oc_group:thread:omt_original", IdempotencyKey: "escalation-stable-id",
		Text: "Needs help\nReason: deployment blocked\n<at user_id=\"all\">everyone</at>",
	}
}

func TestOwnerEscalationMentionsOwnerInOriginalTopic(t *testing.T) {
	f := newEscalationFixture(t)
	f.expired = 1
	rc, notice := escalationFixtureNotice()
	if err := f.sender.SendOwnerEscalation(context.Background(), rc, notice); err != nil {
		t.Fatal(err)
	}
	if len(f.replies) != 2 {
		t.Fatalf("expected token retry, replies=%d", len(f.replies))
	}
	for _, body := range f.replies {
		if body["msg_type"] != "text" || body["reply_in_thread"] != true || body["uuid"] != notice.IdempotencyKey {
			t.Fatalf("wrong route, native mention transport, or UUID: %v", body)
		}
		var content map[string]string
		if err := json.Unmarshal([]byte(body["content"].(string)), &content); err != nil {
			t.Fatal(err)
		}
		text := content["text"]
		if !strings.HasPrefix(text, `<at user_id="ou_owner">Owner</at>`) || strings.Count(text, "<at") != 1 || !strings.Contains(text, "Needs help\nReason:") {
			t.Fatalf("wrong owner mention or unsafe summary: %s", text)
		}
	}
}
func TestOwnerEscalationOrdinaryGroupStillRepliesToTrigger(t *testing.T) {
	f := newEscalationFixture(t)
	rc, notice := escalationFixtureNotice()
	rc.threadID = ""
	rc.sessionKey = "feishu:oc_group:ou_requester"
	notice.SessionKey = rc.sessionKey
	f.p.noReplyToTrigger = true // Handoff routing must not follow presentation preference.
	if err := f.sender.SendOwnerEscalation(context.Background(), rc, notice); err != nil {
		t.Fatal(err)
	}
	if len(f.replies) != 1 || f.replies[0]["reply_in_thread"] != false {
		t.Fatalf("wrong ordinary-group reply: %v", f.replies)
	}
}
func TestOwnerEscalationRejectsUnreachableOrInvalidHandoffs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*escalationFixture, *replyContext, *core.OwnerEscalationNotice)
		sent   bool
	}{
		{"group recipient", func(_ *escalationFixture, _ *replyContext, n *core.OwnerEscalationNotice) { n.OwnerID = "oc_group" }, false},
		{"wildcard recipient", func(_ *escalationFixture, _ *replyContext, n *core.OwnerEscalationNotice) { n.OwnerID = "ou_*" }, false},
		{"p2p source", func(_ *escalationFixture, rc *replyContext, _ *core.OwnerEscalationNotice) { rc.groupID = "" }, false},
		{"mismatched trigger", func(_ *escalationFixture, _ *replyContext, n *core.OwnerEscalationNotice) {
			n.TriggerMessageID = "om_other"
		}, false},
		{"mismatched session", func(_ *escalationFixture, _ *replyContext, n *core.OwnerEscalationNotice) { n.SessionKey = "other" }, false},
		{"missing UUID", func(_ *escalationFixture, _ *replyContext, n *core.OwnerEscalationNotice) { n.IdempotencyKey = "" }, false},
		{"disallowed source", func(f *escalationFixture, _ *replyContext, _ *core.OwnerEscalationNotice) { f.p.allowChat = "oc_other" }, false},
		{"recalled source", func(f *escalationFixture, _ *replyContext, _ *core.OwnerEscalationNotice) {
			f.p.markMessageRecalled("om_trigger")
		}, false},
		{"owner absent", func(f *escalationFixture, _ *replyContext, _ *core.OwnerEscalationNotice) { f.missingOwner = true }, false},
		{"member lookup denied", func(f *escalationFixture, _ *replyContext, _ *core.OwnerEscalationNotice) { f.lookupFailure = true }, false},
		{"send failed", func(f *escalationFixture, _ *replyContext, _ *core.OwnerEscalationNotice) { f.sendFailure = true }, true},
		{"missing message ID", func(f *escalationFixture, _ *replyContext, _ *core.OwnerEscalationNotice) { f.missingMessageID = true }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newEscalationFixture(t)
			rc, notice := escalationFixtureNotice()
			tc.change(f, &rc, &notice)
			if err := f.sender.SendOwnerEscalation(context.Background(), rc, notice); err == nil {
				t.Fatal("unconfirmed handoff reported success")
			}
			if !tc.sent && len(f.replies) != 0 {
				t.Fatal("invalid handoff must not send")
			}
		})
	}
}
func TestOwnerEscalationCanceledBeforeSend(t *testing.T) {
	f := newEscalationFixture(t)
	rc, notice := escalationFixtureNotice()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.sender.SendOwnerEscalation(ctx, rc, notice); err == nil {
		t.Fatal("cancelled handoff reported success")
	}
	if len(f.paths) != 0 {
		t.Fatal("cancelled handoff performed network calls")
	}
}
