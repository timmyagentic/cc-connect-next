package feishu

import (
	"context"
	"fmt"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIssue146_AuditGenericFileMIME(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		_, _ = w.Write([]byte("%PDF-1.7\nbody"))
	}))
	defer server.Close()
	p := resourceTestPlatform(server)
	var got *core.Message
	p.handler = func(_ core.Platform, m *core.Message) { got = m }
	p.dispatchMessage(context.Background(), "file", `{"file_key":"pdf","file_name":"report.pdf"}`, nil, "om", "s", "", "", replyContext{}, "", 0)
	if got == nil || len(got.Files) != 1 || got.Files[0].MimeType != "application/pdf" {
		t.Fatalf("PDF incorrectly classified: %+v", got)
	}
}
func TestIssue146_AuditShortRIFF(t *testing.T) {
	for n := 8; n < 12; n++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("short RIFF panic: %v", r)
				}
			}()
			mt := detectMimeType([]byte("RIFF" + strings.Repeat("x", n-4)))
			if mt == "image/png" {
				t.Error("unknown RIFF misclassified as PNG")
			}
		}()
	}
}
func TestIssue146_AuditJSONCodeFieldIsNotAnEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":999}`))
	}))
	defer server.Close()
	if data, err := resourceTestPlatform(server).downloadResourceContext(context.Background(), "om", "key", "file"); err != nil || string(data) != `{"code":999}` {
		t.Fatalf("user JSON code field misclassified: %s %v", data, err)
	}
}

func TestIssue146_AuditChunkedAPIEnvelope(t *testing.T) {
	body := []byte(`{"code":999,"msg":"failure"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		var start, end int
		_, _ = fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
		if end >= len(body) {
			end = len(body) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(body[start : end+1])
	}))
	defer server.Close()
	if data, err := resourceTestPlatform(server).downloadResourceContext(context.Background(), "om", "key", "file"); err == nil {
		t.Fatalf("assembled API error forwarded: %s", data)
	}
}

func TestIssue146_AuditCancelledImageNeverBuffers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		cancel()
		_, _ = w.Write([]byte("\xff\xd8\xffimage-data"))
	}))
	defer server.Close()
	p := resourceTestPlatform(server)
	p.handler = func(_ core.Platform, _ *core.Message) { t.Error("cancelled image reached Agent") }
	p.dispatchMessage(ctx, "image", `{"image_key":"img"}`, nil, "om", "s", "", "", replyContext{}, "", 0)
	p.flushImageBatchForSession("s")
}
func TestIssue146_AuditHistoricalPostFilesRemainVisible(t *testing.T) {
	raw := `{"en_us":{"files":[{"file_key":"private","file_name":"notes.txt"}]}}`
	p := &Platform{}
	text, _ := p.parsePostContent("old", raw)
	if !strings.Contains(strings.Join(text, ""), "file") {
		t.Fatalf("historical post file silently gone: %v", text)
	}
	if !strings.Contains(extractPostPlainText(raw), "file") {
		t.Fatal("history text dropped file")
	}
}

func TestIssue146_AuditQuotedFailureVisible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		if r.URL.Path == "/open-apis/im/v1/messages/parent" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"msg_type":"file","sender":{"id":"alice","sender_type":"user"},"body":{"content":"{\"file_key\":\"missing\",\"file_name\":\"notes.txt\"}"}}]}}`))
			return
		}
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()
	p := resourceTestPlatform(server)
	p.botOpenID = "bot"
	p.userNameCache.Store("alice", "Alice")
	var got *core.Message
	p.handler = func(_ core.Platform, m *core.Message) { got = m }
	mentions := []*larkim.MentionEvent{{Key: strPtr("@bot"), Id: &larkim.UserId{OpenId: strPtr("bot")}}}
	p.dispatchMessage(context.Background(), "text", `{"text":"inspect"}`, mentions, "current", "s", "alice", "", replyContext{}, "parent", 0)
	if got == nil || !strings.Contains(got.ExtraContent, "unavailable") {
		t.Fatalf("quoted failure not surfaced: %+v", got)
	}
}

func TestIssue146_AuditNonImageResponseRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		_, _ = w.Write([]byte("<html>upstream unavailable</html>"))
	}))
	defer server.Close()
	p := resourceTestPlatform(server)
	if _, _, err := p.downloadImage("om", "img"); err == nil {
		t.Fatal("HTML forwarded as image")
	}
}

func TestIssue146_AuditMessageMatrix(t *testing.T) {
	for _, locale := range []string{"flat", "zh_cn", "en_us", "ja_jp", "es_es"} {
		t.Run(locale, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/img"):
					_, _ = w.Write([]byte("\xff\xd8\xffimage"))
				case strings.HasSuffix(r.URL.Path, "/video"):
					_, _ = w.Write([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"))
				case strings.HasSuffix(r.URL.Path, "/pdf"):
					_, _ = w.Write([]byte("%PDF-1.7\nbody"))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			p := resourceTestPlatform(server)
			p.botOpenID = "bot"
			raw := `{"content":[[{"tag":"at","user_id":"bot"},{"tag":"text","text":"inspect"},{"tag":"img","image_key":"img"},{"tag":"media","file_key":"video","image_key":"img"}]],"files":[{"file_key":"pdf","file_name":"report.pdf"},{"file_key":"missing","file_name":"missing.pdf"}]}`
			if locale != "flat" {
				raw = `{"` + locale + `":` + raw + `}`
			}
			var got *core.Message
			p.handler = func(_ core.Platform, m *core.Message) { got = m }
			p.dispatchMessage(context.Background(), "post", raw, nil, "current", "s", "", "", replyContext{}, "", 0)
			if got == nil || len(got.Images) != 2 || len(got.Files) != 2 || !strings.Contains(got.Content, "inspect") || strings.Count(got.Content, "unavailable") != 1 {
				t.Fatalf("mixed message lost content: %+v", got)
			}
			if got.Files[0].MimeType != "video/mp4" || got.Files[1].MimeType != "application/pdf" {
				t.Fatalf("MIME drift: %+v", got.Files)
			}
		})
	}
}

func TestIssue146_AuditAudioDispatch(t *testing.T) {
	payload := []byte("OggS-synthetic-opus")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		if r.URL.Query().Get("type") != "file" {
			t.Error("audio must request file resource")
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	p := resourceTestPlatform(server)
	var got *core.Message
	p.handler = func(_ core.Platform, m *core.Message) { got = m }
	p.dispatchMessage(context.Background(), "audio", `{"file_key":"voice","duration":2500}`, nil, "om", "s", "", "", replyContext{}, "", 0)
	if got == nil || got.Audio == nil || string(got.Audio.Data) != string(payload) || got.Audio.Duration != 2 || got.Audio.Format != "ogg" {
		t.Fatalf("audio contract: %+v", got)
	}
}

func TestIssue146_AuditForwardedFileScopeAndMIME(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		_, _ = w.Write([]byte("%PDF-1.7\nbody"))
	}))
	defer server.Close()
	p := resourceTestPlatform(server)
	children := map[string][]*larkim.Message{"root": {{MessageId: strPtr("source"), MsgType: strPtr("file"), Body: &larkim.MessageBody{Content: strPtr(`{"file_key":"pdf","file_name":"report.pdf"}`)}}}}
	var text strings.Builder
	var images []core.ImageAttachment
	var files []core.FileAttachment
	p.formatMergeForwardTree("root", children, nil, &text, &images, &files, 0)
	if len(files) != 1 || files[0].MessageID != "source" || files[0].MimeType != "application/pdf" {
		t.Fatalf("forwarded file lost identity/MIME: %+v", files)
	}
}

func TestIssue146_AuditBoundedRetryAndJSONAttachment(t *testing.T) {
	for _, mode := range []string{"retry", "json attachment"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			payload := []byte(`{"code":999,"msg":"user document"}`)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if serveResourceTestToken(t, w, r) {
					return
				}
				calls++
				if mode == "retry" && calls == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Disposition", `attachment; filename="data.json"`)
				_, _ = w.Write(payload)
			}))
			defer server.Close()
			got, err := resourceTestPlatform(server).downloadResourceContext(context.Background(), "om", "key", "file")
			if err != nil || string(got) != string(payload) {
				t.Fatalf("legitimate document rejected/corrupted: %q %v", got, err)
			}
			want := 1
			if mode == "retry" {
				want = 2
			}
			if calls != want {
				t.Fatalf("retry count=%d want %d", calls, want)
			}
		})
	}
}

func TestIssue146_AuditMissingPostFileAndFolder(t *testing.T) {
	for _, raw := range []string{`{"files":[{"file_name":"missing.txt"}]}`, `{"files":[{"file_key":"folder","is_folder":true}]}`} {
		p := &Platform{}
		var got *core.Message
		p.handler = func(_ core.Platform, m *core.Message) { got = m }
		p.dispatchMessage(context.Background(), "post", raw, nil, "om", "s", "", "", replyContext{}, "", 0)
		if got == nil || strings.TrimSpace(got.Content) == "" || len(got.Files) != 0 {
			t.Fatalf("unsupported/missing file silently lost or downloaded: %+v", got)
		}
	}
}
