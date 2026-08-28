package qqbot

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("qqbot", core.DescribePlatformOptions([]string{
		"allow_from", "app_id", "app_secret", "cc_data_dir", "intents", "markdown_support", "sandbox", "share_session_in_channel",
	}))
}
