package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRemoteJSONCacheFallbackCachingAndURLReset(t *testing.T) {
	type response struct {
		Version int `json:"version"`
	}
	var fallbackCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/primary":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		case "/fallback":
			fallbackCalls.Add(1)
			_, _ = fmt.Fprint(w, `{"version":1}`)
		case "/override":
			_, _ = fmt.Fprint(w, `{"version":2}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cache := newRemoteJSONCache[response](remoteJSONCacheConfig{
		name:            "test presets",
		defaultURL:      server.URL + "/primary",
		fallbackURL:     server.URL + "/fallback",
		ttl:             time.Hour,
		timeout:         time.Second,
		fallbackTimeout: time.Second,
	})
	for i := 0; i < 2; i++ {
		got, err := cache.fetch()
		if err != nil || got.Version != 1 {
			t.Fatalf("fetch %d = %#v, %v", i+1, got, err)
		}
	}
	if got := fallbackCalls.Load(); got != 1 {
		t.Fatalf("fallback calls = %d, want one cached fetch", got)
	}

	cache.setURL(server.URL + "/override")
	got, err := cache.fetch()
	if err != nil || got.Version != 2 {
		t.Fatalf("fetch after URL reset = %#v, %v", got, err)
	}
}
