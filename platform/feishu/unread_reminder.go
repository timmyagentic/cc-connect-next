package feishu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

const reminderDelay = time.Hour
const reminderRetention = 7 * 24 * time.Hour
const maxReminderRecords = 1000
const maxReminderBatch = 20

// A claimed record is deliberately never retried after a restart, even when
// the outbound response was lost. Missing one reminder is preferable to
// repeating a private notification whose delivery is uncertain.
type unreadReminder struct {
	Link       string    `json:"-"`
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	SessionKey string    `json:"session_key"`
	ChatID     string    `json:"chat_id"`
	ThreadID   string    `json:"thread_id,omitempty"`
	TriggerID  string    `json:"trigger_id"`
	MessageID  string    `json:"message_id,omitempty"`
	Summary    string    `json:"summary"`
	Header     string    `json:"header"`
	Created    time.Time `json:"created"`
	Due        time.Time `json:"due"`
	Next       time.Time `json:"next"`
	Attempts   int       `json:"attempts"`
	State      string    `json:"state"` // creating, pending, done
}

type reminderQueue struct {
	mu      sync.Mutex
	runMu   sync.Mutex
	records map[string]unreadReminder
	path    string
	users   map[string]struct{}
	now     func() time.Time
	save    func(string, []byte) error
	cancel  context.CancelFunc
	done    chan struct{}
	fault   error // persistence failure disables sending until restart
	stopped bool
}

func parseUnreadReminderOptions(opts map[string]any) (bool, map[string]struct{}, error) {
	enabled := false
	if v, exists := opts["unread_reminder"]; exists {
		var ok bool
		enabled, ok = v.(bool)
		if !ok {
			return false, nil, fmt.Errorf("unread_reminder must be a boolean")
		}
	}
	users := map[string]struct{}{}
	if raw, exists := opts["unread_reminder_users"]; exists {
		var values []string
		switch v := raw.(type) {
		case []string:
			values = v
		case []any:
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					return false, nil, fmt.Errorf("unread_reminder_users must be a string array")
				}
				values = append(values, s)
			}
		default:
			return false, nil, fmt.Errorf("unread_reminder_users must be a string array")
		}
		for _, user := range values {
			if user != strings.TrimSpace(user) || !strings.HasPrefix(user, "ou_") || len(user) <= 3 || strings.ContainsAny(user, " \t\r\n,*") {
				return false, nil, fmt.Errorf("unread_reminder_users requires explicit open_ids starting with ou_")
			}
			users[user] = struct{}{}
		}
	}
	if enabled && len(users) == 0 {
		return false, nil, fmt.Errorf("unread_reminder requires non-empty unread_reminder_users")
	}
	return enabled, users, nil
}

func reminderID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:16])
}

func (p *Platform) configureUnreadReminders(opts map[string]any) error {
	enabled, users, err := parseUnreadReminderOptions(opts)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	dir, _ := opts["cc_data_dir"].(string)
	project, _ := opts["cc_project"].(string)
	if dir == "" || project == "" {
		return fmt.Errorf("unread_reminder requires injected cc_data_dir and cc_project")
	}
	p.reminders = &reminderQueue{records: map[string]unreadReminder{}, users: users, now: time.Now, save: saveReminderFile,
		path: filepath.Join(dir, "feishu-unread-reminders", reminderID(project, p.platformName, p.appID, p.domain)+".json")}
	return nil
}

func saveReminderFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".reminders-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
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
	defer d.Close()
	return d.Sync()
}

func (q *reminderQueue) persistLocked() error {
	data, err := json.Marshal(q.records)
	if err == nil {
		err = q.save(q.path, data)
	}
	if err != nil {
		q.fault = fmt.Errorf("feishu: unread reminder persistence: %w", err)
	}
	return q.fault
}

func (p *Platform) startUnreadReminders() error {
	q := p.reminders
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cancel != nil {
		return nil
	}
	data, err := os.ReadFile(q.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("feishu: read reminders: %w", err)
	}
	if err == nil {
		if err = json.Unmarshal(data, &q.records); err != nil {
			return fmt.Errorf("feishu: decode reminders: %w", err)
		}
		if q.records == nil || len(q.records) > maxReminderRecords {
			return fmt.Errorf("feishu: invalid reminder store")
		}
		for id, r := range q.records {
			if id != r.ID || id != reminderID(r.UserID, r.SessionKey, r.TriggerID) || r.ChatID == "" || r.Created.IsZero() ||
				(r.State != "creating" && r.State != "pending" && r.State != "done") ||
				(r.State == "pending" && (r.MessageID == "" || r.Due.Before(r.Created.Add(reminderDelay)))) {
				return fmt.Errorf("feishu: invalid reminder record")
			}
		}
	}
	q.stopped = false
	ctx, cancel := context.WithCancel(context.Background())
	q.cancel = cancel
	q.done = make(chan struct{})
	done := q.done
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		p.processUnreadReminders(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.processUnreadReminders(ctx)
			}
		}
	}()
	return nil
}

func (p *Platform) stopUnreadReminders() {
	q := p.reminders
	if q == nil {
		return
	}
	q.mu.Lock()
	q.stopped = true
	cancel, done := q.cancel, q.done
	q.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
		q.mu.Lock()
		if q.done == done {
			q.cancel = nil
			q.done = nil
		}
		q.mu.Unlock()
	}
}

func (p *Platform) UnreadRemindersEnabled() bool { return p.reminders != nil }

func (p *Platform) NotifyTurnCompleted(ctx context.Context, replyCtx any, notice core.TurnCompletionNotice) error {
	q := p.reminders
	if q == nil {
		return nil
	}
	if _, ok := q.users[notice.UserID]; !ok || !core.AllowList(p.allowFrom, notice.UserID) {
		return nil
	}
	rc, ok := replyCtx.(replyContext)
	if !ok || rc.chatID == "" || rc.messageID == "" || notice.TriggerMessageID == "" || notice.SessionKey == "" {
		return fmt.Errorf("feishu: completion notice has no original message target")
	}
	if !core.AllowList(p.allowChat, rc.chatID) {
		return nil
	}
	if p.isMessageRecalled(notice.TriggerMessageID) {
		return nil
	}
	id := reminderID(notice.UserID, notice.SessionKey, notice.TriggerMessageID)
	q.mu.Lock()
	if q.fault != nil || q.stopped {
		q.mu.Unlock()
		return fmt.Errorf("feishu: reminder queue unavailable")
	}
	if _, exists := q.records[id]; exists {
		q.mu.Unlock()
		return nil
	}
	if len(q.records) >= maxReminderRecords {
		q.mu.Unlock()
		return fmt.Errorf("feishu: reminder queue full")
	}
	now := q.now()
	r := unreadReminder{ID: id, UserID: notice.UserID, SessionKey: notice.SessionKey, ChatID: rc.chatID, ThreadID: rc.threadID, TriggerID: notice.TriggerMessageID, Summary: reminderText(notice.Summary, 240), Header: reminderText(notice.ReminderText, 240), Created: now, State: "creating"}
	q.records[id] = r
	err := q.persistLocked()
	q.mu.Unlock()
	if err != nil {
		return err
	}
	// Always use a fresh message. A previously read streaming card does not
	// prove that the user saw the final answer written into that same card.
	body := p.buildReplyMessageReqBody(rc, "text", reminderContent(reminderText(notice.Text, 360)))
	if rc.threadID != "" {
		b := true
		body.ReplyInThread = &b
	}
	body.Uuid = &id
	messageID, err := p.replyMessageWithBody(ctx, rc.messageID, body)
	q.mu.Lock()
	defer q.mu.Unlock()
	if err != nil {
		q.records[id] = r
		return err
	} // uncertain creation stays a tombstone
	if messageID == "" {
		return fmt.Errorf("feishu: completion response missing message_id")
	}
	current := q.records[id]
	if current.State != "creating" || q.stopped {
		return nil
	} // continuation/recall won the race
	r.MessageID = messageID
	r.Due = q.now().Add(reminderDelay)
	r.Next = r.Due
	r.State = "pending"
	q.records[id] = r
	return q.persistLocked()
}

func reminderText(s string, limit int) string {
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
func reminderContent(text string) string {
	b, _ := json.Marshal(map[string]string{"text": text})
	return string(b)
}

func (p *Platform) ObserveReminderMessage(_ context.Context, msg *core.Message) error {
	q := p.reminders
	if q == nil || msg == nil {
		return nil
	}
	rc, _ := msg.ReplyCtx.(replyContext)
	q.mu.Lock()
	defer q.mu.Unlock()
	changed := false
	for id, r := range q.records {
		if r.State == "done" {
			continue
		}
		recalled := msg.Recalled && (msg.MessageID == r.TriggerID || msg.MessageID == r.MessageID)
		// In topic_per_message the first root predates its server thread_id.
		// Its isolated root session is already a precise topic boundary.
		sameTopic := rc.threadID == r.ThreadID || (p.threadIsolationEnabled() && r.ThreadID == "" && rc.threadID != "" && isThreadSessionKey(r.SessionKey))
		continued := !msg.Recalled && msg.UserID == r.UserID && msg.SessionKey == r.SessionKey && msg.MessageID != r.TriggerID && msg.MessageID != "" &&
			rc.chatID == r.ChatID && sameTopic && core.AllowList(p.allowFrom, msg.UserID) && core.AllowList(p.allowChat, rc.chatID) &&
			(msg.UserMessageTimeMs == 0 || msg.UserMessageTimeMs >= r.Created.UnixMilli())
		if recalled || continued {
			r.State = "done"
			q.records[id] = r
			changed = true
		}
	}
	if changed {
		return q.persistLocked()
	}
	return nil
}

func (p *Platform) processUnreadReminders(parent context.Context) {
	q := p.reminders
	if q == nil {
		return
	}
	q.runMu.Lock()
	defer q.runMu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	q.mu.Lock()
	if q.fault != nil || q.stopped {
		q.mu.Unlock()
		return
	}
	now := q.now()
	var due []unreadReminder
	changed := false
	for id, r := range q.records {
		if now.Sub(r.Created) >= reminderRetention {
			delete(q.records, id)
			changed = true
			continue
		}
		if _, ok := q.users[r.UserID]; !ok || !core.AllowList(p.allowFrom, r.UserID) || !core.AllowList(p.allowChat, r.ChatID) {
			delete(q.records, id)
			changed = true
			continue
		}
		if r.State == "pending" && !now.Before(r.Due) && !now.Before(r.Next) {
			due = append(due, r)
		}
	}
	if changed && q.persistLocked() != nil {
		q.mu.Unlock()
		return
	}
	q.mu.Unlock()
	sort.Slice(due, func(i, j int) bool { return due[i].Due.Before(due[j].Due) })
	if len(due) > maxReminderBatch {
		due = due[:maxReminderBatch]
	}
	batches := map[string][]unreadReminder{}
	for _, r := range due {
		if ctx.Err() != nil {
			break
		}
		read, err := p.reminderResolved(ctx, &r)
		q.mu.Lock()
		current, exists := q.records[r.ID]
		if !exists || current.State != "pending" || q.fault != nil || q.stopped {
			q.mu.Unlock()
			continue
		}
		if err != nil {
			current.Attempts++
			if current.Attempts >= 6 {
				current.State = "done"
			} else {
				current.Next = now.Add(time.Minute * time.Duration(1<<current.Attempts))
			}
			q.records[r.ID] = current
			if saveErr := q.persistLocked(); saveErr != nil {
				slog.Warn("feishu: unread reminder paused after storage failure")
			}
			slog.Warn("feishu: unread reminder check failed; no reminder sent", "attempt", current.Attempts)
		} else if read {
			current.State = "done"
			q.records[r.ID] = current
			_ = q.persistLocked()
		} else {
			batches[r.UserID] = append(batches[r.UserID], r)
		}
		q.mu.Unlock()
	}
	for user, records := range batches {
		p.sendReminderBatch(ctx, user, records)
	}
}

func (p *Platform) reminderResolved(ctx context.Context, r *unreadReminder) (bool, error) {
	gone, err := p.IsMessageRecalled(ctx, replyContext{messageID: r.TriggerID})
	if err != nil || gone {
		return gone, err
	}
	link, gone, err := p.reminderMessageLink(ctx, *r)
	if err != nil || gone {
		return gone, err
	}
	r.Link = link
	token := ""
	seen := map[string]bool{}
	for page := 0; page < 1000; page++ {
		req := larkim.NewReadUsersMessageReqBuilder().MessageId(r.MessageID).UserIdType("open_id").PageSize(100).PageToken(token).Build()
		var resp *larkim.ReadUsersMessageResp
		err := p.withFreshTenantAccessTokenRetry(ctx, "read completion readers", func(client *lark.Client, opts ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Im.Message.ReadUsers(ctx, req, opts...)
			if err != nil {
				return err
			}
			if resp == nil {
				return fmt.Errorf("feishu: missing completion readers response")
			}
			if !resp.Success() {
				return fmt.Errorf("feishu: completion readers failed code=%d", resp.Code)
			}
			return nil
		})
		if err != nil {
			return false, err
		}
		if resp.Data == nil || resp.Data.HasMore == nil {
			return false, fmt.Errorf("feishu: incomplete reader response")
		}
		for _, reader := range resp.Data.Items {
			if reader != nil && stringValue(reader.UserId) == r.UserID {
				return true, nil
			}
		}
		if !*resp.Data.HasMore {
			return false, nil
		}
		token = stringValue(resp.Data.PageToken)
		if token == "" || seen[token] {
			return false, fmt.Errorf("feishu: incomplete reader pagination")
		}
		seen[token] = true
	}
	return false, fmt.Errorf("feishu: reader pagination limit exceeded")
}

func (p *Platform) sendReminderBatch(ctx context.Context, user string, records []unreadReminder) {
	q := p.reminders
	q.mu.Lock()
	if q.fault != nil || q.stopped || ctx.Err() != nil {
		q.mu.Unlock()
		return
	}
	var selected []unreadReminder
	for _, r := range records {
		if current, ok := q.records[r.ID]; ok && current.State == "pending" {
			current.State = "done"
			q.records[r.ID] = current
			selected = append(selected, r)
		}
	}
	if len(selected) == 0 {
		q.mu.Unlock()
		return
	}
	if q.persistLocked() != nil {
		q.mu.Unlock()
		return
	}
	q.mu.Unlock()
	lines := []string{selected[0].Header}
	ids := []string{user}
	for _, r := range selected {
		lines = append(lines, "• "+r.Summary+"\n"+r.Link)
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	uuid := reminderID(ids...)
	req := larkim.NewCreateMessageReqBuilder().ReceiveIdType("open_id").Body(larkim.NewCreateMessageReqBodyBuilder().ReceiveId(user).MsgType("text").Content(reminderContent(strings.Join(lines, "\n"))).Uuid(uuid).Build()).Build()
	err := p.withTransientRetry(ctx, "send unread reminder", func() error {
		return p.withFreshTenantAccessTokenRetry(ctx, "send unread reminder", func(client *lark.Client, opts ...larkcore.RequestOptionFunc) error {
			resp, err := client.Im.Message.Create(ctx, req, opts...)
			if err != nil {
				return err
			}
			if resp == nil {
				return fmt.Errorf("feishu: missing private reminder response")
			}
			if !resp.Success() {
				return fmt.Errorf("feishu: private reminder failed code=%d", resp.Code)
			}
			return nil
		})
	})
	if err != nil {
		slog.Warn("feishu: private reminder attempt failed; will not resend after uncertain delivery")
	}
}

// Feishu's message-link token is not a message_id. Use the documented chat
// position/topic applinks instead; missing position still opens the right chat.
func (p *Platform) reminderMessageLink(ctx context.Context, r unreadReminder) (string, bool, error) {
	if p.isMessageRecalled(r.MessageID) {
		return "", true, nil
	}
	req := larkim.NewGetMessageReqBuilder().MessageId(r.MessageID).UserIdType("open_id").Build()
	var resp *larkim.GetMessageResp
	err := p.withFreshTenantAccessTokenRetry(ctx, "get completion message", func(client *lark.Client, opts ...larkcore.RequestOptionFunc) error {
		var err error
		resp, err = client.Im.Message.Get(ctx, req, opts...)
		if err != nil {
			return err
		}
		if resp == nil {
			return fmt.Errorf("feishu: missing completion message response")
		}
		if !resp.Success() {
			return fmt.Errorf("feishu: completion message failed code=%d", resp.Code)
		}
		return nil
	})
	if err != nil {
		if resp != nil && isMessageWithdrawnCode(resp.Code, resp.Msg) {
			return "", true, nil
		}
		return "", false, err
	}
	if resp.Data == nil {
		return "", false, fmt.Errorf("feishu: missing completion message data")
	}
	if len(resp.Data.Items) == 0 {
		return "", true, nil
	}
	var item *larkim.Message
	for _, candidate := range resp.Data.Items {
		if candidate != nil && stringValue(candidate.MessageId) == r.MessageID {
			item = candidate
			break
		}
	}
	if item == nil {
		return "", false, fmt.Errorf("feishu: missing completion message")
	}
	if item.Deleted != nil && *item.Deleted {
		return "", true, nil
	}
	host := "applink.feishu.cn"
	if p.platformName == "lark" {
		host = "applink.larksuite.com"
	}
	values := url.Values{}
	path := "/client/chat/open"
	thread := r.ThreadID
	if v := stringValue(item.ThreadId); v != "" {
		thread = v
	}
	if thread != "" {
		path = "/client/thread/open"
		values.Set("openthreadid", thread)
		values.Set("openchatid", r.ChatID)
		values.Set("open_thread_id", thread)
		values.Set("open_chat_id", r.ChatID)
		if pos := stringValue(item.ThreadMessagePosition); pos != "" {
			values.Set("thread_position", pos)
		}
	} else {
		values.Set("openChatId", r.ChatID)
		if pos := stringValue(item.MessagePosition); pos != "" {
			values.Set("position", pos)
		}
	}
	return "https://" + host + path + "?" + values.Encode(), false, nil
}
