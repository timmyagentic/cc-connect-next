package feishu

// callbackCardSchema accepts only the compatibility default or an explicit
// schema 2.0 request. Card callbacks do not carry the source card definition;
// the author must bind this value to each action that replaces a rich card.
func callbackCardSchema(values map[string]any) (rich bool, valid bool) {
	value, present := values["cc_card_schema"]
	if !present {
		return false, true
	}
	schema, ok := value.(string)
	return schema == "2.0", ok && schema == "2.0"
}

// callbackConfirmationCard keeps receipt text and its HMAC input literal.
// Only the envelope changes: div.text remains plain_text, never Markdown.
func callbackConfirmationCard(card map[string]any, rich bool) map[string]any {
	if !rich {
		return card
	}
	config, ok := card["config"].(map[string]any)
	if !ok {
		config = map[string]any{}
		card["config"] = config
	}
	// The command renderer's v1-only width flag cannot enter a v2 config.
	if wide, _ := config["wide_screen_mode"].(bool); wide {
		config["width_mode"] = "fill"
	}
	delete(config, "wide_screen_mode")
	config["update_multi"] = true
	config["enable_forward"] = false
	config["enable_forward_interaction"] = false
	card["schema"] = "2.0"
	card["body"] = map[string]any{"elements": card["elements"]}
	delete(card, "elements")
	return card
}
