package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

// AgentDiagnosticProvider is optional, side-effect-free and bounded. Adapters
// report facts already observed; collecting feedback must never start a CLI,
// probe a network endpoint, or read credential-bearing configuration.
type AgentDiagnosticProvider interface {
	DiagnosticSnapshot() AgentDiagnosticSnapshot
}

type AgentDiagnosticSnapshot struct {
	Backend, Version, Model, Effort, ServiceTier, Mode, SettingsSource string
	LastProtocolEvent                                                  time.Time
	ReadState                                                          string
	ExitCode                                                           *int
	PendingRPC, QueueDepth, QueueHighWater, DroppedEvents              *int
	TerminalReceived, TerminalDelivered                                *bool
}

const feedbackRetain = 72 * time.Hour
const feedbackTurnLimit = 20

// feedbackTurn owns only this participant's user-authored input. It never
// consults conversation history, which has no trustworthy sender provenance.
type feedbackTurn struct {
	mu         sync.Mutex
	key        string
	scope      string
	at         time.Time
	active     bool
	shared     bool
	version    string
	agent      string
	diagnostic appfeatures.FeedbackDiagnostic
}

func (e *Engine) feedbackBinding(sessionKey, userID string) string {
	value := sha256.Sum256([]byte(e.name + "\x00" + sessionKey + "\x00" + userID))
	return hex.EncodeToString(value[:])
}

func (e *Engine) feedbackContextKey(sessionKey, userID string) string {
	interactiveKey := e.interactiveKeyForSessionKey(sessionKey)
	sessions := e.sessions
	if interactiveKey != sessionKey {
		workspace := strings.TrimSuffix(interactiveKey, ":"+sessionKey)
		if e.projectState != nil {
			if override := e.projectState.WorkspaceDirOverride(interactiveKey); override != "" {
				workspace = override
			}
		}
		e.interactiveMu.Lock()
		pool := e.workspacePool
		e.interactiveMu.Unlock()
		sessions = nil
		if pool != nil {
			if ws := pool.Get(workspace); ws != nil {
				ws.mu.Lock()
				sessions = ws.sessions
				ws.mu.Unlock()
			}
		}
		if sessions == nil {
			// A manual report after restart may precede workspace activation.
			// Read existing local metadata; never construct/probe an Agent.
			sessions = NewSessionManager(e.workspaceSessionFile(workspace))
		}
	}
	conversation := ""
	if sessions != nil {
		conversation = sessions.ActiveSessionID(sessionKey)
	}
	return e.feedbackBinding(e.feedbackScope(interactiveKey, conversation), userID)
}

func (e *Engine) feedbackScope(interactiveKey, conversation string) string {
	if e.projectState != nil {
		interactiveKey += "\x00" + e.projectState.WorkspaceDirOverride(interactiveKey)
	}
	return e.feedbackBinding(interactiveKey+"\x00"+conversation, "")
}

func feedbackUserContent(msg *Message) string {
	if msg.feedbackTextSet {
		return msg.feedbackText
	}
	// Direct internal callers have not passed through normalization. Never
	// infer the original text by inspecting or copying enriched history.
	return msg.Content
}

func (e *Engine) beginFeedbackTurn(p Platform, msg *Message, session *Session, agent Agent) *feedbackTurn {
	if !e.feedbackActive() || strings.TrimSpace(msg.UserID) == "" {
		return nil
	}
	now := time.Now()
	scope := e.feedbackScope(e.interactiveKeyForSessionKey(msg.SessionKey), session.ID)
	key := e.feedbackBinding(scope, msg.UserID)
	diagnostic := appfeatures.FeedbackDiagnostic{
		StartedAt: now, OccurredAt: now, Phase: "starting", ErrorCode: "none", Request: feedbackUserContent(msg),
		Runtime: appfeatures.FeedbackRuntime{Platform: p.Name(), IdleTimeoutMS: e.eventIdleTimeout.Milliseconds(), MaxTurnTimeMS: e.maxTurnTime.Milliseconds()},
	}
	applyAgentDiagnostic(&diagnostic, agent)
	diagnostic = appfeatures.CaptureFeedbackDiagnostic(diagnostic)
	turn := &feedbackTurn{key: key, scope: scope, at: now, active: true, version: CurrentVersion, agent: agent.Name(), diagnostic: diagnostic}
	e.feedbackMu.Lock()
	if e.feedbackTurns == nil {
		e.feedbackTurns = make(map[string]*feedbackTurn)
	}
	e.feedbackTurns[key] = turn
	e.pruneFeedbackTurnsLocked(now)
	e.scheduleFeedbackSaveLocked()
	e.feedbackMu.Unlock()
	return turn
}

func applyAgentDiagnostic(d *appfeatures.FeedbackDiagnostic, value any) {
	provider, ok := value.(AgentDiagnosticProvider)
	if !ok {
		return
	}
	facts := provider.DiagnosticSnapshot()
	if facts.SettingsSource != "" {
		// Empty requested values mean the backend default is unknown. Do not
		// accidentally retain a configured value overridden by this turn.
		d.Runtime.Model, d.Runtime.Effort, d.Runtime.ServiceTier = facts.Model, facts.Effort, facts.ServiceTier
	}
	for _, field := range []struct {
		target *string
		value  string
	}{
		{&d.Runtime.Backend, facts.Backend}, {&d.Runtime.AgentVersion, facts.Version},
		{&d.Runtime.Model, facts.Model}, {&d.Runtime.Effort, facts.Effort},
		{&d.Runtime.ServiceTier, facts.ServiceTier}, {&d.Runtime.Mode, facts.Mode},
		{&d.Runtime.SettingsSource, facts.SettingsSource},
	} {
		if field.value != "" {
			*field.target = field.value
		}
	}
	if !facts.LastProtocolEvent.IsZero() && !facts.LastProtocolEvent.Before(d.StartedAt) {
		elapsed := facts.LastProtocolEvent.Sub(d.StartedAt).Milliseconds()
		d.Transport.LastProtocolMS = &elapsed
	}
	d.Transport.ReadState = facts.ReadState
	d.Transport.ExitCode = facts.ExitCode
	d.Transport.PendingRPC = facts.PendingRPC
	d.Transport.QueueDepth = facts.QueueDepth
	d.Transport.QueueHighWater = facts.QueueHighWater
	d.Transport.DroppedEvents = facts.DroppedEvents
	d.Transport.TerminalReceived = facts.TerminalReceived
	d.Transport.TerminalDelivered = facts.TerminalDelivered
}

func (state *interactiveState) feedbackTurnSnapshot() (*feedbackTurn, AgentSession) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.feedbackTurn, state.agentSession
}

func (e *Engine) observeFeedbackEvent(state *interactiveState, event Event) {
	turn, _ := state.feedbackTurnSnapshot()
	if turn == nil {
		return
	}
	turn.mu.Lock()
	defer turn.mu.Unlock()
	if !turn.active {
		return
	}
	elapsed := max(0, time.Since(turn.diagnostic.StartedAt).Milliseconds())
	turn.diagnostic.Transport.LastCoreMS = &elapsed
	kind := string(event.Type)
	activity := turn.diagnostic.Activity
	// High-frequency text/thinking deltas only advance the timestamp. The
	// fixed ring keeps transitions and tool names, never event contents.
	if len(activity) > 0 && activity[len(activity)-1].Kind == kind && event.ToolName == "" {
		return
	}
	item := appfeatures.FeedbackActivity{AfterMS: elapsed, Kind: kind, Name: truncateFeedbackEventName(event.ToolName)}
	if len(activity) == 24 {
		copy(activity, activity[1:])
		activity = activity[:23]
		turn.diagnostic.Truncated = []string{"activity"}
	}
	turn.diagnostic.Activity = append(activity, item)
}

func (e *Engine) observeFeedbackDelivery(state *interactiveState) {
	turn, _ := state.feedbackTurnSnapshot()
	if turn == nil {
		return
	}
	turn.mu.Lock()
	defer turn.mu.Unlock()
	if turn.active {
		elapsed := max(0, time.Since(turn.diagnostic.StartedAt).Milliseconds())
		turn.diagnostic.Transport.LastDeliveryMS = &elapsed
	}
}

func truncateFeedbackEventName(value string) string {
	// Redact the complete name before bounding it; tool args/results never
	// enter this function. Capture applies the final UTF-8 field boundary.
	value = redactFeedbackText(value)
	if len(value) > 96 {
		value = string([]rune(value)[:min(len([]rune(value)), 24)])
	}
	return value
}

func feedbackErrorCode(err error, phase string) string {
	switch {
	case errors.Is(err, ErrAuthenticationRequired):
		return "authentication_required"
	case errors.Is(err, ErrUsageLimit):
		return "usage_limit"
	default:
		return phase
	}
}

func (e *Engine) recordFeedbackFailure(state *interactiveState, phase string, cause error) {
	if state == nil || cause == nil || !e.feedbackActive() {
		return
	}
	turn, as := state.feedbackTurnSnapshot()
	if turn == nil {
		key, user := state.feedbackIdentity()
		e.recordFeedbackError(key, user, cause.Error())
		return
	}
	turn.mu.Lock()
	if !turn.active {
		turn.mu.Unlock()
		if phase == "background_error" || phase == "compression" {
			key, user := state.feedbackIdentity()
			e.recordFeedbackError(key, user, cause.Error())
		}
		return
	}
	turn.active = false
	turn.at = time.Now()
	turn.diagnostic.OccurredAt = turn.at
	turn.diagnostic.Phase = phase
	turn.diagnostic.ErrorCode = feedbackErrorCode(cause, phase)
	turn.diagnostic.Error = cause.Error()
	turn.diagnostic.Missing = nil // recompute after the final adapter snapshot
	if turn.shared {
		turn.diagnostic.Missing = []string{"earlier participant context: excluded"}
	}
	if as != nil {
		applyAgentDiagnostic(&turn.diagnostic, as)
		turn.diagnostic.Transport.ProcessState = "exited"
		if as.Alive() {
			turn.diagnostic.Transport.ProcessState = "alive"
		}
	} else {
		turn.diagnostic.Transport.ProcessState = "not_started"
	}
	turn.diagnostic = appfeatures.CaptureFeedbackDiagnostic(turn.diagnostic)
	turn.mu.Unlock()
	e.feedbackMu.Lock()
	e.retainFeedbackTurnLocked(turn)
	e.scheduleFeedbackSaveLocked()
	e.feedbackMu.Unlock()
}

func (e *Engine) completeFeedbackTurn(state *interactiveState, response string) {
	turn, as := state.feedbackTurnSnapshot()
	if turn == nil {
		return
	}
	turn.mu.Lock()
	if !turn.active {
		turn.mu.Unlock()
		return
	}
	turn.active = false
	turn.at = time.Now()
	turn.diagnostic.OccurredAt = turn.at
	turn.diagnostic.Phase = "completed"
	turn.diagnostic.Response = response
	turn.diagnostic.Missing = nil
	if turn.shared {
		turn.diagnostic.Response = ""
		turn.diagnostic.Missing = []string{"earlier participant context: excluded", "response: omitted after participant handoff"}
	}
	if as != nil {
		applyAgentDiagnostic(&turn.diagnostic, as)
		turn.diagnostic.Transport.ProcessState = "alive"
		if !as.Alive() {
			turn.diagnostic.Transport.ProcessState = "exited"
		}
	}
	elapsed := max(0, turn.at.Sub(turn.diagnostic.StartedAt).Milliseconds())
	turn.diagnostic.Transport.LastDeliveryMS = &elapsed
	turn.diagnostic = appfeatures.CaptureFeedbackDiagnostic(turn.diagnostic)
	turn.mu.Unlock()
	e.feedbackMu.Lock()
	e.retainFeedbackTurnLocked(turn)
	e.scheduleFeedbackSaveLocked()
	e.feedbackMu.Unlock()
}

// Active states retain their own pointer even when the bounded lookup evicts
// it. Restore a completed snapshot only if no successor owns this key.
func (e *Engine) retainFeedbackTurnLocked(turn *feedbackTurn) {
	if current := e.feedbackTurns[turn.key]; current == nil || current == turn {
		e.feedbackTurns[turn.key] = turn
	}
	e.pruneFeedbackTurnsLocked(time.Now())
}

func (e *Engine) adoptQueuedFeedbackTurn(state *interactiveState, queued queuedMessage, session *Session) {
	state.mu.Lock()
	agent := state.agent
	state.mu.Unlock()
	if agent == nil {
		agent = e.agent
	}
	msg := &Message{SessionKey: queued.msgSessionKey, UserID: queued.userID, Content: queued.feedbackText, feedbackTextSet: true, feedbackText: queued.feedbackText}
	turn := e.beginFeedbackTurn(queued.platform, msg, session, agent)
	state.mu.Lock()
	state.feedbackTurn = turn
	state.mu.Unlock()
}

// Called while state.mu is held at the exact accepted-steer ownership change.
func (e *Engine) adoptSteerFeedbackLocked(state *interactiveState, h steerHandoff) {
	turn := state.feedbackTurn
	if turn == nil {
		return
	}
	turn.mu.Lock()
	if !turn.active {
		turn.mu.Unlock()
		return
	}
	if state.currentUserID == h.userID {
		turn.diagnostic.Request += "\n\nAdditional user input:\n" + h.feedbackText
		turn.diagnostic = appfeatures.CaptureFeedbackDiagnostic(turn.diagnostic)
		turn.mu.Unlock()
		e.feedbackMu.Lock()
		e.scheduleFeedbackSaveLocked()
		e.feedbackMu.Unlock()
		return
	}
	// The successor must never inherit another participant's request/answer.
	// Keep timing/counters for this same backend turn, explicitly mark the gap.
	copy := turn.diagnostic
	turn.active = false
	turn.diagnostic.Phase = "handoff"
	turn.at = time.Now()
	turn.mu.Unlock()
	copy.Request = h.feedbackText
	copy.Response = ""
	copy.Activity = nil
	copy.Missing = []string{"earlier participant context: excluded"}
	copy = appfeatures.CaptureFeedbackDiagnostic(copy)
	next := &feedbackTurn{key: e.feedbackBinding(turn.scope, h.userID), scope: turn.scope, at: time.Now(), active: true, shared: true, version: turn.version, agent: turn.agent, diagnostic: copy}
	state.feedbackTurn = next
	e.feedbackMu.Lock()
	e.feedbackTurns[next.key] = next
	e.pruneFeedbackTurnsLocked(time.Now())
	e.scheduleFeedbackSaveLocked()
	e.feedbackMu.Unlock()
}

func (e *Engine) pruneFeedbackTurnsLocked(now time.Time) {
	for key, turn := range e.feedbackTurns {
		turn.mu.Lock()
		expired := !turn.active && now.Sub(turn.at) > feedbackRetain
		turn.mu.Unlock()
		if expired {
			delete(e.feedbackTurns, key)
		}
	}
	for len(e.feedbackTurns) > feedbackTurnLimit {
		oldestKey := ""
		var oldest time.Time
		for key, turn := range e.feedbackTurns {
			turn.mu.Lock()
			at := turn.at
			turn.mu.Unlock()
			if oldestKey == "" || at.Before(oldest) {
				oldestKey, oldest = key, at
			}
		}
		if oldestKey == "" {
			break
		}
		delete(e.feedbackTurns, oldestKey)
	}
}

func (e *Engine) feedbackDiagnosticFor(sessionKey, userID, description string) (*appfeatures.FeedbackDiagnostic, string, string) {
	if userID == "" {
		return nil, "", ""
	}
	key := e.feedbackContextKey(sessionKey, userID)
	e.feedbackMu.Lock()
	turn := e.feedbackTurns[key]
	e.feedbackMu.Unlock()
	if turn == nil {
		return nil, "", ""
	}
	turn.mu.Lock()
	defer turn.mu.Unlock()
	if time.Since(turn.at) > feedbackRetain && !turn.active {
		return nil, "", ""
	}
	// Free-form feedback can describe a new feature. Only a bare command or
	// the prepared error offer selects a failed incident, never a stale error
	// opportunistically attached to unrelated prose.
	if description != "" && turn.diagnostic.Error != "" {
		return nil, "", ""
	}
	value := turn.diagnostic
	value.OccurredAt = time.Now()
	if !turn.active {
		value.OccurredAt = turn.diagnostic.OccurredAt
	}
	value = appfeatures.CaptureFeedbackDiagnostic(value)
	return &value, turn.version, turn.agent
}
