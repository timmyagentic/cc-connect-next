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

const signatureRecoveryContinuation = "Continue the unfinished user request using the existing conversation and recorded tool results. Completed tool calls already ran; use their recorded results and do not repeat completed actions. Preserve the user's existing instructions and authorization. Return the requested result without mentioning this internal retry."

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
	if s.recoveryUsed || !s.alive.Load() || s.currentTurn != turnID {
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
	if err := s.restoreSignatureHistory(r); err != nil {
		s.failSignatureRecovery(r, err, true)
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
	var terminals struct {
		Data       []json.RawMessage `json:"data"`
		NextCursor *string           `json:"nextCursor"`
	}
	if err := s.request("thread/backgroundTerminals/list", map[string]any{"threadId": r.threadID}, &terminals); err != nil {
		return fmt.Errorf("check unfinished commands: %w", err)
	}
	if terminals.Data == nil || len(terminals.Data) != 0 || terminals.NextCursor != nil {
		return fmt.Errorf("background command outcomes are unavailable or unfinished")
	}
	options := s.signatureRecoveryOptions()
	if err := s.stopSignatureTransport(r); err != nil {
		return err
	}
	if _, err := cleanSignatureHistory(s.ctx, path, r.threadID); err != nil {
		return err
	}
	if !s.signatureRecoveryActive(r) {
		return s.contextErr()
	}
	if err := s.connect(); err != nil {
		return err
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if err := s.ensureThread(r.threadID); err != nil {
		return err
	}
	if s.CurrentSessionID() != r.threadID {
		return fmt.Errorf("recovery changed the original thread identity")
	}
	if !s.signatureRecoveryActive(r) {
		return s.contextErr()
	}
	s.stateMu.Lock()
	s.currentTurn = ""
	s.stateMu.Unlock()
	slog.Info("codex app-server retrying original thread after clearing encrypted history", "thread_id", r.threadID)
	input := []map[string]any{{"type": "text", "text": signatureRecoveryContinuation, "text_elements": []any{}}}
	if err := s.startTurn(r.threadID, input, &options); err != nil {
		return fmt.Errorf("retry original conversation: %w", err)
	}
	s.stateMu.Lock()
	if s.recovery == r {
		s.recovery = nil
	}
	s.stateMu.Unlock()
	return nil
}

// Close the dedicated native process before editing its rollout. A live
// app-server keeps the rejected history in memory and can write it back.
func (s *appServerSession) stopSignatureTransport(r *signatureRecovery) error {
	s.stateMu.Lock()
	if s.recovery != r || !s.alive.Load() {
		s.stateMu.Unlock()
		return s.contextErr()
	}
	s.restartingTransport = true
	s.stateMu.Unlock()
	defer func() {
		s.stateMu.Lock()
		s.restartingTransport = false
		s.stateMu.Unlock()
	}()

	s.procMu.Lock()
	stdin, done := s.stdin, s.transportDone
	s.stdin = nil
	s.procMu.Unlock()
	if stdin == nil || done == nil {
		return fmt.Errorf("local recovery transport is unavailable")
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("close recovery transport: %w", err)
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-s.contextDone():
		return s.contextErr()
	case <-done:
		return nil
	case <-timer.C:
		return fmt.Errorf("native process did not finish before history cleanup")
	}
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
