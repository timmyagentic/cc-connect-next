package feishu

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

func groupContextRevision(entry core.ContextMessage) string {
	// Image-only card edits must invalidate receipts even when readable text
	// is unchanged. Resource keys remain transient; only this digest persists.
	snapshot, _ := json.Marshal(struct {
		Text      string
		Resources []core.ContextResource
	}{entry.Text, entry.Resources})
	return fmt.Sprintf("%x", sha256.Sum256(snapshot))
}

const (
	groupContextForwardPending = "[forwarded messages: expansion pending]"
	groupContextContentBytes   = 64 * 1024
	groupContextMaxResources   = 12
)

// Parsing never downloads resources or resolves contacts. Resource references
// are transient and consumed only after core's receipt and text-budget filter.
func parseGroupContextContent(kind, raw, messageID string) core.ContextMessage {
	entry := core.ContextMessage{}
	if kind == "merge_forward" {
		entry.Text = groupContextForwardPending
		return entry
	}
	if kind == "sticker" {
		entry.Text = "[sticker: unavailable; Feishu API does not support sticker download]"
		return entry
	}
	if len(raw) > groupContextContentBytes {
		contextContentUnavailable(&entry, kind, "content size limit")
		return entry
	}
	var body map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &body) != nil || body == nil {
		contextContentUnavailable(&entry, kind, "malformed content")
		return entry
	}
	field := func(key string) string {
		var value string
		_ = json.Unmarshal(body[key], &value)
		return value
	}
	switch kind {
	case "text":
		entry.Text = field("text")
		if entry.Text == "" {
			contextContentUnavailable(&entry, kind, "missing text")
		}
	case "post":
		parseGroupContextPost(&entry, raw, messageID)
	case "interactive":
		entry.Text = extractGroupContextCardText(raw)
		// The API may wrap an original card as a JSON string.
		if wrapped := field("json_card"); wrapped != "" {
			if json.Unmarshal([]byte(wrapped), &body) != nil {
				contextContentUnavailable(&entry, kind, "malformed card")
				break
			}
		}
		var nodes any
		cardJSON, _ := json.Marshal(body)
		_ = json.Unmarshal(cardJSON, &nodes)
		budget := 256
		collectGroupContextCardResources(&entry, nodes, messageID, 0, &budget)
		if strings.TrimSpace(entry.Text) == "" {
			contextContentUnavailable(&entry, kind, "no readable card body")
		}
	case "image":
		appendContextResource(&entry, messageID, field("image_key"), "image", "")
	case "file", "audio", "video", "media":
		entry.Text = "[" + kind + contextMetadata(body, "file_name", "duration", "file_size") + "]"
		name := field("file_name")
		if name == "" {
			switch kind {
			case "audio":
				name = "audio.opus"
			case "video", "media":
				name = "video.mp4"
			default:
				name = "attachment"
			}
		}
		appendContextResource(&entry, messageID, field("file_key"), "file", name)
		if field("image_key") != "" {
			appendContextResource(&entry, messageID, field("image_key"), "image", "video-cover")
		}
	case "share_chat":
		setContextMetadata(&entry, "shared chat", body, "chat_id", "chat_name", "name")
	case "share_user", "user":
		setContextMetadata(&entry, "shared user", body, "user_id", "open_id", "user_name", "name")
	case "location":
		setContextMetadata(&entry, "location", body, "name", "address", "latitude", "longitude")
	case "system":
		setContextMetadata(&entry, "system", body, "text", "content", "template", "type")
	default:
		contextContentUnavailable(&entry, kind, "unsupported message type")
	}
	return entry
}

func setContextMetadata(entry *core.ContextMessage, label string, body map[string]json.RawMessage, keys ...string) {
	metadata := contextMetadata(body, keys...)
	if metadata == "" {
		contextContentUnavailable(entry, label, "missing readable metadata")
		return
	}
	entry.Text = "[" + label + metadata + "]"
}

func contextContentUnavailable(entry *core.ContextMessage, kind, reason string) {
	entry.Incomplete = true
	// Types are server data, too; never allow a malformed type to enlarge a
	// placeholder without bound.
	if len(kind) > 64 {
		kind = kind[:64]
	}
	entry.Text += fmt.Sprintf("\n[%s: unavailable (%s)]", kind, reason)
	entry.Text = strings.TrimSpace(entry.Text)
}

func contextMetadata(body map[string]json.RawMessage, keys ...string) string {
	var parts []string
	for _, key := range keys {
		raw := body[key]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var text string
		if json.Unmarshal(raw, &text) != nil {
			text = string(raw)
		}
		if text != "" {
			parts = append(parts, key+"="+text)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ": " + strings.Join(parts, ", ")
}

func appendContextResource(entry *core.ContextMessage, messageID, key, kind, name string) {
	if strings.TrimSpace(key) == "" {
		contextContentUnavailable(entry, kind, "missing resource key")
		return
	}
	for _, previous := range entry.Resources {
		if previous.MessageID == messageID && previous.Key == key && previous.Kind == kind {
			return
		}
	}
	if len(entry.Resources) >= groupContextMaxResources {
		contextContentUnavailable(entry, kind, "resource count limit")
		return
	}
	entry.Resources = append(entry.Resources, core.ContextResource{MessageID: messageID, Key: key, Kind: kind, Name: name})
	if entry.Text != "" {
		entry.Text += "\n"
	}
	entry.Text += "[" + kind
	if kind == "image" {
		entry.Text += fmt.Sprintf(" #%d", contextImageCount(entry.Resources))
	}
	if name != "" {
		entry.Text += ": " + name
	}
	entry.Text += "]"
}

func contextImageCount(resources []core.ContextResource) int {
	n := 0
	for _, resource := range resources {
		if resource.Kind == "image" {
			n++
		}
	}
	return n
}

func parseGroupContextPost(entry *core.ContextMessage, raw, messageID string) {
	post := decodePost(raw)
	if post == nil {
		contextContentUnavailable(entry, "post", "malformed content")
		return
	}
	entry.Text = post.Title
	for _, line := range post.Content {
		for _, elem := range line {
			switch elem.Tag {
			case "text", "markdown", "code_block":
				entry.Text += "\n" + elem.Text
			case "a":
				entry.Text += "\n" + elem.Text + " (" + elem.Href + ")"
			case "at":
				name := elem.UserName
				if name == "" {
					name = elem.UserId
				}
				entry.Text += "\n@" + name
			case "img":
				appendContextResource(entry, messageID, elem.ImageKey, "image", "")
			case "media", "video", "audio", "file":
				appendContextResource(entry, messageID, elem.FileKey, "file", elem.FileName)
				if elem.ImageKey != "" {
					appendContextResource(entry, messageID, elem.ImageKey, "image", "video-cover")
				}
			case "hr":
				entry.Text += "\n---"
			default:
				contextContentUnavailable(entry, "post "+elem.Tag, "unsupported element")
			}
		}
	}
	for _, file := range post.Files {
		if file.IsFolder {
			contextContentUnavailable(entry, "folder", "folder download unsupported")
			continue
		}
		appendContextResource(entry, messageID, file.FileKey, "file", file.FileName)
	}
	entry.Text = strings.TrimSpace(entry.Text)
	if entry.Text == "" {
		contextContentUnavailable(entry, "post", "empty content")
	}
}

func collectGroupContextCardResources(entry *core.ContextMessage, node any, messageID string, depth int, budget *int) {
	if depth > 16 || *budget <= 0 {
		contextContentUnavailable(entry, "card", "element limit")
		return
	}
	*budget--
	switch value := node.(type) {
	case []any:
		for _, child := range value {
			collectGroupContextCardResources(entry, child, messageID, depth+1, budget)
			if *budget <= 0 {
				break
			}
		}
	case map[string]any:
		tag, _ := value["tag"].(string)
		if tag == "img" || tag == "image" {
			key, _ := value["img_key"].(string)
			if key == "" {
				key, _ = value["image_key"].(string)
			}
			if property, ok := value["property"].(map[string]any); ok && key == "" {
				key, _ = property["img_key"].(string)
				if key == "" {
					key, _ = property["image_key"].(string)
				}
			}
			appendContextResource(entry, messageID, key, "image", "")
		}
		for _, key := range []string{"body", "property", "elements", "columns", "fields", "items", "rows", "cells"} {
			if child, ok := value[key]; ok {
				collectGroupContextCardResources(entry, child, messageID, depth+1, budget)
			}
		}
	}
}

func boundGroupContextText(entry *core.ContextMessage, limit int) {
	if limit <= 0 || limit > maxGroupContextChars {
		limit = defaultGroupContextChars
	}
	runes := []rune(entry.Text)
	if len(runes) > limit {
		const marker = "\n[content truncated]"
		keep := limit - len(marker)
		if keep < 0 {
			keep = 0
		}
		entry.Text = string(runes[:keep]) + marker
		entry.Incomplete = true
	}
}

func replaceContextMentions(text string, mentions []*larkim.Mention) string {
	for _, mention := range mentions {
		if mention != nil && mention.Key != nil && mention.Name != nil {
			text = strings.ReplaceAll(text, *mention.Key, "@"+*mention.Name)
		}
	}
	return text
}
