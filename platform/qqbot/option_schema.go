package qqbot

import (
	"strconv"

	"github.com/timmyagentic/cc-connect-next/core"
)

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "app_id", "app_secret", "cc_data_dir", "intents", "markdown_support", "sandbox", "share_session_in_channel",
	})
	options = core.RequireConfigOptions(options, "app_id", "app_secret")
	options = core.ConfigureOption(options, "intents", strconv.Itoa(defaultIntents))
	options = core.ConfigureOption(options, "sandbox", "false")
	options = core.ConfigureOption(options, "markdown_support", "false")
	core.RegisterPlatformConfigOptions("qqbot", options)
}
