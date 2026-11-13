package codex

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/timmyagentic/cc-connect-next/core"
)

// clientUserMessageId is persisted as userMessage.clientId by app-server.
// Unlike a received turn ID it is available BEFORE turn/start, so an accepted
// submission with a lost RPC response can still be identified exactly.
type codexTurnCheckpoint struct {
	Version            int              `json:"version"`
	ThreadID           string           `json:"thread_id"`
	BoundaryID         string           `json:"boundary_id,omitempty"`
	ClientID           string           `json:"client_id"`
	InputHash          string           `json:"input_hash"`
	Input              []map[string]any `json:"input"`
	WorkDir            string           `json:"work_dir"`
	Mode               string           `json:"mode"`
	Provider           string           `json:"provider"`
	SignatureRecovered bool             `json:"signature_recovered,omitempty"`
}

type recoveryThread struct {
	ID     string `json:"id"`
	Status struct {
		Type string `json:"type"`
	} `json:"status"`
	Turns []recoveryTurn `json:"turns"`
}

type recoveryTurn struct {
	ID        string           `json:"id"`
	Status    string           `json:"status"`
	ItemsView string           `json:"itemsView"`
	Items     []map[string]any `json:"items"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *Agent) SupportsTurnRecovery() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.backend == "app_server" && localAppServer(a.appServerURL)
}

func recoveryInputHash(prompt string, images []core.ImageAttachment, files []core.FileAttachment) string {
	// The durable core envelope omits empty attachment slices. Normalize them
	// so JSON round-tripping [] into nil does not change the input identity.
	if len(images) == 0 {
		images = nil
	}
	if len(files) == 0 {
		files = nil
	}
	data, _ := json.Marshal(struct {
		Prompt string
		Images []core.ImageAttachment
		Files  []core.FileAttachment
	}{prompt, images, files})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func recoveryClientID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "cc-next-" + hex.EncodeToString(value[:]), nil
}

func (s *appServerSession) readRecoveryThread() (recoveryThread, error) {
	var response struct {
		Thread recoveryThread `json:"thread"`
	}
	id := s.CurrentSessionID()
	if id == "" || !s.alive.Load() {
		return response.Thread, fmt.Errorf("codex recovery requires a live, identified thread")
	}
	if err := s.request("thread/read", map[string]any{"threadId": id, "includeTurns": true}, &response); err != nil {
		return response.Thread, fmt.Errorf("codex recovery read thread: %w", err)
	}
	if response.Thread.ID != id {
		return response.Thread, fmt.Errorf("codex recovery thread identity mismatch")
	}
	if response.Thread.Turns == nil {
		return response.Thread, fmt.Errorf("codex recovery requires an explicit complete turn list")
	}
	// A loaded active thread might still execute tools or await permission.
	// Never turn recovery into steering or approve an outstanding request.
	if response.Thread.Status.Type != "idle" {
		return response.Thread, fmt.Errorf("codex recovery thread is not idle (%s)", response.Thread.Status.Type)
	}
	return response.Thread, nil
}

func (s *appServerSession) PrepareTurnRecovery(input core.TurnRecoveryInput) (json.RawMessage, error) {
	s.recoveryMu.Lock()
	fresh := s.recoveryFreshThread
	s.recoveryMu.Unlock()
	var boundary string
	if !fresh {
		thread, err := s.readRecoveryThread()
		if err != nil {
			return nil, err
		}
		if len(thread.Turns) > 0 {
			boundary = thread.Turns[len(thread.Turns)-1].ID
			if boundary == "" {
				return nil, fmt.Errorf("codex recovery history has no turn identity")
			}
		}
	}
	wireInput, err := s.prepareTurnInput(input.Prompt, input.Images, input.Files)
	if err != nil {
		return nil, err
	}
	clientID, err := recoveryClientID()
	if err != nil {
		return nil, err
	}
	cp := &codexTurnCheckpoint{
		Version: 1, ThreadID: s.CurrentSessionID(), BoundaryID: boundary, ClientID: clientID,
		InputHash: recoveryInputHash(input.Prompt, input.Images, input.Files), Input: wireInput,
		WorkDir: filepath.Clean(s.GetWorkDir()), Mode: normalizeMode(s.mode), Provider: s.providerName,
	}
	if cp.ThreadID == "" || !s.alive.Load() {
		return nil, fmt.Errorf("codex recovery requires a live, identified thread")
	}
	raw, err := json.Marshal(cp)
	if err != nil {
		return nil, err
	}
	s.recoveryMu.Lock()
	s.preparedRecovery = cp
	s.recoveryMu.Unlock()
	return raw, nil
}

func (s *appServerSession) ResumeInterruptedTurn(raw json.RawMessage, input core.TurnRecoveryInput, save func(json.RawMessage) error) error {
	var cp codexTurnCheckpoint
	if err := json.Unmarshal(raw, &cp); err != nil {
		return fmt.Errorf("decode codex recovery checkpoint: %w", err)
	}
	if cp.Version != 1 || cp.ThreadID == "" || cp.ClientID == "" || len(cp.Input) == 0 ||
		cp.ThreadID != s.CurrentSessionID() || cp.WorkDir != filepath.Clean(s.GetWorkDir()) ||
		cp.Mode != normalizeMode(s.mode) || cp.Provider != s.providerName ||
		cp.InputHash != recoveryInputHash(input.Prompt, input.Images, input.Files) {
		return fmt.Errorf("codex recovery checkpoint does not match the thread, input, or execution policy")
	}
	thread, err := s.readRecoveryThread()
	if err != nil {
		return err
	}
	turn, err := checkpointTurn(cp, thread.Turns)
	if err != nil {
		return err
	}
	if turn == nil {
		// No submission exists after the durable boundary. Reuse the exact
		// prepared input and client ID; do not stage different attachment paths.
		return s.startRecoveredTurn(cp, input)
	}
	if turn.Status == "failed" {
		message := "original turn failed"
		if turn.Error != nil && strings.TrimSpace(turn.Error.Message) != "" {
			message = turn.Error.Message
		}
		return classifyCodexError(fmt.Errorf("codex recovery: %s", message))
	}
	if turn.Status == "completed" {
		if turn.Error != nil {
			return fmt.Errorf("codex recovery completed turn contains an error")
		}
		answer, err := recoveredAnswer(*turn)
		if err != nil {
			return err
		}
		// Deliver the recorded result only: no model request or tool replay.
		s.emit(core.Event{Type: core.EventResult, SessionID: cp.ThreadID, Content: answer, Done: true})
		return nil
	}
	if turn.Status != "interrupted" && turn.Status != "inProgress" {
		return fmt.Errorf("codex recovery unknown turn status %q", turn.Status)
	}
	// inProgress is recoverable only because readRecoveryThread established
	// that the new local app-server has no active work for this thread.
	if save == nil {
		return fmt.Errorf("codex recovery continuation requires durable checkpoint storage")
	}
	cp.BoundaryID = turn.ID
	cp.ClientID, err = recoveryClientID()
	if err != nil {
		return err
	}
	cp.Input = []map[string]any{{"type": "text", "text": interruptedTurnContinuation, "text_elements": []any{}}}
	next, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if err := save(next); err != nil {
		return fmt.Errorf("save codex recovery continuation: %w", err)
	}
	return s.startRecoveredTurn(cp, input)
}

const interruptedTurnContinuation = "The cc-connect-next process restarted while the previous user task was unfinished. Continue that same task from this conversation's existing history and the current workspace. First reconcile any tool operation whose result was interrupted: inspect its state or receipt before repeating an external action. Preserve the user's scope, decisions, and permission requirements. Do not repeat completed work, resend a delivered message, or treat the restart as approval. If the task already finished, return its verified result; otherwise complete the remaining authorized work."

func (s *appServerSession) startRecoveredTurn(cp codexTurnCheckpoint, input core.TurnRecoveryInput) error {
	s.recoveryMu.Lock()
	s.activeRecovery = &cp
	s.recoveryMu.Unlock()
	if !input.UseOptions {
		return s.startPreparedTurn(cp.Input, nil, cp.ClientID)
	}
	options := input.Options
	if err := s.ValidateTurnDefaults(); err != nil {
		return err
	}
	if strings.TrimSpace(options.Model) == "" {
		options.Model = s.DefaultTurnOptions().Model
	}
	return s.startPreparedTurn(cp.Input, &options, cp.ClientID)
}

// A same-thread signature retry must durably record its new submission and
// consumed retry budget before any model work, including across a daemon restart.
func (s *appServerSession) signatureTurnCheckpoint(previousID, failedTurnID string, input []map[string]any) (json.RawMessage, string, error) {
	s.recoveryMu.Lock()
	active := s.activeRecovery
	var cp codexTurnCheckpoint
	if active != nil {
		cp = *active
	}
	s.recoveryMu.Unlock()
	if active == nil {
		return nil, "", nil
	}
	if cp.ThreadID != previousID || cp.ClientID == "" {
		return nil, "", fmt.Errorf("signature recovery checkpoint does not match the original conversation")
	}
	thread, err := s.readRecoveryThread()
	if err != nil {
		return nil, "", err
	}
	if thread.ID != previousID || len(thread.Turns) == 0 || thread.Turns[len(thread.Turns)-1].ID != failedTurnID || thread.Turns[len(thread.Turns)-1].Status != "failed" {
		return nil, "", fmt.Errorf("signature retry history does not end at the confirmed failed turn")
	}
	cp.BoundaryID = failedTurnID
	cp.ClientID, err = recoveryClientID()
	if err != nil {
		return nil, "", err
	}
	cp.Input = input
	cp.SignatureRecovered = true
	raw, err := json.Marshal(cp)
	if err != nil {
		return nil, "", err
	}
	s.recoveryMu.Lock()
	s.activeRecovery = &cp
	s.recoveryMu.Unlock()
	return raw, cp.ClientID, nil
}

func checkpointTurn(cp codexTurnCheckpoint, turns []recoveryTurn) (*recoveryTurn, error) {
	index := -1
	if cp.BoundaryID != "" {
		for i := range turns {
			if turns[i].ID == cp.BoundaryID {
				index = i
				break
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("codex recovery saved turn boundary is missing")
		}
	}
	after := turns[index+1:]
	if len(after) == 0 {
		return nil, nil
	}
	if len(after) != 1 || after[0].ID == "" {
		return nil, fmt.Errorf("codex recovery found unexpected newer turns")
	}
	turn := &after[0]
	if turn.ItemsView != "" && turn.ItemsView != "full" {
		return nil, fmt.Errorf("codex recovery requires complete turn items")
	}
	// The first user item owns the submission. Later user items may be
	// accepted native steering and must remain part of that original turn.
	for _, item := range turn.Items {
		if item["type"] == "userMessage" {
			if item["clientId"] == cp.ClientID {
				return turn, nil
			}
			break
		}
	}
	return nil, fmt.Errorf("codex recovery newer turn does not match the saved submission")
}

func recoveredAnswer(turn recoveryTurn) (string, error) {
	var final, legacy []string
	for _, item := range turn.Items {
		kind, _ := item["type"].(string)
		if kind == "agentMessage" {
			body, _ := item["text"].(string)
			phase, _ := item["phase"].(string)
			switch phase {
			case "final_answer":
				final = append(final, body)
			case "":
				legacy = append(legacy, body)
			}
		} else if kind != "reasoning" && kind != "userMessage" && kind != "plan" {
			// Mirror the live adapter: prose before a tool is progress, not
			// the final answer. Explicit final_answer phases are authoritative.
			legacy = nil
		}
	}
	if len(final) > 0 {
		return strings.Join(final, "\n\n"), nil
	}
	if len(legacy) > 0 {
		return strings.Join(legacy, "\n\n"), nil
	}
	return "", fmt.Errorf("codex recovery completed turn has no recorded final answer")
}

var _ core.TurnRecoverySession = (*appServerSession)(nil)
