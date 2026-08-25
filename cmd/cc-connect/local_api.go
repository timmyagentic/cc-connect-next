package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/timmyagentic/cc-connect-next/core"
)

func newLocalAPIClient(socketPath string) *http.Client {
	dialer := &net.Dialer{}
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}}
}

func runtimeAPIReachable(ctx context.Context, socketPath string) (bool, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	if err == nil {
		_ = conn.Close()
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED) {
		return false, nil
	}
	return false, err
}

func readRuntimeHealth(ctx context.Context, socketPath string) (core.RuntimeHealth, error) {
	var health core.RuntimeHealth
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/healthz", nil)
	if err != nil {
		return health, err
	}
	client := newLocalAPIClient(socketPath)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return health, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return health, fmt.Errorf("runtime health returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&health); err != nil {
		return health, fmt.Errorf("decode runtime health: %w", err)
	}
	return health, nil
}

func summarizeRuntimeHealth(health core.RuntimeHealth) (string, []string) {
	state := "Starting"
	if health.Ready {
		state = "Ready"
	}
	var lines []string
	for _, project := range health.Projects {
		for _, platform := range project.Platforms {
			line := fmt.Sprintf("%s/%s: %s", project.Name, platform.Name, platform.State)
			if platform.Reason != "" {
				line += " (" + platform.Reason + ")"
			}
			if platform.State == core.RuntimePlatformUnavailable {
				state = "Unavailable"
			}
			lines = append(lines, line)
		}
	}
	sort.Strings(lines)
	return state, lines
}
