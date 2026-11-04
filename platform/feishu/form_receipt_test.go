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
	"github.com/timmyagentic/cc-connect-next/core"
)

func receiptEvent(text string) *callback.CardActionTriggerEvent {
	e := cardActionEvent("ou_allowed", "form:receipt")
	e.Event.Context = &callback.Context{OpenChatID: "oc_fixture", OpenMessageID: "om_first"}
	a := e.Event.Action
	a.Name = "send_text"
	a.Tag = "button"
	a.Value[cardActionProjectKey] = "example"
	a.Value[cardActionDirectUserKey] = "ou_allowed"
	a.Value["receipt_nonce"] = strings.Repeat("0b", 20) + strings.Repeat("00", 12)
	a.Value["form_field"] = "answer"
	a.Value["after_click"] = map[string]any{"title": "Submitted", "color": "green", "markdown": "Receipt"}
	a.FormValue = map[string]any{"answer": text, "other": "different input"}
	return e
}

func TestFormReceiptReturnsOnlyTheSubmittedField(t *testing.T) {
	p, received := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	e := receiptEvent("Hi There")
	// RFC 4231 case 1. Padding its 20-byte key with twelve zero bytes gives
	// the same HMAC and distinguishes the decoded key from its hexadecimal text.
	digest := "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"
	check := func(e *callback.CardActionTriggerEvent, text, digest string) {
		t.Helper()
		resp, err := p.onCardAction(e)
		want := map[string]any{
			"config": map[string]any{"update_multi": true, "enable_forward": false, "enable_forward_interaction": false},
			"header": map[string]any{"title": plainText("Submitted"), "template": "green"},
			"elements": []map[string]any{
				{"tag": "markdown", "content": "Receipt " + digest},
				{"tag": "div", "text": plainText(text)},
			},
		}
		if err != nil || resp == nil || resp.Card == nil || resp.Card.Type != "raw" || !reflect.DeepEqual(resp.Card.Data, want) {
			t.Fatalf("wrong replacement card for %s: %#v, %v", e.Event.Context.OpenMessageID, resp, err)
		}
	}
	check(e, "Hi There", digest)

	// A different card from the same operator supplies a different nonce,
	// field and text. Whitespace and markup remain literal in the receipt.
	text := " \t</div> **hi**\r\n/cmd 👩\u200d💻\u200c🏴\U000e0067\U000e007f\t "
	second := receiptEvent("stale field")
	second.Event.Context.OpenMessageID = "om_second"
	a := second.Event.Action
	a.Value["form_field"] = "second_input"
	a.FormValue["second_input"] = text
	a.InputValue = "unrelated input"
	key := []byte(strings.Repeat("k", 32))
	a.Value["receipt_nonce"] = hex.EncodeToString(key)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(text))
	check(second, text, hex.EncodeToString(mac.Sum(nil)))
	waitNoMessage(t, received, 50*time.Millisecond, "form receipt")
}

func TestFormReceiptRejectsInvalidCallback(t *testing.T) {
	p, received := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	for _, tc := range []struct {
		name   string
		change func(*callback.CardActionTriggerRequest)
	}{
		{"wrong operator", func(r *callback.CardActionTriggerRequest) { r.Operator.OpenID = "ou_other" }},
		{"blocked operator with matching binding", func(r *callback.CardActionTriggerRequest) {
			r.Operator.OpenID = "ou_other"
			r.Action.Value[cardActionDirectUserKey] = "ou_other"
		}},
		{"wrong project", func(r *callback.CardActionTriggerRequest) { r.Action.Value[cardActionProjectKey] = "elsewhere" }},
		{"missing owner", func(r *callback.CardActionTriggerRequest) { delete(r.Action.Value, cardActionDirectUserKey) }},
		{"padded owner", func(r *callback.CardActionTriggerRequest) { r.Action.Value[cardActionDirectUserKey] = "ou_allowed " }},
		{"missing project", func(r *callback.CardActionTriggerRequest) { delete(r.Action.Value, cardActionProjectKey) }},
		{"missing chat", func(r *callback.CardActionTriggerRequest) { r.Context.OpenChatID = "" }},
		{"invalid message", func(r *callback.CardActionTriggerRequest) { r.Context.OpenMessageID = "not a message" }},
		{"wrong tag", func(r *callback.CardActionTriggerRequest) { r.Action.Tag = "input" }},
		{"wrong button", func(r *callback.CardActionTriggerRequest) { r.Action.Name = "another_button" }},
		{"no form", func(r *callback.CardActionTriggerRequest) { r.Action.FormValue = nil }},
		{"wrong field", func(r *callback.CardActionTriggerRequest) { r.Action.Value["form_field"] = "absent" }},
		{"invalid field", func(r *callback.CardActionTriggerRequest) {
			r.Action.Value["form_field"] = "表单"
			r.Action.FormValue["表单"] = "valid text"
		}},
		{"non-text field", func(r *callback.CardActionTriggerRequest) { r.Action.FormValue["answer"] = []string{"x"} }},
		{"invalid nonce", func(r *callback.CardActionTriggerRequest) { r.Action.Value["receipt_nonce"] = strings.Repeat("A", 64) }},
		{"missing template", func(r *callback.CardActionTriggerRequest) { delete(r.Action.Value, "after_click") }},
		{"wrong template color", func(r *callback.CardActionTriggerRequest) {
			r.Action.Value["after_click"].(map[string]any)["color"] = "red"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := receiptEvent("valid text")
			tc.change(e.Event)
			if resp, err := p.onCardAction(e); err != nil || resp != nil {
				t.Fatalf("invalid callback produced response %#v or error %v", resp, err)
			}
		})
	}
	waitNoMessage(t, received, 50*time.Millisecond, "invalid form")
}

func TestFormReceiptTextLimits(t *testing.T) {
	p, _ := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	for _, text := range []string{"", "\t\r \n", "\u200c\u200d\U000e0067", "a\x00b", "a\x1fb",
		"a\x7fb", "a\u0085b", "a\u202eb", "a\u2066b", string([]byte{0xff}), " " + strings.Repeat("界", 1000)} {
		e := receiptEvent(text)
		e.Event.Action.Value["invalid_text"] = "Please check the input."
		resp, err := p.onCardAction(e)
		if err != nil || resp == nil || resp.Card != nil || resp.Toast == nil || resp.Toast.Content != "Please check the input." {
			t.Fatal("expected only the card author's validation hint")
		}
	}
	if resp, err := p.onCardAction(receiptEvent(strings.Repeat("界", 1000))); err != nil || resp == nil || resp.Card == nil {
		t.Fatal("expected exactly 1000 code points to be accepted")
	}
}

func TestFormReceiptRequiresInteractiveAdapter(t *testing.T) {
	p, _ := newAllowFromTestPlatform(t, "ou_allowed")
	p.BindProject("example")
	if got := p.RuntimeCapabilityAvailability("", nil)["direct_user_form_receipts"].State; got != core.CapabilityAvailable {
		t.Fatalf("interactive form receipt availability = %v", got)
	}
	plain := &Platform{}
	plain.BindProject("example")
	if got := plain.RuntimeCapabilityAvailability("", nil)["direct_user_form_receipts"].State; got != core.CapabilityUnavailable {
		t.Fatalf("plain form receipt availability = %v", got)
	}
	if resp, err := plain.onCardAction(receiptEvent("Hi There")); err != nil || resp != nil {
		t.Fatalf("disabled interactive adapter returned %#v, %v", resp, err)
	}
}

func TestLegacyCardCommandIgnoresFormInput(t *testing.T) {
	p, received := newAllowFromTestPlatform(t, "ou_allowed")
	e := cardActionEvent("ou_allowed", "cmd:/help")
	e.Event.Action.Tag, e.Event.Action.Name = "button", "send_text"
	e.Event.Action.FormValue = map[string]any{"answer": "unrelated submission"}
	if resp, err := p.onCardAction(e); err != nil || resp != nil {
		t.Fatalf("unexpected legacy response %#v, %v", resp, err)
	}
	if msg := waitAnyMessage(t, received, time.Second, "legacy command"); msg.Content != "/help" {
		t.Fatalf("legacy command changed to %q", msg.Content)
	}
}
