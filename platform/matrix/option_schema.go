package matrix

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{
		"access_token", "allow_from", "auto_join", "auto_verify", "cc_data_dir", "cross_signing_password",
		"group_reply_all", "homeserver", "proxy", "share_session_in_channel", "user_id",
	})
	options = core.RequireConfigOptions(options, "homeserver", "access_token")
	options = core.RefineConfigOption(options, "cross_signing_password", func(option *core.ConfigOption) {
		option.Requires = []string{"MATRIX_CROSS_SIGNING_PASSWORD may be used instead"}
	})
	core.RegisterPlatformConfigOptions("matrix", options)
}
