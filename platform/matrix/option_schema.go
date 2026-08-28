package matrix

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("matrix", core.DescribePlatformOptions([]string{
		"access_token", "allow_from", "auto_join", "auto_verify", "cc_data_dir", "cross_signing_password",
		"group_reply_all", "homeserver", "proxy", "share_session_in_channel", "user_id",
	}))
}
