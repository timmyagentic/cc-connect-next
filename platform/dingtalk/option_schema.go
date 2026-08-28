package dingtalk

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("dingtalk", core.DescribePlatformOptions([]string{
		"agent_id", "allow_from", "card_template_id", "card_template_key", "card_throttle_ms",
		"client_id", "client_secret", "done_emoji", "reaction_emoji", "robot_code", "share_session_in_channel",
	}))
}
