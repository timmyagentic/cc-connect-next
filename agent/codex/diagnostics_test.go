package codex

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestFeedbackDiagnosticsDistinguishLostTerminalFromMissingTerminal(t *testing.T) {
	s := &appServerSession{events: make(chan core.Event, 1)}
	s.threadID.Store("thread-1")
	s.currentTurn = "turn-1"
	s.beginTurnDiagnostic(map[string]any{"model": "model-for-this-turn", "effort": "high", "serviceTier": "fast"})
	s.events <- core.Event{Type: core.EventThinking}
	s.handleNotification("turn/completed", json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed"}}`))
	d := s.DiagnosticSnapshot()
	if d.TerminalReceived == nil || !*d.TerminalReceived || d.TerminalDelivered == nil || *d.TerminalDelivered || *d.DroppedEvents < 1 {
		t.Fatalf("lost terminal is indistinguishable from missing protocol: %#v", d)
	}
	if d.Model != "model-for-this-turn" || d.Effort != "high" || d.ServiceTier != "fast" || d.SettingsSource != "turn_start_request" {
		t.Fatalf("wrong turn settings: %#v", d)
	}
	*d.DroppedEvents = 999
	if *s.DiagnosticSnapshot().DroppedEvents == 999 {
		t.Fatal("snapshot points into mutable counters")
	}
	<-s.events
	s.beginTurnDiagnostic(map[string]any{"model": nil, "effort": nil, "serviceTier": nil})
	d = s.DiagnosticSnapshot()
	if *d.TerminalReceived || *d.TerminalDelivered || *d.DroppedEvents != 0 || d.Model != "" {
		t.Fatalf("new turn inherited old facts: %#v", d)
	}
}

func TestFeedbackDiagnosticsCaptureEOFWithoutChangingEventBehavior(t *testing.T) {
	s := &appServerSession{ctx: context.Background(), events: make(chan core.Event, 8)}
	s.alive.Store(true)
	s.wg.Add(1)
	s.readLoop(strings.NewReader("{\"method\":\"unrecognized\",\"params\":{}}\n"))
	d := s.DiagnosticSnapshot()
	if d.ReadState != "eof" || d.LastProtocolEvent.IsZero() || s.Alive() {
		t.Fatalf("EOF facts missing: %#v", d)
	}
	if *d.TerminalReceived || *d.TerminalDelivered || len(s.events) != 0 {
		t.Fatal("instrumentation fabricated a terminal event")
	}
}

func TestFeedbackDiagnosticsDoNotProbeCLI(t *testing.T) {
	s := &codexSession{cmd: "a-command-that-must-not-run", model: "configured", effort: "low"}
	s.storeActiveTurnOptions(&core.TurnOptions{Model: "turn-model", ReasoningEffort: "high"})
	d := s.DiagnosticSnapshot()
	if d.Model != "turn-model" || d.Effort != "high" || !s.runtimeCfgFetched.IsZero() {
		t.Fatalf("diagnostic read changed/probed runtime: %#v", d)
	}
}
