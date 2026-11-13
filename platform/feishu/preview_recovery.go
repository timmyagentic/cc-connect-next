package feishu

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

var _ core.PersistentPreviewPlatform = (*interactivePlatform)(nil)

type previewRecoveryIdentity struct {
	Version  int    `json:"version"`
	Platform string `json:"platform"`
	AppID    string `json:"app_id"`
	Project  string `json:"project"`
}

type previewRecoverySnapshot struct {
	previewRecoveryIdentity
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
	CardID    string `json:"card_id,omitempty"`
	Sequence  int    `json:"sequence"`
}

func (p *interactivePlatform) previewRecoveryIdentity() previewRecoveryIdentity {
	return previewRecoveryIdentity{Version: 1, Platform: p.Name(), AppID: p.appID, Project: p.boundProjectName()}
}

func (p *interactivePlatform) CheckpointPreview(handle any, save func(json.RawMessage) error) error {
	h, ok := handle.(*feishuPreviewHandle)
	if !ok || h == nil || save == nil {
		return fmt.Errorf("%s: invalid preview checkpoint target", p.tag())
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	identity := p.previewRecoveryIdentity()
	if err := validatePreviewRecoveryHandle(h); err != nil {
		return fmt.Errorf("%s: %w", p.tag(), err)
	}
	h.checkpoint = func() error {
		snapshot, err := json.Marshal(previewRecoverySnapshot{
			previewRecoveryIdentity: identity, MessageID: h.messageID, ChatID: h.chatID, CardID: h.cardID, Sequence: h.sequence,
		})
		if err != nil {
			return err
		}
		return save(snapshot)
	}
	return h.checkpoint()
}

func (p *interactivePlatform) RestorePreview(snapshot json.RawMessage, save func(json.RawMessage) error) (any, error) {
	var saved previewRecoverySnapshot
	if err := json.Unmarshal(snapshot, &saved); err != nil {
		return nil, fmt.Errorf("%s: decode preview checkpoint: %w", p.tag(), err)
	}
	if saved.previewRecoveryIdentity != p.previewRecoveryIdentity() {
		return nil, fmt.Errorf("%s: preview checkpoint belongs to a different platform, app, project or version", p.tag())
	}
	h := &feishuPreviewHandle{messageID: saved.MessageID, chatID: saved.ChatID, cardID: saved.CardID, sequence: saved.Sequence}
	if err := p.CheckpointPreview(h, save); err != nil {
		return nil, err
	}
	return h, nil
}

func validatePreviewRecoveryHandle(h *feishuPreviewHandle) error {
	if !validRecoveryID(h.messageID) || !validRecoveryID(h.chatID) ||
		(h.cardID != "" && !validRecoveryID(h.cardID)) || h.sequence < 0 || h.sequence == math.MaxInt {
		return fmt.Errorf("invalid preview checkpoint routing or sequence")
	}
	return nil
}

// Reserve before the HTTP call, not after its response: a lost response can
// still mean that CardKit accepted this sequence. Skipping a number is safe;
// reusing one after process restart is not.
func (h *feishuPreviewHandle) reserveSequenceLocked() error {
	if h.sequence == math.MaxInt {
		return fmt.Errorf("card sequence exhausted")
	}
	h.sequence++
	if h.checkpoint != nil {
		return h.checkpoint()
	}
	return nil
}

func (h *feishuPreviewHandle) checkpointBeforeUpdate() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.checkpoint != nil {
		return h.checkpoint()
	}
	return nil
}

// Recovery captures the concrete source message and verified recipient of this
// turn. Keep it separate from the scheduler's replyTargetSnapshot: ordinary
// scheduled work must retain its existing channel-level routing.
type turnReplyRecoverySnapshot struct {
	previewRecoveryIdentity
	MessageID       string          `json:"message_id"`
	ChatID          string          `json:"chat_id"`
	GroupID         string          `json:"group_id,omitempty"`
	SessionKey      string          `json:"session_key"`
	ThreadID        string          `json:"thread_id,omitempty"`
	DirectUserID    string          `json:"direct_user_id,omitempty"`
	ContextEligible bool            `json:"context_eligible,omitempty"`
	ContextTimeMS   int64           `json:"context_time_ms,omitempty"`
	ContextRootID   string          `json:"context_root_id,omitempty"`
	ContextParentID string          `json:"context_parent_id,omitempty"`
	MediaStart      time.Time       `json:"media_start,omitempty"`
	ReferenceIDs    map[string]bool `json:"reference_ids,omitempty"`
}

func (p *interactivePlatform) SnapshotTurnReplyCtx(value any) (json.RawMessage, error) {
	rc, ok := value.(replyContext)
	if !ok {
		return nil, fmt.Errorf("%s: invalid turn reply context %T", p.tag(), value)
	}
	if err := p.validateTurnReplyContext(rc); err != nil {
		return nil, err
	}
	return json.Marshal(turnReplyRecoverySnapshot{
		previewRecoveryIdentity: p.previewRecoveryIdentity(), MessageID: rc.messageID, ChatID: rc.chatID,
		GroupID: rc.groupID, SessionKey: rc.sessionKey, ThreadID: rc.threadID, DirectUserID: rc.directUserID,
		ContextEligible: rc.contextEligible, ContextTimeMS: rc.contextTimeMs, ContextRootID: rc.contextRootID,
		ContextParentID: rc.contextParentID, MediaStart: rc.mediaStart, ReferenceIDs: rc.referenceIDs,
	})
}

func (p *interactivePlatform) RestoreTurnReplyCtx(snapshot json.RawMessage) (any, error) {
	var saved turnReplyRecoverySnapshot
	if err := json.Unmarshal(snapshot, &saved); err != nil {
		return nil, fmt.Errorf("%s: decode turn reply checkpoint: %w", p.tag(), err)
	}
	if saved.previewRecoveryIdentity != p.previewRecoveryIdentity() {
		return nil, fmt.Errorf("%s: turn reply checkpoint belongs to a different platform, app, project or version", p.tag())
	}
	rc := replyContext{
		messageID: saved.MessageID, chatID: saved.ChatID, groupID: saved.GroupID, sessionKey: saved.SessionKey,
		threadID: saved.ThreadID, directUserID: saved.DirectUserID, contextEligible: saved.ContextEligible,
		contextTimeMs: saved.ContextTimeMS, contextRootID: saved.ContextRootID, contextParentID: saved.ContextParentID,
		mediaStart: saved.MediaStart, referenceIDs: saved.ReferenceIDs,
	}
	if err := p.validateTurnReplyContext(rc); err != nil {
		return nil, err
	}
	return rc, nil
}

func (p *interactivePlatform) validateTurnReplyContext(rc replyContext) error {
	parts := strings.SplitN(rc.sessionKey, ":", 3)
	if len(parts) < 2 || parts[0] != p.Name() || parts[1] != rc.chatID ||
		!validRecoveryID(rc.chatID) || !strings.HasPrefix(rc.messageID, "om_") || !validRecoveryID(rc.messageID) ||
		(rc.groupID != "" && rc.groupID != rc.chatID) || (rc.groupID != "" && rc.directUserID != "") ||
		(rc.directUserID != "" && !validRecoveryID(rc.directUserID)) || (rc.threadID != "" && !validRecoveryID(rc.threadID)) {
		return fmt.Errorf("%s: inconsistent turn reply target", p.tag())
	}
	if len(parts) == 3 {
		if kind, id, ok := parseThreadTarget(parts[2]); ok && (!validRecoveryID(id) || (kind == "thread" && id != rc.threadID)) {
			return fmt.Errorf("%s: turn reply checkpoint has inconsistent thread identity", p.tag())
		}
	}
	return nil
}

func validRecoveryID(id string) bool {
	if id == "" {
		return false
	}
	for _, ch := range id {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}
