package slack

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "app_token", "bot_token", "session_scope", "share_session_in_channel",
	})
	options = core.RequireConfigOptions(options, "bot_token", "app_token")
	core.RegisterPlatformConfigOptions("slack", options)
}
