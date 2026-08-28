package qq

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("qq", core.DescribePlatformOptions([]string{
		"allow_from", "http_url", "share_session_in_channel", "token", "ws_url",
	}))
}
