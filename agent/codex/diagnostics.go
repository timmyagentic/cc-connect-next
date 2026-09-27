package codex

import (
	"sync"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

// Only facts already available in memory are exposed. In particular, do not
// call GetModel/runtimeConfig: those may invoke another Codex process.
func (a *Agent) DiagnosticSnapshot() core.AgentDiagnosticSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return core.AgentDiagnosticSnapshot{Backend: a.backend, Model: a.model, Effort: a.reasoningEffort,
		ServiceTier: a.serviceTier, Mode: a.mode, SettingsSource: "configured"}
}

func (cs *codexSession) DiagnosticSnapshot() core.AgentDiagnosticSnapshot {
	cs.turnOptionsMu.RLock()
	defer cs.turnOptionsMu.RUnlock()
	value := core.AgentDiagnosticSnapshot{Backend: "exec", Model: cs.model, Effort: cs.effort,
		ServiceTier: cs.serviceTier, Mode: cs.mode, SettingsSource: "launch_options"}
	if cs.turnOptions != nil {
		value.Model, value.Effort, value.ServiceTier = cs.turnOptions.Model, cs.turnOptions.ReasoningEffort, cs.turnOptions.ServiceTier
	}
	return value
}

type appServerDiagnostics struct {
	mu                                  sync.Mutex
	lastProtocol                        time.Time
	readState                           string
	exitCode                            *int
	model, effort, tier                 string
	turnRequested                       bool
	highWater, dropped                  int
	terminalReceived, terminalDelivered bool
}

func (s *appServerSession) DiagnosticSnapshot() core.AgentDiagnosticSnapshot {
	s.runtimeMu.RLock()
	value := core.AgentDiagnosticSnapshot{Backend: "app_server", Model: s.model, Effort: s.effort,
		ServiceTier: s.serviceTier, Mode: s.mode, SettingsSource: "session_options"}
	s.runtimeMu.RUnlock()
	s.diagnostics.mu.Lock()
	d := &s.diagnostics
	value.LastProtocolEvent, value.ReadState = d.lastProtocol, d.readState
	if d.exitCode != nil {
		value.ExitCode = diagnosticPointer(*d.exitCode)
	}
	value.QueueDepth = diagnosticPointer(len(s.events))
	value.QueueHighWater, value.DroppedEvents = diagnosticPointer(d.highWater), diagnosticPointer(d.dropped)
	value.TerminalReceived, value.TerminalDelivered = diagnosticPointer(d.terminalReceived), diagnosticPointer(d.terminalDelivered)
	if d.turnRequested {
		value.Model, value.Effort, value.ServiceTier = d.model, d.effort, d.tier
		value.SettingsSource = "turn_start_request"
	}
	s.diagnostics.mu.Unlock()
	s.pendingMu.Lock()
	value.PendingRPC = diagnosticPointer(len(s.pending))
	s.pendingMu.Unlock()
	return value
}

func diagnosticPointer[T any](value T) *T { return &value }

func (s *appServerSession) beginTurnDiagnostic(params map[string]any) {
	s.runtimeMu.RLock()
	tier := s.serviceTier
	s.runtimeMu.RUnlock()
	s.diagnostics.mu.Lock()
	defer s.diagnostics.mu.Unlock()
	d := &s.diagnostics
	d.model, _ = params["model"].(string)
	d.effort, _ = params["effort"].(string)
	d.tier, _ = params["serviceTier"].(string)
	if _, present := params["serviceTier"]; !present {
		d.tier = tier
	}
	d.turnRequested = true
	d.highWater, d.dropped = len(s.events), 0
	d.terminalReceived, d.terminalDelivered = false, false
}

func (s *appServerSession) observeProtocolRead(state string, received bool) {
	s.diagnostics.mu.Lock()
	defer s.diagnostics.mu.Unlock()
	s.diagnostics.readState = state
	if received {
		s.diagnostics.lastProtocol = time.Now()
	}
}

func (s *appServerSession) observeTerminalProtocol() {
	s.diagnostics.mu.Lock()
	s.diagnostics.terminalReceived = true
	s.diagnostics.mu.Unlock()
}
