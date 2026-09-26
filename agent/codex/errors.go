package codex

import (
	"errors"
	"strings"

	"github.com/timmyagentic/cc-connect-next/core"
)

// classifyCodexError keeps provider-specific error recognition next to the
// Codex transport. The core engine only consumes the provider-neutral marker.
func classifyCodexError(err error) error {
	if err == nil || isCodexClassifiedTerminalError(err) {
		return err
	}
	if isCodexAuthenticationRequiredMessage(err.Error()) {
		return core.WrapAuthenticationRequired(err)
	}
	if isCodexUsageLimitMessage(err.Error()) {
		return core.WrapUsageLimit(err)
	}
	if isCodexModelCapacityMessage(err.Error()) {
		return core.WrapModelCapacity(err)
	}
	return err
}

func isCodexClassifiedTerminalError(err error) bool {
	return errors.Is(err, core.ErrUsageLimit) || errors.Is(err, core.ErrModelCapacity) || errors.Is(err, core.ErrAuthenticationRequired)
}

func isCodexAuthenticationRequiredMessage(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	for _, marker := range []string{
		"refresh_token_reused", "refresh_token_expired", "refresh_token_invalidated",
		"please run `codex login`", "please run codex login",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	// A failed refresh can also be a temporary network error. Require explicit
	// evidence of a lost login; a generic 401 or API-key error is not enough.
	if !strings.Contains(text, "access token") && !strings.Contains(text, "refresh token") {
		return false
	}
	return strings.Contains(text, "sign in again") || strings.Contains(text, "log in again") ||
		strings.Contains(text, "logged out") || strings.Contains(text, "signed in to another account")
}

func isCodexModelCapacityMessage(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"selected model is at capacity",
		"selected model is currently at capacity",
		"the model is at capacity",
		"the model is currently at capacity",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func isCodexUsageLimitMessage(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}

	for _, marker := range []string{
		"usage limit",
		"usage_limit",
		"you've hit your limit",
		"you have hit your limit",
		"you've reached your limit",
		"you have reached your limit",
		"quota exceeded",
		"quota exhausted",
		"rate limit exceeded",
		"rate limit reached",
		"out of credits",
		"credits exhausted",
		"credit limit",
		"insufficient credits",
		"remaining credits",
		"out of tokens",
		"tokens exhausted",
		"no tokens left",
		"token limit",
		"token quota",
		"token usage limit",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}

	// Cover messages such as "no available usage remaining" and "0 tokens
	// remaining" without treating an unrelated token/path mention as quota.
	hasRemaining := strings.Contains(text, "remaining")
	hasUsageTerm := strings.Contains(text, "token") ||
		strings.Contains(text, "credit") ||
		strings.Contains(text, "usage") ||
		strings.Contains(text, "quota")
	if hasRemaining && hasUsageTerm {
		return strings.Contains(text, "no ") || strings.Contains(text, "0 ")
	}
	return false
}
