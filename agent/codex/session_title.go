package codex

import (
	"regexp"
	"strings"
)

const (
	capabilityBriefMarker = "[cc-connect-next capability brief]"
	senderMetadataMarker  = "[cc-connect-next sender_id="
	maxCodexThreadRunes   = 48
	untitledCodexThread   = "New conversation"
)

var (
	threadTitleMarkdownLink = regexp.MustCompile(`\[([^]]+)\]\([^)]+\)`)
	threadTitleEmail        = regexp.MustCompile(`(?i)[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}`)
	threadTitleURL          = regexp.MustCompile(`(?i)https?://\S+`)
	threadTitleSecret       = regexp.MustCompile(`(?i)\b(?:sk|sess|token|key)[-_][A-Z0-9_-]{12,}\b`)
)

// initialCodexThreadTitle derives a deterministic local title from the first
// real user request. It deliberately excludes cc-connect-next metadata and
// quoted context so Codex App never falls back to displaying internal prompt
// scaffolding or somebody else's quoted message as the conversation name.
func initialCodexThreadTitle(prompt string) string {
	text := strings.TrimSpace(prompt)
	if strings.HasPrefix(text, capabilityBriefMarker) {
		if end := strings.Index(text, "\n\n"); end >= 0 {
			text = strings.TrimSpace(text[end+2:])
		} else {
			text = ""
		}
	}
	if strings.HasPrefix(text, senderMetadataMarker) {
		if end := strings.IndexByte(text, '\n'); end >= 0 {
			text = strings.TrimSpace(text[end+1:])
		} else {
			text = ""
		}
	}

	quotedContext := strings.HasPrefix(text, "[Quoted message from ") ||
		strings.HasPrefix(text, "--- Reply chain (")
	lines := strings.Split(text, "\n")
	var title string
	if quotedContext {
		for i := len(lines) - 1; i >= 0; i-- {
			if candidate := cleanThreadTitleLine(lines[i]); candidate != "" {
				title = candidate
				break
			}
		}
	} else {
		for _, line := range lines {
			if candidate := cleanThreadTitleLine(line); candidate != "" {
				title = candidate
				break
			}
		}
	}
	if title == "" {
		return untitledCodexThread
	}

	runes := []rune(title)
	if len(runes) > maxCodexThreadRunes {
		return string(runes[:maxCodexThreadRunes]) + "..."
	}
	return title
}

func cleanThreadTitleLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#>*+- \t")
	line = strings.Trim(line, "`*_~")
	fields := strings.Fields(line)
	for len(fields) > 0 && strings.HasPrefix(fields[0], "@") {
		fields = fields[1:]
	}
	line = strings.Join(fields, " ")
	if line == "" {
		return ""
	}

	line = threadTitleMarkdownLink.ReplaceAllString(line, "$1")
	line = threadTitleEmail.ReplaceAllString(line, "[email]")
	line = threadTitleURL.ReplaceAllString(line, "[link]")
	line = threadTitleSecret.ReplaceAllString(line, "[secret]")
	return strings.Join(strings.Fields(line), " ")
}
