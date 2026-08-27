package core

const (
	defaultSkillPresetsURL  = "https://raw.githubusercontent.com/timmyagentic/cc-connect-next/main/skill-presets.json"
	fallbackSkillPresetsURL = "https://raw.githubusercontent.com/chenhg5/cc-connect/main/skill-presets.json"
)

// SkillPreset describes a recommended skill available from the remote presets list.
type SkillPreset struct {
	Name          string        `json:"name"`
	DisplayName   string        `json:"display_name"`
	Description   string        `json:"description,omitempty"`
	DescriptionZh string        `json:"description_zh,omitempty"`
	Version       string        `json:"version,omitempty"`
	Author        string        `json:"author,omitempty"`
	URL           string        `json:"url,omitempty"`
	AgentTypes    []string      `json:"agent_types,omitempty"`
	Tags          []string      `json:"tags,omitempty"`
	Featured      bool          `json:"featured,omitempty"`
	Source        *SkillSource  `json:"source,omitempty"`
	Pricing       *SkillPricing `json:"pricing,omitempty"`
}

// SkillSource describes where the skill is hosted / provided from.
type SkillSource struct {
	Provider string `json:"provider"`       // e.g. "github", "skills.sh", "npm"
	Name     string `json:"name,omitempty"` // display name, e.g. "GitHub", "Skills.sh"
	URL      string `json:"url,omitempty"`  // provider home page
}

// SkillPricing describes the pricing model for a skill.
type SkillPricing struct {
	Type     string  `json:"type"`               // "free", "paid", "freemium"
	Price    float64 `json:"price,omitempty"`    // 0 for free
	Currency string  `json:"currency,omitempty"` // "USD", "CNY", etc.
}

// SkillPresetsResponse is the top-level JSON schema for remote skill presets.
type SkillPresetsResponse struct {
	Version   int           `json:"version"`
	UpdatedAt string        `json:"updated_at,omitempty"`
	Skills    []SkillPreset `json:"skills"`
}

var globalSkillPresetsCache = newRemoteJSONCache[SkillPresetsResponse](remoteJSONCacheConfig{
	name:            "skill presets",
	defaultURL:      defaultSkillPresetsURL,
	fallbackURL:     fallbackSkillPresetsURL,
	ttl:             remotePresetsCacheTTL,
	timeout:         remotePresetsHTTPTimeout,
	fallbackTimeout: remotePresetsFallbackTimeout,
})

// SetSkillPresetsURL overrides the default skill presets URL.
func SetSkillPresetsURL(url string) {
	globalSkillPresetsCache.setURL(url)
}

// FetchSkillPresets returns cached or freshly-fetched skill presets.
func FetchSkillPresets() (*SkillPresetsResponse, error) {
	return globalSkillPresetsCache.fetch()
}
