package core

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	// AgentTurnMarkerEnv is injected only into Agent subprocesses created by a
	// runtime that supports turn-bound lifecycle credentials.
	AgentTurnMarkerEnv = "CC_AGENT_TURN"
	// AgentSessionSecretEnv is a random capability unique to one Agent process.
	// It never travels over the local API; the CLI uses it to authenticate the
	// current turn nonce with HMAC-SHA256.
	AgentSessionSecretEnv = "CC_AGENT_SESSION_SECRET"
	// AgentTurnNonceFileEnv points to a runtime-managed, non-secret nonce file.
	// The nonce rotates for every foreground Agent turn.
	AgentTurnNonceFileEnv = "CC_AGENT_TURN_NONCE_FILE"
)

func newAgentTurnNoncePath(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", fmt.Errorf("data directory is empty")
	}
	dir, err := filepath.Abs(filepath.Join(dataDir, "run", "agent-turn-nonces"))
	if err != nil {
		return "", fmt.Errorf("resolve turn nonce directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create turn nonce directory: %w", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		return "", fmt.Errorf("set turn nonce directory mode: %w", err)
	}
	file, err := os.CreateTemp(dir, "turn-*")
	if err != nil {
		return "", fmt.Errorf("create turn nonce file: %w", err)
	}
	path := file.Name()
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("set turn nonce file mode: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close turn nonce file: %w", err)
	}
	return path, nil
}

func newAgentTurnToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate turn credential: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func agentTurnToken(sessionSecret, nonce string) string {
	mac := hmac.New(sha256.New, []byte(sessionSecret))
	_, _ = mac.Write([]byte(nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

// BuildAgentTurnCredential signs one public turn nonce with the private
// session capability inherited by that Agent process.
func BuildAgentTurnCredential(sessionSecret, nonce string) (string, error) {
	if !validAgentTurnToken(sessionSecret) || !validAgentTurnToken(nonce) {
		return "", fmt.Errorf("invalid Agent session secret or turn nonce")
	}
	return agentTurnToken(sessionSecret, nonce), nil
}

func sameAgentTurnToken(left, right string) bool {
	if !validAgentTurnToken(left) || !validAgentTurnToken(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func validAgentTurnToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}

func writeAgentTurnNonce(path, nonce string) error {
	if path == "" {
		return nil
	}
	if err := os.WriteFile(path, []byte(nonce+"\n"), 0o644); err != nil {
		return err
	}
	return os.Chmod(path, 0o644)
}

// activateAgentTurnCredential rotates the capability before an Agent Send.
// State changes first, so a concurrent read of the previous file value fails
// closed while the new value is being written.
func (e *Engine) activateAgentTurnCredential(state *interactiveState, userID, sessionKey string) {
	if state == nil {
		return
	}
	nonce, err := newAgentTurnToken()
	if err != nil {
		slog.Warn("Agent turn credential unavailable", "project", e.name, "error", err)
		nonce = ""
	}

	state.steerMu.Lock()
	state.mu.Lock()
	sessionSecret := state.restartSessionSecret
	path := state.restartNoncePath
	token := ""
	if nonce != "" {
		if sessionSecret != "" && path != "" {
			token = agentTurnToken(sessionSecret, nonce)
		} else {
			// Tests and non-injecting custom Agents can still exercise the Engine
			// lifecycle directly, but no subprocess can discover this fallback.
			token = nonce
		}
	}
	state.currentUserID = userID
	state.currentSessionKey = sessionKey
	state.restartTurnToken = token
	state.mu.Unlock()
	state.steerMu.Unlock()

	if token == "" || path == "" || sessionSecret == "" {
		return
	}
	if err := writeAgentTurnNonce(path, nonce); err != nil {
		state.steerMu.Lock()
		state.mu.Lock()
		if state.restartTurnToken == token {
			state.restartTurnToken = ""
		}
		state.mu.Unlock()
		state.steerMu.Unlock()
		slog.Warn("Agent turn credential write failed", "project", e.name, "error", err)
	}
}

func clearAgentTurnCredential(state *interactiveState, remove bool) {
	if state == nil {
		return
	}
	state.mu.Lock()
	state.restartTurnToken = ""
	path := state.restartNoncePath
	if remove {
		state.restartNoncePath = ""
		state.restartSessionSecret = ""
	}
	state.mu.Unlock()
	if path == "" {
		return
	}
	if remove {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			slog.Debug("remove Agent turn credential failed", "error", err)
		}
		return
	}
	if err := writeAgentTurnNonce(path, ""); err != nil && !os.IsNotExist(err) {
		slog.Debug("clear Agent turn credential failed", "error", err)
	}
}
