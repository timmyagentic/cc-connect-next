package weibo

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("weibo", core.DescribePlatformOptions([]string{
		"allow_from", "app_id", "app_secret", "name", "token_endpoint", "ws_endpoint",
	}))
}
