package feishu

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

const (
	defaultGroupContextMessages = 20
	defaultGroupContextMinutes  = 30
	defaultGroupContextChars    = 8000
	maxGroupContextMessages     = 100
	maxGroupContextMinutes      = 1440
	minGroupContextChars        = 512
	maxGroupContextChars        = 32000
	groupContextPageSize        = 50
	groupContextMaxPages        = 4
)

type groupContextConfig struct {
	enabled         bool
	includePeerBots bool
	maxMessages     int
	minutes         int
	maxChars        int
}

func parseGroupContextConfig(opts map[string]any) (groupContextConfig, error) {
	cfg := groupContextConfig{maxMessages: defaultGroupContextMessages, minutes: defaultGroupContextMinutes, maxChars: defaultGroupContextChars}
	if raw, exists := opts["group_context"]; exists {
		var ok bool
		cfg.enabled, ok = raw.(bool)
		if !ok {
			return cfg, fmt.Errorf("group_context must be a boolean")
		}
	}
	if raw, exists := opts["group_context_include_peer_bots"]; exists {
		var ok bool
		cfg.includePeerBots, ok = raw.(bool)
		if !ok {
			return cfg, fmt.Errorf("group_context_include_peer_bots must be a boolean")
		}
	}
	if cfg.includePeerBots {
		if !cfg.enabled {
			return cfg, fmt.Errorf("group_context_include_peer_bots requires group_context = true")
		}
		peers, ok := opts["peer_bots"].(map[string]any)
		if !ok || len(peers) == 0 {
			return cfg, fmt.Errorf("group_context_include_peer_bots requires a non-empty peer_bots table")
		}
		for id, value := range peers {
			alias, ok := value.(string)
			if !ok || strings.TrimSpace(alias) == "" || strings.TrimSpace(id) != id || id == "" || id == opts["app_id"] {
				return cfg, fmt.Errorf("group_context_include_peer_bots requires other bot app IDs with non-empty string aliases")
			}
		}
	}
	for _, setting := range []struct {
		key      string
		min, max int
		target   *int
	}{
		{"group_context_max_messages", 1, maxGroupContextMessages, &cfg.maxMessages},
		{"group_context_window_minutes", 1, maxGroupContextMinutes, &cfg.minutes},
		{"group_context_max_chars", minGroupContextChars, maxGroupContextChars, &cfg.maxChars},
	} {
		if raw, exists := opts[setting.key]; exists {
			var number float64
			switch value := raw.(type) {
			case int:
				number = float64(value)
			case int64:
				number = float64(value)
			case float64:
				number = value // JSON-originated integral config values
			default:
				return cfg, fmt.Errorf("%s must be an integer", setting.key)
			}
			if math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) || number < float64(setting.min) || number > float64(setting.max) {
				return cfg, fmt.Errorf("%s must be an integer between %d and %d", setting.key, setting.min, setting.max)
			}
			*setting.target = int(number)
		}
	}
	return cfg, nil
}

// MessageContextEnabled is false for private, synthetic, and proactive
// targets. The live event path sets contextEligible only after reply-trigger and
// access checks; persisting a reply target does not preserve that authority.
func (p *Platform) MessageContextEnabled(target any) bool {
	rctx, ok := target.(replyContext)
	return ok && p.groupContext.enabled && rctx.contextEligible && rctx.chatID != "" && core.AllowList(p.allowChat, rctx.chatID)
}

func (p *Platform) RuntimeCapabilityAvailability(_ string, target any) map[string]core.CapabilityAvailability {
	availability := core.CapabilityAvailability{
		State: core.CapabilityConditional, Reason: "Read-only context is enabled; requires an authorized group message accepted by the configured reply rules and existing message-history access. Reads happen only before a turn; no new permissions are requested.",
		ReasonZH: "已启用只读上下文；要求授权群中符合回复规则的消息和已有历史读取权限。仅在回合开始前读取，不申请新权限。",
	}
	if !p.groupContext.enabled {
		availability = core.CapabilityAvailability{State: core.CapabilityUnavailable, Reason: "Recent conversation context is disabled in platform configuration.", ReasonZH: "平台配置未启用近期讨论上下文。"}
	} else if target != nil && !p.MessageContextEnabled(target) {
		availability = core.CapabilityAvailability{State: core.CapabilityUnavailable, Reason: "This target is not a verified group trigger.", ReasonZH: "当前目标不是已核验的群内触发消息。"}
	}
	if p.groupContext.includePeerBots && availability.State == core.CapabilityConditional {
		availability.Reason += " Explicit peer-bot sharing includes attributed public snapshots in the same group/topic and recent window; it does not share native Agent sessions or dispatch bot messages."
		availability.ReasonZH += " 已显式开启对端机器人共享：仅补读同群同话题、近期窗口内带归因的公开快照，不合并原生 Agent 会话，不因机器人消息触发回复。"
	}
	batches := core.CapabilityAvailability{State: core.CapabilityAvailable, Reason: "Multiple images are sent as two-column Card 2.0 galleries, up to five originals per message; single images stay native.", ReasonZH: "多张图片按每批最多 5 张原图发送双列 Card 2.0 图集；单张保持原生图片消息。"}
	if !p.useInteractiveCard {
		batches = core.CapabilityAvailability{State: core.CapabilityUnavailable, Reason: "enable_feishu_card is false; images are sent individually.", ReasonZH: "enable_feishu_card 为 false，图片逐张发送。"}
	}
	return map[string]core.CapabilityAvailability{"recent_conversation_context": availability, "image_batches": batches}
}

func (p *Platform) LoadMessageContext(ctx context.Context, target any) (result core.MessageContext, readErr error) {
	if !p.MessageContextEnabled(target) {
		return core.MessageContext{}, nil
	}
	rctx := target.(replyContext)
	result = core.MessageContext{
		Scope:    p.Name() + ":" + p.appID + ":" + rctx.chatID + ":" + rctx.threadID,
		MaxChars: p.groupContext.maxChars,
	}
	defer func() {
		result.Incomplete = result.Incomplete || readErr != nil
		finishGroupContext(&result, p.groupContext.maxMessages)
	}()
	cutoff := time.UnixMilli(rctx.contextTimeMs)
	if rctx.contextTimeMs <= 0 {
		return result, fmt.Errorf("%s: recent context has no trigger timestamp", p.tag())
	}
	start := cutoff.Add(-time.Duration(p.groupContext.minutes) * time.Minute)
	containerType, containerID := "chat", rctx.chatID
	if rctx.threadID != "" {
		containerType, containerID = "thread", rctx.threadID
	}
	seen := make(map[string]bool)
	pageToken := ""
	for page := 0; page < groupContextMaxPages; page++ {
		builder := larkim.NewListMessageReqBuilder().ContainerIdType(containerType).ContainerId(containerID).
			SortType("ByCreateTimeDesc").PageSize(groupContextPageSize).WithSenderName(true)
		if p.groupContext.includePeerBots {
			builder.CardMsgContentType("user_card_content")
		}
		if pageToken != "" {
			builder.PageToken(pageToken)
		}
		// Thread history does not support server-side time filters. Both
		// container types are checked locally as well, including millisecond bounds.
		if containerType == "chat" {
			builder.StartTime(strconv.FormatInt(start.Unix(), 10)).EndTime(strconv.FormatInt(cutoff.Unix()+1, 10))
		}
		resp, err := p.client.Im.Message.List(ctx, builder.Build())
		if err != nil {
			return result, fmt.Errorf("%s: list recent messages: %w", p.tag(), err)
		}
		if resp == nil || !resp.Success() || resp.Data == nil {
			code := -1
			if resp != nil {
				code = resp.Code
			}
			return result, fmt.Errorf("%s: list recent messages failed (code %d)", p.tag(), code)
		}
		pastWindow := false
		for _, item := range resp.Data.Items {
			entry, usable, incomplete := p.contextMessage(item, rctx, start, cutoff)
			result.Incomplete = result.Incomplete || incomplete
			if item != nil {
				if ms, err := strconv.ParseInt(stringValue(item.CreateTime), 10, 64); err == nil && ms < start.UnixMilli() {
					pastWindow = true
				}
			}
			if !usable || seen[entry.ID] {
				continue
			}
			seen[entry.ID] = true
			result.Messages = append(result.Messages, entry)
		}
		hasMore := resp.Data.HasMore != nil && *resp.Data.HasMore
		if len(result.Messages) > p.groupContext.maxMessages || pastWindow || !hasMore {
			if len(result.Messages) > p.groupContext.maxMessages {
				result.Incomplete = true
			}
			break
		}
		nextToken := stringValue(resp.Data.PageToken)
		if nextToken == "" || nextToken == pageToken || page == groupContextMaxPages-1 {
			result.Incomplete = true
			break
		}
		pageToken = nextToken
	}
	return result, nil
}

func finishGroupContext(result *core.MessageContext, maxMessages int) {
	sort.Slice(result.Messages, func(i, j int) bool {
		if result.Messages[i].Time.Equal(result.Messages[j].Time) {
			return result.Messages[i].ID < result.Messages[j].ID
		}
		return result.Messages[i].Time.Before(result.Messages[j].Time)
	})
	if n := len(result.Messages); n > maxMessages {
		result.Messages = result.Messages[n-maxMessages:]
		result.Incomplete = true
	}
}

func (p *Platform) contextMessage(item *larkim.Message, rctx replyContext, start, cutoff time.Time) (core.ContextMessage, bool, bool) {
	if item == nil || (item.Deleted != nil && *item.Deleted) || p.isMessageRecalled(stringValue(item.MessageId)) {
		return core.ContextMessage{}, false, false
	}
	id := stringValue(item.MessageId)
	if id == rctx.messageID {
		return core.ContextMessage{}, false, false
	}
	senderType, senderID := "", ""
	if item.Sender != nil {
		senderType, senderID = stringValue(item.Sender.SenderType), stringValue(item.Sender.Id)
	}
	if senderType == "app" && (!p.groupContext.includePeerBots || senderID == p.appID || p.peerBots[senderID] == "") {
		return core.ContextMessage{}, false, false
	}
	if stringValue(item.ChatId) != rctx.chatID {
		return core.ContextMessage{}, false, true
	}
	threadID := stringValue(item.ThreadId)
	if (rctx.threadID == "" && threadID != "") || (rctx.threadID != "" && threadID != rctx.threadID && id != rctx.contextRootID) {
		return core.ContextMessage{}, false, false
	}
	ms, err := strconv.ParseInt(stringValue(item.CreateTime), 10, 64)
	if err != nil || id == "" || item.Sender == nil || item.Body == nil ||
		(senderType != "user" && senderType != "app") || senderID == "" {
		return core.ContextMessage{}, false, true
	}
	at := time.UnixMilli(ms).UTC()
	if at.Before(start) || at.After(cutoff) {
		return core.ContextMessage{}, false, false
	}
	// A queued trigger cannot read the old version of a card edited after it
	// arrived. Omit that snapshot visibly; the next trigger can pick it up.
	if senderType == "app" && stringValue(item.UpdateTime) != "" {
		updated, err := strconv.ParseInt(*item.UpdateTime, 10, 64)
		if err != nil || updated > cutoff.UnixMilli() {
			return core.ContextMessage{}, false, true
		}
	}
	content := stringValue(item.Body.Content)
	var text string
	incomplete := false
	switch stringValue(item.MsgType) {
	case "text":
		var body struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &body); err != nil {
			return core.ContextMessage{}, false, true
		}
		text = replaceMentions(body.Text, item.Mentions)
	case "post":
		text = replaceMentions(extractPostPlainText(content), item.Mentions)
	case "interactive":
		if senderType != "app" {
			return core.ContextMessage{}, false, true
		}
		text = extractGroupContextCardText(content)
		if text == "" {
			return core.ContextMessage{}, false, true
		}
	default:
		// Never download another participant's attachments implicitly.
		return core.ContextMessage{}, false, true
	}
	if strings.TrimSpace(text) == "" {
		return core.ContextMessage{}, false, false
	}
	entry := core.ContextMessage{ID: id, SenderID: senderID, SenderName: stringValue(item.Sender.SenderName), SenderType: senderType, Time: at, ReplyTo: stringValue(item.ParentId)}
	if senderType == "app" {
		entry.SenderName = p.peerBots[senderID]
		// Hash the complete readable snapshot before truncation. Streaming
		// cards keep their message ID when the final answer replaces progress.
		entry.Revision = fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
	}
	runes := []rune(text)
	if len(runes) > p.groupContext.maxChars {
		text = string(runes[:p.groupContext.maxChars])
		incomplete = true
	}
	entry.Text = text
	return entry, true, incomplete
}

// Retain the card title: in cc-connect it distinguishes working, completed,
// failed and interrupted replies. streaming_mode=false alone is not completion.
func extractGroupContextCardText(content string) string {
	var wrapper struct {
		JSONCard string `json:"json_card"`
	}
	if json.Unmarshal([]byte(content), &wrapper) == nil && wrapper.JSONCard != "" {
		content = wrapper.JSONCard
	}
	var card struct {
		Body struct {
			Elements []json.RawMessage `json:"elements"`
		} `json:"body"`
		Elements []json.RawMessage `json:"elements"`
		Header   struct {
			Title struct {
				Content string `json:"content"`
			} `json:"title"`
			Property struct {
				Title struct {
					Property struct {
						Content string `json:"content"`
					} `json:"property"`
				} `json:"title"`
			} `json:"property"`
		} `json:"header"`
		Config struct {
			Streaming    bool `json:"streaming_mode"`
			RawStreaming bool `json:"streamingMode"`
		} `json:"config"`
	}
	if json.Unmarshal([]byte(content), &card) != nil {
		return ""
	}
	text := extractInteractiveCardText(content)
	// user_card_content returns original cards, including direct nested
	// elements rather than the AST's property.elements representation.
	elements := append(card.Body.Elements, card.Elements...)
	if len(elements) > 0 {
		var parts []string
		extractContextSourceElements(elements, &parts)
		text = strings.Join(parts, "\n")
	}
	if text == "[interactive card]" {
		return ""
	}
	title := card.Header.Title.Content
	if title == "" {
		title = card.Header.Property.Title.Property.Content
	}
	if strings.TrimSpace(text) == "" || (len(elements) == 0 && text == title) {
		return "" // a title alone does not prove the card body was readable
	}
	if title != "" && text != title && !strings.HasPrefix(text, title+"\n") {
		text = title + "\n" + text
	}
	if card.Config.Streaming || card.Config.RawStreaming {
		text = "[in-progress card snapshot]\n" + text
	}
	return text
}

func extractContextSourceElements(elements []json.RawMessage, parts *[]string) {
	for _, raw := range elements {
		var node struct {
			Tag      string            `json:"tag"`
			Content  string            `json:"content"`
			Text     json.RawMessage   `json:"text"`
			Elements []json.RawMessage `json:"elements"`
			Columns  []json.RawMessage `json:"columns"`
			Fields   []json.RawMessage `json:"fields"`
		}
		if json.Unmarshal(raw, &node) != nil {
			continue
		}
		if node.Content != "" {
			*parts = append(*parts, node.Content)
		}
		if len(node.Text) > 0 {
			var text string
			if json.Unmarshal(node.Text, &text) == nil {
				if text != "" {
					*parts = append(*parts, text)
				}
			} else {
				extractContextSourceElements([]json.RawMessage{node.Text}, parts)
			}
		}
		if node.Tag == "img" {
			*parts = append(*parts, "[image: attachment not provided from history]")
		}
		extractContextSourceElements(node.Elements, parts)
		extractContextSourceElements(node.Columns, parts)
		extractContextSourceElements(node.Fields, parts)
	}
}
