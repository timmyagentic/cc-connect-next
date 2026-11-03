package feishu

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestHistoryMediaExpiredDeadlineMarksSelectedFileUnread(t *testing.T) {
	p, calls := contextAPIPlatform(t, func(http.ResponseWriter, *http.Request) {
		t.Error("started a resource request after the deadline")
	})
	now := time.Now()
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(-time.Second))
	defer cancel()
	entry := core.ContextMessage{ID: "post", Time: now.Add(-time.Second),
		Resources: []core.ContextResource{{MessageID: "post", Key: "key", Kind: "file"}}}
	out, err := p.HydrateMessageContext(ctx, contextMediaTarget(now), []core.ContextMessage{entry})
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 0 || len(out) != 1 || !out[0].Incomplete {
		t.Fatalf("unread attachment not reported: calls=%d entries=%d err=%v", calls.Load(), len(out), err)
	}
}

func TestHistoryMediaRecallRedactsTextAndAlreadyFetchedAttachments(t *testing.T) {
	var p *Platform
	p, _ = contextAPIPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/keep/") {
			p.markMessageRecalled("gone")
		}
		_, _ = io.WriteString(w, "file bytes")
	}, "/open-apis/im/v1/messages/gone/resources/key", "/open-apis/im/v1/messages/keep/resources/key")
	now := time.Now()
	var input []core.ContextMessage
	for _, id := range []string{"gone", "keep"} {
		input = append(input, core.ContextMessage{ID: id, Text: "source text", Time: now.Add(-time.Second),
			Resources: []core.ContextResource{{MessageID: id, Key: "key", Kind: "file", Name: "attachment.txt"}}})
	}
	out, err := p.HydrateMessageContext(context.Background(), contextMediaTarget(now), input)
	if err != nil || len(out) != 2 {
		t.Fatalf("hydrate: %d entries, err=%v", len(out), err)
	}
	gone, keep := out[0], out[1]
	if gone.ID != "gone" || !gone.Incomplete || strings.Contains(gone.Text, "source text") || !strings.Contains(gone.Text, "recalled") ||
		len(gone.Resources)+len(gone.Files)+len(gone.Images) != 0 {
		t.Fatal("recalled source was disclosed or its omission was not marked")
	}
	if keep.ID != "keep" || keep.Incomplete || len(keep.Files) != 1 || string(keep.Files[0].Data) != "file bytes" {
		t.Fatal("lost available attachment from another source")
	}
}
