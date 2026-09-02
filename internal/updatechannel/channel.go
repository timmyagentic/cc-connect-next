// Package updatechannel defines the process-wide release channel contract.
// It is intentionally independent of config, UI, and updater transports so
// every surface uses the same canonical values and defaults.
package updatechannel

import "strings"

type Channel string

const (
	Stable Channel = "stable"
	Beta   Channel = "beta"
)

// Parse canonicalizes a configured or command-line channel. Omission keeps
// the compatibility default: Stable.
func Parse(raw string) (Channel, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(Stable):
		return Stable, true
	case string(Beta):
		return Beta, true
	default:
		return "", false
	}
}

func (channel Channel) Effective() Channel {
	if parsed, ok := Parse(string(channel)); ok {
		return parsed
	}
	return Stable
}

// ReleaseType describes release metadata, not the configured channel. This
// prevents a prerelease discovered through any surface from being called a
// stable release merely because Stable is the default channel.
func (channel Channel) ReleaseType(prerelease bool) string {
	if prerelease {
		return "prerelease"
	}
	return "stable"
}
