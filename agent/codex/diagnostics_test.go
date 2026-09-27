package codex

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestFeedbackDiagnosticsDistinguishPendingTerminalFromMissingTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &appServerSession{ctx: ctx, events: make(chan core.Event, 1)}
	s.threadID.Store("thread-1")
	s.currentTurn = "turn-1"
	s.beginTurnDiagnostic(map[string]any{"model": "model-for-this-turn", "effort": "high", "serviceTier": "fast"})
	s.events <- core.Event{Type: core.EventThinking}
	// Observe the protocol before delivery so the blocked and delivered
	// snapshots are deterministic without holding a diagnostic lock.
	s.observeTerminalProtocol()
	d := s.DiagnosticSnapshot()
	if d.TerminalReceived == nil || !*d.TerminalReceived || d.TerminalDelivered == nil || *d.TerminalDelivered || *d.DroppedEvents != 0 {
		t.Fatalf("pending terminal is indistinguishable from missing protocol: %#v", d)
	}
	if d.Model != "model-for-this-turn" || d.Effort != "high" || d.ServiceTier != "fast" || d.SettingsSource != "turn_start_request" {
		t.Fatalf("wrong turn settings: %#v", d)
	}
	*d.DroppedEvents = 999
	if *s.DiagnosticSnapshot().DroppedEvents == 999 {
		t.Fatal("snapshot points into mutable counters")
	}
	done := make(chan struct{})
	go func() {
		s.handleNotification("turn/completed", json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed"}}`))
		close(done)
	}()
	<-s.events
	awaitTerminalTest(t, done)
	if event := <-s.events; event.Type != core.EventResult || !event.Done {
		t.Fatalf("terminal event = %#v", event)
	}
	d = s.DiagnosticSnapshot()
	if !*d.TerminalReceived || !*d.TerminalDelivered || *d.DroppedEvents != 0 {
		t.Fatalf("terminal delivery facts missing: %#v", d)
	}
	s.beginTurnDiagnostic(map[string]any{"model": nil, "effort": nil, "serviceTier": nil})
	d = s.DiagnosticSnapshot()
	if *d.TerminalReceived || *d.TerminalDelivered || *d.DroppedEvents != 0 || d.Model != "" {
		t.Fatalf("new turn inherited old facts: %#v", d)
	}
}

func TestFeedbackDiagnosticsCaptureIdleEOFWithoutTerminalEvent(t *testing.T) {
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

func TestFeedbackFailedLaunchPreservesActiveTurnOptions(t *testing.T) {
	dir := t.TempDir()
	s, err := newCodexSession(context.Background(), codexSessionParams{
		cliBin: filepath.Join(dir, "missing-codex"), workDir: dir,
		model: "configured", effort: "low", serviceTier: "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.storeActiveTurnOptions(&core.TurnOptions{Model: "previous-model", ReasoningEffort: "medium", ServiceTier: "default"})
	err = s.SendWithTurnOptions("launch must fail", nil, nil, core.TurnOptions{Model: "attempted-model", ReasoningEffort: "high", ServiceTier: "fast"})
	if err == nil {
		t.Fatal("missing executable unexpectedly started")
	}
	if s.GetModel() != "previous-model" || s.GetReasoningEffort() != "medium" || s.turnOptions.ServiceTier != "default" {
		t.Errorf("failed launch replaced active options: %#v", s.turnOptions)
	}
	d := s.DiagnosticSnapshot()
	if d.Model != "attempted-model" || d.Effort != "high" || d.ServiceTier != "fast" || d.SettingsSource != "launch_attempt" {
		t.Errorf("failed launch lost the attempted settings: %#v", d)
	}
	if !s.runtimeCfgFetched.IsZero() {
		t.Fatal("failed-launch diagnostics probed the CLI")
	}
}
