package webex

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("webex", core.DescribePlatformOptions([]string{"allow_from", "token"}))
}
