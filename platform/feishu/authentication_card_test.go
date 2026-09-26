package feishu

import (
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestBuildRichCardAuthenticationRecoveryLocalized(t *testing.T) {
	for _, test := range []struct {
		lang         core.Language
		header, body string
	}{
		{core.LangEnglish, "Sign-in required", "same system user"},
		{core.LangChinese, "需要重新登录", "同一系统用户"},
		{core.LangTraditionalChinese, "需要重新登入", "同一系統使用者"},
		{core.LangJapanese, "再ログインが必要です", "同じシステムユーザー"},
		{core.LangSpanish, "Es necesario iniciar sesión", "mismo usuario del sistema"},
	} {
		t.Run(string(test.lang), func(t *testing.T) {
			card := buildRichCardWithCopy(core.CardStatusError, "authentication_required", nil, "", false, "", core.NewI18n(test.lang).RichCardCopy())
			if !strings.Contains(card, test.header) || !strings.Contains(card, test.body) {
				t.Fatalf("auth card has no localized recovery guidance: %s", card)
			}
		})
	}
}
