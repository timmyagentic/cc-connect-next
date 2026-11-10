package feishu

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

// SendOwnerEscalation mentions the configured owner in the original group/topic.
// It never opens a private conversation or falls back to a top-level message.
// The engine owns durable claims; every transport retry keeps the same UUID.
func (p *Platform) SendOwnerEscalation(ctx context.Context, replyCtx any, notice core.OwnerEscalationNotice) error {
	if notice.OwnerID != strings.TrimSpace(notice.OwnerID) || !strings.HasPrefix(notice.OwnerID, "ou_") || len(notice.OwnerID) <= 3 || strings.ContainsAny(notice.OwnerID, " \t\r\n,*\"<>") {
		return fmt.Errorf("feishu: escalation owner must be an explicit app-scoped open_id")
	}
	rc, ok := replyCtx.(replyContext)
	if !ok || rc.chatID == "" || rc.groupID != rc.chatID || rc.messageID == "" || rc.sessionKey == "" || notice.TriggerMessageID != rc.messageID || notice.SessionKey != rc.sessionKey {
		return fmt.Errorf("feishu: escalation requires a matching original group conversation")
	}
	if notice.IdempotencyKey == "" || len(notice.IdempotencyKey) > 50 || strings.TrimSpace(notice.IdempotencyKey) != notice.IdempotencyKey || strings.TrimSpace(notice.Text) == "" || strings.TrimSpace(notice.OwnerName) == "" {
		return fmt.Errorf("feishu: escalation requires owner name, content and an idempotency key of at most 50 bytes")
	}
	if !core.AllowList(p.allowChat, rc.chatID) || p.isMessageRecalled(notice.TriggerMessageID) {
		return fmt.Errorf("feishu: escalation source is unavailable or disallowed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.requireEscalationMember(ctx, rc.chatID, notice.OwnerID); err != nil {
		return err
	}
	content := fmt.Sprintf(`<at user_id="%s">%s</at>`, notice.OwnerID, html.EscapeString(notice.OwnerName)) + "\n" + ownerEscalationText(notice.Text)
	// Use native text: post/card at-tags can render without notifying the user.
	msgType, body := buildReplyContentWithResolvedMention(content, true)
	req := larkim.NewReplyMessageReqBuilder().MessageId(notice.TriggerMessageID).Body(larkim.NewReplyMessageReqBodyBuilder().
		MsgType(msgType).Content(body).ReplyInThread(rc.threadID != "" || p.shouldReplyInThread(rc)).Uuid(notice.IdempotencyKey).Build()).Build()
	return p.withTransientRetry(ctx, "send owner escalation", func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if p.isMessageRecalled(notice.TriggerMessageID) {
			return fmt.Errorf("feishu: escalation source was recalled")
		}
		return p.withFreshTenantAccessTokenRetry(ctx, "send owner escalation", func(client *lark.Client, opts ...larkcore.RequestOptionFunc) error {
			resp, err := client.Im.Message.Reply(ctx, req, opts...)
			if err != nil {
				return err
			}
			if resp == nil {
				return fmt.Errorf("feishu: missing owner escalation response")
			}
			if !resp.Success() {
				return fmt.Errorf("feishu: owner escalation failed code=%d", resp.Code)
			}
			if resp.Data == nil || stringValue(resp.Data.MessageId) == "" {
				return fmt.Errorf("feishu: owner escalation response missing message_id")
			}
			return nil
		})
	})
}

// Read by ID instead of the name cache: duplicate display names must not hide
// the configured owner, and missing permissions must never pretend to notify.
func (p *Platform) requireEscalationMember(ctx context.Context, chatID, ownerID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if p.client == nil {
		return fmt.Errorf("feishu: escalation client is unavailable")
	}
	token, err := p.fetchFreshTenantAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("feishu: escalation member lookup unavailable")
	}
	req := larkim.NewGetChatMembersReqBuilder().ChatId(chatID).MemberIdType("open_id").PageSize(100).Build()
	iter, err := p.client.Im.ChatMembers.GetByIterator(ctx, req, larkcore.WithTenantAccessToken(token))
	if err != nil {
		return fmt.Errorf("feishu: escalation member lookup failed")
	}
	for {
		more, member, err := iter.Next()
		if err != nil {
			return fmt.Errorf("feishu: escalation member lookup failed")
		}
		if member != nil && stringValue(member.MemberId) == ownerID {
			return nil
		}
		if !more {
			break
		}
	}
	return fmt.Errorf("feishu: escalation owner is not in the original group")
}

func ownerEscalationText(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(text), "<", "＜"), ">", "＞")
	runes := []rune(text)
	if len(runes) > 6000 {
		return string(runes[:6000]) + "…"
	}
	return text
}
