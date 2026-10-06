package core

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestProcessInteractiveEvents_OmitsRemovedTokenTelemetry(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	p := &stubPlatformEngine{n: "test"}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	t.Cleanup(e.cancel)
	sessionKey := "test:user1"
	session := e.sessions.GetOrCreateActive(sessionKey)
	agentSession := newControllableSession("s1")
	state := &interactiveState{agentSession: agentSession, platform: p, replyCtx: "ctx-1"}
	e.interactiveStates[sessionKey] = state

	agentSession.events <- Event{Type: EventResult, Done: false, Metadata: map[string]any{"compaction_continue": true}}
	agentSession.events <- Event{Type: EventResult, Content: "done", Done: true}
	e.processInteractiveEvents(state, session, e.sessions, sessionKey, "m1", time.Now(), nil, nil, nil)

	if got := p.getSent(); len(got) != 1 || got[0] != "done" {
		t.Fatalf("sent = %#v, want the completed reply after compaction", got)
	}

	var sawCompaction, sawCompletion bool
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode log record: %v", err)
		}
		for _, field := range []string{"input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"} {
			if _, ok := record[field]; ok {
				t.Errorf("log %q still contains removed token telemetry %q", record["msg"], field)
			}
		}
		switch record["msg"] {
		case "EventResult: non-terminal result event, continuing event loop":
			sawCompaction = true
		case "turn complete":
			sawCompletion = true
			if record["response_len"] != float64(len("done")) || record["session"] != session.ID {
				t.Errorf("completion log lost response diagnostics: %#v", record)
			}
		}
	}
	if !sawCompaction || !sawCompletion {
		t.Fatalf("compaction log = %t, completion log = %t; want both", sawCompaction, sawCompletion)
	}
}
