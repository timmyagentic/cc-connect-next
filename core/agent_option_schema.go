package core

import (
	"fmt"
	"sort"
	"strings"
)

// AgentOptionSchema is an optional Agent capability: implementations declare
// the exhaustive set of [projects.agent.options] keys they consume. The
// declared surface powers the capability brief injected into the LLM's
// session context, so the model answers configuration questions from the
// real option set instead of inventing keys.
type AgentOptionSchema interface {
	KnownOptionKeys() []string
}

// BuildCapabilityBrief renders the configuration-capability primer injected
// once per session. It tells the model exactly which options exist for the
// active agent and what to do when a user asks for something outside them:
// say so plainly and point at /feedback — prevention at the source, instead
// of detecting misconfiguration after the fact.
func BuildCapabilityBrief(agentType string, keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	return fmt.Sprintf(
		"[cc-connect-next capability brief]\n"+
			"This conversation is bridged through cc-connect-next %s. The active agent adapter is %q; "+
			"the complete set of configurable options in config.toml under [projects.agent.options] is: `%s`. "+
			"There are no other agent options in this build. If the user asks to configure this bridge beyond these options, "+
			"tell them plainly that this deployment cannot do it, and that they can send `/feedback <description>` in this chat "+
			"to report the need directly to the project author (no GitHub account required).",
		CurrentVersion, agentType, strings.Join(sorted, "`, `"))
}
