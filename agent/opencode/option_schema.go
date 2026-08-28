package opencode

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	agent := &Agent{}
	options := core.ConfigurePermissionModeOption(core.DescribeAgentOptions(agent.KnownOptionKeys()), agent.PermissionModes())
	options = append(options, core.DescribeAgentOptions([]string{"provider"})...)
	core.RegisterAgentConfigOptions("opencode", options)
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
		"agent",
		"cc_data_dir",
		"cc_project",
		"cli_path",
		"cmd",
		"command",
		"env",
		"mode",
		"model",
		"work_dir",
	}
}
