package feishu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestInboundOrder_CanceledNameLookupReleasesSession(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		if strings.Contains(r.URL.Path, "/contact/") {
			once.Do(func() { close(started) })
			<-release
		}
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{}})
	}))
	defer srv.Close()
	defer close(release)
	p := resourceTestPlatform(srv)
	p.dedup = &core.MessageDedup{}
	p.chatNameCache.Store("oc_review", "cached chat")
	received := make(chan *core.Message, 4)
	p.handler = func(_ core.Platform, m *core.Message) { received <- m }
	send := func(ctx context.Context, id, user string) {
		chat, kind, content, stamp := "oc_review", "text", `{"text":"hello"}`, strconv.FormatInt(time.Now().UnixMilli(), 10)
		err := p.onMessage(ctx, &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &user}}, Message: &larkim.EventMessage{MessageId: &id, ChatId: &chat, ChatType: stringPtr("p2p"), MessageType: &kind, Content: &content, CreateTime: &stamp}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	send(ctx, "om_first", "ou_lookup")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("lookup not started")
	}
	cancel()
	p.userNameCache.Store("ou_lookup", "cached by another chat")
	send(context.Background(), "om_second", "ou_lookup")
	select {
	case m := <-received:
		if m.MessageID != "om_second" {
			t.Fatal("unexpected message")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled first lookup still blocks later message in session")
	}
}

// Metadata is optional: an unavailable API must fall back to IDs instead of
// holding the session FIFO indefinitely, even without caller cancellation.
func TestNameLookupsHaveFiniteTimeoutAndFallback(t *testing.T) {
	for _, kind := range []string{"user", "chat"} {
		t.Run(kind, func(t *testing.T) {
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			p := resourceTestPlatform(server)
			done := make(chan string, 1)
			go func() {
				if kind == "user" {
					done <- p.resolveUserName("ou_timeout")
				} else {
					done <- p.resolveChatName("oc_timeout")
				}
			}()
			select {
			case got := <-done:
				want := "ou_timeout"
				if kind == "chat" {
					want = "oc_timeout"
				}
				if got != want {
					t.Fatalf("got %q want fallback %q", got, want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("metadata lookup has no finite timeout")
			}
		})
	}
}
