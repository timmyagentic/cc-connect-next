package feishu

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestIssue146_MissingProbeRangeFallsBack(t *testing.T) {
	for _, kind := range []string{"image", "file"} {
		t.Run(kind, func(t *testing.T) {
			payload := bytes.Repeat([]byte("image-body"), 100)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				calls++
				if r.Header.Get("Authorization") != "Bearer synthetic-resource-token" || r.Header.Get("Accept-Encoding") != "identity" {
					t.Error("request lost authentication/encoding")
				}
				if calls == 1 {
					if r.Header.Get("Range") != "bytes=0-0" {
						t.Error("missing probe")
					}
					w.WriteHeader(206)
				} else if r.Header.Get("Range") != "" {
					t.Error("fallback still ranged")
				}
				_, _ = w.Write(payload)
			}))
			defer server.Close()
			got, err := resourceTestPlatform(server).downloadResourceContext(context.Background(), "om", "key", kind)
			if err != nil || !bytes.Equal(got, payload) || calls != 2 {
				t.Fatalf("got %d bytes, error %v, calls %d", len(got), err, calls)
			}
		})
	}
}

func TestIssue146_CurrentMediaAndFailuresReachAgent(t *testing.T) {
	cases := []struct {
		name, kind, raw         string
		images, files, failures int
	}{
		{"post image", "post", `{"content":[[{"tag":"img","image_key":"ok"},{"tag":"text","text":"look"}]]}`, 1, 0, 0},
		{"partial images", "post", `{"content":[[{"tag":"img","image_key":"ok"},{"tag":"img","image_key":"missing"}]]}`, 1, 0, 1},
		{"only failed image", "post", `{"content":[[{"tag":"img","image_key":"missing"}]]}`, 0, 0, 1},
		{"missing image key", "post", `{"content":[[{"tag":"img"}]]}`, 0, 0, 1},
		{"post video", "post", `{"content":[[{"tag":"media","file_key":"video","image_key":"ok"}],[{"tag":"text","text":"look"}]]}`, 1, 1, 0},
		{"localized video", "post", `{"zh_cn":{"content":[[{"tag":"media","file_key":"video"}]]}}`, 0, 1, 0},
		{"standalone video", "media", `{"file_key":"video","image_key":"ok"}`, 1, 1, 0},
		{"failed video", "post", `{"content":[[{"tag":"media","file_key":"missing","image_key":"ok"}]]}`, 1, 0, 1},
		{"failed file", "post", `{"files":[{"file_key":"missing","file_name":"notes.txt"}]}`, 0, 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				if strings.HasSuffix(r.URL.Path, "/missing") {
					http.Error(w, "secret error detail", http.StatusForbidden)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/video") {
					_, _ = w.Write([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"))
					return
				}
				_, _ = w.Write([]byte("\xff\xd8\xffimage-data"))
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			var got *core.Message
			p.handler = func(_ core.Platform, m *core.Message) { got = m }
			p.dispatchMessage(context.Background(), tc.kind, tc.raw, nil, "om_current", "s", "", "", replyContext{}, "", 0)
			if got == nil {
				t.Fatal("media-only message disappeared")
			}
			if len(got.Images) != tc.images || len(got.Files) != tc.files {
				t.Fatalf("images=%d files=%d want %d/%d", len(got.Images), len(got.Files), tc.images, tc.files)
			}
			if tc.failures > 0 && !strings.Contains(got.Content, "unavailable") {
				t.Fatalf("missing safe failure notice: %q", got.Content)
			}
			if strings.Contains(got.Content, "secret error") {
				t.Fatal("leaked error")
			}
			for _, f := range got.Files {
				if f.MimeType != "video/mp4" || !strings.HasSuffix(f.FileName, ".mp4") {
					t.Fatalf("video metadata: %+v", f)
				}
			}
		})
	}
}

func TestIssue146_FallbackRemainsBounded(t *testing.T) {
	for _, mode := range []string{"partial fallback", "oversize", "api error", "auth", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				calls++
				if calls == 1 {
					w.WriteHeader(206)
					_, _ = w.Write([]byte("ignored probe body"))
					return
				}
				if r.Header.Get("Range") != "" {
					t.Error("fallback must be plain GET")
				}
				switch mode {
				case "partial fallback":
					w.WriteHeader(206)
				case "oversize":
					_, _ = w.Write(bytes.Repeat([]byte("x"), 129))
				case "api error":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"code":999,"msg":"secret"}`))
				case "auth":
					w.WriteHeader(401)
				case "cancel":
					cancel()
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			p.resourceMaxBytes = 128
			if data, err := p.downloadResourceContext(ctx, "om", "key", "image"); err == nil || len(data) > 0 {
				t.Fatalf("accepted invalid fallback: %q %v", data, err)
			}
			if calls != 2 {
				t.Fatalf("calls=%d want 2", calls)
			}
		})
	}
}

func TestIssue146_HistoricalPostVideoNeverDownloads(t *testing.T) {
	p := &Platform{}
	text, images := p.parsePostContent("old", `{"content":[[{"tag":"media","file_key":"private","image_key":"private_cover"}]]}`)
	if len(images) != 0 || !strings.Contains(strings.Join(text, ""), "video") {
		t.Fatalf("historical video: %v %v", text, images)
	}
}

func TestIssue146_LaterMissingRangeNeverFallsBack(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		calls++
		if calls == 1 {
			w.Header().Set("Content-Range", "bytes 0-0/10")
			w.WriteHeader(206)
			_, _ = w.Write([]byte("0"))
			return
		}
		if r.Header.Get("Range") == "" {
			t.Error("later range failure triggered plain GET")
		}
		w.WriteHeader(206)
		_, _ = w.Write([]byte("123456789"))
	}))
	defer server.Close()
	if data, err := resourceTestPlatform(server).downloadResourceContext(context.Background(), "om", "key", "image"); err == nil || len(data) != 0 {
		t.Fatalf("invalid later chunk accepted: %q %v", data, err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestIssue146_CancelledCurrentMediaDoesNotDispatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("cancelled request reached HTTP") }))
	defer server.Close()
	p := resourceTestPlatform(server)
	p.handler = func(_ core.Platform, _ *core.Message) { t.Error("cancelled media dispatched") }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.dispatchMessage(ctx, "post", `{"content":[[{"tag":"media","file_key":"video"}]]}`, nil, "om", "s", "", "", replyContext{}, "", 0)
}
