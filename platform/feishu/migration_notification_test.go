package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestSendDirectUserUsesOpenID(t *testing.T) {
	const userID = "ou_migration_operator"
	const message = "迁移完成，cc-connect-next 已运行"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			writeJSON(t, w, map[string]any{
				"code": 0, "msg": "success", "expire": 7200,
				"tenant_access_token": "test-token",
			})
		case "/open-apis/im/v1/messages":
			if got := r.URL.Query().Get("receive_id_type"); got != "open_id" {
				t.Fatalf("receive_id_type = %q, want open_id", got)
			}
			var body struct {
				ReceiveID string `json:"receive_id"`
				MsgType   string `json:"msg_type"`
				Content   string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ReceiveID != userID || body.MsgType != "text" {
				t.Fatalf("request = %+v", body)
			}
			var text map[string]string
			if err := json.Unmarshal([]byte(body.Content), &text); err != nil {
				t.Fatal(err)
			}
			if text["text"] != message {
				t.Fatalf("text = %q", text["text"])
			}
			writeJSON(t, w, map[string]any{
				"code": 0, "msg": "success",
				"data": map[string]any{"message_id": "om_migration_done"},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	platform, err := newPlatform("feishu", srv.URL, map[string]any{
		"app_id": "cli_test", "app_secret": "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	sender, ok := platform.(interface {
		SendDirectUser(context.Context, string, string) error
	})
	if !ok {
		t.Fatalf("interactive Feishu wrapper does not expose SendDirectUser: %T", platform)
	}
	if err := sender.SendDirectUser(context.Background(), userID, message); err != nil {
		t.Fatal(err)
	}
}

func TestSendDirectUserRejectsNonOpenIDBeforeAPI(t *testing.T) {
	p := &Platform{platformName: "feishu"}
	if err := p.SendDirectUser(context.Background(), "oc_group_chat", "update"); err == nil || !strings.Contains(err.Error(), "open_id") {
		t.Fatalf("invalid direct user error = %v", err)
	}
}

func TestSendDirectUserCardUsesOpenID(t *testing.T) {
	const userID = "ou_update_admin"
	const projectName = "release-project"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			writeJSON(t, w, map[string]any{"code": 0, "msg": "success", "expire": 7200, "tenant_access_token": "test-token"})
		case "/open-apis/im/v1/messages":
			if got := r.URL.Query().Get("receive_id_type"); got != "open_id" {
				t.Fatalf("receive_id_type = %q, want open_id", got)
			}
			var body struct {
				ReceiveID string `json:"receive_id"`
				MsgType   string `json:"msg_type"`
				Content   string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ReceiveID != userID || body.MsgType != "interactive" || !strings.Contains(body.Content, "cmd:/upgrade") {
				t.Fatalf("direct card request = %+v", body)
			}
			if !strings.Contains(body.Content, `"cc_project":"`+projectName+`"`) ||
				!strings.Contains(body.Content, `"cc_direct_user":"`+userID+`"`) {
				t.Fatalf("direct card action is not bound to project/recipient: %s", body.Content)
			}
			writeJSON(t, w, map[string]any{"code": 0, "msg": "success", "data": map[string]any{"message_id": "om_update_notice"}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	platform, err := newPlatform("feishu", srv.URL, map[string]any{"app_id": "cli_test", "app_secret": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	core.NewEngine(projectName, nil, []core.Platform{platform}, "", core.LangEnglish)
	sender, ok := platform.(core.DirectUserCardSender)
	if !ok {
		t.Fatalf("interactive Feishu wrapper does not expose DirectUserCardSender: %T", platform)
	}
	card := core.NewCard().Markdown("update available").Buttons(core.CardButton{
		Text: "Review", Value: "cmd:/upgrade",
		Extra: map[string]string{"cc_project": "forged-project", "cc_direct_user": "ou_other"},
	}).Build()
	if err := sender.SendDirectUserCard(context.Background(), userID, card); err != nil {
		t.Fatal(err)
	}
}
