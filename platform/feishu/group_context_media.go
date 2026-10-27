package feishu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

const (
	groupContextMediaBytes   int64 = 20 * 1024 * 1024
	groupContextMediaTimeout       = 12 * time.Second
)

var _ core.MessageContextMediaProvider = (*Platform)(nil)

// HydrateMessageContext receives only unseen, text-budget-selected entries.
// Keep identity/order and partial successes intact, even on cancellation.
func (p *Platform) HydrateMessageContext(ctx context.Context, target any, messages []core.ContextMessage) ([]core.ContextMessage, error) {
	result := append([]core.ContextMessage(nil), messages...)
	if !p.MessageContextEnabled(target) {
		for i := range result {
			result[i].Images, result[i].Files = nil, nil
			if len(result[i].Resources) > 0 {
				contextContentUnavailable(&result[i], "attachments", "target not authorized")
			}
		}
		return result, fmt.Errorf("%s: history media target not authorized", p.tag())
	}
	ctx, cancel := context.WithTimeout(ctx, groupContextMediaTimeout)
	defer cancel()
	rc := target.(replyContext)
	cutoff := time.UnixMilli(rc.contextTimeMs)
	start := cutoff.Add(-time.Duration(p.groupContext.minutes) * time.Minute)

	// Reuse the validated range downloader with an isolated configuration:
	// never copy Platform's locks or mutate its live download limits.
	httpClient := &http.Client{Timeout: groupContextMediaTimeout}
	if p.resourceDownloadHTTP != nil {
		copyClient := *p.resourceDownloadHTTP
		httpClient = &copyClient
	}
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	budget := &groupContextByteBudget{remaining: groupContextMediaBytes, transport: transport}
	httpClient.Transport = budget
	downloader := &Platform{
		platformName: p.platformName, appID: p.appID, appSecret: p.appSecret, domain: p.domain,
		resourceDownloadHTTP: httpClient, resourceChunkSize: p.resourceChunkSize,
		replayClient: p.replayAPIClient(),
	}
	attempts := 0
	for i := range result {
		entry := &result[i]
		entry.Images, entry.Files = nil, nil
		imageNumber := 0
		for _, resource := range entry.Resources {
			label := resource.Kind
			if resource.Kind == "image" {
				imageNumber++
				label = fmt.Sprintf("image #%d", imageNumber)
			} else if resource.Name != "" {
				label = "file " + resource.Name
			}
			reason := ""
			switch {
			case rc.contextTimeMs <= 0 || entry.Time.Before(start) || entry.Time.After(cutoff) || p.isMessageRecalled(entry.ID):
				reason = "message outside authorized snapshot"
			case entry.ID == "" || resource.MessageID != entry.ID || strings.TrimSpace(resource.Key) == "":
				reason = "invalid resource source"
			case resource.Kind != "image" && resource.Kind != "file":
				reason = "unsupported resource kind"
			case ctx.Err() != nil:
				reason = "download cancelled or timed out"
			case attempts >= groupContextMaxResources || budget.remaining <= 0:
				reason = "resource budget exhausted"
			}
			if reason != "" {
				contextMediaUnavailable(entry, label, reason)
				continue
			}
			attempts++
			downloader.resourceMaxBytes = budget.remaining
			if p.resourceMaxBytes > 0 && p.resourceMaxBytes < downloader.resourceMaxBytes {
				downloader.resourceMaxBytes = p.resourceMaxBytes
			}
			data, err := downloader.downloadResourceBytes(ctx, resource.MessageID, resource.Key, resource.Kind)
			if err != nil || len(data) == 0 {
				contextMediaUnavailable(entry, label, "download failed or size limit")
				continue
			}
			if resource.Kind == "image" {
				mimeType := detectMimeType(data)
				if !strings.HasPrefix(mimeType, "image/") {
					contextMediaUnavailable(entry, label, "resource is not an image")
					continue
				}
				name := fmt.Sprintf("history-image-%d", imageNumber)
				entry.Images = append(entry.Images, core.ImageAttachment{Data: data, MimeType: mimeType, FileName: name})
				// Successful attachment order can differ from source order
				// when an earlier image fails. Keep that mapping in-place.
				entry.Text = strings.ReplaceAll(entry.Text, "["+label+"]", fmt.Sprintf("[%s: attached image %d (%s)]", label, len(entry.Images), name))
				entry.Text = strings.ReplaceAll(entry.Text, "["+label+": "+resource.Name+"]", fmt.Sprintf("[%s: attached image %d (%s), %s]", label, len(entry.Images), name, resource.Name))
			} else {
				name := resource.Name
				if name == "" {
					name = "attachment"
				}
				entry.Files = append(entry.Files, core.FileAttachment{Data: data, MimeType: http.DetectContentType(data), FileName: name, MessageID: resource.MessageID})
			}
		}
		boundGroupContextText(entry, p.groupContext.maxChars)
	}
	return result, ctx.Err()
}

func contextMediaUnavailable(entry *core.ContextMessage, label, reason string) {
	marker := core.ContextMessage{}
	contextContentUnavailable(&marker, label, reason)
	// Keep failures visible even when the source text already fills its cap.
	entry.Text = marker.Text + "\n" + entry.Text
	entry.Incomplete = true
}

// Count actual response bytes, including failed downloads/retries, across the
// whole hydration. This prevents several malformed images from each consuming
// the entire history budget. Hydration is deliberately sequential.
type groupContextByteBudget struct {
	remaining int64
	transport http.RoundTripper
}

func (b *groupContextByteBudget) RoundTrip(req *http.Request) (*http.Response, error) {
	if b.remaining <= 0 {
		return nil, fmt.Errorf("history resource byte budget exhausted")
	}
	response, err := b.transport.RoundTrip(req)
	if err == nil && response != nil {
		response.Body = &groupContextBudgetBody{ReadCloser: response.Body, budget: b, remaining: response.ContentLength}
	}
	return response, err
}

type groupContextBudgetBody struct {
	io.ReadCloser
	budget    *groupContextByteBudget
	remaining int64
}

func (b *groupContextBudgetBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}
	if b.budget.remaining <= 0 {
		// Known-length and range responses do not need an extra read. For
		// unknown-length bodies, refusing an EOF probe is conservative.
		return 0, io.ErrUnexpectedEOF
	}
	if int64(len(p)) > b.budget.remaining {
		p = p[:b.budget.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.budget.remaining -= int64(n)
	if b.remaining > 0 {
		b.remaining -= int64(n)
	}
	return n, err
}
