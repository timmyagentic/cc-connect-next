package feishu

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	"github.com/timmyagentic/cc-connect-next/core"
)

const maxCardHookPayload = 64 * 1024

var _ core.CardActionHookSource = (*interactivePlatform)(nil)

func (p *interactivePlatform) SetCardActionHandler(handler func(string, core.HookEvent) error) {
	p.mu.Lock()
	p.cardHookHandler = handler
	p.mu.Unlock()
}

// dispatchCardHook runs after the ordinary callback admission checks. Metadata
// comes from the SDK callback; only opaque business data comes from the card.
func (p *Platform) dispatchCardHook(action *callback.CallBackAction, userID, chatID, messageID string) *callback.CardActionTriggerResponse {
	values := action.Value
	project := p.boundProjectName()
	_, validSchema := callbackCardSchema(values)
	name := strings.TrimPrefix(cardActionString(values, "action"), "hook:")
	actionName := action.Name
	if declared, exists := values["hook_action"]; exists {
		var valid bool
		actionName, valid = declared.(string)
		if !valid {
			return cardHookToast(core.MsgCardHookUnavailable, "error")
		}
	}
	if !p.useInteractiveCard || project == "" || values[cardActionProjectKey] != project ||
		!validSchema || !receiptChatID.MatchString(chatID) || !receiptMessageID.MatchString(messageID) ||
		action.Tag != "button" || !core.ValidHookName(actionName) || !core.ValidHookName(name) {
		return cardHookToast(core.MsgCardHookUnavailable, "error")
	}
	data, validData := values["data"].(map[string]any)
	if values["data"] != nil && !validData {
		return cardHookToast(core.MsgCardHookUnavailable, "error")
	}
	if data == nil {
		data = map[string]any{}
	}
	formValues := action.FormValue
	if formValues == nil {
		formValues = map[string]any{}
	}
	extra := map[string]any{
		"hook": name, "action": actionName, "message_id": messageID, "chat_id": chatID,
		"data": data, "form_values": formValues,
	}
	payload, err := json.Marshal(extra)
	if err != nil || len(payload) > maxCardHookPayload {
		return cardHookToast(core.MsgCardHookUnavailable, "error")
	}
	p.mu.RLock()
	handler := p.cardHookHandler
	p.mu.RUnlock()
	if handler == nil {
		return cardHookToast(core.MsgCardHookUnavailable, "error")
	}
	event := core.HookEvent{
		Event: core.HookEventCardAction, Platform: p.platformName, UserID: userID,
		SessionKey: p.sessionKeyFromCardAction(chatID, userID, values), Extra: extra,
	}
	if err := handler(name, event); err != nil {
		// Handler errors can contain private configuration or card data.
		slog.Warn(p.tag() + ": named card hook dispatch rejected")
		return cardHookToast(core.MsgCardHookUnavailable, "error")
	}
	return cardHookToast(core.MsgCardHookAccepted, "info")
}

func cardHookToast(key core.MsgKey, kind string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{Toast: &callback.Toast{
		Type:    kind,
		Content: core.NewI18n(core.LangEnglish).T(key),
		I18nContent: map[string]string{
			"en_us": core.NewI18n(core.LangEnglish).T(key),
			"zh_cn": core.NewI18n(core.LangChinese).T(key),
			"zh_tw": core.NewI18n(core.LangTraditionalChinese).T(key),
			"ja_jp": core.NewI18n(core.LangJapanese).T(key),
			"es_es": core.NewI18n(core.LangSpanish).T(key),
		},
	}}
}
