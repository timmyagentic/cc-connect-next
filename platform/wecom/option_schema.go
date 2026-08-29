package wecom

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"agent_id", "allow_from", "api_base_url", "bot_id", "bot_secret", "callback_aes_key",
		"callback_path", "callback_token", "corp_id", "corp_secret", "enable_markdown", "mode", "port",
		"proxy", "proxy_password", "proxy_username",
	})
	options = core.ConfigureOption(options, "mode", "callback", "callback", "websocket")
	for _, key := range []string{"corp_id", "corp_secret", "agent_id", "callback_token", "callback_aes_key"} {
		options = core.ConfigureConditionalOption(options, key, "mode is unset or mode = callback")
	}
	for _, key := range []string{"bot_id", "bot_secret"} {
		options = core.ConfigureConditionalOption(options, key, "mode = websocket")
	}
	options = core.ConfigureOption(options, "port", "8081")
	options = core.ConfigureOption(options, "callback_path", "/wecom/callback")
	options = core.ConfigureOption(options, "api_base_url", defaultAPIBaseURL)
	options = core.ConfigureOption(options, "enable_markdown", "false")
	core.RegisterPlatformConfigOptions("wecom", options)
}
