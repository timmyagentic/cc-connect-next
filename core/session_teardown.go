package core

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Covers normal Stop-hook grace, termination, and bounded kill confirmation.
const sessionCloseTimeout = 150 * time.Second

type sessionCloseGroup struct {
	done    chan struct{}
	pending int
}

type sessionCloseTarget struct {
	platform Platform
	replyCtx any
	userStop bool
}

func (e *Engine) beginSessionClose(key string) func() {
	if key == "" {
		return func() {}
	}
	e.closingMu.Lock()
	if e.closingSessions == nil {
		e.closingSessions = make(map[string]*sessionCloseGroup)
	}
	group := e.closingSessions[key]
	if group == nil {
		group = &sessionCloseGroup{done: make(chan struct{})}
		e.closingSessions[key] = group
	}
	group.pending++
	e.closingMu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			e.closingMu.Lock()
			defer e.closingMu.Unlock()
			group.pending--
			if group.pending == 0 {
				delete(e.closingSessions, key)
				close(group.done)
			}
		})
	}
}

func (e *Engine) awaitSessionClose(key string) bool {
	timer := time.NewTimer(sessionCloseTimeout + 30*time.Second)
	defer timer.Stop()
	for {
		e.closingMu.Lock()
		group := e.closingSessions[key]
		e.closingMu.Unlock()
		if group == nil {
			return true
		}
		select {
		case <-group.done:
		case <-timer.C:
			return false
		case <-e.ctx.Done():
			return false
		}
	}
}

func (e *Engine) sessionClosePending(key string) bool {
	e.closingMu.Lock()
	defer e.closingMu.Unlock()
	return e.closingSessions[key] != nil
}

func (e *Engine) markUnsafeResume(key string) {
	if key == "" {
		return
	}
	e.closingMu.Lock()
	defer e.closingMu.Unlock()
	if e.unsafeResume == nil {
		e.unsafeResume = make(map[string]bool)
	}
	e.unsafeResume[key] = true
}

func (e *Engine) consumeUnsafeResume(key string) bool {
	e.closingMu.Lock()
	defer e.closingMu.Unlock()
	unsafe := e.unsafeResume[key]
	delete(e.unsafeResume, key)
	return unsafe
}

func (e *Engine) closeAgentSessionAsync(key string, session AgentSession, targets ...sessionCloseTarget) {
	if session == nil {
		return
	}
	finish := e.beginSessionClose(key)
	go func() { defer finish(); e.closeAgentSessionRegistered(key, session, targets...) }()
}

func (e *Engine) closeAgentSessionWithTimeout(key string, session AgentSession, targets ...sessionCloseTarget) {
	if session == nil {
		return
	}
	finish := e.beginSessionClose(key)
	defer finish()
	e.closeAgentSessionRegistered(key, session, targets...)
}

func (e *Engine) closeAgentSessionRegistered(key string, session AgentSession, targets ...sessionCloseTarget) {
	if session == nil {
		return
	}
	var target sessionCloseTarget
	if len(targets) > 0 {
		target = targets[0]
	}
	started := time.Now()
	result := make(chan error, 1)
	go func() {
		defer func() {
			if value := recover(); value != nil {
				result <- fmt.Errorf("session close panicked: %v", value)
			}
		}()
		if closer, ok := session.(AgentSessionStopCloser); target.userStop && ok {
			result <- closer.CloseForStop()
		} else {
			result <- session.Close()
		}
	}()
	var closeErr error
	select {
	case closeErr = <-result:
	case <-time.After(sessionCloseTimeout):
		closeErr = fmt.Errorf("session close exceeded %s", sessionCloseTimeout)
	}
	if closeErr != nil {
		e.markUnsafeResume(key)
		slog.Error("agent session close not confirmed", "session", key, "error", closeErr)
		if target.platform != nil && target.replyCtx != nil {
			e.send(target.platform, target.replyCtx, e.i18n.T(MsgSessionCloseFailed))
		}
	} else if elapsed := time.Since(started); elapsed >= slowAgentClose {
		slog.Warn("slow agent session close", "session", key, "elapsed", elapsed)
	}
}
