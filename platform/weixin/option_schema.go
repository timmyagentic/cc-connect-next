package weixin

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"account_id", "allow_from", "base_url", "burst_limit", "burst_window_secs", "cc_data_dir", "cc_project",
		"cdn_base_url", "long_poll_timeout_ms", "proxy", "proxy_password", "proxy_username", "route_tag", "state_dir", "token",
	})
	options = core.RequireConfigOptions(options, "token")
	options = core.ConfigureOption(options, "base_url", defaultBaseURL)
	options = core.ConfigureOption(options, "cdn_base_url", defaultCDNBaseURL)
	core.RegisterPlatformConfigOptions("weixin", options)
}
