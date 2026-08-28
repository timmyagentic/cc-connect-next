package main

import (
	"fmt"
	"sort"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

// annotateUnknownPluginConfigKeys closes the map[string]any blind spot in the
// TOML decoder: typed config typos already appear in md.Undecoded(), but Agent
// and Platform option maps consume every key. Built-in adapters now declare
// their exact surfaces, so unsupported or misspelled dynamic keys can use the
// same startup warning and /feedback capability-gap path without changing
// config compatibility or refusing startup.
func annotateUnknownPluginConfigKeys(cfg *config.Config) {
	if cfg == nil {
		return
	}
	seen := make(map[string]bool, len(cfg.UnknownConfigKeys))
	for _, key := range cfg.UnknownConfigKeys {
		seen[key] = true
	}
	appendUnknown := func(path string) {
		if !seen[path] {
			cfg.UnknownConfigKeys = append(cfg.UnknownConfigKeys, path)
			seen[path] = true
		}
	}
	for projectIndex, project := range cfg.Projects {
		if options := core.AgentConfigOptions(project.Agent.Type); len(options) > 0 {
			known := publicOptionSet(options)
			for key := range project.Agent.Options {
				if !known[key] {
					appendUnknown(fmt.Sprintf("projects[%d].agent.options.%s", projectIndex, key))
				}
			}
		}
		for platformIndex, platform := range project.Platforms {
			if options := core.PlatformConfigOptions(platform.Type); len(options) > 0 {
				known := publicOptionSet(options)
				for key := range platform.Options {
					if !known[key] {
						appendUnknown(fmt.Sprintf("projects[%d].platforms[%d].options.%s", projectIndex, platformIndex, key))
					}
				}
			}
		}
	}
	sort.Strings(cfg.UnknownConfigKeys)
}

func publicOptionSet(options []core.ConfigOption) map[string]bool {
	set := make(map[string]bool, len(options))
	for _, option := range options {
		if !option.Internal {
			set[option.Key] = true
		}
	}
	return set
}
