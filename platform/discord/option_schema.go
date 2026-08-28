package discord

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("discord", core.DescribePlatformOptions([]string{
		"allow_from", "group_reply_all", "group_reply_all_guilds", "guild_id", "progress_style",
		"proxy", "proxy_password", "proxy_username", "respond_to_at_everyone_and_here",
		"share_session_in_channel", "thread_isolation", "token",
	}))
}
