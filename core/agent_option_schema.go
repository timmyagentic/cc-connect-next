package core

import (
	"fmt"
	"sort"
	"strings"
)

// AgentOptionSchema is the adapter-local compatibility surface: implementations
// declare the [projects.agent.options] keys consumed by their constructor or
// shared parsing helpers. Compiled adapters derive their richer ConfigOption
// catalog from this list and may add host-level capabilities such as provider
// selection separately.
type AgentOptionSchema interface {
	KnownOptionKeys() []string
}

// BuildCapabilityBrief renders the legacy key-only primer.
// Deprecated: runtime sessions use BuildConfigurationCapabilityBrief, which
// includes typed/global/project/platform metadata and the local lookup contract.
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
