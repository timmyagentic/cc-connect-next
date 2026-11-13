package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/timmyagentic/cc-connect-next/core"
)

// cleanSignatureHistory runs only after the native writer has exited. Ordinary
// records retain their exact bytes. Edited records lose encrypted state while
// the original thread metadata, messages and tool receipts stay in place.
func cleanSignatureHistory(ctx context.Context, path, threadID string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return 0, fmt.Errorf("recovery history has an incomplete record")
	}
	var cleaned bytes.Buffer
	removed := 0
	for index, line := range bytes.SplitAfter(data, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(line, &record); err != nil {
			return 0, fmt.Errorf("invalid recovery record %d: %w", index+1, err)
		}
		var kind string
		if err := json.Unmarshal(record["type"], &kind); err != nil {
			return 0, fmt.Errorf("invalid recovery record type")
		}
		if index != 0 && kind != "response_item" && kind != "compacted" {
			cleaned.Write(line)
			continue
		}
		var payload map[string]any
		decoder := json.NewDecoder(bytes.NewReader(record["payload"]))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			return 0, fmt.Errorf("invalid recovery payload: %w", err)
		}
		if index == 0 && (kind != "session_meta" || payload["id"] != threadID) {
			return 0, fmt.Errorf("recovery history thread identity does not match")
		}
		count := 0
		switch kind {
		case "response_item":
			// Tool schemas can legitimately declare encrypted_content fields.
			if payload["type"] != "additional_tools" {
				value, n := clearEncryptedState(payload)
				payload, _ = value.(map[string]any)
				count = n
			}
		case "compacted":
			value, n := clearEncryptedState(payload["replacement_history"])
			if n != 0 {
				payload["replacement_history"] = value
				count = n
				if items, ok := value.([]any); ok && len(items) == 0 {
					// With no readable checkpoint, replay earlier plain history.
					payload = nil
				}
			}
		}
		removed += count
		if count == 0 {
			cleaned.Write(line)
			continue
		}
		if payload != nil {
			record["payload"], err = json.Marshal(payload)
			if err != nil {
				return 0, err
			}
			updated, err := json.Marshal(record)
			if err != nil {
				return 0, err
			}
			cleaned.Write(updated)
			cleaned.WriteByte('\n')
		}
	}
	if removed == 0 {
		return 0, fmt.Errorf("recovery history contains no encrypted state to clear")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, data) {
		return 0, fmt.Errorf("recovery history changed before cleanup")
	}
	if err := preserveSignatureHistory(path, data); err != nil {
		return 0, fmt.Errorf("preserve original recovery history: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	current, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(current, data) {
		return 0, fmt.Errorf("recovery history changed before replacement")
	}
	if err := core.AtomicWriteFile(path, cleaned.Bytes(), info.Mode().Perm()); err != nil {
		return 0, fmt.Errorf("save cleaned recovery history: %w", err)
	}
	return removed, nil
}

// clearEncryptedState visits response items, not arbitrary application JSON.
// The same schema applies to compacted replacement_history entries.
func clearEncryptedState(value any) (any, int) {
	if items, ok := value.([]any); ok {
		result := make([]any, 0, len(items))
		removed := 0
		for _, item := range items {
			clean, n := clearEncryptedState(item)
			removed += n
			if clean != nil || n == 0 {
				result = append(result, clean)
			}
		}
		return result, removed
	}
	item, ok := value.(map[string]any)
	if !ok {
		return value, 0
	}
	removed := 0
	remove := func(key string) {
		if _, exists := item[key]; exists {
			delete(item, key)
			removed++
		}
	}
	switch item["type"] {
	case "encrypted_content":
		return nil, 1
	case "compaction":
		if _, encrypted := item["encrypted_content"]; encrypted {
			// Drop a purely opaque native block, never a readable summary.
			remove("encrypted_content")
			plain := false
			for key := range item {
				if key != "type" && key != "id" {
					plain = true
				}
			}
			if !plain {
				return nil, removed
			}
		}
	case "reasoning":
		remove("encrypted_content")
	case "function_call", "custom_tool_call":
		remove("encrypted_function_args")
	case "message", "agent_message":
		item["content"], removed = clearEncryptedBlocks(item["content"])
	case "function_call_output", "custom_tool_call_output":
		item["output"], removed = clearEncryptedBlocks(item["output"])
	}
	return item, removed
}

// Only explicit native content blocks are encrypted. Strings, structured tool
// receipts and their fields (including schemas) are opaque application data.
func clearEncryptedBlocks(value any) (any, int) {
	items, ok := value.([]any)
	if !ok {
		return value, 0
	}
	result := make([]any, 0, len(items))
	removed := 0
	for _, block := range items {
		if item, ok := block.(map[string]any); ok && item["type"] == "encrypted_content" {
			removed++
			continue
		}
		result = append(result, block)
	}
	return result, removed
}

// Each original gets its own private non-JSONL file, never replacing an earlier
// backup or resembling a native rollout. Both data and directory entry must be
// durable before modifying the original history.
func preserveSignatureHistory(path string, data []byte) (resultErr error) {
	dir := filepath.Dir(path)
	backup, err := os.CreateTemp(dir, ".signature-original-*.bak")
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			if err := os.Remove(backup.Name()); err != nil {
				slog.Warn("codex: cleanup incomplete history backup", "error", err)
			}
		}
	}()
	if err = backup.Chmod(0o600); err == nil {
		_, err = backup.Write(data)
	}
	if err == nil {
		err = backup.Sync()
	}
	closeErr := backup.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr = directory.Close()
	if err != nil {
		return err
	}
	return closeErr
}
