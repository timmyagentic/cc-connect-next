package wpsxiezuo

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "app_id", "app_secret", "base_url", "clean_reply",
	})
	options = core.RequireConfigOptions(options, "app_id", "app_secret")
	options = core.ConfigureOption(options, "base_url", tokenEndpoint)
	core.RegisterPlatformConfigOptions("wps-xiezuo", options)
}
