package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/timmyagentic/cc-connect-next/core"
)

type completionFixture struct {
	t                                      *testing.T
	p                                      *Platform
	mu                                     sync.Mutex
	opts                                   map[string]any
	now                                    time.Time
	privateBodies                          []map[string]any
	sourceIDs, authorizations, retryUUIDs  []string
	replies, reads, authCalls, expired     int
	lookupFailure, deleted, privateFailure bool
	sourceThread, sourceChat               string
}

func newCompletionFixture(t *testing.T, overrides ...map[string]any) *completionFixture {
	t.Helper()
	f := &completionFixture{t: t, now: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.opts = map[string]any{"app_id": "cli_" + t.Name(), "app_secret": "test", "domain": srv.URL, "enable_feishu_card": false,
		"completion_notify": true, "completion_notify_users": []string{"ou_owner"}, "cc_data_dir": t.TempDir(), "cc_project": "project"}
	for _, override := range overrides {
		for k, v := range override {
			f.opts[k] = v
		}
	}
	raw, err := New(f.opts)
	if err != nil {
		t.Fatal(err)
	}
	f.p = raw.(*Platform)
	f.p.client = lark.NewClient(f.p.appID, "test", lark.WithOpenBaseUrl(srv.URL), lark.WithHttpClient(srv.Client()))
	if f.p.completionNotifications != nil {
		f.p.completionNotifications.now = func() time.Time { return f.now }
	}
	if err := f.p.startCompletionNotifications(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.p.stopCompletionNotifications)
	return f
}

func (f *completionFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	respond := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	var body map[string]any
	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	switch {
	case strings.Contains(r.URL.Path, "tenant_access_token"):
		if body["app_id"] != f.p.appID {
			f.t.Error("notification borrowed another bot identity")
		}
		f.authCalls++
		respond(map[string]any{"code": 0, "tenant_access_token": "fixture-tenant-token", "expire": 3600})
	case strings.HasSuffix(r.URL.Path, "/reply"):
		f.replies++
		respond(map[string]any{"code": 0, "data": map[string]any{"message_id": "om_notice"}})
	case strings.HasSuffix(r.URL.Path, "/read_users"):
		f.reads++
		respond(map[string]any{"code": 0, "data": map[string]any{"items": []any{}, "has_more": false}})
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/messages/"):
		f.sourceIDs = append(f.sourceIDs, filepath.Base(r.URL.Path))
		if f.lookupFailure {
			respond(map[string]any{"code": 999, "msg": "lookup unavailable"})
			return
		}
		item := map[string]any{"message_id": filepath.Base(r.URL.Path), "chat_id": f.sourceChat, "deleted": f.deleted, "message_position": "42", "thread_message_position": "3", "thread_id": f.sourceThread}
		respond(map[string]any{"code": 0, "data": map[string]any{"items": []any{item}}})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
		if r.URL.Query().Get("receive_id_type") != "open_id" {
			f.t.Error("notification must address an app-scoped user")
		}
		f.authorizations = append(f.authorizations, r.Header.Get("Authorization"))
		if id, ok := body["uuid"].(string); ok {
			f.retryUUIDs = append(f.retryUUIDs, id)
		}
		if f.expired > 0 {
			f.expired--
			respond(map[string]any{"code": 99991663, "msg": "expired"})
			return
		}
		f.privateBodies = append(f.privateBodies, body)
		if f.privateFailure {
			respond(map[string]any{"code": 999, "msg": "uncertain send"})
			return
		}
		respond(map[string]any{"code": 0, "data": map[string]any{"message_id": "om_private"}})
	default:
		f.t.Errorf("unexpected API %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}
}

func (f *completionFixture) notice(ctx context.Context, trigger, thread, user string) error {
	return f.p.NotifyTurnCompleted(ctx, replyContext{messageID: trigger, chatID: "oc_group", threadID: thread, sessionKey: "session"}, core.TurnCompletionNotice{
		UserID: user, SessionKey: "session", TriggerMessageID: trigger, Summary: "project: result", Text: "Completed: result", LinkLabel: "Open original conversation:"})
}
func (f *completionFixture) restart() {
	f.t.Helper()
	f.p.stopCompletionNotifications()
	if err := f.p.startCompletionNotifications(); err != nil {
		f.t.Fatal(err)
	}
}
func (f *completionFixture) privateText(i int) string {
	f.t.Helper()
	var body struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(f.privateBodies[i]["content"].(string)), &body); err != nil {
		f.t.Fatal(err)
	}
	return body.Text
}

func TestCompletionNotificationIsImmediatePrivateWithoutTopicReply(t *testing.T) {
	f := newCompletionFixture(t)
	if err := f.notice(context.Background(), "om_trigger", "omt_topic", "ou_owner"); err != nil {
		t.Fatal(err)
	}
	if len(f.privateBodies) != 1 || f.replies != 0 || f.reads != 0 {
		t.Fatalf("completion must send one immediate private notice, no topic notice/read query: private=%d replies=%d reads=%d", len(f.privateBodies), f.replies, f.reads)
	}
	body := f.privateBodies[0]
	if body["receive_id"] != "ou_owner" || f.authorizations[0] != "Bearer fixture-tenant-token" {
		t.Fatalf("wrong recipient/bot authentication: %v", body)
	}
	content := f.privateText(0)
	if !strings.Contains(content, "Completed: result") || !strings.Contains(content, "open_thread_id=omt_topic") || !strings.Contains(content, "open_chat_id=oc_group") {
		t.Fatalf("missing result/original topic link: %s", content)
	}
	if len(f.sourceIDs) != 1 || f.sourceIDs[0] != "om_trigger" {
		t.Fatalf("must link to original trigger: %v", f.sourceIDs)
	}
}

func TestCompletionNotificationLinksToOriginalChatOrTopic(t *testing.T) {
	for _, tc := range []struct {
		name, thread, serverThread, serverChat, platform, path, key, value string
		lookupFailure                                                      bool
	}{
		{name: "ordinary chat", platform: "feishu", path: "/client/chat/open", key: "position", value: "42"},
		{name: "existing topic", thread: "omt_topic", platform: "feishu", path: "/client/thread/open", key: "open_thread_id", value: "omt_topic"},
		{name: "newly created topic", serverThread: "omt_new", platform: "feishu", path: "/client/thread/open", key: "open_thread_id", value: "omt_new"},
		{name: "lookup failure uses known topic", thread: "omt_known", lookupFailure: true, platform: "feishu", path: "/client/thread/open", key: "open_thread_id", value: "omt_known"},
		{name: "lookup failure uses known chat", lookupFailure: true, platform: "feishu", path: "/client/chat/open", key: "openChatId", value: "oc_group"},
		{name: "wrong chat metadata cannot redirect", serverThread: "omt_wrong", serverChat: "oc_other", platform: "feishu", path: "/client/chat/open", key: "openChatId", value: "oc_group"},
		{name: "lark", platform: "lark", path: "/client/chat/open", key: "openChatId", value: "oc_group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCompletionFixture(t)
			f.p.platformName = tc.platform
			f.sourceThread, f.sourceChat, f.lookupFailure = tc.serverThread, tc.serverChat, tc.lookupFailure
			if err := f.notice(context.Background(), "om_trigger", tc.thread, "ou_owner"); err != nil {
				t.Fatal(err)
			}
			content := f.privateText(0)
			lines := strings.Split(content, "\n")
			u, err := url.Parse(lines[len(lines)-1])
			if err != nil {
				t.Fatal(err)
			}
			host := "applink.feishu.cn"
			if tc.platform == "lark" {
				host = "applink.larksuite.com"
			}
			if u.Host != host || u.Path != tc.path || u.Query().Get(tc.key) != tc.value {
				t.Fatalf("wrong source link: %s", u)
			}
		})
	}
}

func TestCompletionNotificationAccessAndCancellation(t *testing.T) {
	for _, kind := range []string{"disabled", "unlisted user", "sender denied", "chat denied", "recalled", "deleted", "cancelled", "stopped"} {
		t.Run(kind, func(t *testing.T) {
			opts := map[string]any{}
			if kind == "disabled" {
				opts["completion_notify"] = false
			}
			f := newCompletionFixture(t, opts)
			user := "ou_owner"
			ctx := context.Background()
			switch kind {
			case "unlisted user":
				user = "ou_other"
			case "sender denied":
				f.p.allowFrom = "ou_other"
			case "chat denied":
				f.p.allowChat = "oc_other"
			case "recalled":
				f.p.markMessageRecalled("om_trigger")
			case "deleted":
				f.deleted = true
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "stopped":
				f.p.stopCompletionNotifications()
			}
			_ = f.notice(ctx, "om_trigger", "omt_topic", user)
			if len(f.privateBodies) != 0 || f.replies != 0 || f.reads != 0 {
				t.Fatalf("%s sent notification", kind)
			}
		})
	}
}

func TestCompletionNotificationReceiptsPreventDuplicateAndUncertainSends(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "delivered", true: "uncertain"}[failed], func(t *testing.T) {
			f := newCompletionFixture(t)
			f.privateFailure = failed
			err := f.notice(context.Background(), "om_trigger", "", "ou_owner")
			if (err != nil) != failed {
				t.Fatalf("send error=%v", err)
			}
			f.restart()
			if err := f.notice(context.Background(), "om_trigger", "", "ou_owner"); err != nil {
				t.Fatal(err)
			}
			if len(f.privateBodies) != 1 {
				t.Fatalf("duplicate after restart: %d", len(f.privateBodies))
			}
			info, err := os.Stat(f.p.completionNotifications.path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("receipt permissions", err)
			}
			data, _ := os.ReadFile(f.p.completionNotifications.path)
			if strings.Contains(string(data), "ou_owner") || strings.Contains(string(data), "result") {
				t.Fatal("receipt store must contain hashes/times only")
			}
		})
	}
}

func TestCompletionNotificationConcurrentCompletionOnlySendsOnce(t *testing.T) {
	f := newCompletionFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := f.notice(context.Background(), "om_trigger", "", "ou_owner"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(f.privateBodies) != 1 {
		t.Fatalf("duplicate concurrent notifications: %d", len(f.privateBodies))
	}
}

func TestCompletionNotificationStorageFailureStopsSending(t *testing.T) {
	f := newCompletionFixture(t)
	f.p.completionNotifications.save = func(string, []byte) error { return errors.New("disk unavailable") }
	if err := f.notice(context.Background(), "om_trigger", "", "ou_owner"); err == nil {
		t.Fatal("expected save failure")
	}
	if err := f.notice(context.Background(), "om_second", "", "ou_owner"); err == nil {
		t.Fatal("fault must disable further sends")
	}
	if len(f.privateBodies) != 0 {
		t.Fatal("sent without durable receipt")
	}
	if err := os.MkdirAll(filepath.Dir(f.p.completionNotifications.path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.p.completionNotifications.path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	f.p.stopCompletionNotifications()
	if err := f.p.startCompletionNotifications(); err == nil {
		t.Fatal("must reject corrupt receipt store")
	}
}

func TestCompletionNotificationExpiredTokenRetryKeepsUUID(t *testing.T) {
	f := newCompletionFixture(t)
	f.expired = 1
	if err := f.notice(context.Background(), "om_trigger", "", "ou_owner"); err != nil {
		t.Fatal(err)
	}
	if len(f.privateBodies) != 1 || len(f.retryUUIDs) != 2 || f.retryUUIDs[0] == "" || f.retryUUIDs[0] != f.retryUUIDs[1] {
		t.Fatalf("token refresh lost idempotency: %v", f.retryUUIDs)
	}
}

func TestCompletionNotificationLegacyConfigAndOldQueueNeverReplay(t *testing.T) {
	f := newCompletionFixture(t)
	p := &Platform{platformName: f.p.platformName, appID: f.p.appID, domain: f.p.domain}
	legacy := map[string]any{"unread_reminder": true, "unread_reminder_users": []string{"ou_owner"}, "unread_reminder_delay_mins": 60, "cc_data_dir": f.opts["cc_data_dir"], "cc_project": "project"}
	if err := p.configureCompletionNotifications(legacy); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(f.opts["cc_data_dir"].(string), "feishu-unread-reminders")
	if err := os.MkdirAll(oldDir, 0700); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(oldDir, p.completionNotifications.scope+".json")
	if err := os.WriteFile(oldPath, []byte(`{"old":{"state":"pending"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	f.p.completionNotifications = p.completionNotifications
	f.p.completionNotifications.now = func() time.Time { return f.now }
	if err := f.p.startCompletionNotifications(); err != nil {
		t.Fatal(err)
	}
	if len(f.privateBodies) != 0 {
		t.Fatal("legacy queue replayed on startup")
	}
	if err := f.notice(context.Background(), "om_trigger", "", "ou_owner"); err != nil {
		t.Fatal(err)
	}
	if len(f.privateBodies) != 1 {
		t.Fatal("legacy delay prevented immediate delivery")
	}
	data, _ := os.ReadFile(oldPath)
	if string(data) != `{"old":{"state":"pending"}}` {
		t.Fatal("old queue was modified")
	}
}

func TestCompletionNotificationConfigAndScopeIsolation(t *testing.T) {
	for _, opts := range []map[string]any{
		{"completion_notify": "true"}, {"completion_notify": nil}, {"completion_notify_users": nil}, {"completion_notify": true}, {"completion_notify_users": "ou_owner"}, {"completion_notify_users": []any{1}},
		{"completion_notify_users": []string{"*"}}, {"completion_notify_users": []string{"ou_a,ou_b"}}, {"unread_reminder_delay_mins": 0}, {"unread_reminder_delay_mins": "60"},
	} {
		if _, _, err := parseCompletionNotificationOptions(opts); err == nil {
			t.Fatalf("accepted invalid config: %#v", opts)
		}
	}
	enabled, users, err := parseCompletionNotificationOptions(map[string]any{"completion_notify": false, "unread_reminder": true, "completion_notify_users": []string{}, "unread_reminder_users": []string{"ou_owner"}})
	if err != nil || enabled || len(users) != 0 {
		t.Fatalf("canonical settings must override legacy: %v %v %v", enabled, users, err)
	}
	enabled, _, err = parseCompletionNotificationOptions(nil)
	if err != nil || enabled {
		t.Fatal("must default off")
	}
	f := newCompletionFixture(t)
	original := f.p.completionNotifications.scope
	for _, change := range []string{"project", "app", "domain"} {
		p := &Platform{platformName: "feishu", appID: f.p.appID, domain: f.p.domain}
		opts := map[string]any{}
		for k, v := range f.opts {
			opts[k] = v
		}
		switch change {
		case "project":
			opts["cc_project"] = "other"
		case "app":
			p.appID = "other"
		case "domain":
			p.domain = "other"
		}
		if err := p.configureCompletionNotifications(opts); err != nil {
			t.Fatal(err)
		}
		if p.completionNotifications.scope == original || completionNotificationID(p.completionNotifications.scope, "ou_owner", "session", "om_trigger") == completionNotificationID(original, "ou_owner", "session", "om_trigger") {
			t.Fatal("scope not isolated", change)
		}
	}
}

type blockedCompletionLookupTransport struct {
	base    http.RoundTripper
	entered chan struct{}
	once    sync.Once
}

func (b *blockedCompletionLookupTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/messages/") {
		b.once.Do(func() { close(b.entered) })
		<-r.Context().Done()
		return nil, r.Context().Err()
	}
	return b.base.RoundTrip(r)
}

func TestCompletionNotificationCancellationDuringLookupPreventsSend(t *testing.T) {
	for _, stopPlatform := range []bool{false, true} {
		t.Run(map[bool]string{false: "parent cancelled", true: "platform stopped"}[stopPlatform], func(t *testing.T) {
			f := newCompletionFixture(t)
			transport := &blockedCompletionLookupTransport{base: http.DefaultTransport, entered: make(chan struct{})}
			f.p.client = lark.NewClient(f.p.appID, "test", lark.WithOpenBaseUrl(f.p.domain), lark.WithHttpClient(&http.Client{Transport: transport}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- f.notice(ctx, "om_trigger", "omt_topic", "ou_owner") }()
			select {
			case <-transport.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("lookup did not start")
			}
			if stopPlatform {
				f.p.stopCompletionNotifications()
			} else {
				cancel()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("expected cancellation")
				}
			case <-time.After(time.Second):
				t.Fatal("notification did not stop promptly")
			}
			if len(f.privateBodies) != 0 {
				t.Fatal("private notification sent after cancellation")
			}
		})
	}
}
