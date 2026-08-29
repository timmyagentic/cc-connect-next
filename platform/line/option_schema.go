package line

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "callback_path", "channel_secret", "channel_token", "port",
	})
	options = core.RequireConfigOptions(options, "channel_secret", "channel_token")
	options = core.ConfigureOption(options, "port", "8080")
	options = core.ConfigureOption(options, "callback_path", "/callback")
	core.RegisterPlatformConfigOptions("line", options)
}
