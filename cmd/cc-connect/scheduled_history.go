package main

import (
	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

func openScheduledHistory(cfg *config.Config) (*core.ScheduledHistory, error) {
	if !cfg.History.Enabled {
		return nil, nil
	}
	return core.NewScheduledHistory(cfg.DataDir)
}
