package wpsxiezuo

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("wps-xiezuo", core.DescribePlatformOptions([]string{
		"allow_from", "app_id", "app_secret", "base_url", "clean_reply",
	}))
}
