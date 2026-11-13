package codex

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

// The failed native turn must finish before its history can be replayed. The
// reader remains free to receive both that terminal event and recovery RPCs.
type signatureRecovery struct {
	threadID string
	turnID   string
	err      error
	terminal chan struct{}
	once     sync.Once
}

const signatureRecoveryContinuation = "Continue the unfinished user request using the imported conversation and recorded tool results. The previous model request failed because its encrypted context was rejected. Completed tool calls already ran; use their recorded results and do not repeat completed actions. Preserve the user's existing instructions and authorization."

func isSignatureError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.TrimSpace(err.Error())
	text = strings.TrimPrefix(text, "stream disconnected before completion: ")
	for _, code := range []string{"thinking_signature_invalid", "invalid_encrypted_content"} {
		if text == code || strings.HasPrefix(text, code+":") {
			return true
		}
	}
	if strings.HasPrefix(text, "unexpected status 400") {
		if start := strings.IndexByte(text, '{'); start >= 0 {
			text = text[start:]
		}
	}
	var body struct {
		Code  string `json:"code"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(text), &body) != nil {
		return false
	}
	code := body.Error.Code
	if code == "" {
		code = body.Code
	}
	return code == "thinking_signature_invalid" || code == "invalid_encrypted_content"
}

func (s *appServerSession) deferSignatureRecovery(err error, threadID, turnID string, terminal bool) bool {
	if !isSignatureError(err) || threadID == "" || turnID == "" || !localAppServer(s.url) {
		return false
	}
	s.stateMu.Lock()
	if s.recovery != nil {
		r := s.recovery
		s.stateMu.Unlock()
		return r.threadID == threadID && r.turnID == turnID
	}
	if s.recoveryUsed || s.unsafeRecoveryTurn == turnID || s.pendingSteers != 0 || !s.alive.Load() || s.currentTurn != turnID {
		s.stateMu.Unlock()
		return false
	}
	r := &signatureRecovery{threadID: threadID, turnID: turnID, err: err, terminal: make(chan struct{})}
	s.recovery = r
	s.recoveryUsed = true
	s.turnRevision++
	s.pendingMsgs = nil
	s.wg.Add(1)
	s.stateMu.Unlock()
	if terminal {
		r.once.Do(func() { close(r.terminal) })
	}
	go func() {
		defer s.wg.Done()
		s.recoverSignature(r)
	}()
	return true
}

func (s *appServerSession) finishSignatureRecoveryTurn(notif turnNotification) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	r := s.recovery
	if r == nil || notif.ThreadID != r.threadID || notif.Turn.ID != r.turnID {
		return false
	}
	select {
	case <-r.terminal:
		return true
	default:
	}
	if notif.Turn.Status != "failed" || (notif.Turn.Error != nil && !isSignatureError(fmt.Errorf("%s", notif.Turn.Error.Message))) {
		s.recovery = nil
		r.once.Do(func() { close(r.terminal) })
		return false
	}
	r.once.Do(func() { close(r.terminal) })
	return true
}

func (s *appServerSession) signatureRecoveryWaiting(threadID string) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.recovery != nil && s.recovery.threadID == threadID
}

func (s *appServerSession) recoverSignature(r *signatureRecovery) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-s.contextDone():
		return
	case <-timer.C:
		s.failSignatureRecovery(r, fmt.Errorf("failed turn completion was not confirmed"), true)
		return
	case <-r.terminal:
	}
	if !s.signatureRecoveryActive(r) {
		return
	}
	s.emit(core.Event{Type: core.EventSessionRecovery})
	if err := s.restoreSignatureHistory(r); err != nil {
		s.failSignatureRecovery(r, err, false)
	}
}

func (s *appServerSession) signatureRecoveryActive(r *signatureRecovery) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.recovery == r && s.alive.Load() && (s.ctx == nil || s.ctx.Err() == nil)
}

func (s *appServerSession) restoreSignatureHistory(r *signatureRecovery) error {
	var history struct {
		Thread struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"thread"`
	}
	if err := s.request("thread/read", map[string]any{"threadId": r.threadID, "includeTurns": false}, &history); err != nil {
		return fmt.Errorf("read recovery history: %w", err)
	}
	if history.Thread.ID != r.threadID || history.Thread.Path == "" {
		return fmt.Errorf("recovery history identity or path is unavailable")
	}
	path, err := s.signatureHistoryPath(history.Thread.Path)
	if err != nil {
		return err
	}
	// A completed shell tool can have left a background command running. Its
	// outcome belongs to the old thread and cannot be assumed by a replacement.
	var terminals struct {
		Data       []json.RawMessage `json:"data"`
		NextCursor *string           `json:"nextCursor"`
	}
	if err := s.request("thread/backgroundTerminals/list", map[string]any{"threadId": r.threadID}, &terminals); err != nil {
		return fmt.Errorf("check unfinished commands: %w", err)
	}
	if terminals.Data == nil {
		return fmt.Errorf("background command state is unavailable")
	}
	if len(terminals.Data) != 0 || terminals.NextCursor != nil {
		return fmt.Errorf("background commands still have an unresolved outcome")
	}
	items, err := loadSignatureRecoveryHistory(s.ctx, path, r.threadID, r.turnID)
	if err != nil {
		return err
	}
	if !s.signatureRecoveryActive(r) {
		return s.contextErr()
	}
	options := s.signatureRecoveryOptions()
	params := s.threadRequestParams()
	params["model"] = options.Model
	params["cwd"] = s.GetWorkDir()
	if options.ReasoningEffort != "" {
		params["config"] = map[string]any{"model_reasoning_effort": options.ReasoningEffort}
	}
	params["serviceTier"] = nullableTurnOption(options.ServiceTier)
	var replacement threadStartResponse
	if err := s.request("thread/start", params, &replacement); err != nil {
		return fmt.Errorf("start replacement conversation: %w", err)
	}
	newID := replacement.Thread.ID
	if newID == "" || newID == r.threadID {
		return fmt.Errorf("replacement conversation did not return a fresh identity")
	}
	if !s.signatureRecoveryActive(r) {
		return s.contextErr()
	}
	if err := s.request("thread/inject_items", map[string]any{"threadId": newID, "items": items}, nil); err != nil {
		return fmt.Errorf("import recovery history: %w", err)
	}
	// Publish the new binding only after its complete history is acknowledged.
	// Keeping the same AgentSession preserves the engine's lock, queue and card.
	s.stateMu.Lock()
	if s.recovery != r || !s.alive.Load() || (s.ctx != nil && s.ctx.Err() != nil) {
		s.stateMu.Unlock()
		return s.contextErr()
	}
	s.threadID.Store(newID)
	s.currentTurn = ""
	s.pendingMsgs = nil
	s.stateMu.Unlock()
	ready := make(chan error, 1)
	s.emit(core.Event{Type: core.EventSessionRecovery, SessionID: newID, RecoveryReady: ready})
	// No new model request may run until the caller has durably adopted the
	// replacement. /stop before acknowledgement therefore cannot lose new work.
	ackTimer := time.NewTimer(appServerRequestTimeout)
	defer ackTimer.Stop()
	select {
	case <-s.contextDone():
		return s.contextErr()
	case err := <-ready:
		if err != nil {
			return fmt.Errorf("persist restored conversation: %w", err)
		}
	case <-ackTimer.C:
		return fmt.Errorf("restored conversation was not acknowledged")
	}
	if !s.signatureRecoveryActive(r) {
		return s.contextErr()
	}
	slog.Info("codex app-server conversation restored after signature failure", "previous_thread_id", r.threadID, "thread_id", newID)
	input := []map[string]any{{"type": "text", "text": signatureRecoveryContinuation, "text_elements": []any{}}}
	if err := s.startTurn(newID, input, &options); err != nil {
		// An RPC timeout can hide an accepted turn. Retire this transport so
		// later input cannot accidentally steer or duplicate an unknown turn.
		s.failSignatureRecovery(r, fmt.Errorf("continue restored conversation: %w", err), true)
		return nil
	}
	s.stateMu.Lock()
	if s.recovery == r {
		s.recovery = nil
	}
	s.stateMu.Unlock()
	return nil
}

func (s *appServerSession) signatureRecoveryOptions() core.TurnOptions {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	if s.turnOptions != nil {
		return *s.turnOptions
	}
	return core.TurnOptions{Model: s.model, ReasoningEffort: s.effort, ServiceTier: s.serviceTier}
}

func (s *appServerSession) signatureHistoryPath(path string) (string, error) {
	root, err := filepath.EvalSymlinks(filepath.Join(resolveCodexHomeDir(s.codexHome), "sessions"))
	if err != nil {
		return "", fmt.Errorf("local recovery history is unavailable")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("local recovery history is unavailable")
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("recovery history is outside this agent's session directory")
	}
	return resolved, nil
}

func (s *appServerSession) failSignatureRecovery(r *signatureRecovery, err error, retire bool) {
	s.stateMu.Lock()
	if s.recovery != r || !s.alive.Load() || (s.ctx != nil && s.ctx.Err() != nil) {
		s.stateMu.Unlock()
		return
	}
	s.recovery = nil
	if retire {
		s.alive.Store(false)
	}
	s.currentTurn = ""
	s.pendingMsgs = nil
	s.turnRevision++
	s.stateMu.Unlock()
	s.emitError(fmt.Errorf("%w (automatic conversation recovery stopped: %v)", r.err, err))
	if retire {
		s.abortTransport()
	}
}
