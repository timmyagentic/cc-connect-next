package core

import (
	"sort"
	"strings"
)

// AgentOptionSchema is an optional Agent capability: implementations declare
// the exhaustive set of [projects.agent.options] keys they consume. The
// options table is free-form (map[string]any), so TOML-level unknown-key
// detection cannot see inside it — an unsupported key like a service_tier
// on a build that predates it would otherwise be silently ignored. Detected
// gaps feed the feedback channel's capability-gap prompt.
type AgentOptionSchema interface {
	KnownOptionKeys() []string
}

// bootstrapConsumedOptionKeys are option keys read by the bootstrap layer
// (cmd/cc-connect) for every agent type rather than by the agent itself.
var bootstrapConsumedOptionKeys = map[string]bool{
	"provider": true,
}

// UnknownAgentOptionKeys returns the configured option keys that neither the
// agent's declared schema nor the bootstrap layer consumes, sorted. Agents
// that do not implement AgentOptionSchema yield nil — no declaration means
// no claim, not "everything is unknown".
func UnknownAgentOptionKeys(configuredKeys []string, agent Agent) []string {
	schema, ok := agent.(AgentOptionSchema)
	if !ok {
		return nil
	}
	known := make(map[string]bool)
	for _, k := range schema.KnownOptionKeys() {
		known[strings.ToLower(strings.TrimSpace(k))] = true
	}
	var unknown []string
	for _, k := range configuredKeys {
		norm := strings.ToLower(strings.TrimSpace(k))
		if norm == "" || known[norm] || bootstrapConsumedOptionKeys[norm] {
			continue
		}
		unknown = append(unknown, k)
	}
	sort.Strings(unknown)
	return unknown
}
