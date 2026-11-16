package core

import "fmt"

func agentWithSessionBackend(agent Agent, backend string) (Agent, error) {
	selector, ok := agent.(SessionBackendProvider)
	if !ok {
		return nil, fmt.Errorf("agent %q does not support a task backend override", agent.Name())
	}
	return selector.WithSessionBackend(backend)
}
