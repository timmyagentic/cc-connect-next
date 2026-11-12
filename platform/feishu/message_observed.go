package feishu

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

var _ core.MessageObservationSource = (*Platform)(nil)

func (p *Platform) SetMessageObserver(observer func(core.HookEvent) error) {
	p.mu.Lock()
	p.messageObserver = observer
	p.mu.Unlock()
}

func (p *Platform) observeMessage(msg *larkim.EventMessage, sender *larkim.EventSender) error {
	p.mu.RLock()
	observer := p.messageObserver
	p.mu.RUnlock()
	if observer == nil || sender == nil || stringValue(sender.SenderType) != "user" || sender.SenderId == nil {
		return nil
	}
	userID := stringValue(sender.SenderId.OpenId)
	chatID, chatType := stringValue(msg.ChatId), stringValue(msg.ChatType)
	messageID := stringValue(msg.MessageId)
	if userID == "" || chatID == "" || messageID == "" || !core.AllowList(p.allowFrom, userID) {
		return nil
	}
	if (chatType != "group" && chatType != "p2p") || (chatType == "group" && !core.AllowList(p.allowChat, chatID)) ||
		(chatType == "p2p" && p.groupOnly) {
		return nil
	}
	mentioned := make([]string, 0, len(msg.Mentions))
	seen := make(map[string]bool)
	for _, mention := range msg.Mentions {
		if mention == nil || mention.Id == nil {
			continue
		}
		id := stringValue(mention.Id.OpenId)
		if strings.HasPrefix(id, "ou_") && len(id) > 3 && !seen[id] {
			mentioned = append(mentioned, id)
			seen[id] = true
		}
	}
	if len(mentioned) == 0 {
		return nil
	}
	createdAt, err := strconv.ParseInt(stringValue(msg.CreateTime), 10, 64)
	if err != nil || createdAt <= 0 {
		return fmt.Errorf("%s: observed message has invalid creation time", p.platformName)
	}
	err = observer(core.HookEvent{
		Event: core.HookEventMessageObserved, Platform: p.platformName, UserID: userID,
		Extra: map[string]any{
			"message_id": messageID, "chat_id": chatID, "chat_type": chatType, "created_at_ms": createdAt,
			"thread_id": stringValue(msg.ThreadId), "root_id": stringValue(msg.RootId),
			"parent_id": stringValue(msg.ParentId), "sender_type": "user", "mentioned_user_ids": mentioned,
		},
	})
	if err != nil {
		return fmt.Errorf("%s: observe message: %w", p.platformName, err)
	}
	return nil
}

// Run every project sharing the transport, then let the SDK retry when any
// observer failed. Successful consumers receive duplicates and must deduplicate.
func (p *Platform) onSharedMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	var errs []error
	for _, sibling := range p.sharedGroup.allPlatforms() {
		if err := sibling.onMessage(ctx, event); err != nil {
			slog.Error("shared ws: onMessage error", "err", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
