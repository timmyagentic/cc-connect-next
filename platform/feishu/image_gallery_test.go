package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestImageGallery_OriginalsInOneCardAndRouting(t *testing.T) {
	for _, tc := range []struct {
		name          string
		reply, thread bool
	}{
		{"new message", false, false}, {"quoted reply", true, false}, {"topic", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uploads, sends := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/open-apis/im/v1/images" {
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Error(err)
					}
					f, _, err := r.FormFile("image")
					if err != nil {
						t.Error(err)
						return
					}
					defer f.Close()
					b, _ := io.ReadAll(f)
					if string(b) != fmt.Sprintf("original-%d", uploads) {
						t.Errorf("original changed: %q", b)
					}
					uploads++
					fmt.Fprintf(w, `{"code":0,"data":{"image_key":"img_%d"}}`, uploads)
					return
				}
				sends++
				var body struct {
					Content, MsgType, ReceiveID, UUID string
					ReplyInThread                     bool
				}
				var raw map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
					t.Error(err)
				}
				json.Unmarshal(raw["content"], &body.Content)
				json.Unmarshal(raw["msg_type"], &body.MsgType)
				json.Unmarshal(raw["receive_id"], &body.ReceiveID)
				json.Unmarshal(raw["uuid"], &body.UUID)
				json.Unmarshal(raw["reply_in_thread"], &body.ReplyInThread)
				wantPath := "/open-apis/im/v1/messages"
				if tc.reply || tc.thread {
					wantPath = "/open-apis/im/v1/messages/om_trigger/reply"
				}
				if r.URL.Path != wantPath || body.MsgType != "interactive" || body.UUID == "" {
					t.Errorf("wrong delivery %s %+v", r.URL.Path, body)
				}
				if body.ReplyInThread != tc.thread {
					t.Errorf("thread locality lost: %+v", body)
				}
				if !tc.reply && !tc.thread && body.ReceiveID != "oc_target" {
					t.Error("wrong chat")
				}
				var card struct {
					Schema string
					Body   struct{ Elements []galleryElement }
				}
				if err := json.Unmarshal([]byte(body.Content), &card); err != nil {
					t.Error(err)
				}
				if card.Schema != "2.0" || len(card.Body.Elements) != 1 {
					t.Errorf("invalid gallery: %s", body.Content)
					return
				}
				el := card.Body.Elements[0]
				if el.Tag != "img_combination" || el.Mode != "bisect" || len(el.Images) != 5 {
					t.Errorf("wrong layout: %+v", el)
				}
				for i, img := range el.Images {
					if img.Key != fmt.Sprintf("img_%d", i+1) {
						t.Error("gallery order changed")
					}
				}
				fmt.Fprint(w, `{"code":0,"data":{"message_id":"om_gallery"}}`)
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			p.useInteractiveCard = true
			p.noReplyToTrigger = !tc.reply
			rc := replyContext{chatID: "oc_target", messageID: "om_trigger"}
			if tc.thread {
				rc.threadID = "omt_topic"
				rc.sessionKey = "feishu:oc_target:root:om_root"
			}
			images := make([]core.ImageAttachment, 5)
			for i := range images {
				images[i].Data = []byte(fmt.Sprintf("original-%d", i))
			}
			if err := p.SendImageBatch(context.Background(), rc, images); err != nil {
				t.Fatal(err)
			}
			if uploads != 5 || sends != 1 {
				t.Fatalf("uploads=%d sends=%d", uploads, sends)
			}
		})
	}
}

func TestImageGallery_FailedUploadNeverPublishesPartialCard(t *testing.T) {
	uploads, sends := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/images") {
			uploads++
			if uploads == 2 {
				fmt.Fprint(w, `{"code":999,"msg":"rejected"}`)
				return
			}
			fmt.Fprint(w, `{"code":0,"data":{"image_key":"img_ok"}}`)
			return
		}
		sends++
		fmt.Fprint(w, `{"code":0}`)
	}))
	defer server.Close()
	p := resourceTestPlatform(server)
	p.useInteractiveCard = true
	err := p.SendImageBatch(context.Background(), replyContext{chatID: "oc"}, []core.ImageAttachment{{Data: []byte("one")}, {Data: []byte("two")}, {Data: []byte("three")}})
	if err == nil || uploads != 2 || sends != 0 {
		t.Fatalf("partial card leaked: uploads=%d sends=%d err=%v", uploads, sends, err)
	}
}

func TestImageGallery_RetryReusesUUIDWithoutReuploading(t *testing.T) {
	for _, reply := range []bool{false, true} {
		t.Run(fmt.Sprint(reply), func(t *testing.T) {
			uploads, attempts := 0, 0
			var uuid string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				if strings.HasSuffix(r.URL.Path, "/images") {
					uploads++
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"code":0,"data":{"image_key":"img_ok"}}`)
					return
				}
				attempts++
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				got, _ := body["uuid"].(string)
				if got == "" {
					t.Error("missing idempotency key")
				}
				if attempts == 1 {
					uuid = got
					// Simulate acceptance followed by connection loss before the response.
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					conn.Close()
					return
				}
				if uuid != got {
					t.Error("retry generated a different message UUID")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"code":0,"data":{"message_id":"om_same"}}`)
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			p.useInteractiveCard = true
			p.noReplyToTrigger = !reply
			if err := p.SendImageBatch(context.Background(), replyContext{chatID: "oc", messageID: "om"}, []core.ImageAttachment{{Data: []byte("one")}, {Data: []byte("two")}}); err != nil {
				t.Fatal(err)
			}
			if uploads != 2 || attempts != 2 {
				t.Fatalf("uploads=%d attempts=%d", uploads, attempts)
			}
		})
	}
}

func TestImageGallery_DisabledAndOversizedDoNotUpload(t *testing.T) {
	p := &Platform{}
	if p.ImageBatchSize() != 1 {
		t.Fatal("cards disabled but galleries enabled")
	}
	if err := p.SendImageBatch(context.Background(), replyContext{}, []core.ImageAttachment{{}}); err == nil {
		t.Fatal("disabled card accepted")
	}
	p.useInteractiveCard = true
	if err := p.SendImageBatch(context.Background(), replyContext{}, make([]core.ImageAttachment, 6)); err == nil {
		t.Fatal("oversized batch silently truncated")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.SendImageBatch(ctx, replyContext{}, []core.ImageAttachment{{}}); err == nil {
		t.Fatal("cancellation ignored")
	}
}
