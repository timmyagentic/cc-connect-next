package core

import "testing"

// The reply footer carries model + effort only (product decision 2026-08-21):
// no elapsed line, token counts, ctx %, or workdir. One composer serves every
// delivery path, including rich cards, which previously dropped the footer.

func TestComposeStatusFooter_ModelAndEffort(t *testing.T) {
	e := &Engine{}
	e.SetReplyFooterEnabled(true)
	session := &controllableAgentSession{model: "gpt-5.6-sol", reasoningEffort: "max"}

	if got := e.composeStatusFooter(nil, session); got != "gpt-5.6-sol · effort:max" {
		t.Errorf("composeStatusFooter = %q, want %q", got, "gpt-5.6-sol · effort:max")
	}
}

func TestComposeStatusFooter_ModelOnly(t *testing.T) {
	e := &Engine{}
	e.SetReplyFooterEnabled(true)
	session := &controllableAgentSession{model: "o3"}

	if got := e.composeStatusFooter(nil, session); got != "o3" {
		t.Errorf("composeStatusFooter = %q, want %q", got, "o3")
	}
}

func TestComposeStatusFooter_DisabledReturnsEmpty(t *testing.T) {
	e := &Engine{}
	session := &controllableAgentSession{model: "o3", reasoningEffort: "high"}

	if got := e.composeStatusFooter(nil, session); got != "" {
		t.Errorf("footer must be empty when reply_footer is off, got %q", got)
	}
}

func TestComposeStatusFooter_NoMetadataReturnsEmpty(t *testing.T) {
	e := &Engine{}
	e.SetReplyFooterEnabled(true)

	if got := e.composeStatusFooter(nil, &controllableAgentSession{}); got != "" {
		t.Errorf("footer with no model/effort must be empty, got %q", got)
	}
}
