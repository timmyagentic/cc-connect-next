package feishu

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/timmyagentic/cc-connect-next/core"
)

func resourceTestPlatform(server *httptest.Server) *Platform {
	return &Platform{
		platformName: "feishu", domain: server.URL, appID: "cli_resource_regression", appSecret: "synthetic-secret",
		client: lark.NewClient("cli_resource_regression", "synthetic-secret", lark.WithOpenBaseUrl(server.URL), lark.WithHttpClient(server.Client()), lark.WithEnableTokenCache(false)),
	}
}

func serveResourceTestToken(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	t.Helper()
	if r.URL.Path != "/open-apis/auth/v3/tenant_access_token/internal" {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(t, w, map[string]any{"code": 0, "expire": 7200, "tenant_access_token": "synthetic-resource-token"})
	return true
}

func TestDownloadResource_LargeFileAndImageRequireRange(t *testing.T) {
	payload := bytes.Repeat([]byte("0123456789"), 900000)
	for _, kind := range []string{"file", "image"} {
		t.Run(kind, func(t *testing.T) {
			var ranges atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				if r.Header.Get("Authorization") != "Bearer synthetic-resource-token" {
					t.Error("missing resource authentication")
				}
				if r.URL.Query().Get("type") != kind {
					t.Errorf("type = %q", r.URL.Query().Get("type"))
				}
				if r.Header.Get("Range") == "" {
					w.Header().Set("Content-Type", "application/json")
					writeJSON(t, w, map[string]any{"code": 234037, "msg": "download interrupted"})
					return
				}
				ranges.Add(1)
				var start, end int
				if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil || start < 0 || start >= len(payload) {
					http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
					return
				}
				if end >= len(payload) {
					end = len(payload) - 1
				}
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(payload[start : end+1])
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			var got []byte
			var err error
			if kind == "image" {
				got, _, err = p.downloadImage("om_large", "file_large")
			} else {
				got, err = p.downloadResourceContext(context.Background(), "om_large", "file_large", kind)
			}
			if err != nil {
				t.Fatalf("large resource must use Range: %v", err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatalf("resource bytes = %d, want %d", len(got), len(payload))
			}
			if ranges.Load() < 2 {
				t.Fatalf("range calls = %d, want chunked transfer", ranges.Load())
			}
		})
	}
}

func TestDispatchMessagePostFiles_CurrentAttachmentsReachAgent(t *testing.T) {
	for _, raw := range []string{
		`{"content":[[{"tag":"text","text":"review this"}]],"files":[{"file_key":"file_new","file_name":"new.txt"},{"file_key":"folder","is_folder":true},{"file_key":""}]}`,
		`{"zh_cn":{"content":[[{"tag":"text","text":"review this"}]],"files":[{"file_key":"file_new","file_name":"new.txt"}]}}`,
		`{"files":[{"file_key":"file_new","file_name":"new.txt"}]}`,
		`{"content":[[{"tag":"text","text":"review this"}]],"files":[{"file_key":"missing","file_name":"missing.txt"},{"file_key":"file_new","file_name":"new.txt"}]}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var downloads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				if strings.HasSuffix(r.URL.Path, "/resources/missing") {
					http.NotFound(w, r)
					return
				}
				if !strings.HasSuffix(r.URL.Path, "/resources/file_new") {
					t.Errorf("unexpected resource %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				downloads.Add(1)
				w.Header().Set("Content-Type", "application/octet-stream")
				_, _ = w.Write([]byte("new file body"))
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			p.userNameCache.Store("ou_alice", "Alice")
			p.chatNameCache.Store("oc_chat", "Chat")
			got := make(chan *core.Message, 1)
			p.handler = func(_ core.Platform, msg *core.Message) { got <- msg }
			rctx := replyContext{messageID: "om_post", chatID: "oc_chat", sessionKey: "feishu:oc_chat:ou_alice"}
			p.dispatchMessage(context.Background(), "post", raw, nil, "om_post", rctx.sessionKey, "ou_alice", "oc_chat", rctx, "", 0)
			select {
			case msg := <-got:
				if len(msg.Files) != 1 || msg.Files[0].FileName != "new.txt" || string(msg.Files[0].Data) != "new file body" || msg.Files[0].MimeType == "" {
					t.Fatalf("new post attachment lost: %+v", msg.Files)
				}
			case <-time.After(time.Second):
				t.Fatal("file-only post must dispatch")
			}
			if downloads.Load() != 1 {
				t.Fatalf("downloads = %d, want only valid file", downloads.Load())
			}
		})
	}
}

func TestResourceDownload_RejectsMalformedRangesAndOversizeAndAuth(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		header, body string
		max          int64
	}{
		{"wrong offset", 206, "bytes 1-1/10", "x", 32},
		{"missing header", 206, "", "x", 32},
		{"short first body", 206, "bytes 0-0/10", "", 32},
		{"oversized total", 206, "bytes 0-0/1000", "x", 32},
		{"oversized plain body", 200, "", strings.Repeat("x", 33), 32},
		{"unauthorized", 401, "", "", 32},
		{"API error envelope", 200, "", `{"code":234037,"msg":"download interrupted"}`, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				calls.Add(1)
				if tc.header != "" {
					w.Header().Set("Content-Range", tc.header)
				}
				if tc.name == "API error envelope" {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			p.resourceMaxBytes = tc.max
			if data, err := p.downloadResourceContext(context.Background(), "om", "file", "file"); err == nil {
				t.Fatalf("invalid resource accepted: %q", data)
			}
			if calls.Load() != 1 {
				t.Fatalf("invalid/auth response retried through another path: %d", calls.Load())
			}
		})
	}
}

func TestResourceDownload_RejectsChangedRangeAndHonorsCancellation(t *testing.T) {
	for _, mode := range []string{"changed total", "changed offset", "ignored second range", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				if r.Header.Get("Range") == "bytes=0-0" {
					w.Header().Set("Content-Range", "bytes 0-0/10")
					w.WriteHeader(206)
					_, _ = w.Write([]byte("0"))
					return
				}
				if mode == "cancel" {
					cancel()
					<-r.Context().Done()
					return
				}
				if mode == "ignored second range" {
					_, _ = w.Write([]byte("123456789"))
					return
				}
				header := "bytes 1-9/11"
				if mode == "changed offset" {
					header = "bytes 0-8/10"
				}
				w.Header().Set("Content-Range", header)
				w.WriteHeader(206)
				_, _ = w.Write([]byte("123456789"))
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			if data, err := p.downloadResourceContext(ctx, "om", "file", "file"); err == nil {
				t.Fatalf("corrupt/cancelled download accepted: %q", data)
			}
		})
	}
}
