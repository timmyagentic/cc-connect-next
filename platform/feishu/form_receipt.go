package feishu

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

var (
	receiptFieldName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
	receiptChatID    = regexp.MustCompile(`^oc_[A-Za-z0-9]+$`)
	receiptMessageID = regexp.MustCompile(`^om_[A-Za-z0-9]+$`)
)

func validReceiptText(text string) bool {
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > 1000 {
		return false
	}
	visible := false
	for _, r := range text {
		switch {
		case unicode.IsControl(r):
			if r != '\t' && r != '\n' && r != '\r' {
				return false
			}
		case unicode.Is(unicode.Cf, r):
			// Preserve joiners and emoji tags, but reject other format controls.
			if r != '\u200c' && r != '\u200d' && (r < '\U000e0020' || r > '\U000e007f') {
				return false
			}
		case !unicode.IsSpace(r):
			visible = true
		}
	}
	return visible
}

// formReceipt runs after the ordinary callback admission checks. A receipt also
// requires explicit owner bindings, even on installations that accept legacy cards.
func (p *Platform) formReceipt(action *callback.CallBackAction, userID, chatID, messageID string) *callback.CardActionTriggerResponse {
	values := action.Value
	rich, validSchema := callbackCardSchema(values)
	if !validSchema {
		return nil
	}
	project := p.boundProjectName()
	if !p.useInteractiveCard || project == "" || values[cardActionProjectKey] != project || values[cardActionDirectUserKey] != userID ||
		!receiptChatID.MatchString(chatID) || !receiptMessageID.MatchString(messageID) ||
		action.Tag != "button" || action.Name != "send_text" {
		return nil
	}
	field, _ := values["form_field"].(string)
	nonce, _ := values["receipt_nonce"].(string)
	afterClick, _ := values["after_click"].(map[string]any)
	title, _ := afterClick["title"].(string)
	markdown, _ := afterClick["markdown"].(string)
	if !receiptFieldName.MatchString(field) || len(nonce) != 64 || strings.ToLower(nonce) != nonce ||
		afterClick["color"] != "green" || strings.TrimSpace(title) == "" || strings.TrimSpace(markdown) == "" {
		return nil
	}
	key, err := hex.DecodeString(nonce)
	if err != nil {
		return nil
	}
	text, ok := action.FormValue[field].(string)
	if !ok || !validReceiptText(text) {
		if hint, _ := values["invalid_text"].(string); strings.TrimSpace(hint) != "" && utf8.RuneCountInString(hint) <= 120 {
			return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "error", Content: hint}}
		}
		return nil
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(text))
	card := map[string]any{
		"config": map[string]any{"update_multi": true, "enable_forward": false, "enable_forward_interaction": false},
		"header": map[string]any{"title": plainText(title), "template": "green"},
		"elements": []map[string]any{
			{"tag": "markdown", "content": markdown + " " + hex.EncodeToString(mac.Sum(nil))},
			{"tag": "div", "text": plainText(text)},
		},
	}
	slog.Info("feishu: card form receipt", "user", userID)
	return &callback.CardActionTriggerResponse{Card: &callback.Card{Type: "raw", Data: callbackConfirmationCard(card, rich)}}
}
