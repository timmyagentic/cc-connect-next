package codex

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

const reportedAuthFailure = "Your access token could not be refreshed because you have since logged out or signed in to another account. Please sign in again."

func TestCodexAuthenticationRequiredClassification(t *testing.T) {
	for _, test := range []struct {
		message string
		auth    bool
	}{
		{reportedAuthFailure, true},
		{"Your access token could not be refreshed because your refresh token was already used. Please log out and sign in again.", true},
		{"Your refresh token has expired. Please sign in again.", true},
		{`{"code":"refresh_token_invalidated"}`, true},
		{"Please run `codex login` to authenticate.", true},
		{"could not refresh token: network request timed out", false},
		{"HTTP 401 Unauthorized from upstream proxy", false},
		{"Incorrect API key provided", false},
		{"You've reached your usage limit", false},
		{"failed to read token file at /tmp/auth.json", false},
	} {
		t.Run(test.message, func(t *testing.T) {
			original := errors.New(test.message)
			got := classifyCodexError(original)
			if errors.Is(got, core.ErrAuthenticationRequired) != test.auth {
				t.Fatalf("classification = %v, want auth=%t", got, test.auth)
			}
			if test.auth && (!errors.Is(got, original) || !isCodexClassifiedTerminalError(got)) {
				t.Fatalf("auth error lost cause or terminal marker: %v", got)
			}
		})
	}
}

func TestCodexAuthenticationRequiredIsEmittedByBothBackends(t *testing.T) {
	for _, method := range []string{"error", "turn.failed"} {
		t.Run("exec/"+method, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &codexSession{ctx: ctx, events: make(chan core.Event, 1)}
			s.handleEvent(map[string]any{"type": method, "message": reportedAuthFailure, "error": map[string]any{"message": reportedAuthFailure}})
			select {
			case event := <-s.events:
				if !errors.Is(event.Error, core.ErrAuthenticationRequired) {
					t.Fatalf("auth error = %v", event.Error)
				}
			default:
				t.Fatal("authentication failure was discarded")
			}
		})
	}
	for _, method := range []string{"error", "turn/completed"} {
		t.Run("app_server/"+method, func(t *testing.T) {
			s := &appServerSession{events: make(chan core.Event, 1), currentTurn: "turn-1"}
			s.threadID.Store("thread-1")
			payload, err := json.Marshal(map[string]any{"threadId": "thread-1", "turnId": "turn-1", "message": reportedAuthFailure, "turn": map[string]any{"id": "turn-1", "error": map[string]any{"message": reportedAuthFailure}}})
			if err != nil {
				t.Fatal(err)
			}
			s.handleNotification(method, payload)
			select {
			case event := <-s.events:
				if !errors.Is(event.Error, core.ErrAuthenticationRequired) {
					t.Fatalf("auth error = %v", event.Error)
				}
			default:
				t.Fatal("authentication failure was discarded")
			}
		})
	}
}
