package codex

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
		"reasoning_effort",
		"service_tier",
		"system_prompt",
		"work_dir",
	}
}
