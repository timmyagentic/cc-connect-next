package core

import (
	"context"
	"errors"
	"sync"
)

var errUpdateAdmissionClosed = errors.New("service update is waiting for running tasks; new work was not started")

// UpdateCoordinator belongs to the process, not a chat or an individual bot.
// Closing admission and observing the last accepted task share one mutex.
// Waiting is event-driven; there is no background polling of sessions.
type UpdateCoordinator struct {
	mu             sync.Mutex
	active         int
	draining       bool
	restartPending bool
	changed        chan struct{}
	operation      *updateOperation
	engines        []*Engine
}

func NewUpdateCoordinator() *UpdateCoordinator {
	return &UpdateCoordinator{changed: make(chan struct{})}
}

// SetUpdateCoordinator must be called before starting the Engine. Every Engine
// sharing an executable must use the same coordinator.
func (e *Engine) SetUpdateCoordinator(c *UpdateCoordinator) {
	e.updateCoordinator = c
	c.mu.Lock()
	c.engines = append(c.engines, e)
	c.mu.Unlock()
}
func (e *Engine) updates() *UpdateCoordinator {
	e.updateServiceMu.Lock()
	defer e.updateServiceMu.Unlock()
	if e.updateCoordinator == nil {
		e.updateCoordinator = NewUpdateCoordinator()
		e.updateCoordinator.engines = []*Engine{e}
	}
	return e.updateCoordinator
}
func (c *UpdateCoordinator) signalLocked() { close(c.changed); c.changed = make(chan struct{}) }
func (c *UpdateCoordinator) beginWork() (func(), bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.draining {
		return nil, false
	}
	c.active++
	var once sync.Once
	return func() { once.Do(func() { c.mu.Lock(); c.active--; c.signalLocked(); c.mu.Unlock() }) }, true
}
func (c *UpdateCoordinator) activeCount() int { c.mu.Lock(); defer c.mu.Unlock(); return c.active }
func (c *UpdateCoordinator) reserve(op *updateOperation) (*updateOperation, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.operation != nil || c.restartPending {
		return c.operation, false
	}
	c.operation = op
	return op, true
}
func (c *UpdateCoordinator) release(op *updateOperation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.operation == op {
		c.operation = nil
		c.draining = false
		c.signalLocked()
	}
}
func (c *UpdateCoordinator) waitIdle(ctx context.Context, count func(int)) error {
	c.mu.Lock()
	c.draining = true
	c.signalLocked()
	for {
		n, changed := c.active, c.changed
		c.mu.Unlock()
		if count != nil {
			count(n)
		}
		if n == 0 {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
		c.mu.Lock()
	}
}

func (e *Engine) beginUpdateWork() (func(), error) {
	done, ok := e.updates().beginWork()
	if !ok {
		return nil, errUpdateAdmissionClosed
	}
	return done, nil
}

func updateControlCommand(cmd string) bool {
	switch cmd {
	case "upgrade", "status", "version", "help", "stop", "cancel", "allow", "deny", "approve", "history", "current", "list":
		return true
	}
	return false
}

// retainWork is used only while an already-admitted caller transfers work to
// a goroutine. The existing admission remains held until this count is added.
func (c *UpdateCoordinator) retainWork() func() {
	c.mu.Lock()
	c.active++
	c.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { c.mu.Lock(); c.active--; c.signalLocked(); c.mu.Unlock() }) }
}

func (c *UpdateCoordinator) engineSnapshot() []*Engine {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*Engine(nil), c.engines...)
}
func (c *UpdateCoordinator) connectionsReady() bool {
	for _, e := range c.engineSnapshot() {
		if !PlatformsReady(e.PlatformStatuses()) {
			return false
		}
	}
	return true
}
func (c *UpdateCoordinator) recoverReceipts() {
	for _, e := range c.engineSnapshot() {
		e.RecoverUpdateResult()
	}
}

// A previously accepted restart takes priority over starting an update. Once an
// update owns the service, manual and deferred restarts cannot interrupt Apply.
func (c *UpdateCoordinator) reserveRestart() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.operation != nil {
		return false
	}
	c.restartPending = true
	return true
}
func (e *Engine) queueServiceRestart(req RestartRequest) bool {
	c := e.updates()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.operation != nil {
		return false
	}
	select {
	case RestartCh <- req:
		c.restartPending = true
		return true
	default:
		return false
	}
}
