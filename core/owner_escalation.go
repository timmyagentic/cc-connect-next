package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// OwnerEscalationConfig is opt-in and independent of the foreground agent.
type OwnerEscalationConfig struct {
	Enabled        bool   `toml:"enabled"`
	OwnerID        string `toml:"owner_id"`
	OwnerName      string `toml:"owner_name"`
	BaseURL        string `toml:"base_url"`
	APIKey         string `toml:"api_key"`
	Model          string `toml:"model"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
}

func NormalizeOwnerEscalationConfig(c OwnerEscalationConfig) (OwnerEscalationConfig, error) {
	if c.BaseURL == "" {
		c.BaseURL = "https://api.openai.com/v1"
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.Model == "" {
		c.Model = "gpt-6-luna"
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 20
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 120 {
		return c, errors.New("owner_escalation.timeout_seconds must be between 1 and 120")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return c, errors.New("owner_escalation.base_url must be an absolute HTTP(S) URL without credentials, query or fragment")
	}
	if ip := net.ParseIP(u.Hostname()); u.Scheme == "http" && u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return c, errors.New("owner_escalation.base_url requires HTTPS except on loopback")
	}
	if c.Enabled && (c.OwnerID == "" || strings.ContainsAny(c.OwnerID, " \t\r\n,*") || strings.TrimSpace(c.OwnerName) == "" || strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(c.Model) == "") {
		return c, errors.New("owner_escalation requires one explicit owner_id, owner_name, api_key and model")
	}
	return c, nil
}

// OwnerEscalationSender sends a real mention in the original conversation.
// Nil means the platform confirmed the message; never silently skip or DM.
type OwnerEscalationSender interface {
	SendOwnerEscalation(context.Context, any, OwnerEscalationNotice) error
}

type OwnerEscalationNotice struct {
	OwnerID          string
	OwnerName        string
	TriggerMessageID string
	SessionKey       string
	IdempotencyKey   string
	Text             string
}

type escalationToolResult struct {
	Name     string `json:"name"`
	Result   string `json:"result"`
	Status   string `json:"status,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Success  *bool  `json:"success,omitempty"`
}

type escalationInput struct {
	OwnerName string                 `json:"owner_name"`
	History   []HistoryEntry         `json:"recent_conversation"`
	Tools     []escalationToolResult `json:"execution_results"`
}

type escalationDecision struct {
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	Summary     string `json:"summary"`
	OwnerAction string `json:"owner_action"`
}

type ownerEscalation struct {
	config   OwnerEscalationConfig
	judge    func(context.Context, escalationInput) (escalationDecision, error)
	mu       sync.Mutex
	receipts map[string]time.Time
	path     string
}

// SetOwnerEscalation is a startup-only setter, like other engine configuration.
func (e *Engine) SetOwnerEscalation(c OwnerEscalationConfig) error {
	c, err := NormalizeOwnerEscalationConfig(c)
	if err != nil {
		return err
	}
	if !c.Enabled {
		e.ownerEscalation = nil
		return nil
	}
	q := &ownerEscalation{config: c, receipts: map[string]time.Time{}}
	if path := e.sessions.StorePath(); path != "" {
		q.path = path + ".owner-escalation.json"
		data, err := os.ReadFile(q.path)
		if err != nil && !os.IsNotExist(err) {
			return errors.New("owner escalation receipts unreadable")
		}
		if err == nil {
			if json.Unmarshal(data, &q.receipts) != nil || q.receipts == nil || len(q.receipts) > 1000 {
				return errors.New("invalid owner escalation receipts")
			}
			for key, at := range q.receipts {
				if decoded, err := hex.DecodeString(key); err != nil || len(decoded) != 16 || at.IsZero() {
					return errors.New("invalid owner escalation receipt")
				}
			}
		}
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	q.judge = func(ctx context.Context, in escalationInput) (escalationDecision, error) {
		return judgeOwnerEscalation(ctx, client, c, in)
	}
	e.ownerEscalation = q
	return nil
}

// Claim before sending. Uncertain platform failures are not automatically
// replayed after restart. The platform receives this same stable UUID on retry.
func (q *ownerEscalation) claim(id string) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	for key, at := range q.receipts {
		if now.Sub(at) >= 7*24*time.Hour {
			delete(q.receipts, key)
		}
	}
	if _, exists := q.receipts[id]; exists {
		return false, nil
	}
	if len(q.receipts) >= 1000 {
		return false, errors.New("owner escalation receipt store full")
	}
	q.receipts[id] = now
	if q.path != "" {
		data, err := json.Marshal(q.receipts)
		if err == nil {
			err = AtomicWriteFile(q.path, data, 0600)
		}
		if err != nil {
			return false, errors.New("owner escalation receipt could not be persisted")
		}
	}
	return true, nil
}

func escalationText(s string, limit int) string {
	return truncateIf(redactFeedbackText(s), limit)
}

func (t *turnProcessor) captureEscalationTool(event Event) {
	if t.engine.ownerEscalation == nil {
		return
	}
	result := event.ToolResult
	if result == "" {
		result = event.Content
	}
	t.escalationTools = append(t.escalationTools, escalationToolResult{Name: escalationText(event.ToolName, 100), Result: escalationText(result, 1500), Status: escalationText(event.ToolStatus, 100), ExitCode: event.ToolExitCode, Success: event.ToolSuccess})
	if len(t.escalationTools) > 8 {
		t.escalationTools = t.escalationTools[len(t.escalationTools)-8:]
	}
}

func (c *turnCompletion) escalateOwner(response preparedTurnResponse) {
	t, e := c.processor, c.processor.engine
	tools := t.escalationTools
	t.escalationTools = nil
	q := e.ownerEscalation
	if q == nil || response.isSilent || c.event.Error != nil || c.msgID == "" {
		return
	}
	sender, ok := c.platform.(OwnerEscalationSender)
	if !ok {
		return
	}
	t.state.mu.Lock()
	userID, key := t.state.currentReplyUserID, t.state.currentSessionKey
	t.state.mu.Unlock()
	if userID == "" || key == "" || userID == q.config.OwnerID || t.state.isStopped() || t.state.shouldDiscardTurn(c.msgID) {
		return
	}
	history := t.session.GetHistory(12)
	for i := range history {
		history[i].Content = escalationText(history[i].Content, 4000)
	}
	ctx, cancel := context.WithCancel(c.terminalDeliveryCtx)
	defer cancel()
	// Recall/stop can occur during the judge call, after the ordinary answer.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-c.stop:
			cancel()
		case <-c.silentCancel:
			cancel()
		case <-done:
		case <-ctx.Done():
		}
	}()
	judgeCtx, stopJudge := context.WithTimeout(ctx, time.Duration(q.config.TimeoutSeconds)*time.Second)
	decision, err := q.judge(judgeCtx, escalationInput{OwnerName: q.config.OwnerName, History: history, Tools: tools})
	stopJudge()
	if err == nil {
		err = validateEscalationDecision(decision)
	}
	if err != nil {
		slog.Warn("owner escalation judgment failed", "message_id", c.msgID, "error", err)
		return
	}
	slog.Info("owner escalation judgment", "message_id", c.msgID, "status", decision.Status)
	if decision.Status != "needs_owner" || ctx.Err() != nil || t.state.isStopped() || t.state.shouldDiscardTurn(c.msgID) {
		return
	}
	hash := sha256.Sum256([]byte(e.name + "\x00" + c.platform.Name() + "\x00" + q.config.OwnerID + "\x00" + key + "\x00" + c.msgID))
	id := hex.EncodeToString(hash[:16])
	claimed, err := q.claim(id)
	if err != nil {
		slog.Warn("owner escalation claim failed", "message_id", c.msgID, "error", err)
		return
	}
	if !claimed {
		return
	}
	text := e.i18n.Tf(MsgOwnerEscalationRequest, escalationText(decision.Summary, 500), escalationText(decision.OwnerAction, 500))
	notice := OwnerEscalationNotice{OwnerID: q.config.OwnerID, OwnerName: q.config.OwnerName, TriggerMessageID: c.msgID, SessionKey: key, IdempotencyKey: id, Text: text}
	sendCtx, stopSend := context.WithTimeout(ctx, 15*time.Second)
	err = sender.SendOwnerEscalation(sendCtx, c.replyCtx, notice)
	stopSend()
	if err != nil {
		slog.Warn("owner escalation mention failed", "message_id", c.msgID, "error", redactFeedbackText(err.Error()))
		if ctx.Err() == nil && !t.state.isStopped() && !t.state.shouldDiscardTurn(c.msgID) {
			failureCtx, stopFailure := context.WithTimeout(ctx, 5*time.Second)
			defer stopFailure()
			if err := c.platform.Reply(failureCtx, c.replyCtx, e.i18n.Tf(MsgOwnerEscalationFailed, q.config.OwnerName)); err != nil {
				slog.Warn("owner escalation failure reply unavailable", "message_id", c.msgID)
			}
		}
		return
	}
	t.session.AddHistory("assistant", fmt.Sprintf("@%s %s", q.config.OwnerName, text))
	t.sessions.Save()
}
