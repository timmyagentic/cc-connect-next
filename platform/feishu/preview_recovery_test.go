package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/timmyagentic/cc-connect-next/core"
)

type previewRecoveryFixture struct {
	mu       sync.Mutex
	snapshot json.RawMessage
	requests []capturedRichCardRequest
	saveErr  error
	apiError bool
}

func newPreviewRecoveryFixture(t *testing.T) (*interactivePlatform, *previewRecoveryFixture) {
	t.Helper()
	f := &previewRecoveryFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "tenant_access_token") {
			writeJSON(t, w, map[string]any{"code": 0, "tenant_access_token": "fixture", "expire": 3600})
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		f.mu.Lock()
		f.requests = append(f.requests, capturedRichCardRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		var durable map[string]any
		if err := json.Unmarshal(f.snapshot, &durable); err != nil {
			t.Errorf("request sent without durable handle: %v", err)
		}
		if sequence, ok := body["sequence"]; ok && durable["sequence"] != sequence {
			t.Errorf("sequence was not saved before HTTP: request=%v checkpoint=%v", sequence, durable["sequence"])
		}
		apiError := f.apiError
		f.mu.Unlock()
		if apiError {
			w.WriteHeader(http.StatusInternalServerError)
		}
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{}})
	}))
	t.Cleanup(server.Close)
	client := lark.NewClient("cli_preview_recovery", "not-persisted-secret", lark.WithOpenBaseUrl(server.URL), lark.WithHttpClient(server.Client()))
	p := &interactivePlatform{Platform: &Platform{
		platformName: "feishu", appID: "cli_preview_recovery", appSecret: "not-persisted-secret", projectName: "project",
		domain: server.URL, useInteractiveCard: true, client: client, replayClient: client,
	}}
	return p, f
}

func (f *previewRecoveryFixture) save(snapshot json.RawMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.snapshot = append(json.RawMessage(nil), snapshot...)
	return nil
}

func previewRecoverySupport(t *testing.T, p *interactivePlatform) core.PersistentPreviewPlatform {
	t.Helper()
	support, ok := any(p).(core.PersistentPreviewPlatform)
	if !ok {
		t.Fatal("interactive platform cannot preserve an in-flight card across restart")
	}
	return support
}

func TestPreviewRecovery_ReusesCardEntityAndPersistsSequenceBeforeHTTP(t *testing.T) {
	p, f := newPreviewRecoveryFixture(t)
	support := previewRecoverySupport(t, p)
	handle := &feishuPreviewHandle{messageID: "om_answer", chatID: "oc_chat", cardID: "card_original", lastContent: "private draft"}
	if err := support.CheckpointPreview(handle, f.save); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private draft", "not-persisted-secret", "tenant_access_token"} {
		if strings.Contains(string(f.snapshot), secret) {
			t.Fatalf("snapshot contains content or credentials: %s", f.snapshot)
		}
	}
	if err := p.StreamRichCardText(context.Background(), handle, "first answer segment"); err != nil {
		t.Fatal(err)
	}
	// The old process and its handle disappear. Restore from durable state only.
	restored, err := support.RestorePreview(append(json.RawMessage(nil), f.snapshot...), f.save)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.StreamRichCardText(context.Background(), restored, "continued answer"); err != nil {
		t.Fatal(err)
	}
	final := buildRichCard(core.CardStatusDone, "done", nil, "completed answer", false, "")
	if err := p.UpdateMessage(context.Background(), restored, final); err != nil {
		t.Fatal(err)
	}
	// Delivery may have succeeded immediately before another process exit.
	// Replaying completion must still update that same card, not send a duplicate.
	restored, err = support.RestorePreview(append(json.RawMessage(nil), f.snapshot...), f.save)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateMessage(context.Background(), restored, final); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) != 4 {
		t.Fatalf("requests = %v", f.requests)
	}
	for i, request := range f.requests {
		if request.Method != http.MethodPut || !strings.HasPrefix(request.Path, "/open-apis/cardkit/v1/cards/card_original") || request.Body["sequence"] != float64(i+1) {
			t.Fatalf("request %d did not continue the original card: %+v", i, request)
		}
	}
}

func TestPreviewRecovery_SaveFailurePreventsRemoteMutation(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "full-card", true: "text-stream"}[stream], func(t *testing.T) {
			p, f := newPreviewRecoveryFixture(t)
			support := previewRecoverySupport(t, p)
			handle := &feishuPreviewHandle{messageID: "om_answer", chatID: "oc_chat", cardID: "card_original"}
			if err := support.CheckpointPreview(handle, f.save); err != nil {
				t.Fatal(err)
			}
			want := errors.New("disk unavailable")
			f.saveErr = want
			var err error
			if stream {
				err = p.StreamRichCardText(context.Background(), handle, "answer")
			} else {
				err = p.UpdateMessage(context.Background(), handle, buildRichCard(core.CardStatusDone, "done", nil, "answer", false, ""))
			}
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want persistence failure", err)
			}
			if len(f.requests) != 0 {
				t.Fatalf("remote mutation proceeded after save failure: %v", f.requests)
			}
		})
	}
}

func TestPreviewRecovery_UnknownRemoteOutcomeKeepsReservedSequence(t *testing.T) {
	p, f := newPreviewRecoveryFixture(t)
	support := previewRecoverySupport(t, p)
	handle := &feishuPreviewHandle{messageID: "om_answer", chatID: "oc_chat", cardID: "card_original"}
	if err := support.CheckpointPreview(handle, f.save); err != nil {
		t.Fatal(err)
	}
	f.apiError = true
	if err := p.StreamRichCardText(context.Background(), handle, "possibly accepted"); err == nil {
		t.Fatal("expected uncertain remote failure")
	}
	f.apiError = false
	restored, err := support.RestorePreview(append(json.RawMessage(nil), f.snapshot...), f.save)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateMessage(context.Background(), restored, buildRichCard(core.CardStatusDone, "done", nil, "answer", false, "")); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 2 || f.requests[0].Body["sequence"] != float64(1) || f.requests[1].Body["sequence"] != float64(2) {
		t.Fatalf("sequence reused after uncertain delivery: %v", f.requests)
	}
}

func TestPreviewRecovery_InlineFallbackUpdatesOriginalMessage(t *testing.T) {
	p, f := newPreviewRecoveryFixture(t)
	support := previewRecoverySupport(t, p)
	if err := support.CheckpointPreview(&feishuPreviewHandle{messageID: "om_inline", chatID: "oc_chat"}, f.save); err != nil {
		t.Fatal(err)
	}
	restored, err := support.RestorePreview(append(json.RawMessage(nil), f.snapshot...), f.save)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateMessage(context.Background(), restored, "answer"); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 1 || f.requests[0].Method != http.MethodPatch || f.requests[0].Path != "/open-apis/im/v1/messages/om_inline" {
		t.Fatalf("inline fallback did not use the original message: %v", f.requests)
	}
	f.saveErr = errors.New("disk unavailable")
	if err := p.UpdateMessage(context.Background(), restored, "another answer"); !errors.Is(err, f.saveErr) {
		t.Fatalf("inline mutation did not stop on save failure: %v", err)
	}
	if len(f.requests) != 1 {
		t.Fatal("inline mutation proceeded after save failure")
	}
}

func TestPreviewRecovery_RejectsForeignAndInvalidSnapshot(t *testing.T) {
	p, f := newPreviewRecoveryFixture(t)
	support := previewRecoverySupport(t, p)
	if err := support.CheckpointPreview(&feishuPreviewHandle{messageID: "om_answer", chatID: "oc_chat", cardID: "card_original"}, f.save); err != nil {
		t.Fatal(err)
	}
	for _, change := range []map[string]any{
		{"platform": "lark"}, {"app_id": "foreign"}, {"project": "foreign"}, {"version": 99},
		{"message_id": ""}, {"chat_id": ""}, {"sequence": -1}, {"card_id": "../foreign"},
	} {
		var snapshot map[string]any
		_ = json.Unmarshal(f.snapshot, &snapshot)
		for key, value := range change {
			snapshot[key] = value
		}
		blob, _ := json.Marshal(snapshot)
		if _, err := support.RestorePreview(blob, f.save); err == nil {
			t.Errorf("accepted invalid snapshot change %v", change)
		}
	}
	if len(f.requests) != 0 {
		t.Fatalf("restore reached HTTP: %v", f.requests)
	}
}

func TestTurnReplyRecovery_PreservesExactSourceAndVerifiedRecipient(t *testing.T) {
	p, _ := newPreviewRecoveryFixture(t)
	support, ok := any(p).(interface {
		SnapshotTurnReplyCtx(any) (json.RawMessage, error)
		RestoreTurnReplyCtx(json.RawMessage) (any, error)
	})
	if !ok {
		t.Fatal("ordinary turn reply targets cannot be recovered")
	}
	for _, original := range []replyContext{
		{messageID: "om_trigger", chatID: "oc_chat", groupID: "oc_chat", sessionKey: "feishu:oc_chat", contextEligible: true, contextTimeMs: 123, contextRootID: "om_root", referenceIDs: map[string]bool{"om_quote": true}},
		{messageID: "om_trigger", chatID: "oc_chat", groupID: "oc_chat", sessionKey: "feishu:oc_chat:root:om_root"},
		{messageID: "om_trigger", chatID: "oc_chat", directUserID: "ou_owner", sessionKey: "feishu:oc_chat:ou_owner"},
		{messageID: "om_trigger", chatID: "oc_chat", directUserID: "ou_owner", sessionKey: "feishu:oc_chat:thread:omt_topic", threadID: "omt_topic"},
	} {
		blob, err := support.SnapshotTurnReplyCtx(original)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := support.RestoreTurnReplyCtx(blob)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(restored, original) {
			t.Fatalf("routing or recipient changed: got %+v, want %+v", restored, original)
		}
		if !strings.Contains(original.sessionKey, ":thread:") {
			if _, err := p.SnapshotReplyCtx(original); !errors.Is(err, core.ErrNotSupported) {
				t.Fatalf("ordinary scheduler reply routing changed: %v", err)
			}
		}
	}
}

func TestTurnReplyRecovery_RejectsForeignOrMismatchedRoute(t *testing.T) {
	p, _ := newPreviewRecoveryFixture(t)
	support := previewRecoverySupport(t, p)
	router := any(support).(interface {
		SnapshotTurnReplyCtx(any) (json.RawMessage, error)
		RestoreTurnReplyCtx(json.RawMessage) (any, error)
	})
	blob, err := router.SnapshotTurnReplyCtx(replyContext{
		messageID: "om_trigger", chatID: "oc_chat", directUserID: "ou_owner", sessionKey: "feishu:oc_chat:thread:omt_topic", threadID: "omt_topic",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []map[string]any{
		{"platform": "lark"}, {"app_id": "foreign"}, {"project": "foreign"}, {"version": 99},
		{"chat_id": "oc_other"}, {"thread_id": "omt_other"}, {"message_id": "omt_topic"}, {"group_id": "oc_chat"},
	} {
		var snapshot map[string]any
		_ = json.Unmarshal(blob, &snapshot)
		for key, value := range change {
			snapshot[key] = value
		}
		foreign, _ := json.Marshal(snapshot)
		if _, err := router.RestoreTurnReplyCtx(foreign); err == nil {
			t.Errorf("accepted invalid turn reply route change %v", change)
		}
	}
}
