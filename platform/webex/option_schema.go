package webex

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	options := core.DescribePlatformOptions([]string{"allow_from", "token"})
	options = core.RequireConfigOptions(options, "token")
	core.RegisterPlatformConfigOptions("webex", options)
}
