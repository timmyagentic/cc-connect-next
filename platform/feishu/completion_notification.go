package feishu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

const completionReceiptRetention = 7 * 24 * time.Hour
const maxCompletionReceipts = 1000

// Receipts suppress duplicate and uncertain sends; there is no pending queue,
// timer, background worker, or restart replay. Only hashes and times persist.
type completionNotifications struct {
	mu          sync.Mutex
	receipts    map[string]time.Time
	users       map[string]struct{}
	path, scope string
	now         func() time.Time
	save        func(string, []byte) error
	ctx         context.Context
	cancel      context.CancelFunc
	fault       error
}

func completionOption(opts map[string]any, key, legacy string) (any, bool) {
	if v, ok := opts[key]; ok {
		return v, true
	}
	v, exists := opts[legacy]
	return v, exists
}

func parseCompletionNotificationOptions(opts map[string]any) (bool, map[string]struct{}, error) {
	enabled := false
	if raw, exists := completionOption(opts, "completion_notify", "unread_reminder"); exists {
		var ok bool
		enabled, ok = raw.(bool)
		if !ok {
			return false, nil, fmt.Errorf("completion_notify must be a boolean")
		}
	}
	users := map[string]struct{}{}
	if raw, exists := completionOption(opts, "completion_notify_users", "unread_reminder_users"); exists {
		var values []string
		switch v := raw.(type) {
		case []string:
			values = v
		case []any:
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					return false, nil, fmt.Errorf("completion_notify_users must be a string array")
				}
				values = append(values, s)
			}
		default:
			return false, nil, fmt.Errorf("completion_notify_users must be a string array")
		}
		for _, user := range values {
			if user != strings.TrimSpace(user) || !strings.HasPrefix(user, "ou_") || len(user) <= 3 || strings.ContainsAny(user, " \t\r\n,*") {
				return false, nil, fmt.Errorf("completion_notify_users requires explicit open_ids starting with ou_")
			}
			users[user] = struct{}{}
		}
	}
	if enabled && len(users) == 0 {
		return false, nil, fmt.Errorf("completion_notify requires non-empty completion_notify_users")
	}
	// Retain validation for the beta option, but it no longer delays delivery.
	if raw, exists := opts["unread_reminder_delay_mins"]; exists {
		var minutes float64
		switch v := raw.(type) {
		case int:
			minutes = float64(v)
		case int64:
			minutes = float64(v)
		case float64:
			minutes = v
		default:
			return false, nil, fmt.Errorf("unread_reminder_delay_mins is deprecated and must be an integer if present; remove it")
		}
		if math.IsNaN(minutes) || math.IsInf(minutes, 0) || math.Trunc(minutes) != minutes || minutes < 1 || minutes > 10079 {
			return false, nil, fmt.Errorf("unread_reminder_delay_mins is deprecated and must be between 1 and 10079 if present; remove it")
		}
	}
	return enabled, users, nil
}

func completionNotificationID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:16])
}

func (p *Platform) configureCompletionNotifications(opts map[string]any) error {
	enabled, users, err := parseCompletionNotificationOptions(opts)
	if err != nil {
		return err
	}
	for _, key := range []string{"unread_reminder", "unread_reminder_users", "unread_reminder_delay_mins"} {
		if _, exists := opts[key]; exists {
			slog.Warn(p.tag() + ": unread reminder options are deprecated; notifications are now immediate private messages, migrate to completion_notify/completion_notify_users and remove the delay")
			break
		}
	}
	if !enabled {
		return nil
	}
	dir, _ := opts["cc_data_dir"].(string)
	project, _ := opts["cc_project"].(string)
	if dir == "" || project == "" {
		return fmt.Errorf("completion_notify requires injected cc_data_dir and cc_project")
	}
	scope := completionNotificationID(project, p.platformName, p.appID, p.domain)
	p.completionNotifications = &completionNotifications{
		receipts: map[string]time.Time{}, users: users, scope: scope, now: time.Now, save: saveCompletionNotificationFile,
		path: filepath.Join(dir, "feishu-completion-notifications", scope+".json"),
	}
	return nil
}

func (p *Platform) startCompletionNotifications() error {
	q := p.completionNotifications
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ctx != nil && q.ctx.Err() == nil {
		return nil
	}
	receipts := map[string]time.Time{}
	data, err := os.ReadFile(q.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("feishu: read completion receipts: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &receipts); err != nil {
			return fmt.Errorf("feishu: decode completion receipts: %w", err)
		}
		if receipts == nil || len(receipts) > maxCompletionReceipts {
			return fmt.Errorf("feishu: invalid completion receipt store")
		}
		for id, at := range receipts {
			if decoded, err := hex.DecodeString(id); err != nil || len(decoded) != 16 || at.IsZero() {
				return fmt.Errorf("feishu: invalid completion receipt")
			}
		}
	}
	q.receipts, q.fault = receipts, nil
	q.ctx, q.cancel = context.WithCancel(context.Background())
	return nil
}

func (p *Platform) stopCompletionNotifications() {
	if q := p.completionNotifications; q != nil {
		q.mu.Lock()
		defer q.mu.Unlock()
		if q.cancel != nil {
			q.cancel()
		}
	}
}

func (p *Platform) CompletionNotificationsEnabled() bool { return p.completionNotifications != nil }

func (p *Platform) NotifyTurnCompleted(ctx context.Context, replyCtx any, notice core.TurnCompletionNotice) error {
	q := p.completionNotifications
	if q == nil {
		return nil
	}
	if _, ok := q.users[notice.UserID]; !ok || !core.AllowList(p.allowFrom, notice.UserID) {
		return nil
	}
	rc, ok := replyCtx.(replyContext)
	if !ok || rc.chatID == "" || notice.TriggerMessageID == "" || notice.SessionKey == "" {
		return fmt.Errorf("feishu: completion notification has no original conversation")
	}
	if !core.AllowList(p.allowChat, rc.chatID) || p.isMessageRecalled(notice.TriggerMessageID) {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	id := completionNotificationID(q.scope, notice.UserID, notice.SessionKey, notice.TriggerMessageID)
	q.mu.Lock()
	if q.ctx == nil || q.ctx.Err() != nil || q.fault != nil || ctx.Err() != nil {
		q.mu.Unlock()
		return fmt.Errorf("feishu: completion notifications unavailable or cancelled")
	}
	lifeCtx := q.ctx
	stop := context.AfterFunc(lifeCtx, cancel)
	defer stop()
	claimed, err := q.claimLocked(id)
	q.mu.Unlock()
	if err != nil || !claimed {
		return err
	}

	// Enrich the link with the trigger's current topic/position. A failed
	// lookup still permits a link to the original chat or known topic.
	linkCtx, stopLookup := context.WithTimeout(ctx, 3*time.Second)
	link, recalled := p.completionConversationLink(linkCtx, rc, notice.TriggerMessageID)
	stopLookup()
	if recalled || p.isMessageRecalled(notice.TriggerMessageID) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	content := completionNotificationText(notice.Text, 360) + "\n" + completionNotificationText(notice.LinkLabel, 100) + "\n" + link
	req := larkim.NewCreateMessageReqBuilder().ReceiveIdType("open_id").Body(larkim.NewCreateMessageReqBodyBuilder().
		ReceiveId(notice.UserID).MsgType("text").Content(completionNotificationContent(content)).Uuid(id).Build()).Build()
	return p.withTransientRetry(ctx, "send private completion", func() error {
		if err := lifeCtx.Err(); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return p.withFreshTenantAccessTokenRetry(ctx, "send private completion", func(client *lark.Client, opts ...larkcore.RequestOptionFunc) error {
			resp, err := client.Im.Message.Create(ctx, req, opts...)
			if err != nil {
				return err
			}
			if resp == nil {
				return fmt.Errorf("feishu: missing private completion response")
			}
			if !resp.Success() {
				return fmt.Errorf("feishu: private completion failed code=%d", resp.Code)
			}
			if resp.Data == nil || stringValue(resp.Data.MessageId) == "" {
				return fmt.Errorf("feishu: private completion response missing message_id")
			}
			return nil
		})
	})
}

func (q *completionNotifications) claimLocked(id string) (bool, error) {
	now := q.now()
	for key, created := range q.receipts {
		if now.Sub(created) >= completionReceiptRetention {
			delete(q.receipts, key)
		}
	}
	if _, exists := q.receipts[id]; exists {
		return false, nil
	}
	if len(q.receipts) >= maxCompletionReceipts {
		return false, fmt.Errorf("feishu: completion receipt store full")
	}
	q.receipts[id] = now
	data, err := json.Marshal(q.receipts)
	if err == nil {
		err = q.save(q.path, data)
	}
	if err != nil {
		q.fault = fmt.Errorf("feishu: completion receipt persistence: %w", err)
	}
	return q.fault == nil, q.fault
}

func saveCompletionNotificationFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".completion-notifications-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}

func completionNotificationText(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	// Text messages accept native mention tags; escape them so model-produced
	// summaries cannot accidentally ping additional people.
	s = strings.ReplaceAll(strings.ReplaceAll(s, "<", "＜"), ">", "＞")
	runes := []rune(s)
	if len(runes) > limit {
		s = string(runes[:limit]) + "…"
	}
	return s
}
func completionNotificationContent(text string) string {
	b, _ := json.Marshal(map[string]string{"text": text})
	return string(b)
}

func (p *Platform) completionConversationLink(ctx context.Context, rc replyContext, trigger string) (string, bool) {
	req := larkim.NewGetMessageReqBuilder().MessageId(trigger).UserIdType("open_id").Build()
	var resp *larkim.GetMessageResp
	err := p.withFreshTenantAccessTokenRetry(ctx, "get completion source", func(client *lark.Client, opts ...larkcore.RequestOptionFunc) error {
		var err error
		resp, err = client.Im.Message.Get(ctx, req, opts...)
		if err != nil {
			return err
		}
		if resp == nil {
			return fmt.Errorf("feishu: missing completion source response")
		}
		if !resp.Success() {
			return fmt.Errorf("feishu: completion source failed code=%d", resp.Code)
		}
		return nil
	})
	var position, threadPosition string
	if resp != nil && isMessageWithdrawnCode(resp.Code, resp.Msg) {
		return "", true
	}
	if err == nil && resp.Data != nil {
		for _, item := range resp.Data.Items {
			if item == nil || stringValue(item.MessageId) != trigger || (stringValue(item.ChatId) != "" && stringValue(item.ChatId) != rc.chatID) {
				continue
			}
			if item.Deleted != nil && *item.Deleted {
				return "", true
			}
			if thread := stringValue(item.ThreadId); thread != "" {
				rc.threadID = thread
			}
			position, threadPosition = stringValue(item.MessagePosition), stringValue(item.ThreadMessagePosition)
			break
		}
	} else {
		slog.Warn(p.tag() + ": completion source lookup unavailable; linking to known conversation")
	}
	host := "applink.feishu.cn"
	if p.platformName == "lark" {
		host = "applink.larksuite.com"
	}
	values := url.Values{}
	path := "/client/chat/open"
	if rc.threadID != "" {
		path = "/client/thread/open"
		values.Set("openthreadid", rc.threadID)
		values.Set("openchatid", rc.chatID)
		values.Set("open_thread_id", rc.threadID)
		values.Set("open_chat_id", rc.chatID)
		if threadPosition != "" {
			values.Set("thread_position", threadPosition)
		}
	} else {
		values.Set("openChatId", rc.chatID)
		if position != "" {
			values.Set("position", position)
		}
	}
	return "https://" + host + path + "?" + values.Encode(), false
}
