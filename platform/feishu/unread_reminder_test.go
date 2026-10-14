package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

type reminderFixture struct {
	invalidPath                                       string
	invalidLeft, authCalls                            int
	retryUUIDs                                        []string
	t                                                 *testing.T
	p                                                 *Platform
	now                                               time.Time
	mu                                                sync.Mutex
	replies, private                                  int
	reads                                             []string
	pageTwoRead, readFailure, deleted, privateFailure bool
	replyBodies, privateBodies                        []map[string]any
}

func newReminderFixture(t *testing.T, overrides ...map[string]any) *reminderFixture {
	t.Helper()
	f := &reminderFixture{t: t, now: time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	opts := map[string]any{"app_id": "cli_" + t.Name(), "app_secret": "test", "domain": srv.URL, "enable_feishu_card": false, "unread_reminder": true, "unread_reminder_users": []string{"ou_owner"}, "cc_data_dir": t.TempDir(), "cc_project": "project"}
	for _, override := range overrides {
		for key, value := range override {
			opts[key] = value
		}
	}
	raw, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	f.p = raw.(*Platform)
	f.p.client = lark.NewClient(f.p.appID, "test", lark.WithOpenBaseUrl(srv.URL), lark.WithHttpClient(srv.Client()))
	f.p.reminders.now = func() time.Time { return f.now }
	return f
}

func (f *reminderFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if f.invalidPath != "" && strings.HasSuffix(r.URL.Path, f.invalidPath) {
		if uuid, ok := body["uuid"].(string); ok {
			f.retryUUIDs = append(f.retryUUIDs, uuid)
		}
		if f.invalidLeft > 0 {
			f.invalidLeft--
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 99991663, "msg": "expired"})
			return
		}
	}
	switch {
	case strings.Contains(r.URL.Path, "tenant_access_token"):
		f.authCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "fixture-token", "expire": 3600})
	case strings.HasSuffix(r.URL.Path, "/reply"):
		f.replies++
		f.replyBodies = append(f.replyBodies, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"message_id": "om_notice_" + strconv.Itoa(f.replies)}})
	case strings.HasSuffix(r.URL.Path, "/read_users"):
		if r.URL.Query().Get("user_id_type") != "open_id" {
			f.t.Error("reader query must use open_id")
		}
		token := r.URL.Query().Get("page_token")
		f.reads = append(f.reads, token)
		if f.readFailure {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 999, "msg": "unavailable"})
			return
		}
		if f.pageTwoRead && token == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"items": []any{map[string]any{"user_id": "ou_other"}}, "has_more": true, "page_token": "second"}})
			return
		}
		items := []any{}
		if f.pageTwoRead {
			items = append(items, map[string]any{"user_id": "ou_owner"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"items": items, "has_more": false}})
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/messages/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"items": []any{map[string]any{"message_id": filepath.Base(r.URL.Path), "deleted": f.deleted, "message_position": "42", "thread_message_position": "3"}}}})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
		if r.URL.Query().Get("receive_id_type") != "open_id" || body["receive_id"] != "ou_owner" {
			f.t.Error("private reminder wrong recipient", body)
		}
		f.private++
		f.privateBodies = append(f.privateBodies, body)
		if f.privateFailure {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 999, "msg": "failed"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"message_id": "om_private"}})
	default:
		f.t.Errorf("unexpected API %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}
}

func (f *reminderFixture) notice(trigger, thread string) error {
	return f.p.NotifyTurnCompleted(context.Background(), replyContext{messageID: trigger, chatID: "oc_group", threadID: thread, sessionKey: "session"}, core.TurnCompletionNotice{UserID: "ou_owner", SessionKey: "session", TriggerMessageID: trigger, Summary: "project: finished result", Text: "Completed: result", ReminderText: "Please read your results"})
}
func (f *reminderFixture) tick() { f.p.processUnreadReminders(context.Background()) }
func (f *reminderFixture) reload() {
	f.t.Helper()
	data, err := os.ReadFile(f.p.reminders.path)
	if err != nil {
		f.t.Fatal(err)
	}
	records := map[string]unreadReminder{}
	if err := json.Unmarshal(data, &records); err != nil {
		f.t.Fatal(err)
	}
	f.p.reminders.records = records
}

func TestUnreadReminderOneHourGroupedPrivateAndRestart(t *testing.T) {
	f := newReminderFixture(t)
	for _, trigger := range []string{"om_one", "om_two"} {
		if err := f.notice(trigger, "omt_topic"); err != nil {
			t.Fatal(err)
		}
	}
	if f.replies != 2 {
		t.Fatal("missing fresh completion notices")
	}
	for _, body := range f.replyBodies {
		if body["reply_in_thread"] != true || body["uuid"] == "" {
			t.Fatal("must preserve topic and stable UUID", body)
		}
	}
	f.now = f.now.Add(time.Hour - time.Second)
	f.tick()
	if f.private != 0 || len(f.reads) != 0 {
		t.Fatal("reminded before one hour")
	}
	f.reload()
	f.now = f.now.Add(time.Second)
	f.tick()
	if f.private != 1 {
		t.Fatalf("want grouped private reminder, got %d", f.private)
	}
	if f.privateBodies[0]["uuid"] == "" {
		t.Fatal("missing idempotency UUID")
	}
	if !strings.Contains(f.privateBodies[0]["content"].(string), "open_thread_id=omt_topic") {
		t.Fatal("missing original message link")
	}
	f.reload()
	f.tick()
	if f.private != 1 {
		t.Fatal("resent reminder after restart")
	}
	if err := f.notice("om_one", "omt_topic"); err != nil {
		t.Fatal(err)
	}
	if f.replies != 2 {
		t.Fatal("duplicate completion notice")
	}
	info, err := os.Stat(f.p.reminders.path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("state must be private", err)
	}
}

func TestUnreadReminderLaterPageReadAndQueryFailure(t *testing.T) {
	for _, kind := range []string{"later page", "failure", "deleted"} {
		t.Run(kind, func(t *testing.T) {
			f := newReminderFixture(t)
			f.pageTwoRead = kind == "later page"
			f.readFailure = kind == "failure"
			f.deleted = kind == "deleted"
			if err := f.notice("om_trigger", ""); err != nil {
				t.Fatal(err)
			}
			f.now = f.now.Add(time.Hour)
			f.tick()
			if f.private != 0 {
				t.Fatal("read/unknown/deleted must not be reminded")
			}
			if f.pageTwoRead && (len(f.reads) != 2 || f.reads[1] != "second") {
				t.Fatal("reader pagination truncated", f.reads)
			}
			if f.readFailure {
				for i := 0; i < 10; i++ {
					f.now = f.now.Add(time.Hour)
					f.tick()
				}
				if len(f.reads) != 6 {
					t.Fatalf("unbounded retry: %d", len(f.reads))
				}
			}
		})
	}
}

func TestUnreadReminderCancellationOnlySameTopicAndUser(t *testing.T) {
	for _, kind := range []string{"same", "other topic", "other user", "old", "recall trigger", "recall notice"} {
		t.Run(kind, func(t *testing.T) {
			f := newReminderFixture(t)
			if err := f.notice("om_trigger", "omt_topic"); err != nil {
				t.Fatal(err)
			}
			msg := &core.Message{UserID: "ou_owner", SessionKey: "session", MessageID: "om_followup", UserMessageTimeMs: f.now.Add(time.Second).UnixMilli(), ReplyCtx: replyContext{chatID: "oc_group", threadID: "omt_topic"}}
			switch kind {
			case "other topic":
				msg.ReplyCtx = replyContext{chatID: "oc_group", threadID: "omt_other"}
			case "other user":
				msg.UserID = "ou_other"
			case "old":
				msg.UserMessageTimeMs = f.now.Add(-time.Second).UnixMilli()
			case "recall trigger":
				msg.Recalled = true
				msg.MessageID = "om_trigger"
			case "recall notice":
				msg.Recalled = true
				for _, r := range f.p.reminders.records {
					msg.MessageID = r.MessageID
				}
			}
			if err := f.p.ObserveReminderMessage(context.Background(), msg); err != nil {
				t.Fatal(err)
			}
			f.reload()
			f.now = f.now.Add(time.Hour)
			f.tick()
			want := 0
			if kind == "other topic" || kind == "other user" || kind == "old" {
				want = 1
			}
			if f.private != want {
				t.Fatalf("private=%d want=%d", f.private, want)
			}
		})
	}
}

func TestUnreadReminderStorageFailuresAndUncertainSendNeverDuplicate(t *testing.T) {
	for _, kind := range []string{"before notice", "before claim", "uncertain send"} {
		t.Run(kind, func(t *testing.T) {
			f := newReminderFixture(t)
			fail := func(string, []byte) error { return errors.New("disk full") }
			if kind == "before notice" {
				f.p.reminders.save = fail
				if err := f.notice("om_trigger", ""); err == nil {
					t.Fatal("expected persistence error")
				}
				if f.replies != 0 {
					t.Fatal("notice sent without durable claim")
				}
				return
			}
			if err := f.notice("om_trigger", ""); err != nil {
				t.Fatal(err)
			}
			f.now = f.now.Add(time.Hour)
			if kind == "before claim" {
				f.p.reminders.save = fail
			} else {
				f.privateFailure = true
			}
			f.tick()
			if kind == "before claim" && f.private != 0 {
				t.Fatal("sent without durable claim")
			}
			if kind == "uncertain send" {
				f.reload()
				f.privateFailure = false
				f.tick()
				if f.private != 1 {
					t.Fatal("uncertain delivery retried after restart")
				}
			}
		})
	}
}

func TestUnreadReminderConfigurationAndIsolation(t *testing.T) {
	cases := []map[string]any{
		{"unread_reminder": "true"}, {"unread_reminder": true}, {"unread_reminder": true, "unread_reminder_users": "ou_owner"},
		{"unread_reminder": true, "unread_reminder_users": []any{1}}, {"unread_reminder": true, "unread_reminder_users": []string{"*"}},
	}
	for _, opts := range cases {
		if _, _, err := parseUnreadReminderOptions(opts); err == nil {
			t.Fatal("invalid config accepted", opts)
		}
	}
	enabled, _, err := parseUnreadReminderOptions(nil)
	if err != nil || enabled {
		t.Fatal("must default off")
	}
	f := newReminderFixture(t)
	p := &Platform{platformName: "feishu", appID: "cli_second", domain: f.p.domain}
	opts := map[string]any{"unread_reminder": true, "unread_reminder_users": []string{"ou_owner"}, "cc_data_dir": filepath.Dir(filepath.Dir(f.p.reminders.path)), "cc_project": "project"}
	if err := p.configureUnreadReminders(opts); err != nil {
		t.Fatal(err)
	}
	if p.reminders.path == f.p.reminders.path {
		t.Fatal("bot state collision")
	}
	p.appID = f.p.appID
	opts["cc_project"] = "other project"
	if err := p.configureUnreadReminders(opts); err != nil {
		t.Fatal(err)
	}
	if p.reminders.path == f.p.reminders.path {
		t.Fatal("project state collision")
	}
}

func TestUnreadReminderExpiryStopAndConcurrentTicks(t *testing.T) {
	f := newReminderFixture(t)
	if err := f.notice("om_trigger", ""); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); f.tick() }()
	}
	wg.Wait()
	if f.private != 1 {
		t.Fatal("concurrent sends", f.private)
	}
	f.now = f.now.Add(reminderRetention)
	f.tick()
	if len(f.p.reminders.records) != 0 {
		t.Fatal("stale state retained")
	}
	if err := f.p.startUnreadReminders(); err != nil {
		t.Fatal(err)
	}
	f.p.stopUnreadReminders()
	f.p.stopUnreadReminders()
	if err := f.notice("om_stopped", ""); err == nil {
		t.Fatal("accepted completion after stop")
	}
}

func TestUnreadReminderExpiredTokenRefreshKeepsUUID(t *testing.T) {
	for _, path := range []string{"/read_users", "/messages/om_notice_1", "/messages"} {
		t.Run(path, func(t *testing.T) {
			f := newReminderFixture(t)
			if err := f.notice("om_trigger", ""); err != nil {
				t.Fatal(err)
			}
			f.invalidPath = path
			f.invalidLeft = 2
			f.now = f.now.Add(time.Hour)
			f.tick()
			if f.private != 1 || f.authCalls < 2 {
				t.Fatalf("refresh failed: private=%d auth=%d", f.private, f.authCalls)
			}
			for _, uuid := range f.retryUUIDs {
				if uuid == "" || uuid != f.retryUUIDs[0] {
					t.Fatal("UUID changed during retry", f.retryUUIDs)
				}
			}
		})
	}
}

func TestUnreadReminderWorkerRecoversDueRecord(t *testing.T) {
	f := newReminderFixture(t)
	if err := f.notice("om_trigger", ""); err != nil {
		t.Fatal(err)
	}
	f.p.reminders.records = map[string]unreadReminder{}
	f.now = f.now.Add(time.Hour)
	if err := f.p.startUnreadReminders(); err != nil {
		t.Fatal(err)
	}
	defer f.p.stopUnreadReminders()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		count := f.private
		f.mu.Unlock()
		if count == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("worker did not recover persisted reminder")
}

func TestUnreadReminderUnmentionedContinuationCancelsWithoutDispatch(t *testing.T) {
	f := newReminderFixture(t)
	f.now = time.Now().Add(-time.Minute)
	msg := &larkim.EventMessage{MessageId: stringPtr("om_followup"), ChatId: stringPtr("oc_group"), ChatType: stringPtr("group"), ThreadId: stringPtr("omt_topic"), MessageType: stringPtr("text"), Content: stringPtr(`{"text":"thanks"}`), CreateTime: stringPtr(strconv.FormatInt(time.Now().UnixMilli(), 10))}
	session := f.p.makeSessionKey(msg, "oc_group", "ou_owner")
	rc := replyContext{messageID: "om_trigger", chatID: "oc_group", threadID: "omt_topic", sessionKey: session}
	if err := f.p.NotifyTurnCompleted(context.Background(), rc, core.TurnCompletionNotice{UserID: "ou_owner", SessionKey: session, TriggerMessageID: "om_trigger", Summary: "result", Text: "completed", ReminderText: "unread"}); err != nil {
		t.Fatal(err)
	}
	f.p.botOpenID = "ou_bot"
	f.p.handler = func(core.Platform, *core.Message) { t.Error("unmentioned continuation dispatched to agent") }
	event := &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{Message: msg, Sender: &larkim.EventSender{SenderType: stringPtr("user"), SenderId: &larkim.UserId{OpenId: stringPtr("ou_owner")}}}}
	if err := f.p.onMessage(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(2 * time.Hour)
	f.tick()
	if f.private != 0 {
		t.Fatal("continued topic was reminded")
	}
}

func TestUnreadReminderNewTopicContinuationMatchesOriginalRoot(t *testing.T) {
	for _, mode := range []threadIsolationMode{threadIsolationTopicPerMessage, threadIsolationOff} {
		t.Run(mode.String(), func(t *testing.T) {
			f := newReminderFixture(t)
			f.p.threadMode = mode
			root := &larkim.EventMessage{MessageId: stringPtr("om_root"), ChatType: stringPtr("group")}
			session := f.p.makeSessionKey(root, "oc_group", "ou_owner")
			rc := replyContext{messageID: "om_root", chatID: "oc_group", sessionKey: session}
			if err := f.p.NotifyTurnCompleted(context.Background(), rc, core.TurnCompletionNotice{UserID: "ou_owner", SessionKey: session, TriggerMessageID: "om_root", Summary: "result", Text: "completed", ReminderText: "unread"}); err != nil {
				t.Fatal(err)
			}
			child := &larkim.EventMessage{MessageId: stringPtr("om_child"), RootId: stringPtr("om_root"), ThreadId: stringPtr("omt_new"), ChatType: stringPtr("group")}
			nextSession := f.p.makeSessionKey(child, "oc_group", "ou_owner")
			err := f.p.ObserveReminderMessage(context.Background(), &core.Message{UserID: "ou_owner", SessionKey: nextSession, MessageID: "om_child", UserMessageTimeMs: f.now.Add(time.Second).UnixMilli(), ReplyCtx: replyContext{chatID: "oc_group", threadID: "omt_new"}})
			if err != nil {
				t.Fatal(err)
			}
			f.reload()
			f.now = f.now.Add(time.Hour)
			f.tick()
			want := 0
			if mode == threadIsolationOff {
				want = 1
			}
			if f.private != want {
				t.Fatalf("got reminders=%d want=%d", f.private, want)
			}
		})
	}
}
