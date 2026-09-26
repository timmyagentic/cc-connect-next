package feishu

import (
	"context"
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
	enabled     bool
	maxMessages int
	minutes     int
	maxChars    int
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
	if cfg.enabled {
		chats, err := parseGroupReplyAllChats(opts["group_reply_all_chats"])
		if err != nil {
			return cfg, err
		}
		all, _ := opts["group_reply_all"].(bool)
		requireMention, mentionSet := opts["require_mention"].(bool)
		everyone, _ := opts["respond_to_at_everyone_and_here"].(bool)
		if all || len(chats) > 0 || (mentionSet && !requireMention) || everyone {
			return cfg, fmt.Errorf("group_context requires explicit bot mentions: disable group_reply_all, group_reply_all_chats, require_mention=false, and respond_to_at_everyone_and_here")
		}
	}
	return cfg, nil
}

// MessageContextEnabled is false for private, synthetic, and proactive
// targets. The live event path sets contextEligible only after mention and
// access checks; persisting a reply target does not preserve that authority.
func (p *Platform) MessageContextEnabled(target any) bool {
	rctx, ok := target.(replyContext)
	return ok && p.groupContext.enabled && rctx.contextEligible && rctx.chatID != "" && core.AllowList(p.allowChat, rctx.chatID)
}

func (p *Platform) RuntimeCapabilityAvailability(_ string, target any) map[string]core.CapabilityAvailability {
	availability := core.CapabilityAvailability{
		State: core.CapabilityConditional, Reason: "Read-only context is enabled; requires an authorized group mention and existing message-history access. Reads happen only before a turn; no new permissions are requested.",
		ReasonZH: "已启用只读上下文；要求授权群中的明确 @ 和已有历史读取权限。仅在回合开始前读取，不申请新权限。",
	}
	if !p.groupContext.enabled {
		availability = core.CapabilityAvailability{State: core.CapabilityUnavailable, Reason: "Recent conversation context is disabled in platform configuration.", ReasonZH: "平台配置未启用近期讨论上下文。"}
	} else if target != nil && !p.MessageContextEnabled(target) {
		availability = core.CapabilityAvailability{State: core.CapabilityUnavailable, Reason: "This target is not a verified group mention.", ReasonZH: "当前目标不是已核验的群内 @ 消息。"}
	}
	return map[string]core.CapabilityAvailability{"recent_conversation_context": availability}
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
	if id == rctx.messageID || (item.Sender != nil && stringValue(item.Sender.SenderType) == "app") {
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
		stringValue(item.Sender.SenderType) != "user" || stringValue(item.Sender.Id) == "" {
		return core.ContextMessage{}, false, true
	}
	at := time.UnixMilli(ms).UTC()
	if at.Before(start) || at.After(cutoff) {
		return core.ContextMessage{}, false, false
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
	default:
		// Never download another participant's attachments implicitly.
		return core.ContextMessage{}, false, true
	}
	if strings.TrimSpace(text) == "" {
		return core.ContextMessage{}, false, false
	}
	runes := []rune(text)
	if len(runes) > p.groupContext.maxChars {
		text = string(runes[:p.groupContext.maxChars])
		incomplete = true
	}
	return core.ContextMessage{ID: id, SenderID: stringValue(item.Sender.Id), SenderName: stringValue(item.Sender.SenderName), Time: at, ReplyTo: stringValue(item.ParentId), Text: text}, true, incomplete
}
