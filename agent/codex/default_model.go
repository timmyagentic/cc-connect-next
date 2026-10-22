package codex

import (
	"fmt"
	"strings"
)

// Codex resolves an omitted config model from the catalog's isDefault entry.
// turn/start model:null does NOT perform that resolution: it keeps the thread's
// last model. Resolve a concrete model before promising a later default restore.
// These requests inspect configuration/catalog metadata and never start a turn.
func codexCatalogDefaultModel(request func(string, any, any) error) (string, error) {
	var cursor string
	seen := map[string]bool{}
	for page := 0; page < 20; page++ {
		params := map[string]any{"limit": 100, "includeHidden": true}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var response struct {
			Data []struct {
				Model     string `json:"model"`
				IsDefault bool   `json:"isDefault"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if err := request("model/list", params, &response); err != nil {
			return "", fmt.Errorf("codex read native default model catalog: %w", err)
		}
		for _, model := range response.Data {
			if model.IsDefault && strings.TrimSpace(model.Model) != "" {
				return strings.TrimSpace(model.Model), nil
			}
		}
		if response.NextCursor == nil || *response.NextCursor == "" {
			break
		}
		cursor = *response.NextCursor
		if seen[cursor] {
			break
		}
		seen[cursor] = true
	}
	return "", fmt.Errorf("codex native default model is unavailable; explicit per-turn models require a resolvable default")
}
