package weixin

import (
	"fmt"
	"strings"
)

func isMediaItemType(t int) bool {
	switch t {
	case messageItemImage, messageItemVoice, messageItemFile, messageItemVideo:
		return true
	default:
		return false
	}
}

// bodyFromItemList extracts user-visible text from Weixin item_list (text, quotes, voice ASR).
func bodyFromItemList(items []messageItem) string {
	body, extra := bodyPartsFromItemList(items)
	if extra == "" {
		return body
	}
	if body == "" {
		return extra
	}
	return extra + "\n" + body
}

// bodyPartsFromItemList keeps reply context separate from the current user's
// text while preserving bodyFromItemList's combined agent prompt.
func bodyPartsFromItemList(items []messageItem) (string, string) {
	if len(items) == 0 {
		return "", ""
	}
	for _, item := range items {
		switch item.Type {
		case messageItemText:
			if item.TextItem == nil {
				continue
			}
			text := strings.TrimSpace(item.TextItem.Text)
			ref := item.RefMsg
			if ref == nil {
				return text, ""
			}
			if ref.MessageItem != nil && isMediaItemType(ref.MessageItem.Type) {
				return text, ""
			}
			var parts []string
			if ref.Title != "" {
				parts = append(parts, ref.Title)
			}
			if ref.MessageItem != nil {
				refBody := bodyFromItemList([]messageItem{*ref.MessageItem})
				if refBody != "" {
					parts = append(parts, refBody)
				}
			}
			if len(parts) == 0 {
				return text, ""
			}
			extra := fmt.Sprintf("[引用: %s]", strings.Join(parts, " | "))
			return text, extra
		case messageItemVoice:
			if item.VoiceItem != nil && strings.TrimSpace(item.VoiceItem.Text) != "" {
				return strings.TrimSpace(item.VoiceItem.Text), ""
			}
		}
	}
	return "", ""
}
