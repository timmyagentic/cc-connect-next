package config

import (
	"fmt"

	"github.com/timmyagentic/cc-connect-next/core"
)

// OwnerEscalationConfig shares the runtime's types, defaults and validation.
// It is opt-in for each project and never inherits the admin allowlist.
type OwnerEscalationConfig = core.OwnerEscalationConfig

func validateOwnerEscalation(prefix string, cfg OwnerEscalationConfig) error {
	if _, err := core.NormalizeOwnerEscalationConfig(cfg); err != nil {
		return fmt.Errorf("config: %s.owner_escalation: %w", prefix, err)
	}
	return nil
}
