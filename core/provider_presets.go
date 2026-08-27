package core

const (
	defaultPresetsURL  = "https://raw.githubusercontent.com/timmyagentic/cc-connect-next/main/provider-presets.json"
	fallbackPresetsURL = "https://raw.githubusercontent.com/chenhg5/cc-connect/main/provider-presets.json"
)

// ProviderPreset describes a recommended provider available from the remote presets list.
type ProviderPreset struct {
	Name          string                       `json:"name"`
	DisplayName   string                       `json:"display_name"`
	Agents        map[string]PresetAgentConfig `json:"agents"` // per-agent-type configuration (keys: "claudecode", "codex", "gemini", "opencode", ...)
	InviteURL     string                       `json:"invite_url,omitempty"`
	Description   string                       `json:"description,omitempty"`
	DescriptionZh string                       `json:"description_zh,omitempty"`
	Features      []string                     `json:"features,omitempty"`
	Thinking      string                       `json:"thinking,omitempty"`
	Tier          int                          `json:"tier"`
	Featured      bool                         `json:"featured,omitempty"`
	Website       string                       `json:"website,omitempty"`
}

// PresetAgentConfig holds per-agent-type settings within a provider preset.
type PresetAgentConfig struct {
	BaseURL     string             `json:"base_url"`
	Model       string             `json:"model"`
	Models      []string           `json:"models,omitempty"`
	CodexConfig *PresetCodexConfig `json:"codex_config,omitempty"`
}

// PresetCodexConfig holds Codex-specific provider settings that get written
// to Codex's config.toml as [model_providers.<name>].
type PresetCodexConfig struct {
	EnvKey      string            `json:"env_key,omitempty"`
	WireAPI     string            `json:"wire_api,omitempty"`
	HTTPHeaders map[string]string `json:"http_headers,omitempty"`
}

// SupportsAgent returns true if the preset supports the given agent type.
func (p *ProviderPreset) SupportsAgent(agentType string) bool {
	_, ok := p.Agents[agentType]
	return ok
}

// AgentConfig returns the agent-specific config, or nil if unsupported.
func (p *ProviderPreset) AgentConfig(agentType string) *PresetAgentConfig {
	ac, ok := p.Agents[agentType]
	if !ok {
		return nil
	}
	return &ac
}

// ProviderPresetsResponse is the top-level JSON schema for remote presets.
type ProviderPresetsResponse struct {
	Version   int              `json:"version"`
	UpdatedAt string           `json:"updated_at,omitempty"`
	Providers []ProviderPreset `json:"providers"`
}

var globalPresetsCache = newRemoteJSONCache[ProviderPresetsResponse](remoteJSONCacheConfig{
	name:            "provider presets",
	defaultURL:      defaultPresetsURL,
	fallbackURL:     fallbackPresetsURL,
	ttl:             remotePresetsCacheTTL,
	timeout:         remotePresetsHTTPTimeout,
	fallbackTimeout: remotePresetsFallbackTimeout,
})

// SetPresetsURL overrides the default presets URL. Call before first fetch.
func SetPresetsURL(url string) {
	globalPresetsCache.setURL(url)
}

// FetchProviderPresets returns cached or freshly-fetched provider presets.
func FetchProviderPresets() (*ProviderPresetsResponse, error) {
	return globalPresetsCache.fetch()
}
