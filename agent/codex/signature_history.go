package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

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
	if err := core.AtomicWriteFile(path, cleaned.Bytes(), info.Mode().Perm()); err != nil {
		return 0, fmt.Errorf("save cleaned recovery history: %w", err)
	}
	return removed, nil
}

func clearEncryptedState(value any) (any, int) {
	removed := 0
	switch item := value.(type) {
	case map[string]any:
		if item["type"] == "encrypted_content" || item["type"] == "compaction" {
			return nil, 1
		}
		for key, child := range item {
			if key == "encrypted_content" || key == "encrypted_function_args" {
				delete(item, key)
				removed++
				continue
			}
			cleaned, count := clearEncryptedState(child)
			item[key] = cleaned
			removed += count
		}
	case []any:
		cleaned := make([]any, 0, len(item))
		for _, child := range item {
			retained, count := clearEncryptedState(child)
			removed += count
			if retained != nil || count == 0 {
				cleaned = append(cleaned, retained)
			}
		}
		value = cleaned
	}
	return value, removed
}
