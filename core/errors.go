package core

import (
	"errors"
	"fmt"
)

// ErrUsageLimit marks an agent error caused by an exhausted provider usage
// allowance. The underlying error remains available for logs, while callers
// can render a safe, user-facing quota message without exposing it.
var ErrUsageLimit = errors.New("agent usage limit reached")

// Steer failure sentinels. SteerableSession implementations wrap these so the
// engine can pick a deterministic fallback without inspecting error text.
//
// The distinction that matters for safety is "definitive non-acceptance"
// versus "outcome unknown": a definitively rejected steer never delivered the
// user's input, so falling back to the FIFO queue cannot duplicate it. An
// unknown outcome (timeout after the request may have been written) must NOT
// be re-queued automatically, because the input may already be inside the
// active turn.
var (
	// ErrSteerUnsupported: the agent backend cannot steer at all
	// (e.g. the Codex exec backend, where every Send is a separate process).
	ErrSteerUnsupported = errors.New("steering not supported by this agent backend")
	// ErrSteerNoActiveTurn: there is definitively no in-flight turn to steer.
	ErrSteerNoActiveTurn = errors.New("no active turn to steer")
	// ErrSteerTurnMismatch: the active turn changed before the steer was
	// accepted (expectedTurnId precondition failed). Definitive rejection.
	ErrSteerTurnMismatch = errors.New("active turn changed before steer was accepted")
	// ErrSteerRejected: the backend definitively rejected the steer request
	// for another reason; the input was not delivered.
	ErrSteerRejected = errors.New("steer request rejected")
	// ErrSteerOutcomeUnknown: the steer may or may not have been accepted
	// (timeout / transport failure after write). Never re-queue on this.
	ErrSteerOutcomeUnknown = errors.New("steer outcome unknown")
)

// SteerFailureAllowsQueueFallback reports whether a failed Steer call is a
// definitive non-acceptance, making it safe to fall back to queueing the same
// message without risking duplicate delivery.
func SteerFailureAllowsQueueFallback(err error) bool {
	return errors.Is(err, ErrSteerUnsupported) ||
		errors.Is(err, ErrSteerNoActiveTurn) ||
		errors.Is(err, ErrSteerTurnMismatch) ||
		errors.Is(err, ErrSteerRejected)
}

// WrapUsageLimit preserves the provider error for diagnostics and marks it as
// a usage-limit failure for user-facing rendering.
func WrapUsageLimit(err error) error {
	if err == nil {
		return ErrUsageLimit
	}
	if errors.Is(err, ErrUsageLimit) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrUsageLimit, err)
}
