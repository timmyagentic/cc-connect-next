package codex

import (
	"regexp"
	"strings"
)

const (
	capabilityBriefMarker     = "[cc-connect-next capability brief]"
	senderMetadataMarker      = "[cc-connect-next sender_id="
	maxCodexThreadRunes       = 48
	untitledCodexThread       = "New conversation"
	defaultSessionTitlePrefix = "[飞书]"
)

var (
	threadTitleMarkdownLink   = regexp.MustCompile(`\[([^]]+)\]\([^)]+\)`)
	threadTitleEmail          = regexp.MustCompile(`(?i)[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}`)
	threadTitleURL            = regexp.MustCompile(`(?i)https?://\S+`)
	threadTitleAssignedSecret = regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|auth[_-]?token|token|secret|password|passwd|pwd)\s*[:=]\s*["']?[A-Z0-9_./+=-]{8,}`)
	threadTitlePrefixedSecret = regexp.MustCompile(`(?i)\b(?:sk|sess|ghp|gho|ghu|ghs|ghr|github_pat|xox[baprs]|pat)[-_][A-Z0-9_-]{8,}\b`)
	threadTitleJWT            = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
	threadTitleAWSKey         = regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)
	threadTitleOpaqueToken    = regexp.MustCompile(`\b[A-Za-z0-9_+/=-]{32,}\b`)
)

func normalizeSessionTitlePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return defaultSessionTitlePrefix
	}
	return prefix
}

func formatSessionTitle(prefix, title string) string {
	prefix = normalizeSessionTitlePrefix(prefix)
	title = strings.TrimSpace(title)
	if title == "" {
		title = untitledCodexThread
	}
	if title == prefix || strings.HasPrefix(title, prefix+" ") {
		return title
	}
	return prefix + " " + title
}

// initialCodexThreadTitle derives a deterministic local title from the first
// real user request. It deliberately excludes cc-connect-next metadata and
// quoted context so Codex App never falls back to displaying internal prompt
// scaffolding or somebody else's quoted message as the conversation name.
func initialCodexThreadTitle(prompt string) string {
	text := prompt
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
	text = strings.TrimSpace(text)

	quotedContext := strings.HasPrefix(text, "[Quoted message from ") ||
		strings.HasPrefix(text, "--- Reply chain (")
	if quotedContext {
		boundary := strings.LastIndex(text, "\n\n\n")
		if boundary < 0 {
			return untitledCodexThread
		}
		text = strings.TrimSpace(text[boundary+3:])
		if text == "" {
			return untitledCodexThread
		}
	}
	lines := strings.Split(text, "\n")
	var title string
	if quotedContext {
		for _, line := range lines {
			if candidate := cleanThreadTitleLine(line); candidate != "" {
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
	line = threadTitleAssignedSecret.ReplaceAllString(line, "$1=[secret]")
	line = threadTitleJWT.ReplaceAllString(line, "[secret]")
	line = threadTitleAWSKey.ReplaceAllString(line, "[secret]")
	line = threadTitlePrefixedSecret.ReplaceAllString(line, "[secret]")
	line = threadTitleOpaqueToken.ReplaceAllString(line, "[secret]")
	return strings.Join(strings.Fields(line), " ")
}
