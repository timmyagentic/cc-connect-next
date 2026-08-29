package dingtalk

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"agent_id", "allow_from", "card_template_id", "card_template_key", "card_throttle_ms",
		"client_id", "client_secret", "done_emoji", "reaction_emoji", "robot_code", "share_session_in_channel",
	})
	options = core.RequireConfigOptions(options, "client_id", "client_secret")
	options = core.RefineConfigOption(options, "agent_id", func(option *core.ConfigOption) {
		option.Type = "integer"
		option.Default = "0"
		option.DefaultSource = core.ConfigDefaultBuiltin
		option.Requirement = core.ConfigRequirementConditional
		option.RequiredWhen = []string{"proactive work notifications are used"}
		option.Example = `agent_id = 123456`
	})
	options = core.ConfigureOption(options, "robot_code", "client_id")
	options = core.ConfigureOption(options, "reaction_emoji", defaultReactionEmoji)
	options = core.ConfigureOption(options, "card_template_key", "content")
	options = core.ConfigureOption(options, "card_throttle_ms", "300")
	core.RegisterPlatformConfigOptions("dingtalk", options)
}
