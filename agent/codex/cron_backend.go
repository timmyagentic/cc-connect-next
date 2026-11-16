package codex

import (
	"context"
	"fmt"

	"github.com/timmyagentic/cc-connect-next/core"
)

// backendAgent preserves the base Agent's optional capabilities while selecting
// a transport only for sessions started by this scheduled run.
type backendAgent struct {
	*Agent
	backendOverride string
}

func (a *Agent) WithSessionBackend(backend string) (core.Agent, error) {
	switch backend {
	case "exec", "app_server":
		return &backendAgent{Agent: a, backendOverride: backend}, nil
	default:
		return nil, fmt.Errorf("unsupported Codex task backend %q (want exec or app_server)", backend)
	}
}

func (a *backendAgent) StartSession(ctx context.Context, sessionID string) (core.AgentSession, error) {
	return a.Agent.startSession(ctx, sessionID, a.backendOverride)
}
