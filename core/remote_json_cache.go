package core

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	remotePresetsCacheTTL        = 6 * time.Hour
	remotePresetsHTTPTimeout     = 15 * time.Second
	remotePresetsFallbackTimeout = 10 * time.Second
	remoteJSONMaxBytes           = 1 << 20
)

type remoteJSONCacheConfig struct {
	name            string
	defaultURL      string
	fallbackURL     string
	ttl             time.Duration
	timeout         time.Duration
	fallbackTimeout time.Duration
}

type remoteJSONCache[T any] struct {
	mu        sync.RWMutex
	data      *T
	fetchedAt time.Time
	url       string
	config    remoteJSONCacheConfig
}

func newRemoteJSONCache[T any](config remoteJSONCacheConfig) *remoteJSONCache[T] {
	return &remoteJSONCache[T]{config: config}
}

func (c *remoteJSONCache[T]) setURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.url = url
	c.data = nil
	c.fetchedAt = time.Time{}
}

func (c *remoteJSONCache[T]) fetch() (*T, error) {
	c.mu.RLock()
	if c.data != nil && time.Since(c.fetchedAt) < c.config.ttl {
		defer c.mu.RUnlock()
		return c.data, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data != nil && time.Since(c.fetchedAt) < c.config.ttl {
		return c.data, nil
	}

	primaryURL := c.url
	if primaryURL == "" {
		primaryURL = c.config.defaultURL
	}
	result, err := fetchRemoteJSON[T](primaryURL, c.config.timeout, c.config.name)
	if err != nil {
		slog.Warn("primary "+c.config.name+" fetch failed, trying fallback", "url", primaryURL, "error", err)
		result, err = fetchRemoteJSON[T](c.config.fallbackURL, c.config.fallbackTimeout, c.config.name)
	}
	if err != nil {
		if c.data != nil {
			slog.Warn("all "+c.config.name+" sources failed, using stale cache", "error", err)
			return c.data, nil
		}
		return nil, fmt.Errorf("fetch %s: %w", c.config.name, err)
	}

	c.data = result
	c.fetchedAt = time.Now()
	return c.data, nil
}

func fetchRemoteJSON[T any](url string, timeout time.Duration, name string) (*T, error) {
	slog.Debug("fetching "+name, "url", url)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, remoteJSONMaxBytes))
	if err != nil {
		return nil, fmt.Errorf("read body from %s: %w", url, err)
	}
	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse JSON from %s: %w", url, err)
	}
	return &result, nil
}
