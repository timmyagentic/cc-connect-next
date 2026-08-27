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

	copy := replyFooterDurationCopyForLanguage(LangEnglish)
	if got := e.composeStatusFooter(nil, session, 12*time.Second+340*time.Millisecond, copy); got != "gpt-5.6-sol · effort:max · ⏱ 12.3s" {
		t.Errorf("composeStatusFooter = %q, want %q", got, "gpt-5.6-sol · effort:max · ⏱ 12.3s")
	}
}

func TestComposeStatusFooter_ModelAndElapsed(t *testing.T) {
	e := &Engine{}
	e.SetReplyFooterEnabled(true)
	session := &controllableAgentSession{model: "o3"}

	copy := replyFooterDurationCopyForLanguage(LangEnglish)
	if got := e.composeStatusFooter(nil, session, time.Minute+23*time.Second, copy); got != "o3 · ⏱ 1m 23s" {
		t.Errorf("composeStatusFooter = %q, want %q", got, "o3 · ⏱ 1m 23s")
	}
}

func TestComposeStatusFooter_DisabledReturnsEmpty(t *testing.T) {
	e := &Engine{}
	session := &controllableAgentSession{model: "o3", reasoningEffort: "high"}

	copy := replyFooterDurationCopyForLanguage(LangEnglish)
	if got := e.composeStatusFooter(nil, session, 5*time.Second, copy); got != "" {
		t.Errorf("footer must be empty when reply_footer is off, got %q", got)
	}
}

func TestComposeStatusFooter_NoMetadataStillShowsElapsed(t *testing.T) {
	e := &Engine{}
	e.SetReplyFooterEnabled(true)

	copy := replyFooterDurationCopyForLanguage(LangEnglish)
	if got := e.composeStatusFooter(nil, &controllableAgentSession{}, 250*time.Millisecond, copy); got != "⏱ <1s" {
		t.Errorf("footer with no model/effort = %q, want elapsed-only footer", got)
	}
}

func TestFormatReplyFooterDuration_Localized(t *testing.T) {
	tests := []struct {
		name string
		lang Language
		in   time.Duration
		want string
	}{
		{name: "english_sub_second", lang: LangEnglish, in: 999 * time.Millisecond, want: "<1s"},
		{name: "english_seconds", lang: LangEnglish, in: 12*time.Second + 340*time.Millisecond, want: "12.3s"},
		{name: "chinese_minutes", lang: LangChinese, in: time.Minute + 23*time.Second, want: "1分 23秒"},
		{name: "traditional_chinese_hours", lang: LangTraditionalChinese, in: 2*time.Hour + 5*time.Minute + 40*time.Second, want: "2小時 06分"},
		{name: "japanese_seconds", lang: LangJapanese, in: 12*time.Second + 340*time.Millisecond, want: "12.3秒"},
		{name: "spanish_minutes", lang: LangSpanish, in: time.Minute + 23*time.Second, want: "1 min 23 s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := replyFooterDurationCopyForLanguage(tt.lang)
			if got := formatReplyFooterDuration(tt.in, copy); got != tt.want {
				t.Errorf("formatReplyFooterDuration(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
