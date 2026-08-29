package discord

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "group_reply_all", "group_reply_all_guilds", "guild_id", "progress_style",
		"proxy", "proxy_password", "proxy_username", "respond_to_at_everyone_and_here",
		"share_session_in_channel", "thread_isolation", "token",
	})
	options = core.RequireConfigOptions(options, "token")
	options = core.ConfigureOption(options, "progress_style", "compact", "legacy", "compact", "card")
	options = core.RefineConfigOption(options, "group_reply_all_guilds", func(option *core.ConfigOption) {
		option.Type = "string"
		option.Default = "empty"
		option.DefaultSource = core.ConfigDefaultBuiltin
		option.Description = "Enable mention-free replies for a comma-separated list of Discord guild IDs; a non-empty list takes precedence over group_reply_all."
		option.DescriptionZH = "为逗号分隔的 Discord 服务器 ID 列表启用无需 @ 的回复；非空列表优先于 group_reply_all。"
		option.Example = `group_reply_all_guilds = "guild-a,guild-b"`
	})
	core.RegisterPlatformConfigOptions("discord", options)
}
