package acp

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	agent := &Agent{}
	options := core.ConfigurePermissionModeOption(core.DescribeAgentOptions(agent.KnownOptionKeys()), agent.PermissionModes())
	for _, key := range []string{"cmd", "cli_path", "command"} {
		options = core.RefineConfigOption(options, key, func(option *core.ConfigOption) {
			option.Requirement = core.ConfigRequirementConditional
			option.RequiredWhen = []string{"one of cmd, cli_path, or command must be set"}
		})
	}
	options = core.ConfigureOption(options, "display_name", "ACP")
	core.RegisterAgentConfigOptions("acp", options)
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
		"auth_method",
		"cli_path",
		"cmd",
		"command",
		"display_name",
		"env",
		"mode",
		"work_dir",
	}
}
