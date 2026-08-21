package feishu

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

// Rich cards must render the reply footer instead of dropping it: a dim
// notation-size block after the answer body, mirroring the legacy
// buildCardJSONWithStatusFooter styling.

func richFooterElements(t *testing.T, cardJSON string) []map[string]any {
	t.Helper()
	var card struct {
		Body struct {
			Elements []map[string]any `json:"elements"`
		} `json:"body"`
	}
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("card JSON unmarshal: %v", err)
	}
	return card.Body.Elements
}

func TestBuildRichCard_DoneRendersStatusFooter(t *testing.T) {
	copy := core.RichCardCopy{Done: "Done"}
	card := buildRichCardWithCopy(core.CardStatusDone, "done", nil, "answer body", false, "gpt-5.6-sol · effort:max", copy)

	elements := richFooterElements(t, card)
	var footerFound bool
	for _, el := range elements {
		if el["text_size"] == "notation" {
			content, _ := el["content"].(string)
			if strings.Contains(content, "gpt-5.6-sol · effort:max") {
				footerFound = true
			}
		}
	}
	if !footerFound {
		t.Errorf("done card must contain a notation-size footer element, got %s", card)
	}
}

func TestBuildRichCard_EmptyFooterAddsNoElement(t *testing.T) {
	copy := core.RichCardCopy{Done: "Done"}
	card := buildRichCardWithCopy(core.CardStatusDone, "done", nil, "answer body", false, "", copy)

	for _, el := range richFooterElements(t, card) {
		if el["text_size"] == "notation" {
			t.Errorf("empty footer must not add a footer element, got %s", card)
		}
	}
}

func TestBuildRichCard_StreamingCardHasNoFooter(t *testing.T) {
	copy := core.RichCardCopy{Done: "Done"}
	card := buildRichCardWithCopy(core.CardStatusWorking, "answer", nil, "partial", true, "gpt-5.6-sol · effort:max", copy)

	for _, el := range richFooterElements(t, card) {
		if el["text_size"] == "notation" {
			t.Errorf("streaming card must not render the footer, got %s", card)
		}
	}
}
