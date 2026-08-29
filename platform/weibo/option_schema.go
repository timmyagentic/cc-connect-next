package weibo

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "app_id", "app_secret", "name", "token_endpoint", "ws_endpoint",
	})
	options = core.RequireConfigOptions(options, "app_id", "app_secret")
	options = core.ConfigureOption(options, "name", "weibo")
	options = core.ConfigureOption(options, "token_endpoint", defaultTokenEndpoint)
	options = core.ConfigureOption(options, "ws_endpoint", defaultWSEndpoint)
	core.RegisterPlatformConfigOptions("weibo", options)
}
