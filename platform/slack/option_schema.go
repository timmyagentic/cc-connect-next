package slack

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("slack", core.DescribePlatformOptions([]string{
		"allow_from", "app_token", "bot_token", "session_scope", "share_session_in_channel",
	}))
}
