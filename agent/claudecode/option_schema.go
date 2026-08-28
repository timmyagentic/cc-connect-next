package claudecode

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	agent := &Agent{}
	options := core.ConfigurePermissionModeOption(core.DescribeAgentOptions(agent.KnownOptionKeys()), agent.PermissionModes())
	options = core.ConfigureOption(options, "reasoning_effort", "unset / adapter default", "low", "medium", "high", "max")
	options = append(options, core.DescribeAgentOptions([]string{"provider"})...)
	core.RegisterAgentConfigOptions("claudecode", options)
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
		"allowed_tools",
		"append_system_prompt",
		"cc_data_dir",
		"cli_args_flag",
		"cli_path",
		"cmd",
		"cmd_args_flag",
		"command",
		"disallowed_tools",
		"env",
		"max_context_tokens",
		"mode",
		"model",
		"plugin_dir",
		"reasoning_effort",
		"router_api_key",
		"router_url",
		"run_as_env",
		"run_as_user",
		"system_prompt",
		"work_dir",
	}
}
