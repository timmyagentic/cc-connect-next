package core

import (
	"testing"
	"time"
)

// The reply footer carries model + effort + elapsed processing time. One
// composer serves every delivery path, including rich cards.

func TestComposeStatusFooter_ModelEffortAndElapsed(t *testing.T) {
	e := &Engine{}
	e.SetReplyFooterEnabled(true)
	session := &controllableAgentSession{model: "gpt-5.6-sol", reasoningEffort: "max"}

	if got := e.composeStatusFooter(nil, session, 12*time.Second+340*time.Millisecond); got != "gpt-5.6-sol · effort:max · ⏱ 12.3s" {
		t.Errorf("composeStatusFooter = %q, want %q", got, "gpt-5.6-sol · effort:max · ⏱ 12.3s")
	}
}

func TestComposeStatusFooter_ModelAndElapsed(t *testing.T) {
	e := &Engine{}
	e.SetReplyFooterEnabled(true)
	session := &controllableAgentSession{model: "o3"}

	if got := e.composeStatusFooter(nil, session, time.Minute+23*time.Second); got != "o3 · ⏱ 1m 23s" {
		t.Errorf("composeStatusFooter = %q, want %q", got, "o3 · ⏱ 1m 23s")
	}
}

func TestComposeStatusFooter_DisabledReturnsEmpty(t *testing.T) {
	e := &Engine{}
	session := &controllableAgentSession{model: "o3", reasoningEffort: "high"}

	if got := e.composeStatusFooter(nil, session, 5*time.Second); got != "" {
		t.Errorf("footer must be empty when reply_footer is off, got %q", got)
	}
}

func TestComposeStatusFooter_NoMetadataStillShowsElapsed(t *testing.T) {
	e := &Engine{}
	e.SetReplyFooterEnabled(true)

	if got := e.composeStatusFooter(nil, &controllableAgentSession{}, 250*time.Millisecond); got != "⏱ <1s" {
		t.Errorf("footer with no model/effort = %q, want elapsed-only footer", got)
	}
}

func TestFormatReplyFooterDuration(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "negative", in: -time.Second, want: "<1s"},
		{name: "sub_second", in: 999 * time.Millisecond, want: "<1s"},
		{name: "seconds", in: 12*time.Second + 340*time.Millisecond, want: "12.3s"},
		{name: "minutes", in: time.Minute + 23*time.Second, want: "1m 23s"},
		{name: "hours", in: 2*time.Hour + 5*time.Minute + 40*time.Second, want: "2h 06m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatReplyFooterDuration(tt.in); got != tt.want {
				t.Errorf("formatReplyFooterDuration(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
