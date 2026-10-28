package main

import (
	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/internal/privacyguard"
)

func configureMacOSPrivacy(cfg *config.Config) error {
	return privacyguard.Configure(privacyguard.Settings{
		Enabled:          cfg.MacOSPrivacy.Enabled == nil || *cfg.MacOSPrivacy.Enabled,
		AllowDirectories: cfg.MacOSPrivacy.AllowDirectories,
	})
}
