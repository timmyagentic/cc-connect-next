package feishu

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

func TestFormReceipt_RichSchemaPreservesLiteralTextAndSignature(t *testing.T) {
	p, received := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	text := " \t<at id=all></at> **literal**\r\n/cmd 👩\u200d💻\u200c🏴\U000e0067\U000e007f\t "
	for _, rich := range []bool{false, true} {
		e := receiptEvent(text)
		if rich {
			e.Event.Action.Value["cc_card_schema"] = "2.0"
		}
		resp, err := p.onCardAction(e)
		if err != nil || resp == nil || resp.Card == nil {
			t.Fatalf("rich=%t: missing receipt %#v, %v", rich, resp, err)
		}
		card := resp.Card.Data.(map[string]any)
		elements, ok := card["elements"].([]map[string]any)
		if rich {
			body, bodyOK := card["body"].(map[string]any)
			if !bodyOK || card["schema"] != "2.0" || card["elements"] != nil {
				t.Fatalf("wrong rich receipt envelope: %#v", card)
			}
			elements, ok = body["elements"].([]map[string]any)
		} else if card["body"] != nil || card["schema"] != nil {
			t.Fatalf("default receipt changed its v1 envelope: %#v", card)
		}
		if !ok || len(elements) != 2 {
			t.Fatalf("wrong receipt elements: %#v", elements)
		}
		key, err := hex.DecodeString(e.Event.Action.Value["receipt_nonce"].(string))
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(text))
		if elements[0]["content"] != "Receipt "+hex.EncodeToString(mac.Sum(nil)) ||
			!reflect.DeepEqual(elements[1], map[string]any{"tag": "div", "text": plainText(text)}) {
			t.Fatalf("rich=%t: literal text or signature changed: %#v", rich, elements)
		}
		wantConfig := map[string]any{"update_multi": true, "enable_forward": false, "enable_forward_interaction": false}
		if !reflect.DeepEqual(card["config"], wantConfig) {
			t.Fatalf("private receipt config changed: %#v", card["config"])
		}
		if !reflect.DeepEqual(card["header"], map[string]any{"title": plainText("Submitted"), "template": "green"}) {
			t.Fatalf("receipt header changed: %#v", card["header"])
		}
	}
	waitNoMessage(t, received, 50*time.Millisecond, "rich form receipt")
}

func TestCallbackReceipt_UnknownSchemaHasNoReplacementOrCommand(t *testing.T) {
	p, received := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	p.chatNameCache.Store("oc_fixture", "Fixture")
	for _, schema := range []any{"1.0", "", "3.0", " 2.0", nil, 2, true, []string{"2.0"}} {
		for _, command := range []bool{false, true} {
			e := receiptEvent("valid text")
			e.Event.Action.Value["cc_card_schema"] = schema
			if command {
				e.Event.Action.Value["action"] = "cmd:/help"
			}
			if resp, err := p.onCardAction(e); err != nil || resp != nil {
				t.Fatalf("unknown schema %v command=%t produced response %#v, %v", schema, command, resp, err)
			}
		}
	}
	waitNoMessage(t, received, 50*time.Millisecond, "unsupported schema")
}

func TestCmdAfterClick_RichSchemaPreservesReplacementAndSingleDispatch(t *testing.T) {
	p, received := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	p.chatNameCache.Store("oc_fixture", "Fixture")
	for _, rich := range []bool{false, true} {
		e := receiptEvent("unused form field")
		e.Event.Action.Value["action"] = "cmd:/help"
		e.Event.Action.Value["after_click"] = map[string]any{
			"title": "Confirmed", "color": "green", "markdown": "Exact confirmation.",
			"link_text": "Open", "link_url": "https://example.com/result",
		}
		if rich {
			e.Event.Action.Value["cc_card_schema"] = "2.0"
		}
		resp, err := p.onCardAction(e)
		if err != nil || resp == nil || resp.Card == nil {
			t.Fatalf("rich=%t: missing after_click receipt %#v, %v", rich, resp, err)
		}
		card := resp.Card.Data.(map[string]any)
		var elements []map[string]any
		if rich {
			if card["schema"] != "2.0" || card["elements"] != nil {
				t.Fatalf("wrong rich command envelope: %#v", card)
			}
			elements = card["body"].(map[string]any)["elements"].([]map[string]any)
			config := card["config"].(map[string]any)
			if config["wide_screen_mode"] != nil || config["enable_forward"] != false ||
				config["enable_forward_interaction"] != false || config["update_multi"] != true {
				t.Fatalf("invalid private rich config: %#v", config)
			}
		} else {
			if card["schema"] != nil || card["body"] != nil {
				t.Fatalf("legacy after_click envelope changed: %#v", card)
			}
			elements = card["elements"].([]map[string]any)
		}
		want := []map[string]any{{"tag": "markdown", "content": "Exact confirmation."},
			{"tag": "hr"}, {"tag": "markdown", "content": "[Open](https://example.com/result)"}}
		if !reflect.DeepEqual(elements, want) {
			t.Fatalf("confirmation content changed: %#v", elements)
		}
		select {
		case msg := <-received:
			if msg.Content != "/help" || !msg.IsCardAction {
				t.Fatalf("wrong dispatched command: %#v", msg)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("expected the original command dispatch")
		}
		waitNoMessage(t, received, 50*time.Millisecond, "extra after_click command")
	}
}

func TestRichReceiptSchemaDoesNotBypassExistingAdmission(t *testing.T) {
	p, received := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	for _, change := range []func(*callback.CardActionTriggerEvent){
		func(e *callback.CardActionTriggerEvent) { e.Event.Operator.OpenID = "ou_other" },
		func(e *callback.CardActionTriggerEvent) { e.Event.Action.Value[cardActionProjectKey] = "other" },
		func(e *callback.CardActionTriggerEvent) { e.Event.Action.Value[cardActionDirectUserKey] = "ou_other" },
		func(e *callback.CardActionTriggerEvent) {
			e.Event.Action.Value["receipt_nonce"] = strings.Repeat("X", 64)
		},
	} {
		e := receiptEvent("valid text")
		e.Event.Action.Value["cc_card_schema"] = "2.0"
		change(e)
		if resp, err := p.onCardAction(e); err != nil || resp != nil {
			t.Fatalf("rich schema bypassed admission: %#v, %v", resp, err)
		}
	}
	waitNoMessage(t, received, 50*time.Millisecond, "unauthorized rich receipt")
}
