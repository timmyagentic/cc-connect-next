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

// Drive the SDK entrypoint: helper-only batch tests miss downloads that have
// not reached bufferImage yet when a newer text event arrives.
func TestInboundOrder_SlowImageCannotBeOvertakenByText(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		if strings.Contains(r.URL.Path, "/resources/") {
			select {
			case <-started:
			default:
				close(started)
			}
			<-release
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nimage"))
			return
		}
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{}})
	}))
	defer srv.Close()
	// Release blocked HTTP requests even when an assertion fails.
	defer once.Do(func() { close(release) })
	p := resourceTestPlatform(srv)
	p.dedup = &core.MessageDedup{}
	p.imageBatchWindow = time.Hour
	received := make(chan *core.Message, 8)
	p.handler = func(_ core.Platform, m *core.Message) { received <- m }
	now := time.Now().UnixMilli()
	send := func(id, chat, kind, body string, ts int64) {
		t.Helper()
		stamp := strconv.FormatInt(ts, 10)
		err := p.onMessage(context.Background(), &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{
			Sender:  &larkim.EventSender{SenderId: &larkim.UserId{}},
			Message: &larkim.EventMessage{MessageId: &id, ChatId: &chat, ChatType: stringPtr("p2p"), MessageType: &kind, Content: &body, CreateTime: &stamp},
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
	send("om_image", "oc_one", "image", `{"image_key":"slow"}`, now)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("image download never started")
	}
	send("om_text", "oc_one", "text", `{"text":"inspect this"}`, now+1)
	send("om_other", "oc_other", "text", `{"text":"independent chat"}`, now+2)
	select {
	case m := <-received:
		if m.MessageID != "om_other" {
			t.Fatalf("newer text overtook pending image: %s", m.MessageID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("unrelated chat blocked behind download")
	}
	once.Do(func() { close(release) })
	for _, want := range []string{"om_image", "om_text"} {
		select {
		case m := <-received:
			if m.MessageID != want {
				t.Fatalf("got %s, want %s", m.MessageID, want)
			}
			if want == "om_image" && len(m.Images) != 1 {
				t.Fatal("image payload lost")
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("missing %s", want)
		}
	}
}

func TestInboundOrder_BatchTimerCannotOvertakePendingDispatch(t *testing.T) {
	received := make(chan *core.Message, 2)
	p := &Platform{platformName: "feishu", imageBatchWindow: time.Millisecond,
		handler: func(_ core.Platform, m *core.Message) { received <- m }}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	p.enqueueInboundDispatch("s", func() {
		close(started)
		<-release
		p.dispatchCoreMessage(&core.Message{MessageID: "earlier", SessionKey: "s"})
	})
	<-started
	p.bufferImage("s", &imageBatchEntry{sessionKey: "s", messageIDs: []string{"image"},
		images: []core.ImageAttachment{{Data: []byte("image")}}})
	select {
	case <-received:
		t.Fatal("batch timer overtook an earlier inbound dispatch")
	case <-time.After(50 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	for _, want := range []string{"earlier", "image"} {
		select {
		case m := <-received:
			if m.MessageID != want {
				t.Fatalf("got %s, want %s", m.MessageID, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("missing %s", want)
		}
	}
}

// A timer may have fired and joined the FIFO while a later download is still
// running. Resetting that batch must invalidate the already-queued callback.
func TestInboundOrder_QueuedOldTimerCannotFlushResetBatch(t *testing.T) {
	received := make(chan *core.Message, 2)
	p := &Platform{platformName: "feishu", imageBatchWindow: time.Hour,
		handler: func(_ core.Platform, m *core.Message) { received <- m }}
	entry := func(id string) *imageBatchEntry {
		return &imageBatchEntry{sessionKey: "s", messageIDs: []string{id},
			images: []core.ImageAttachment{{Data: []byte(id)}}}
	}
	p.bufferImage("s", entry("a"))
	defer p.flushImageBatches()
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	p.enqueueInboundDispatch("s", func() {
		close(started)
		<-release
		p.bufferImage("s", entry("b")) // begins a fresh one-hour quiet window
	})
	<-started
	p.inboundOrderMu.Lock()
	blockedTail := p.inboundOrderTail["s"]
	p.inboundOrderMu.Unlock()
	// Expire A's timer while B's simulated download holds the FIFO head.
	p.imageBatchMu.Lock()
	p.imageBatch["s"].timer.Reset(time.Millisecond)
	p.imageBatchMu.Unlock()
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		p.inboundOrderMu.Lock()
		queued := p.inboundOrderTail["s"] != blockedTail
		p.inboundOrderMu.Unlock()
		if queued {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatal("old timer never joined the dispatch queue")
		}
	}
	drained := make(chan struct{})
	p.enqueueInboundDispatch("s", func() { close(drained) })
	once.Do(func() { close(release) })
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		t.Fatal("queued timer did not complete")
	}
	select {
	case m := <-received:
		t.Fatalf("stale timer flushed reset batch before its quiet window: %s", m.MessageID)
	default:
	}
	// The batch is still available for its new timer or a subsequent text flush.
	p.flushImageBatchForSession("s")
	select {
	case m := <-received:
		if m.MessageID != "b" || len(m.Images) != 2 {
			t.Fatalf("reset batch lost images: %+v", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reset batch was lost")
	}
}
