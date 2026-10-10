package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

const (
	defaultResourceChunkSize int64 = 8 * 1024 * 1024
	defaultResourceMaxBytes  int64 = 512 * 1024 * 1024
	resourceDownloadTimeout        = 2 * time.Minute
)

var errResourceRangeUnsupported = errors.New("resource server does not support Range")

type resourceChunk struct {
	apiJSON bool // JSON without attachment disposition; inspect assembled body too
	data    []byte
	total   int64 // zero means the server returned a complete 200 response
}

// downloadResourceBytes adapts upstream #1746. Unlike the SDK file reader,
// this path can request Range and validate both its coordinates and byte count.
// Defaults stay local: zero-value Platforms remain safe under concurrent calls.
func (p *Platform) downloadResourceBytes(ctx context.Context, messageID, fileKey, resType string) ([]byte, error) {
	if strings.TrimSpace(messageID) == "" || strings.TrimSpace(fileKey) == "" {
		return nil, fmt.Errorf("%s: resource download requires messageID and fileKey", p.tag())
	}
	ctx, cancel := context.WithTimeout(ctx, resourceDownloadTimeout)
	defer cancel()
	token, err := p.fetchFreshTenantAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	client := p.resourceDownloadHTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	maxBytes := p.resourceMaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultResourceMaxBytes
	}
	chunkSize := p.resourceChunkSize
	if chunkSize <= 0 || chunkSize > 64*1024*1024 {
		chunkSize = defaultResourceChunkSize
	}
	domain := strings.TrimRight(p.domain, "/")
	if domain == "" {
		domain = lark.FeishuBaseUrl
	}
	endpoint := domain + "/open-apis/im/v1/messages/" + url.PathEscape(messageID) + "/resources/" + url.PathEscape(fileKey) + "?" + url.Values{"type": {resType}}.Encode()
	first, err := fetchResourceChunk(ctx, client, endpoint, token, 0, 0, 0, maxBytes, true)
	if errors.Is(err, errResourceRangeUnsupported) {
		first, err = fetchResourceChunk(ctx, client, endpoint, token, 0, 0, 0, maxBytes, false)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: resource download: %w", p.tag(), err)
	}
	if first.total == 0 {
		return first.data, nil
	}
	buf := bytes.NewBuffer(first.data)
	for offset := int64(len(first.data)); offset < first.total; {
		end := offset + chunkSize - 1
		if end >= first.total {
			end = first.total - 1
		}
		chunk, err := fetchResourceChunk(ctx, client, endpoint, token, offset, end, first.total, maxBytes, true)
		if err != nil {
			return nil, fmt.Errorf("%s: resource chunk at %d: %w", p.tag(), offset, err)
		}
		buf.Write(chunk.data)
		offset += int64(len(chunk.data))
	}
	if first.apiJSON {
		if err := resourceEnvelopeError(buf.Bytes()); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func fetchResourceChunk(ctx context.Context, client *http.Client, endpoint, token string, start, end, expectedTotal, maxBytes int64, ranged bool) (resourceChunk, error) {
	delay := transientRetryInitial
	for attempt := 0; ; attempt++ {
		chunk, retry, err := requestResourceChunk(ctx, client, endpoint, token, start, end, expectedTotal, maxBytes, ranged)
		if err == nil || !retry || attempt >= maxTransientRetries {
			return chunk, err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return resourceChunk{}, ctx.Err()
		case <-timer.C:
		}
		delay *= 2
		if delay > transientRetryMaxDelay {
			delay = transientRetryMaxDelay
		}
	}
}

func requestResourceChunk(ctx context.Context, client *http.Client, endpoint, token string, start, end, expectedTotal, maxBytes int64, ranged bool) (resourceChunk, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return resourceChunk{}, false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept-Encoding", "identity")
	if ranged {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	}
	resp, err := client.Do(req)
	if err != nil {
		return resourceChunk{}, ctx.Err() == nil && isTransientError(err), err
	}
	defer func() { _ = resp.Body.Close() }()
	if ranged && expectedTotal == 0 && (resp.StatusCode == 400 || resp.StatusCode == 405 || resp.StatusCode == 416 || resp.StatusCode == 501) {
		return resourceChunk{}, false, errResourceRangeUnsupported
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return resourceChunk{}, true, fmt.Errorf("resource API status=%d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return resourceChunk{}, false, fmt.Errorf("resource API status=%d", resp.StatusCode)
	}
	limit := maxBytes
	total := int64(0)
	if resp.StatusCode == http.StatusPartialContent {
		var gotStart, gotEnd int64
		header := strings.TrimSpace(resp.Header.Get("Content-Range"))
		// Only the initial one-byte probe may negotiate a plain GET. Never
		// accept this nonconforming body, or retry a broken later chunk.
		if ranged && start == 0 && end == 0 && expectedTotal == 0 && header == "" {
			if resp.ContentLength > maxBytes {
				return resourceChunk{}, false, errors.New("resource Content-Length exceeds download limit")
			}
			return resourceChunk{}, false, errResourceRangeUnsupported
		}
		if !ranged {
			return resourceChunk{}, false, errors.New("resource plain GET returned partial content")
		}
		_, err := fmt.Sscanf(header, "bytes %d-%d/%d", &gotStart, &gotEnd, &total)
		if err != nil || header != fmt.Sprintf("bytes %d-%d/%d", gotStart, gotEnd, total) || total <= 0 || gotStart != start || gotEnd < start || gotEnd >= total || gotEnd > end || expectedTotal > 0 && total != expectedTotal {
			return resourceChunk{}, false, errors.New("resource Content-Range does not match requested range")
		}
		if total > maxBytes {
			return resourceChunk{}, false, fmt.Errorf("resource size %d exceeds limit %d", total, maxBytes)
		}
		// A shorter final chunk is valid only at the end of the resource.
		if gotEnd != end && gotEnd != total-1 {
			return resourceChunk{}, false, errors.New("resource Content-Range is incomplete")
		}
		limit = gotEnd - gotStart + 1
	} else if expectedTotal > 0 {
		return resourceChunk{}, false, errors.New("resource server stopped honoring Range")
	}
	if resp.ContentLength > limit {
		return resourceChunk{}, false, errors.New("resource Content-Length exceeds download limit")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return resourceChunk{}, ctx.Err() == nil && isTransientError(err), err
	}
	if int64(len(data)) > limit || total > 0 && int64(len(data)) != limit {
		return resourceChunk{}, false, errors.New("resource body length does not match download limit/range")
	}
	// Feishu can return an API error envelope with HTTP 200. Do not forward
	// that envelope as if it were the user's attachment.
	apiJSON := resp.Header.Get("Content-Disposition") == "" && strings.EqualFold(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]), "application/json")
	if apiJSON {
		if err := resourceEnvelopeError(data); err != nil {
			return resourceChunk{}, false, err
		}
	}

	return resourceChunk{data: data, total: total, apiJSON: apiJSON}, false, nil
}

func resourceEnvelopeError(data []byte) error {
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(data, &envelope) == nil && envelope.Code != 0 && envelope.Msg != "" {
		return fmt.Errorf("resource API code=%d", envelope.Code)
	}
	return nil
}
