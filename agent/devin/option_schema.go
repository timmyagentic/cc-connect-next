package devin

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribeAgentOptions((&Agent{}).KnownOptionKeys())
	options = core.ConfigureOption(options, "command", "devin")
	options = core.RefineConfigOption(options, "args", func(option *core.ConfigOption) {
		option.Default = `["acp"]`
		option.DefaultSource = core.ConfigDefaultBuiltin
		option.Example = `args = ["acp"]`
	})
	options = core.ConfigureOption(options, "display_name", "Devin")
	core.RegisterAgentConfigOptions("devin", options)
}

// KnownOptionKeys implements core.AgentOptionSchema: the exhaustive set of
// [projects.agent.options] keys this agent consumes (directly or via the
// shared core.ParseCmdOpts / core.ParseConfigEnv helpers). Keys configured
// but absent from this list are surfaced through the feedback channel's
// capability-gap prompt instead of being silently ignored.
//
// KEEP IN SYNC when adding an option read anywhere in this package.
func (a *Agent) KnownOptionKeys() []string {
	return []string{
		"args",
		"command",
		"display_name",
	}
}
