package codex

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	agent := &Agent{}
	options := core.ConfigurePermissionModeOption(core.DescribeAgentOptions(agent.KnownOptionKeys()), agent.PermissionModes())
	for i := range options {
		if options[i].Key != "mode" {
			continue
		}
		options[i].Default = "suggest"
		options[i].DefaultSource = core.ConfigDefaultAdapter
		options[i].Description = "Choose the Codex approval and sandbox mode. Omitting the key keeps the suggest compatibility fallback; fresh generated configs explicitly set yolo."
		options[i].DescriptionZH = "选择 Codex 审批与沙箱模式。省略该键时保留 suggest 兼容回落；全新生成的配置会显式写入 yolo。"
		options[i].PresetValues = []core.ConfigPresetValue{{Preset: "starter", Value: "yolo", Description: "Fresh generated configuration.", DescriptionZH: "全新生成配置。"}}
	}
	options = core.RefineConfigOption(options, "backend", func(option *core.ConfigOption) {
		option.PresetValues = []core.ConfigPresetValue{{Preset: "starter", Value: "app_server", Description: "Native steering and approval protocol.", DescriptionZH: "使用原生 steer 与审批协议。"}}
	})
	options = core.RefineConfigOption(options, "app_server_url", func(option *core.ConfigOption) {
		option.PresetValues = []core.ConfigPresetValue{{Preset: "starter", Value: "stdio", Description: "Launch a local app-server subprocess.", DescriptionZH: "启动本地 app-server 子进程。"}}
	})
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
