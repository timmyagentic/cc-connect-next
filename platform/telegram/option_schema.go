package telegram

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "enable_reactions", "group_reply_all", "progress_style", "proxy",
		"proxy_password", "proxy_username", "share_session_in_channel", "token",
	})
	options = core.RequireConfigOptions(options, "token")
	options = core.ConfigureOption(options, "progress_style", "compact", "legacy", "compact", "card")
	core.RegisterPlatformConfigOptions("telegram", options)
}
