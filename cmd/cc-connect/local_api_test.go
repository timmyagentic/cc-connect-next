package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestReadRuntimeHealthDecodesReadyAndUnavailableResponses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket fixture")
	}
	for _, ready := range []bool{false, true} {
		t.Run(map[bool]string{false: "starting", true: "ready"}[ready], func(t *testing.T) {
			dir, err := os.MkdirTemp("/tmp", "ccn-health-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			socketPath := filepath.Join(dir, "api.sock")
			listener, err := net.Listen("unix", socketPath)
			if err != nil {
				t.Fatal(err)
			}
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if !ready {
					w.WriteHeader(http.StatusServiceUnavailable)
				}
				_ = json.NewEncoder(w).Encode(core.RuntimeHealth{Ready: ready})
			})}
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() { _ = server.Close() })

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			health, err := readRuntimeHealth(ctx, socketPath)
			if err != nil || health.Ready != ready {
				t.Fatalf("health = %+v err=%v", health, err)
			}
		})
	}
}

func TestSummarizeRuntimeHealth(t *testing.T) {
	health := core.RuntimeHealth{Projects: []core.RuntimeProjectHealth{{
		Name: "alpha",
		Platforms: []core.RuntimePlatformHealth{
			{Name: "feishu", State: core.RuntimePlatformReady},
			{Name: "telegram", State: core.RuntimePlatformUnavailable, Reason: "connection refused"},
		},
	}}}
	state, lines := summarizeRuntimeHealth(health)
	if state != "Unavailable" || len(lines) != 2 || !strings.Contains(lines[1], "alpha/telegram") {
		t.Fatalf("summary = %q / %v", state, lines)
	}
}

func TestRuntimeAPIReachableDistinguishesMissingAndListeningSockets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket fixture")
	}
	dir, err := os.MkdirTemp("/tmp", "ccn-reachable-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "api.sock")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if reachable, err := runtimeAPIReachable(ctx, socketPath); err != nil || reachable {
		t.Fatalf("missing socket reachable=%t err=%v", reachable, err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	if reachable, err := runtimeAPIReachable(ctx, socketPath); err != nil || !reachable {
		t.Fatalf("listening socket reachable=%t err=%v", reachable, err)
	}
}
