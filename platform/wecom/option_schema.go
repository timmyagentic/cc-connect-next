package wecom

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"agent_id", "allow_from", "api_base_url", "bot_id", "bot_secret", "callback_aes_key",
		"callback_path", "callback_token", "corp_id", "corp_secret", "enable_markdown", "mode", "port",
		"proxy", "proxy_password", "proxy_username",
	})
	core.RegisterPlatformConfigOptions("wecom", core.ConfigureOption(options, "port", "8081"))
}
