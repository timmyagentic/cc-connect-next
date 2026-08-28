package line

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "callback_path", "channel_secret", "channel_token", "port",
	})
	core.RegisterPlatformConfigOptions("line", core.ConfigureOption(options, "port", "8080"))
}
