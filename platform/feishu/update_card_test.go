package feishu

import (
	"context"
	"encoding/json"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/timmyagentic/cc-connect-next/core"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateCardReference_BindsExactMessageAppProjectAndRecipient(t *testing.T) {
	var paths []string
	var content string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "tenant_access_token") {
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"fixture","expire":3600}`))
			return
		}
		paths = append(paths, r.Method+" "+r.URL.Path)
		var body struct {
			Content string `json:"content"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		content = body.Content
		_, _ = w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	defer server.Close()
	raw, err := New(map[string]any{"app_id": "cli_update_reference_test", "app_secret": "fixture", "cc_project": "project"})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*interactivePlatform)
	p.BindProject("project")
	p.client = lark.NewClient(p.appID, "fixture", lark.WithOpenBaseUrl(server.URL), lark.WithHttpClient(server.Client()))
	ref, err := p.CardReference(replyContext{messageID: "om_exact", chatID: "oc_chat", sessionKey: "feishu:oc_chat:ou_owner", directUserID: "ou_owner"})
	if err != nil {
		t.Fatal(err)
	}
	var saved durableCardReference
	if err := json.Unmarshal(ref, &saved); err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []durableCardReference{{AppID: "other", Project: saved.Project, MessageID: saved.MessageID}, {AppID: saved.AppID, Project: "other", MessageID: saved.MessageID}} {
		blob, _ := json.Marshal(foreign)
		if err := p.UpdateCardByReference(context.Background(), blob, core.NewCard().Build()); err == nil {
			t.Fatal("foreign reference accepted")
		}
	}
	if len(paths) != 0 {
		t.Fatal("foreign reference reached API")
	}
	card := core.NewCard().Buttons(core.PrimaryBtn("Retry", "cmd:/upgrade confirm exact")).Build()
	if err := p.UpdateCardByReference(context.Background(), ref, card); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "PATCH /open-apis/im/v1/messages/om_exact" {
		t.Fatalf("wrong message: %v", paths)
	}
	for _, value := range []string{"feishu:oc_chat:ou_owner", "project", "ou_owner", "cmd:/upgrade confirm exact"} {
		if !strings.Contains(content, value) {
			t.Fatalf("lost binding %s: %s", value, content)
		}
	}
}
