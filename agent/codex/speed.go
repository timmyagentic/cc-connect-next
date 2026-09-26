package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/timmyagentic/cc-connect-next/core"
)

type speedConfig struct {
	Model            string `toml:"model"`
	ServiceTier      string `toml:"service_tier"`
	ModelCatalogJSON string `toml:"model_catalog_json"`
	Profile          string `toml:"profile"`
	Profiles         map[string]struct {
		Model       string `toml:"model"`
		ServiceTier string `toml:"service_tier"`
	} `toml:"profiles"`
}

type catalogServiceTier struct {
	ID string `json:"id"`
}

// The live catalog also works for remote app-server connections and custom CLI
// profiles; neither should accidentally use metadata from the bridge machine.
func (s *appServerSession) ServiceTierCapabilities(model string) (core.ServiceTierCapabilities, error) {
	defaults := s.DefaultTurnOptions()
	if model == "" {
		model = defaults.Model
	}
	deadline := time.Now().Add(5 * time.Second)
	return readAppServerServiceTiers(model, defaults.ServiceTier, func(params map[string]any, out any) error {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("model catalog request timed out")
		}
		return s.requestWithTimeout("model/list", params, out, remaining)
	})
}

func readAppServerServiceTiers(model, tier string, request func(map[string]any, any) error) (core.ServiceTierCapabilities, error) {
	params := map[string]any{"includeHidden": true, "limit": 100}
	seen := map[string]bool{}
	for page := 0; page < 20; page++ {
		var response struct {
			Data []struct {
				ID                   string               `json:"id"`
				Model                string               `json:"model"`
				DefaultServiceTier   string               `json:"defaultServiceTier"`
				ServiceTiers         []catalogServiceTier `json:"serviceTiers"`
				AdditionalSpeedTiers []string             `json:"additionalSpeedTiers"`
			} `json:"data"`
			NextCursor string `json:"nextCursor"`
		}
		if err := request(params, &response); err != nil {
			return core.ServiceTierCapabilities{}, fmt.Errorf("read live Codex model catalog: %w", err)
		}
		for _, entry := range response.Data {
			if model != "" && (entry.Model == model || entry.ID == model) {
				return modelServiceTiers(model, tier, entry.DefaultServiceTier, entry.ServiceTiers, entry.AdditionalSpeedTiers), nil
			}
		}
		if response.NextCursor == "" || seen[response.NextCursor] {
			break
		}
		seen[response.NextCursor] = true
		params["cursor"] = response.NextCursor
	}
	return core.ServiceTierCapabilities{}, fmt.Errorf("model %q is absent from the live Codex model catalog", model)
}

// ServiceTierCapabilities uses the same native home as the subprocess, never
// the bridge user's unrelated default catalog. Missing capability evidence is
// an error: accepting an arbitrary tier would silently degrade on some models.
func (a *Agent) ServiceTierCapabilities(model string) (core.ServiceTierCapabilities, error) {
	a.mu.RLock()
	home, tier, remote := a.codexHome, a.serviceTier, a.appServerURL
	env := append([]string(nil), a.configEnv...)
	env = append(env, a.providerEnvLocked()...)
	env = append(env, a.sessionEnv...)
	extraArgs := append([]string(nil), a.cliExtraArgs...)
	a.mu.RUnlock()
	if remote != "" && remote != "stdio://" {
		return core.ServiceTierCapabilities{}, fmt.Errorf("remote app-server model catalog is not available locally")
	}
	// Custom CLI configuration may select a different profile or catalog.
	// Fail closed rather than checking capabilities against the wrong account.
	if len(extraArgs) > 0 {
		return core.ServiceTierCapabilities{}, fmt.Errorf("cannot verify the model catalog with custom Codex CLI arguments")
	}
	if home != "" {
		env = append(env, "CODEX_HOME="+home)
	}
	home, err := resolveCodexHome(env)
	if err != nil {
		return core.ServiceTierCapabilities{}, err
	}
	var cfg speedConfig
	if _, err := toml.DecodeFile(filepath.Join(home, "config.toml"), &cfg); err != nil && !os.IsNotExist(err) {
		return core.ServiceTierCapabilities{}, fmt.Errorf("read Codex speed configuration: %w", err)
	}
	if profile, ok := cfg.Profiles[cfg.Profile]; cfg.Profile != "" && ok {
		if profile.Model != "" {
			cfg.Model = profile.Model
		}
		if profile.ServiceTier != "" {
			cfg.ServiceTier = profile.ServiceTier
		}
	}
	if model == "" {
		model = a.GetModel()
	}
	if model == "" {
		model = cfg.Model
	}
	if model == "" {
		return core.ServiceTierCapabilities{}, fmt.Errorf("the current Codex model is unknown; start a conversation or configure a model first")
	}
	if tier == "" {
		tier = cfg.ServiceTier
	}
	catalog := cfg.ModelCatalogJSON
	if catalog == "" {
		catalog = filepath.Join(home, "models_cache.json")
	} else if strings.HasPrefix(catalog, "~/") {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return core.ServiceTierCapabilities{}, err
		}
		catalog = filepath.Join(userHome, catalog[2:])
	} else if !filepath.IsAbs(catalog) {
		catalog = filepath.Join(home, catalog)
	}
	data, err := os.ReadFile(catalog)
	if err != nil {
		return core.ServiceTierCapabilities{}, fmt.Errorf("Codex model catalog unavailable; refresh it with Codex CLI: %w", err)
	}
	return parseServiceTierCapabilities(data, model, tier)
}

func parseServiceTierCapabilities(data []byte, model, tier string) (core.ServiceTierCapabilities, error) {
	var catalog struct {
		Models []struct {
			Slug                 string               `json:"slug"`
			DefaultServiceTier   string               `json:"default_service_tier"`
			ServiceTiers         []catalogServiceTier `json:"service_tiers"`
			AdditionalSpeedTiers []string             `json:"additional_speed_tiers"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return core.ServiceTierCapabilities{}, fmt.Errorf("invalid Codex model catalog: %w", err)
	}
	for _, entry := range catalog.Models {
		if entry.Slug != model {
			continue
		}
		return modelServiceTiers(model, tier, entry.DefaultServiceTier, entry.ServiceTiers, entry.AdditionalSpeedTiers), nil
	}
	return core.ServiceTierCapabilities{}, fmt.Errorf("model %q is absent from the Codex model catalog", model)
}

func modelServiceTiers(model, tier, catalogDefault string, tiers []catalogServiceTier, legacy []string) core.ServiceTierCapabilities {
	if tier == "" {
		tier = catalogDefault
	}
	if tier == "" {
		tier = "default"
	}
	caps := core.ServiceTierCapabilities{Model: model, Default: tier,
		Options: []core.ServiceTierOption{{Name: "default", Value: "default"}}}
	seen := map[string]bool{"default": true}
	add := func(id string) {
		name := id
		if id == "priority" {
			name = "fast"
		}
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		caps.Options = append(caps.Options, core.ServiceTierOption{Name: name, Value: id})
	}
	if tiers != nil {
		for _, option := range tiers {
			add(option.ID)
		}
	} else {
		for _, option := range legacy {
			add(option)
		}
	}
	return caps
}
