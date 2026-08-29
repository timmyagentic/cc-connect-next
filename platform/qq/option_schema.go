package qq

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "http_url", "share_session_in_channel", "token", "ws_url",
	})
	options = core.ConfigureOption(options, "ws_url", "ws://127.0.0.1:3001")
	core.RegisterPlatformConfigOptions("qq", options)
}
