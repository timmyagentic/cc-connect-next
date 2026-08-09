package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type plusIdentityMode string

const (
	plusIdentityLegacy     plusIdentityMode = "legacy"
	plusIdentityFailClosed plusIdentityMode = "fail_closed"
	plusIdentityRetry      plusIdentityMode = "retry"

	defaultIdentityRetryInitial = 5 * time.Second
	defaultIdentityRetryMax     = 5 * time.Minute
)

// plusConfig contains independently maintained Feishu Plus behavior. Keeping
// this separate from the native platform fields makes the compatibility
// boundary explicit: when Plus is disabled, cc-connect follows the original
// code path and defaults.
type plusConfig struct {
	enabled      bool
	identityMode plusIdentityMode
}

func parsePlusConfig(opts map[string]any) (plusConfig, error) {
	cfg := plusConfig{identityMode: plusIdentityLegacy}
	if raw, ok := opts["plus_enabled"]; ok {
		enabled, ok := raw.(bool)
		if !ok {
			return plusConfig{}, fmt.Errorf("feishu: plus_enabled must be a boolean, got %T", raw)
		}
		cfg.enabled = enabled
	}

	rawMode, hasMode := opts["plus_identity_mode"]
	if hasMode && !cfg.enabled {
		return plusConfig{}, fmt.Errorf("feishu: plus_identity_mode requires plus_enabled=true")
	}
	if !cfg.enabled {
		return cfg, nil
	}

	// The Plus default is safe and self-healing: group messages requiring an
	// explicit bot mention are blocked while identity is unknown, while a
	// background retry restores normal routing without a daemon restart.
	cfg.identityMode = plusIdentityRetry
	if !hasMode {
		return cfg, nil
	}
	mode, ok := rawMode.(string)
	if !ok {
		return plusConfig{}, fmt.Errorf("feishu: plus_identity_mode must be a string, got %T", rawMode)
	}
	switch plusIdentityMode(strings.ToLower(strings.TrimSpace(mode))) {
	case plusIdentityLegacy:
		cfg.identityMode = plusIdentityLegacy
	case plusIdentityFailClosed:
		cfg.identityMode = plusIdentityFailClosed
	case plusIdentityRetry:
		cfg.identityMode = plusIdentityRetry
	default:
		return plusConfig{}, fmt.Errorf(
			"feishu: invalid plus_identity_mode %q (want legacy, fail_closed, or retry)", mode,
		)
	}
	return cfg, nil
}

func (p *Platform) shouldDropForUnknownBotIdentity(chatType string) bool {
	if chatType != "group" || p.groupReplyAll || p.shouldUseWebhookMode() {
		return false
	}
	if !p.plus.enabled || p.plus.identityMode == plusIdentityLegacy {
		return false
	}
	return p.getBotOpenID() == ""
}

func (p *Platform) fetchBotIdentity() (string, error) {
	var (
		openID string
		err    error
	)
	if p.fetchBotOpenIDOverride != nil {
		openID, err = p.fetchBotOpenIDOverride()
	} else {
		openID, err = p.fetchBotOpenID()
	}
	if err != nil {
		return "", err
	}
	if p.plus.enabled && strings.TrimSpace(openID) == "" {
		return "", fmt.Errorf("bot info returned an empty open_id")
	}
	return openID, nil
}

func (p *Platform) setBotOpenID(openID string) {
	p.mu.Lock()
	p.botOpenID = openID
	p.mu.Unlock()
}

func (p *Platform) startBotIdentityRetry() {
	if !p.plus.enabled || p.plus.identityMode != plusIdentityRetry {
		return
	}

	p.identityRetryMu.Lock()
	if p.identityRetryCancel != nil {
		p.identityRetryMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.identityRetryCancel = cancel
	initial := p.identityRetryInitial
	if initial <= 0 {
		initial = defaultIdentityRetryInitial
	}
	maximum := p.identityRetryMax
	if maximum <= 0 {
		maximum = defaultIdentityRetryMax
	}
	if maximum < initial {
		maximum = initial
	}
	p.identityRetryMu.Unlock()

	go p.runBotIdentityRetry(ctx, initial, maximum)
}

func (p *Platform) runBotIdentityRetry(ctx context.Context, delay, maximum time.Duration) {
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}

		openID, err := p.fetchBotIdentity()
		if err == nil {
			p.setBotOpenID(openID)
			slog.Info(p.platformName+": Feishu Plus recovered bot identity; group mention filtering restored",
				"open_id", openID)
			return
		}

		next := delay * 2
		if next <= 0 || next > maximum {
			next = maximum
		}
		slog.Warn(p.platformName+": Feishu Plus bot identity retry failed; group messages remain blocked",
			"error", err, "retry_in", next)
		delay = next
	}
}

func (p *Platform) stopBotIdentityRetry() {
	p.identityRetryMu.Lock()
	cancel := p.identityRetryCancel
	p.identityRetryCancel = nil
	p.identityRetryMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
