package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const signatureHistoryMaxLine = 10 * 1024 * 1024

type recoveryRolloutRecord struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type recoveryRolloutMetadata struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"`
	TurnID             string            `json:"turn_id"`
	ReplacementHistory []json.RawMessage `json:"replacement_history"`
	GuardianHistory    *json.RawMessage  `json:"guardian_history"`
	RetainedContext    struct {
		VerifiedAnswers json.RawMessage `json:"verified_answers"`
	} `json:"retained_context"`
	Item recoveryRolloutTool `json:"item"`
}

type recoveryRolloutTool struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// loadSignatureRecoveryHistory reads the local rollout of a definitively
// failed turn. The caller resolves the path inside the agent's sessions
// directory and must not start recovery while that turn is still running.
func loadSignatureRecoveryHistory(ctx context.Context, path, threadID, turnID string) ([]json.RawMessage, error) {
	if threadID == "" || turnID == "" {
		return nil, errors.New("recovery requires an exact thread and turn")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("recovery rollout is unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("recovery rollout could not be opened")
	}
	defer f.Close()

	records := bufio.NewScanner(f)
	records.Buffer(make([]byte, 64*1024), signatureHistoryMaxLine)
	records.Split(recoveryRolloutLine)
	var retained []json.RawMessage
	var lastTurn string
	var checkpointSize int
	var sawMeta, sawInput, sawTerminal, compacted bool
	var checkpointContextUnsupported bool
	activeTools := make(map[string]struct{})
	for line := 1; records.Scan(); line++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var rec recoveryRolloutRecord
		var meta recoveryRolloutMetadata
		if json.Unmarshal(records.Bytes(), &rec) != nil || rec.Type == "" || len(rec.Payload) == 0 {
			return nil, fmt.Errorf("recovery rollout record %d is invalid", line)
		}
		if !sawMeta && rec.Type != "session_meta" {
			return nil, errors.New("recovery rollout has no initial session metadata")
		}
		if rec.Type != "response_item" {
			if json.Unmarshal(rec.Payload, &meta) != nil {
				return nil, fmt.Errorf("recovery rollout metadata at record %d is invalid", line)
			}
		}
		switch rec.Type {
		case "session_meta":
			if sawMeta || meta.ID != threadID {
				return nil, errors.New("recovery rollout thread identity does not match")
			}
			sawMeta = true
		case "turn_context":
			if meta.TurnID == "" || sawTerminal {
				return nil, errors.New("recovery rollout has an ambiguous turn boundary")
			}
			lastTurn = meta.TurnID
		case "response_item":
			if sawTerminal {
				return nil, errors.New("recovery rollout contains model input after the failed turn")
			}
			retained = append(retained, rec.Payload)
			if lastTurn == turnID && !sawInput {
				sawInput = recoveryItemIsUserInput(rec.Payload, turnID, false)
			}
		case "compacted":
			if sawTerminal {
				return nil, errors.New("recovery rollout changed after the failed turn")
			}
			// Codex resumes from the newest replacement history and replays
			// its suffix. Older checkpoints have already been superseded.
			compacted = true
			checkpointSize = len(meta.ReplacementHistory)
			checkpointContextUnsupported = meta.GuardianHistory != nil
			switch string(bytes.TrimSpace(meta.RetainedContext.VerifiedAnswers)) {
			case "", "null", "[]", "{}":
			default:
				checkpointContextUnsupported = true
			}
			retained = meta.ReplacementHistory
			// The interrupted request must be present in the new checkpoint
			// or be recorded again in its suffix.
			sawInput = false
			if lastTurn == turnID {
				for _, item := range retained {
					if recoveryItemIsUserInput(item, turnID, true) {
						sawInput = true
						break
					}
				}
			}
		case "event_msg":
			if meta.Type == "thread_rolled_back" {
				return nil, errors.New("recovery rollout contains a thread rollback")
			}
			if meta.TurnID != turnID {
				continue
			}
			switch meta.Type {
			case "task_started", "turn_started":
				if sawTerminal {
					return nil, errors.New("recovery rollout has an ambiguous turn boundary")
				}
			case "task_complete", "turn_complete":
				if lastTurn != turnID || sawTerminal {
					return nil, errors.New("recovery rollout has an ambiguous terminal turn")
				}
				sawTerminal = true
			case "task_aborted", "turn_aborted":
				return nil, errors.New("recovery rollout shows the turn was interrupted")
			case "item_started", "item_completed":
				if err := observeRecoveryTool(activeTools, meta); err != nil {
					return nil, err
				}
			}
		case "token_usage_record", "world_state", "inter_agent_communication_metadata", "security_risk_score":
			// Telemetry and bookkeeping are not model input.
		default:
			return nil, errors.New("recovery rollout contains unsupported history records")
		}
	}
	if err := records.Err(); err != nil {
		return nil, fmt.Errorf("recovery rollout is incomplete: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if lastTurn != turnID || !sawInput || !sawTerminal {
		return nil, errors.New("recovery rollout has not recorded the failed turn and its user input")
	}
	if len(activeTools) != 0 {
		return nil, errors.New("recovery rollout has unfinished tool operations")
	}
	if compacted {
		if checkpointSize == 0 {
			return nil, errors.New("recovery compaction has no replayable replacement history")
		}
		if checkpointContextUnsupported {
			return nil, errors.New("recovery compaction retains context outside its replacement history")
		}
		if err := checkRecoveryCheckpoint(retained[:checkpointSize]); err != nil {
			return nil, err
		}
	}
	return replayableSignatureItems(ctx, retained)
}

// Scanner's default split silently accepts a final unterminated record. A
// crash or an app-server flush race must not produce a partial replay instead.
func recoveryRolloutLine(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return 0, nil, io.ErrUnexpectedEOF
	}
	return 0, nil, nil
}

type recoveryUserItem struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Metadata struct {
		TurnID string `json:"turn_id"`
	} `json:"internal_chat_message_metadata_passthrough"`
}

func recoveryItemIsUserInput(raw json.RawMessage, turnID string, checkpoint bool) bool {
	var item recoveryUserItem
	if json.Unmarshal(raw, &item) != nil || item.Type != "message" || item.Role != "user" {
		return false
	}
	if item.Metadata.TurnID != turnID && (checkpoint || item.Metadata.TurnID != "") {
		return false
	}
	for _, part := range item.Content {
		if part.Type == "input_image" || part.Type == "input_audio" {
			return true
		}
		if part.Type == "input_text" && isUserPrompt(part.Text) {
			return true
		}
	}
	return false
}

func observeRecoveryTool(active map[string]struct{}, meta recoveryRolloutMetadata) error {
	switch meta.Item.Type {
	case "CommandExecution", "McpToolCall", "DynamicToolCall", "FileChange":
	default:
		return nil
	}
	if meta.Item.ID == "" {
		return errors.New("recovery rollout tool operation is missing its identity")
	}
	if meta.Type == "item_started" {
		active[meta.Item.ID] = struct{}{}
		return nil
	}
	switch meta.Item.Status {
	case "completed", "failed", "declined":
		delete(active, meta.Item.ID)
	default:
		return errors.New("recovery rollout has a tool operation with an unknown result")
	}
	return nil
}

func checkRecoveryCheckpoint(items []json.RawMessage) error {
	// In a checkpoint every item can be the only surviving copy of earlier
	// context. Require a fully readable snapshot rather than stripping it.
	for _, item := range items {
		var kind struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(item, &kind) == nil && kind.Type == "reasoning" {
			continue
		}
		if opaque, err := recoveryItemIsOpaque(item); err != nil || opaque {
			return errors.New("recovery compaction contains unreadable context")
		}
	}
	return nil
}

func replayableSignatureItems(ctx context.Context, retained []json.RawMessage) ([]json.RawMessage, error) {
	replay := make([]json.RawMessage, 0, len(retained))
	// Leave completed calls in the map to reject reuse of a call ID.
	calls := make(map[string]string)
	for _, raw := range retained {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var item struct {
			Type    string            `json:"type"`
			Role    string            `json:"role"`
			CallID  string            `json:"call_id"`
			Status  string            `json:"status"`
			Content []json.RawMessage `json:"content"`
			Output  json.RawMessage   `json:"output"`
		}
		if json.Unmarshal(raw, &item) != nil || item.Type == "" {
			return nil, errors.New("recovery history contains an invalid response item")
		}
		switch item.Type {
		case "reasoning":
			continue
		case "additional_tools":
			if item.Role != "developer" {
				return nil, errors.New("recovery history contains unsupported tool definitions")
			}
			// The new thread reconstructs its current native/MCP tool catalog.
			continue
		case "message":
			switch item.Role {
			case "developer", "system":
				continue
			case "user", "assistant":
			default:
				return nil, errors.New("recovery history has an unsupported message role")
			}
			if !recoveryPlainContent(item.Content, item.Role == "assistant") {
				return nil, errors.New("recovery history contains an unreadable message")
			}
		case "agent_message":
			// Keep plaintext inter-agent messages in their original form.
			if !recoveryPlainContent(item.Content, false) {
				return nil, errors.New("recovery history contains an unreadable agent message")
			}
		case "function_call", "custom_tool_call":
			if item.CallID == "" || item.Status != "" && item.Status != "completed" {
				return nil, errors.New("recovery history has a tool call with an unknown result")
			}
			if _, duplicate := calls[item.CallID]; duplicate {
				return nil, errors.New("recovery history contains a duplicate tool call")
			}
			calls[item.CallID] = item.Type
		case "function_call_output", "custom_tool_call_output":
			if calls[item.CallID] != strings.TrimSuffix(item.Type, "_output") {
				return nil, errors.New("recovery history contains an unmatched tool output")
			}
			if !recoveryToolOutputIsReadable(item.Output) {
				return nil, errors.New("recovery history has a tool call without a readable result")
			}
			calls[item.CallID] = ""
		default:
			return nil, errors.New("recovery history contains an unsupported response item")
		}
		if opaque, err := recoveryItemIsOpaque(raw); err != nil || opaque {
			return nil, errors.New("recovery history contains unreadable message or tool content")
		}
		replay = append(replay, raw)
	}
	for _, pending := range calls {
		if pending != "" {
			return nil, errors.New("recovery history has a tool call without a recorded result")
		}
	}
	return replay, nil
}

func recoveryToolOutputIsReadable(raw json.RawMessage) bool {
	var text *string
	if json.Unmarshal(raw, &text) == nil && text != nil {
		return true
	}
	var content []json.RawMessage
	return json.Unmarshal(raw, &content) == nil && recoveryPlainContent(content, false)
}

func recoveryPlainContent(content []json.RawMessage, assistant bool) bool {
	if len(content) == 0 {
		return false
	}
	for _, raw := range content {
		var part struct {
			Type     string  `json:"type"`
			Text     *string `json:"text"`
			ImageURL string  `json:"image_url"`
			AudioURL string  `json:"audio_url"`
		}
		if json.Unmarshal(raw, &part) != nil {
			return false
		}
		switch part.Type {
		case "input_text", "output_text":
			if part.Text == nil || assistant != (part.Type == "output_text") {
				return false
			}
		case "input_image":
			if assistant || part.ImageURL == "" {
				return false
			}
		case "input_audio":
			if assistant || part.AudioURL == "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func recoveryItemIsOpaque(raw json.RawMessage) (bool, error) {
	var item any
	if err := json.Unmarshal(raw, &item); err != nil {
		return false, err
	}
	return recoveryContentIsOpaque(item), nil
}

func recoveryContentIsOpaque(value any) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if recoveryContentIsOpaque(item) {
				return true
			}
		}
	case map[string]any:
		if v["type"] == "encrypted_content" {
			return true
		}
		for key, item := range v {
			switch key {
			case "encrypted_content", "encrypted_function_args":
				if item != nil && item != "" {
					return true
				}
			default:
				if recoveryContentIsOpaque(item) {
					return true
				}
			}
		}
	}
	return false
}
