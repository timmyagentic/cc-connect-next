package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func contextTypedMessage(id, kind, raw string, at time.Time) map[string]any {
	m := contextAPIMessage(id, "oc_group", "", "ou_alice", "Supplied Alice", "", at)
	m["msg_type"] = kind
	m["body"] = map[string]string{"content": raw}
	return m
}

func contextMediaTarget(at time.Time) replyContext {
	return replyContext{chatID: "oc_group", messageID: "trigger", contextEligible: true, contextTimeMs: at.UnixMilli()}
}

func TestGroupContextCommonTypesParseWithoutIO(t *testing.T) {
	for _, tc := range []struct {
		kind, raw, want string
		resources       int
		incomplete      bool
	}{
		{"image", `{"image_key":"img"}`, "[image #1]", 1, false},
		{"post", `{"zh_cn":{"title":"Title","content":[[{"tag":"text","text":"Body"},{"tag":"at","user_id":"ou_never_lookup"},{"tag":"img","image_key":"img"},{"tag":"media","file_key":"video","image_key":"cover"}]],"files":[{"file_key":"report","file_name":"report.pdf"}]}}`, "ou_never_lookup", 4, false},
		{"post", `{"content":[[{"tag":"file","file_key":"inline","file_name":"inline.csv"}]]}`, "inline.csv", 1, false},
		{"interactive", `{"schema":"2.0","header":{"title":{"content":"Human card"}},"body":{"elements":[{"tag":"markdown","content":"A readable card"},{"tag":"img","img_key":"card_image"}]}}`, "Human card", 1, false},
		{"file", `{"file_key":"file","file_name":"report.pdf"}`, "report.pdf", 1, false},
		{"audio", `{"file_key":"audio","duration":4000}`, "duration=4000", 1, false},
		{"video", `{"file_key":"video","image_key":"cover","duration":5000}`, "duration=5000", 2, false},
		{"media", `{"file_key":"video","file_name":"clip.mp4"}`, "clip.mp4", 1, false},
		{"share_chat", `{"chat_id":"oc_shared","chat_name":"Planning"}`, "Planning", 0, false},
		{"share_user", `{"user_id":"ou_shared"}`, "ou_shared", 0, false},
		{"location", `{"name":"Office","latitude":"31.2","longitude":"121.5"}`, "31.2", 0, false},
		{"system", `{"text":"Alice joined the group"}`, "Alice joined", 0, false},
		{"sticker", `{"file_key":"unsupported"}`, "does not support sticker download", 0, false},
		{"unknown", `{"future":"schema"}`, "unsupported message type", 0, true},
		{"image", `{}`, "missing resource key", 0, true},
		{"text", `{"text":42}`, "missing text", 0, true},
		{"post", `null`, "malformed", 0, true},
		{"interactive", `{"header":{"title":{"content":"Title only"}}}`, "no readable card body", 0, true},
		{"file", `{broken`, "malformed", 0, true},
		{"share_chat", `{}`, "missing readable metadata", 0, true},
		{"post", `{"content":[[{"tag":"future"}]]}`, "unsupported element", 0, true},
		{"post", `{"files":[{"file_key":"folder","is_folder":true}]}`, "folder download unsupported", 0, true},
	} {
		t.Run(tc.kind+"/"+tc.want, func(t *testing.T) {
			got := parseGroupContextContent(tc.kind, tc.raw, "outer")
			if !strings.Contains(got.Text, tc.want) || len(got.Resources) != tc.resources || got.Incomplete != tc.incomplete {
				t.Fatalf("got %+v", got)
			}
			for _, resource := range got.Resources {
				if resource.MessageID != "outer" {
					t.Fatalf("lost root source: %+v", resource)
				}
			}
			data, err := json.Marshal(got)
			if err != nil || strings.Contains(string(data), `"Resources"`) || strings.Contains(string(data), `"MessageID"`) {
				t.Fatalf("resource descriptors leaked to JSON: %s %v", data, err)
			}
		})
	}
}

func TestGroupContextForwardRootResourcesAndImageBinaryPayload(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	root := contextTypedMessage("outer", "merge_forward", `{}`, now.Add(-time.Minute))
	text := contextTypedMessage("child-text", "text", `{"text":"original decision"}`, now.Add(-time.Hour))
	image := contextTypedMessage("child-image", "image", `{"image_key":"one"}`, now.Add(-time.Hour))
	nested := contextTypedMessage("nested", "merge_forward", `{}`, now.Add(-time.Hour))
	second := contextTypedMessage("nested-image", "image", `{"image_key":"two"}`, now.Add(-time.Hour))
	second["upper_message_id"] = "nested"
	deleted := contextTypedMessage("removed", "image", `{"image_key":"never"}`, now)
	deleted["deleted"] = true
	png := []byte("\x89PNG\r\n\x1a\nbinary-image-payload")
	var downloads, gets int
	p, _ := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/im/v1/messages":
			writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{root}}})
		case "/open-apis/im/v1/messages/outer":
			gets++
			if r.URL.Query().Get("with_sender_name") != "true" {
				t.Error("forward source did not request supplied names")
			}
			writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{root, text, image, nested, second, deleted}}})
		default:
			downloads++
			if r.URL.Query().Get("type") != "image" {
				t.Error("wrong resource type")
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		}
	}, "/open-apis/im/v1/messages/outer", "/open-apis/im/v1/messages/outer/resources/one", "/open-apis/im/v1/messages/outer/resources/two")
	target := contextMediaTarget(now)
	loaded, err := p.LoadMessageContext(context.Background(), target, time.Time{})
	if err != nil || loaded.Incomplete || len(loaded.Messages) != 1 || downloads != 0 || gets != 1 {
		t.Fatalf("load downloaded resources or lost forward: %+v %v downloads=%d gets=%d", loaded, err, downloads, gets)
	}
	entry := loaded.Messages[0]
	for _, want := range []string{"original decision", "Supplied Alice", now.Add(-time.Hour).Format(time.RFC3339Nano), "[image #1]", "[image #2]", "nested-image"} {
		if !strings.Contains(entry.Text, want) {
			t.Fatalf("missing %q: %s", want, entry.Text)
		}
	}
	if strings.Contains(entry.Text, "removed") || entry.Incomplete || len(entry.Resources) != 2 {
		t.Fatalf("wrong forward tree: %+v", entry)
	}
	got, err := p.HydrateMessageContext(context.Background(), target, loaded.Messages)
	if err != nil || len(got) != 1 || got[0].ID != "outer" || got[0].Incomplete || len(got[0].Images) != 2 || downloads != 2 {
		t.Fatalf("hydrate = %+v %v downloads=%d", got, err, downloads)
	}
	for _, img := range got[0].Images {
		if !bytes.Equal(img.Data, png) || img.MimeType != "image/png" {
			t.Fatalf("invalid image payload: %+v", img)
		}
	}
	if len(loaded.Messages[0].Images) != 0 {
		t.Fatal("hydration mutated its input")
	}
}

func TestGroupContextContentFailureIsPerEntryAndStickerIsExpected(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	p, _ := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{
			contextTypedMessage("bad", "image", `{}`, now.Add(-time.Minute)),
			contextTypedMessage("sticker", "sticker", `{"file_key":"s"}`, now.Add(-time.Second)),
			contextTypedMessage("unknown", "future", `{"x":1}`, now.Add(-2*time.Minute)),
		}}})
	})
	got, err := p.LoadMessageContext(context.Background(), contextMediaTarget(now), time.Time{})
	if err != nil || got.Incomplete || len(got.Messages) != 3 {
		t.Fatalf("entry failure polluted list state: %+v %v", got, err)
	}
	for _, entry := range got.Messages {
		if (entry.ID != "sticker") != entry.Incomplete || !strings.Contains(entry.Text, "unavailable") {
			t.Fatalf("bad incomplete/placeholder: %+v", entry)
		}
	}
}

func TestGroupContextHydratePartialFailuresAndResourceIsolation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	png := []byte("\x89PNG\r\n\x1a\nimage")
	p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/bad"):
			w.WriteHeader(http.StatusForbidden)
		case strings.HasSuffix(r.URL.Path, "/not-image"):
			_, _ = w.Write([]byte("not an image"))
		case strings.HasSuffix(r.URL.Path, "/report"):
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.7\npdf"))
		default:
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		}
	}, "/open-apis/im/v1/messages/outer/resources/bad", "/open-apis/im/v1/messages/outer/resources/good",
		"/open-apis/im/v1/messages/outer/resources/not-image", "/open-apis/im/v1/messages/outer/resources/report")
	entry := parseGroupContextContent("post", `{"content":[[{"tag":"img","image_key":"bad"},{"tag":"img","image_key":"good"},{"tag":"img","image_key":"not-image"}]],"files":[{"file_key":"report","file_name":"report.pdf"}]}`, "outer")
	entry.ID, entry.Time = "outer", now.Add(-time.Minute)
	before := core.ContextMessage{ID: "before", Text: "preserved", Time: now.Add(-2 * time.Minute)}
	input := []core.ContextMessage{before, entry}
	got, err := p.HydrateMessageContext(context.Background(), contextMediaTarget(now), input)
	if err != nil || !reflect.DeepEqual(got[0], before) || !got[1].Incomplete || len(got[1].Images) != 1 || len(got[1].Files) != 1 || got[1].Files[0].FileName != "report.pdf" {
		t.Fatalf("lost partial messages/attachments: %+v %v", got, err)
	}
	if got[1].Images[0].FileName != "history-image-2" || got[1].ID != input[1].ID {
		t.Fatal("failed image shifted stable filename or changed receipt")
	}
	for _, want := range []string{"image #1: unavailable", "image #2: attached image 1", "image #3: unavailable"} {
		if !strings.Contains(got[1].Text, want) {
			t.Fatalf("missing %q: %s", want, got[1].Text)
		}
	}
	count := calls.Load()
	for _, target := range []any{"restored", replyContext{chatID: "oc_group"}, replyContext{chatID: "oc_other", contextEligible: true}} {
		out, err := p.HydrateMessageContext(context.Background(), target, input)
		if err == nil || !out[1].Incomplete || calls.Load() != count || len(out[1].Images) != 0 {
			t.Fatal("unauthorized target downloaded media")
		}
	}
	for _, change := range []string{"foreign-resource", "future", "expired", "recalled"} {
		invalid := entry
		invalid.Resources = append([]core.ContextResource(nil), entry.Resources...)
		switch change {
		case "foreign-resource":
			for i := range invalid.Resources {
				invalid.Resources[i].MessageID = "other-chat-source"
			}
		case "future":
			invalid.Time = now.Add(time.Second)
		case "expired":
			invalid.Time = now.Add(-time.Hour)
		case "recalled":
			// Exercise the event-maintained recall cache without another GET.
			p.markMessageRecalled("outer")
		}
		out, _ := p.HydrateMessageContext(context.Background(), contextMediaTarget(now), []core.ContextMessage{invalid})
		if !out[0].Incomplete || calls.Load() != count {
			t.Fatalf("%s crossed resource boundary: %+v", change, out)
		}
	}
}

func TestGroupContextHydrateCountAndTotalByteBudgets(t *testing.T) {
	for _, mode := range []string{"count", "bytes", "failed-bytes", "existing-max"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Millisecond)
			payload := []byte("small")
			count := groupContextMaxResources + 2
			if mode == "bytes" || mode == "failed-bytes" {
				payload = bytes.Repeat([]byte("x"), int(groupContextMediaBytes/2))
				count = 3
			}
			paths := make([]string, count)
			var entries []core.ContextMessage
			for i := range paths {
				id := fmt.Sprintf("message-%d", i)
				paths[i] = "/open-apis/im/v1/messages/" + id + "/resources/file"
				entries = append(entries, core.ContextMessage{ID: id, Time: now.Add(-time.Minute), Resources: []core.ContextResource{{MessageID: id, Key: "file", Kind: "file", Name: "file.txt"}}})
			}
			if mode == "failed-bytes" {
				entries[0].Resources[0].Kind = "image" // invalid binary still consumes network budget
			}
			p, calls := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
				_, _ = w.Write(payload)
			}, paths...)
			if mode == "existing-max" {
				p.resourceMaxBytes = 2
			}
			got, err := p.HydrateMessageContext(context.Background(), contextMediaTarget(now), entries)
			if err != nil || len(got) != count {
				t.Fatalf("hydrate: %v", err)
			}
			successes := 0
			var total int64
			for _, entry := range got {
				if len(entry.Files) > 0 {
					successes++
					total += int64(len(entry.Files[0].Data))
				} else if !entry.Incomplete || !strings.Contains(entry.Text, "unavailable") {
					t.Fatal("silently lost bounded resource")
				}
			}
			want := groupContextMaxResources
			switch mode {
			case "bytes":
				want = 2
			case "failed-bytes":
				want = 1
			case "existing-max":
				want = 0
			}
			if successes != want || total > groupContextMediaBytes || calls.Load() > groupContextMaxResources {
				t.Fatalf("budget successes=%d want=%d bytes=%d calls=%d", successes, want, total, calls.Load())
			}
		})
	}
}

func TestGroupContextForwardBoundedAndNoExtraScopeReads(t *testing.T) {
	for _, mode := range []string{"request-limit", "depth", "count", "failure", "cancelled", "wrong-root", "filtered"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Millisecond)
			var items []any
			var paths []string
			for i := 0; i < groupContextForwardRequests+2; i++ {
				id := fmt.Sprintf("root-%d", i)
				m := contextTypedMessage(id, "merge_forward", `{}`, now.Add(-time.Duration(i+1)*time.Minute))
				if mode == "filtered" {
					m["thread_id"] = "other-topic"
				}
				items = append(items, m)
				paths = append(paths, "/open-apis/im/v1/messages/"+id)
			}
			var gets atomic.Int64
			p, _ := contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/open-apis/im/v1/messages" {
					writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": items}})
					return
				}
				gets.Add(1)
				if mode == "failure" {
					writeJSON(t, w, map[string]any{"code": 999})
					return
				}
				if mode == "cancelled" {
					<-r.Context().Done()
					return
				}
				id := strings.TrimPrefix(r.URL.Path, "/open-apis/im/v1/messages/")
				root := contextTypedMessage(id, "merge_forward", `{}`, now.Add(-time.Minute))
				if mode == "wrong-root" {
					root["chat_id"] = "foreign"
				}
				children := []any{root}
				n, parent := 1, id
				switch mode {
				case "count":
					n = groupContextForwardMessages + 2
				case "depth":
					n = groupContextForwardDepth + 2
				}
				for i := 0; i < n; i++ {
					childID := fmt.Sprintf("child-%d", i)
					child := contextTypedMessage(childID, "text", `{"text":"source fact"}`, now.Add(-time.Hour))
					child["upper_message_id"] = parent
					if mode == "depth" {
						child["msg_type"], parent = "merge_forward", childID
					}
					children = append(children, child)
				}
				writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": children}})
			}, paths...)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, err := p.LoadMessageContext(ctx, contextMediaTarget(now), time.Time{})
			if err != nil || got.Incomplete || gets.Load() > groupContextForwardRequests {
				t.Fatalf("list/expansion state = %+v %v gets=%d", got, err, gets.Load())
			}
			if mode == "filtered" {
				if len(got.Messages) != 0 || gets.Load() != 0 {
					t.Fatal("other topic expanded")
				}
				return
			}
			if mode == "request-limit" {
				for _, entry := range got.Messages {
					wantIncomplete := entry.ID == "root-4" || entry.ID == "root-5"
					if entry.Incomplete != wantIncomplete {
						t.Fatalf("did not prioritize newest source: %+v", entry)
					}
				}
			}
			if len(got.Messages) != len(items) {
				t.Fatal("forward failure dropped entry")
			}
			incomplete := 0
			for _, entry := range got.Messages {
				if entry.Incomplete {
					incomplete++
					if !strings.Contains(entry.Text, "unavailable") {
						t.Fatal("missing expansion marker")
					}
				}
				if len([]rune(entry.Text)) > p.groupContext.maxChars || len(entry.Resources) > groupContextMaxResources {
					t.Fatal("unbounded forward output")
				}
				if mode == "wrong-root" && strings.Contains(entry.Text, "source fact") {
					t.Fatal("wrong root data leaked")
				}
			}
			if incomplete == 0 {
				t.Fatal("forward limits/failures not disclosed")
			}
		})
	}
}
