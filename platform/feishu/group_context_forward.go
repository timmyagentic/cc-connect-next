package feishu

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

const (
	groupContextForwardRequests = 4
	groupContextForwardMessages = 100
	groupContextForwardDepth    = 4
	groupContextForwardTimeout  = 3 * time.Second
)

func (p *Platform) expandGroupContextForwards(ctx context.Context, target replyContext, entries []core.ContextMessage, forwardIDs map[string]bool) {
	ctx, cancel := context.WithTimeout(ctx, groupContextForwardTimeout)
	defer cancel()
	requests, remaining := 0, groupContextForwardMessages
	// Match core's newest-first text selection when source reads are bounded.
	for i := len(entries) - 1; i >= 0; i-- {
		entry := &entries[i]
		if !forwardIDs[entry.ID] {
			continue
		}
		entry.Text = "[forwarded messages]"
		if ctx.Err() != nil || requests >= groupContextForwardRequests || remaining <= 0 {
			contextContentUnavailable(entry, "forwarded messages", "expansion budget or timeout")
			continue
		}
		requests++
		resp, err := p.client.Im.Message.Get(ctx, larkim.NewGetMessageReqBuilder().
			MessageId(entry.ID).WithSenderName(true).CardMsgContentType("user_card_content").Build())
		if err != nil || resp == nil || !resp.Success() || resp.Data == nil || len(resp.Data.Items) == 0 {
			contextContentUnavailable(entry, "forwarded messages", "source read failed")
			continue
		}
		// Get returns the root plus a flat tree keyed by upper_message_id.
		// Only descend that tree; never fetch child IDs or their original chats.
		children := make(map[string][]*larkim.Message)
		rootInvalid := false
		limit := remaining + 1 // allow the root container
		for n, item := range resp.Data.Items {
			if n >= limit {
				contextContentUnavailable(entry, "forwarded messages", "message count limit")
				break
			}
			if item == nil {
				contextContentUnavailable(entry, "forwarded messages", "malformed child")
				continue
			}
			id := stringValue(item.MessageId)
			if id == entry.ID {
				updated, updateErr := strconv.ParseInt(stringValue(item.UpdateTime), 10, 64)
				if (item.Deleted != nil && *item.Deleted) ||
					(stringValue(item.ChatId) != "" && stringValue(item.ChatId) != target.chatID) ||
					(stringValue(item.ThreadId) != "" && stringValue(item.ThreadId) != target.threadID) ||
					(item.UpdateTime != nil && (updateErr != nil || updated > target.contextTimeMs)) {
					rootInvalid = true
				}
				continue
			}
			parent := stringValue(item.UpperMessageId)
			if parent == "" {
				parent = entry.ID
			}
			children[parent] = append(children[parent], item)
		}
		if rootInvalid || p.isMessageRecalled(entry.ID) {
			contextContentUnavailable(entry, "forwarded messages", "source no longer matches snapshot")
			continue
		}
		if len(children[entry.ID]) == 0 {
			contextContentUnavailable(entry, "forwarded messages", "no readable children")
			continue
		}
		visited := map[string]bool{entry.ID: true}
		p.appendGroupContextForward(ctx, entry, entry.ID, children, visited, 0, &remaining)
		boundGroupContextText(entry, p.groupContext.maxChars)
	}
}

func (p *Platform) appendGroupContextForward(ctx context.Context, entry *core.ContextMessage, parent string, children map[string][]*larkim.Message, visited map[string]bool, depth int, remaining *int) {
	for _, item := range children[parent] {
		if ctx.Err() != nil || depth >= groupContextForwardDepth || *remaining <= 0 || len([]rune(entry.Text)) >= maxGroupContextChars {
			contextContentUnavailable(entry, "forwarded messages", "expansion limit")
			return
		}
		*remaining--
		id := stringValue(item.MessageId)
		if item.Deleted != nil && *item.Deleted || p.isMessageRecalled(id) {
			continue // deleting a container also excludes its descendants
		}
		if id == "" || visited[id] {
			contextContentUnavailable(entry, "forwarded messages", "missing or repeated child ID")
			continue
		}
		visited[id] = true
		sender, senderID, senderType := "", "", ""
		if item.Sender != nil {
			sender, senderID, senderType = stringValue(item.Sender.SenderName), stringValue(item.Sender.Id), stringValue(item.Sender.SenderType)
		}
		if senderType == "app" {
			if !p.groupContext.includePeerBots || senderID == p.appID || p.peerBots[senderID] == "" {
				continue
			}
			sender = p.peerBots[senderID]
		}
		if sender == "" {
			sender = senderID
		}
		if sender == "" {
			sender = "unknown sender"
		}
		// Preserve the API's supplied sender name and time, including old
		// original messages intentionally forwarded into the current window.
		at := stringValue(item.CreateTime)
		if ms, err := strconv.ParseInt(at, 10, 64); err == nil {
			at = time.UnixMilli(ms).UTC().Format(time.RFC3339Nano)
		}
		entry.Text += fmt.Sprintf("\n%s[%s] %s (%s): ", strings.Repeat("  ", depth), at, sender, id)
		if stringValue(item.MsgType) == "merge_forward" {
			entry.Text += "[nested forwarded messages]"
			if len(children[id]) == 0 {
				contextContentUnavailable(entry, "forwarded messages", "nested source absent")
			} else {
				p.appendGroupContextForward(ctx, entry, id, children, visited, depth+1, remaining)
			}
			continue
		}
		raw := ""
		if item.Body != nil {
			raw = stringValue(item.Body.Content)
		}
		child := parseGroupContextContent(stringValue(item.MsgType), raw, entry.ID)
		child.Text = replaceContextMentions(child.Text, item.Mentions)
		offset := contextImageCount(entry.Resources)
		var replacements []string
		for n := 1; n <= contextImageCount(child.Resources); n++ {
			for _, suffix := range []string{"]", ": "} {
				replacements = append(replacements, fmt.Sprintf("[image #%d%s", n, suffix), fmt.Sprintf("[image #%d%s", offset+n, suffix))
			}
		}
		if len(replacements) > 0 {
			child.Text = strings.NewReplacer(replacements...).Replace(child.Text)
		}
		entry.Text += child.Text
		entry.Incomplete = entry.Incomplete || child.Incomplete
		available := groupContextMaxResources - len(entry.Resources)
		if len(child.Resources) > available {
			child.Resources = child.Resources[:available]
			contextContentUnavailable(entry, "attachments", "resource count limit")
		}
		entry.Resources = append(entry.Resources, child.Resources...)
	}
}
