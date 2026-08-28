package codex

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	agent := &Agent{}
	options := core.ConfigurePermissionModeOption(core.DescribeAgentOptions(agent.KnownOptionKeys()), agent.PermissionModes())
	for i := range options {
		if options[i].Key != "mode" {
			continue
		}
		options[i].Default = "suggest when omitted; generated starter config writes yolo"
		options[i].Description = "Choose the Codex approval and sandbox mode. Omitting the key keeps the suggest compatibility fallback; fresh generated configs explicitly set yolo."
		options[i].DescriptionZH = "选择 Codex 审批与沙箱模式。省略该键时保留 suggest 兼容回落；全新生成的配置会显式写入 yolo。"
	}
	options = core.ConfigureOption(options, "reasoning_effort", "unset / adapter default", "low", "medium", "high", "xhigh", "max")
	options = append(options, core.DescribeAgentOptions([]string{"provider"})...)
	core.RegisterAgentConfigOptions("codex", options)
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
		"app_server_url",
		"append_system_prompt",
		"backend",
		"cli_path",
		"cmd",
		"codex_home",
		"command",
		"env",
		"mode",
		"model",
		"model_context_window",
		"reasoning_effort",
		"session_title_model",
		"session_title_prefix",
		"service_tier",
		"system_prompt",
		"work_dir",
	}
}
