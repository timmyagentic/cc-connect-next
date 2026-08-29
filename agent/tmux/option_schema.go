package tmux

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribeAgentOptions((&Agent{}).KnownOptionKeys())
	options = core.RefineConfigOption(options, "session", func(option *core.ConfigOption) {
		option.Example = `session = "agent-session"`
	})
	options = core.ConfigureOption(options, "prompt_pattern", `[❯\$#>%]\s*$`)
	options = core.RefineConfigOption(options, "strip_patterns", func(option *core.ConfigOption) {
		option.Default = "built-in Claude mode-status pattern"
		option.DefaultSource = core.ConfigDefaultBuiltin
		option.Example = `strip_patterns = ["^status:"]`
	})
	core.RegisterAgentConfigOptions("tmux", options)
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
		"auto_create",
		"init_command",
		"pane",
		"poll_interval_ms",
		"prompt_pattern",
		"session",
		"shell",
		"startup_wait_ms",
		"strip_input_block",
		"strip_patterns",
		"window_per_session",
		"work_dir",
	}
}
