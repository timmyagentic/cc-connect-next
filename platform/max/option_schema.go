package max

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"allow_from", "api_base", "token", "webhook_listen", "webhook_path",
		"webhook_resubscribe_interval", "webhook_secret", "webhook_url",
	})
	options = core.RequireConfigOptions(options, "token")
	options = core.ConfigureOption(options, "api_base", defaultAPIBase)
	options = core.ConfigureOption(options, "webhook_path", "/webhook")
	options = core.RefineConfigOption(options, "webhook_listen", func(option *core.ConfigOption) {
		option.Default = "unset"
		option.DefaultSource = core.ConfigDefaultRuntime
		option.Description = "Set the local webhook listen address; when webhook_url is set and this option is omitted, runtime uses :8080."
		option.DescriptionZH = "设置本地 Webhook 监听地址；设置 webhook_url 且省略本项时，运行时使用 :8080。"
	})
	core.RegisterPlatformConfigOptions("max", options)
}
