package max

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("max", core.DescribePlatformOptions([]string{
		"allow_from", "api_base", "token", "webhook_listen", "webhook_path",
		"webhook_resubscribe_interval", "webhook_secret", "webhook_url",
	}))
}
