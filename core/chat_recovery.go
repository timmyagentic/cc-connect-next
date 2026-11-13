package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Only ordinary, resumable chats with an updatable durable answer participate.
// Scheduler receipts remain observations and are never used to replay work.
type chatRecoveryState struct {
	Version        int             `json:"version"`
	Project        string          `json:"project"`
	Platform       string          `json:"platform"`
	SessionID      string          `json:"session_id"`
	SessionKey     string          `json:"session_key"`
	InteractiveKey string          `json:"interactive_key"`
	Workspace      string          `json:"workspace,omitempty"`
	AgentSessionID string          `json:"agent_session_id,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	Reply          json.RawMessage `json:"reply"`
	Native         json.RawMessage `json:"native,omitempty"`
	Preview        json.RawMessage `json:"preview,omitempty"`
	HistorySaved   bool            `json:"history_saved"`
	Closed         bool            `json:"closed,omitempty"`
}

type chatRecovery struct {
	mu                sync.Mutex
	dir               string
	state             chatRecoveryState
	message           Message
	input             TurnRecoveryInput
	resuming          bool
	previewGeneration uint64
	previewReady      chan struct{}
	previewReadyOnce  sync.Once
	previewErr        error
}

func writeChatRecoveryFile(path string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".checkpoint-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func (r *chatRecovery) saveLocked() error {
	return writeChatRecoveryFile(filepath.Join(r.dir, "state.json"), r.state)
}

func (r *chatRecovery) saveNative(native json.RawMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state.Closed {
		return errors.New("chat recovery was cancelled")
	}
	r.state.Native = append(json.RawMessage(nil), native...)
	return r.saveLocked()
}

func (r *chatRecovery) finish() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state.Closed {
		_ = os.RemoveAll(r.dir)
		return
	}
	r.state.Closed = true
	// Keep the terminal marker if cleanup fails. A finished/stopped turn must
	// never come back merely because its directory could not be removed.
	if err := r.saveLocked(); err != nil {
		slog.Error("chat recovery: persist terminal state", "error", err)
		if removeErr := os.RemoveAll(r.dir); removeErr != nil {
			slog.Error("chat recovery: remove terminal checkpoint", "error", removeErr)
		}
		return
	}
	if err := os.RemoveAll(r.dir); err != nil {
		slog.Warn("chat recovery: cleanup", "error", err)
	}
}

func (r *chatRecovery) closed() bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.Closed
}

func (e *Engine) chatRecoveryRoot() string {
	if e.sessions.StorePath() == "" {
		return ""
	}
	return e.sessions.StorePath() + ".recovery"
}

func (e *Engine) beginChatRecovery(p Platform, msg *Message, session *Session, agent Agent, key, workspace string) (*chatRecovery, error) {
	if msg.chatRecovery != nil {
		return msg.chatRecovery, nil
	}
	support, ok := agent.(TurnRecoverySupporter)
	platform, durable := p.(PersistentPreviewPlatform)
	display, _ := e.displayRuntimeSnapshot()
	root := e.chatRecoveryRoot()
	if !ok || !support.SupportsTurnRecovery() || !durable || display.CardMode != "rich" || root == "" || msg.scheduledResult != nil || msg.MessageID == "" {
		return nil, nil
	}
	reply, err := platform.SnapshotTurnReplyCtx(msg.ReplyCtx)
	if err != nil {
		return nil, err
	}
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, err
	}
	dir := filepath.Join(root, hex.EncodeToString(id[:]))
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	copy := *msg
	copy.ReplyCtx, copy.chatRecovery = nil, nil
	r := &chatRecovery{dir: dir, message: copy, previewReady: make(chan struct{}), state: chatRecoveryState{
		Version: 1, Project: e.name, Platform: p.Name(), SessionID: session.ID,
		SessionKey: msg.SessionKey, InteractiveKey: key, Workspace: workspace,
		CreatedAt: time.Now(), Reply: reply,
	}}
	if err = writeChatRecoveryFile(filepath.Join(dir, "message.json"), &copy); err == nil {
		err = r.saveLocked()
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	msg.chatRecovery = r
	return r, nil
}

func (e *Engine) sendRecoverableTurn(as AgentSession, input TurnRecoveryInput, recovery []*chatRecovery) error {
	var r *chatRecovery
	if len(recovery) > 0 {
		r = recovery[0]
	}
	if r != nil {
		if r.previewReady != nil {
			select {
			case <-r.previewReady:
			case <-e.ctx.Done():
				return e.ctx.Err()
			}
			r.mu.Lock()
			previewErr := r.previewErr
			r.mu.Unlock()
			if previewErr != nil {
				return previewErr
			}
		}
		backend, ok := as.(TurnRecoverySession)
		if !ok {
			return errors.New("agent no longer supports interrupted chat recovery")
		}
		r.mu.Lock()
		if r.state.Closed {
			r.mu.Unlock()
			return errors.New("chat turn was cancelled")
		}
		resume, native, savedInput := r.resuming, append(json.RawMessage(nil), r.state.Native...), r.input
		r.mu.Unlock()
		if resume && len(native) > 0 {
			return backend.ResumeInterruptedTurn(native, savedInput, r.saveNative)
		}
		native, err := backend.PrepareTurnRecovery(input)
		if err != nil {
			return err
		}
		r.mu.Lock()
		if r.state.Closed {
			r.mu.Unlock()
			return errors.New("chat turn was cancelled")
		}
		r.input = input
		err = writeChatRecoveryFile(filepath.Join(r.dir, "input.json"), &input)
		if err == nil {
			r.state.Native = native
			r.state.AgentSessionID = as.CurrentSessionID()
			err = r.saveLocked()
		}
		r.mu.Unlock()
		if err != nil {
			return err
		}
	}
	if input.UseOptions {
		return as.(TurnOptionsSession).SendWithTurnOptions(input.Prompt, input.Images, input.Files, input.Options)
	}
	return as.Send(input.Prompt, input.Images, input.Files)
}

func (s *interactiveState) recoveryTurn() *chatRecovery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chatRecovery
}

func (s *interactiveState) suspendedForRestart() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restartSuspended
}

func (e *Engine) restoreChatRecoveries() {
	root := e.chatRecoveryRoot()
	if root == "" {
		return
	}
	dirs, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		slog.Error("chat recovery: list", "error", err)
		return
	}
	var records []*chatRecovery
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		r := &chatRecovery{dir: filepath.Join(root, dir.Name()), resuming: true, previewReady: make(chan struct{})}
		read := func(name string, dst any) error {
			b, err := os.ReadFile(filepath.Join(r.dir, name))
			if err != nil {
				return err
			}
			return json.Unmarshal(b, dst)
		}
		if err := read("state.json", &r.state); err != nil {
			slog.Warn("chat recovery: unreadable checkpoint", "error", err)
			continue
		}
		if r.state.Version != 1 || r.state.Project != e.name || r.state.Closed {
			continue
		}
		if err := read("message.json", &r.message); err != nil {
			slog.Warn("chat recovery: unreadable input", "error", err)
			continue
		}
		if len(r.state.Native) > 0 {
			if err := read("input.json", &r.input); err != nil {
				slog.Warn("chat recovery: missing prepared input", "error", err)
				continue
			}
		}
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].state.CreatedAt.Before(records[j].state.CreatedAt) })
	groups := make(map[string][]*chatRecovery)
	for _, r := range records {
		groups[r.state.InteractiveKey] = append(groups[r.state.InteractiveKey], r)
	}
	for _, group := range groups {
		e.reserveChatRecovery(group)
	}
}

// Reserve before starting platform intake: a new user message must queue behind
// recovery, and /stop or /new must be able to cancel it even before readiness.
func (e *Engine) reserveChatRecovery(group []*chatRecovery) {
	first := group[0]
	saved := first.state
	var p Platform
	for _, candidate := range e.platforms {
		if candidate.Name() == saved.Platform {
			p = candidate
			break
		}
	}
	platform, ok := p.(PersistentPreviewPlatform)
	if !ok {
		return
	}
	agent, sessions, key := e.agent, e.sessions, saved.InteractiveKey
	if saved.Workspace != "" {
		var err error
		agent, sessions, _, _, err = e.workspaceContext(saved.Workspace, saved.SessionKey)
		if err != nil {
			slog.Error("chat recovery: workspace", "error", err)
			return
		}
	}
	support, ok := agent.(TurnRecoverySupporter)
	if !ok || !support.SupportsTurnRecovery() {
		return
	}
	if sessions.ActiveSessionID(saved.SessionKey) != saved.SessionID {
		for _, r := range group {
			r.finish()
		}
		return
	}
	session := sessions.GetOrCreateActive(saved.SessionKey)
	if saved.AgentSessionID != "" && session.GetAgentSessionID() != saved.AgentSessionID {
		slog.Warn("chat recovery: native session identity changed", "session", session.ID)
		return
	}
	if !session.TryLock() {
		return
	}
	messages := make([]*Message, 0, len(group))
	for _, r := range group {
		if r.state.SessionID != saved.SessionID {
			r.finish()
			continue
		}
		ctx, err := platform.RestoreTurnReplyCtx(r.state.Reply)
		if err != nil {
			slog.Warn("chat recovery: reply target", "error", err)
			continue
		}
		msg := r.message
		msg.ReplyCtx, msg.chatRecovery = ctx, r
		messages = append(messages, &msg)
	}
	if len(messages) == 0 {
		session.UnlockWithoutUpdate()
		return
	}
	firstMsg := messages[0]
	e.ensureInteractiveStateForQueueing(key, p, firstMsg.ReplyCtx)
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	state.mu.Lock()
	state.chatRecovery = firstMsg.chatRecovery
	state.currentMessageID = firstMsg.MessageID
	state.currentReplyUserID = firstMsg.UserID
	state.currentTurnUserMessageTimeMs = firstMsg.UserMessageTimeMs
	state.mu.Unlock()
	e.interactiveMu.Unlock()
	for _, msg := range messages[1:] {
		e.queueMessageForBusySession(p, msg, key, session)
	}
	done := e.updates().retainWork()
	e.platformLifecycleMu.Lock()
	if e.chatRecoveryStarts == nil {
		e.chatRecoveryStarts = make(map[string][]func())
	}
	e.chatRecoveryStarts[p.Name()] = append(e.chatRecoveryStarts[p.Name()], func() {
		defer done()
		if firstMsg.chatRecovery.closed() || state.isStopped() {
			session.UnlockWithoutUpdate()
			return
		}
		slog.Info("chat recovery: resuming interrupted turn", "project", e.name, "session", session.ID)
		e.processInteractiveMessageWith(p, firstMsg, session, agent, sessions, key, saved.Workspace, saved.SessionKey)
	})
	e.platformLifecycleMu.Unlock()
}

func (e *Engine) startChatRecoveries(p Platform) {
	e.platformLifecycleMu.Lock()
	pending := e.chatRecoveryStarts[p.Name()]
	delete(e.chatRecoveryStarts, p.Name())
	e.platformLifecycleMu.Unlock()
	for _, start := range pending {
		go start()
	}
}

func (e *Engine) recoverPreview(state *interactiveState, p Platform, fresh func() any) any {
	r := state.recoveryTurn()
	if r == nil {
		return fresh()
	}
	platform, ok := p.(PersistentPreviewPlatform)
	if !ok {
		return fresh()
	}
	r.mu.Lock()
	preview := append(json.RawMessage(nil), r.state.Preview...)
	r.mu.Unlock()
	if len(preview) > 0 {
		handle, err := platform.RestorePreview(preview, r.previewWriter())
		if err == nil {
			r.signalPreview(nil)
			return handle
		}
		r.signalPreview(fmt.Errorf("restore interrupted answer card: %w", err))
		slog.Error("chat recovery: restore answer card", "error", err)
		return nil
	}
	handle := fresh()
	err := e.checkpointChatPreview(state, p, handle)
	r.signalPreview(err)
	return handle
}

func (e *Engine) updateRecoveryReply(state *interactiveState, p Platform, msg *Message, handle any) {
	r := state.recoveryTurn()
	platform, ok := p.(PersistentPreviewPlatform)
	if r == nil || !ok {
		return
	}
	reply, err := platform.SnapshotTurnReplyCtx(msg.ReplyCtx)
	if err != nil {
		slog.Error("chat recovery: save steered target", "error", err)
		return
	}
	r.mu.Lock()
	r.state.Reply = reply
	r.message.MessageID, r.message.UserID, r.message.Content = msg.MessageID, msg.UserID, msg.Content
	r.message.UserMessageTimeMs, r.message.UserName, r.message.FromVoice = msg.UserMessageTimeMs, msg.UserName, msg.FromVoice
	err = writeChatRecoveryFile(filepath.Join(r.dir, "message.json"), &r.message)
	if err == nil {
		err = r.saveLocked()
	}
	r.mu.Unlock()
	if err == nil && handle != nil {
		err = platform.CheckpointPreview(handle, r.previewWriter())
	}
	if err != nil {
		slog.Error("chat recovery: save steered card", "error", err)
	}
}

func (r *chatRecovery) saveHistory() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.HistorySaved = true
	return r.saveLocked()
}

func (r *chatRecovery) hadHistory() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.HistorySaved
}

func recoveryError(err error) error { return fmt.Errorf("preserve interrupted chat: %w", err) }

func (e *Engine) isChatRecoveryShutdown() bool {
	e.platformLifecycleMu.Lock()
	defer e.platformLifecycleMu.Unlock()
	return e.stopping
}

func (r *chatRecovery) signalPreview(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.previewReadyOnce.Do(func() {
		r.previewErr = err
		if r.previewReady != nil {
			close(r.previewReady)
		}
	})
}

func (r *chatRecovery) previewWriter() func(json.RawMessage) error {
	r.mu.Lock()
	r.previewGeneration++
	generation := r.previewGeneration
	r.mu.Unlock()
	return func(snapshot json.RawMessage) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.state.Closed || r.previewGeneration != generation {
			return nil
		}
		r.state.Preview = append(json.RawMessage(nil), snapshot...)
		return r.saveLocked()
	}
}

func (e *Engine) checkpointChatPreview(state *interactiveState, p Platform, handle any) error {
	r := state.recoveryTurn()
	platform, ok := p.(PersistentPreviewPlatform)
	if r == nil || handle == nil || !ok {
		return nil
	}
	if err := platform.CheckpointPreview(handle, r.previewWriter()); err != nil {
		slog.Error("chat recovery: checkpoint answer card", "error", err)
		return err
	}
	return nil
}

func (r *chatRecovery) resumesNativeTurn() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.resuming && len(r.state.Native) > 0
}
