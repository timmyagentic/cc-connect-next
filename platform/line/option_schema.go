package line

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("line", core.DescribePlatformOptions([]string{
		"allow_from", "callback_path", "channel_secret", "channel_token", "port",
	}))
}
