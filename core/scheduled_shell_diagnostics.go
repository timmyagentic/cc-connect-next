package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const scheduledShellDiagnosticLimit = 2048

// Shell traces are not reliably parseable as argv: quoted credentials can span
// lines and concatenate quoted/unquoted fragments. Suppress the entire output
// when a credential flag is present instead of risking a surviving suffix.
var scheduledShellCredentialFlag = regexp.MustCompile(`(?i)(?:^|[\s"'` + "`" + `])(?:--(?:[a-z0-9]+[_-])*(?:api[_-]?key|apikey|token|secret|password|passwd|credential[s]?|authorization|cookie|private[_-]?key)|-k)(?:$|[\s="'` + "`" + `])`)

// Redact before taking the tail: truncating first could cut off the key that
// identifies a credential. Preserve the end, where commands usually explain
// their failure, without persisting unlimited process output in LastError.
func scheduledShellDiagnostic(output string) string {
	if scheduledShellCredentialFlag.MatchString(output) {
		return "[REDACTED-SHELL-OUTPUT]"
	}
	output = strings.TrimSpace(redactFeedbackText(output))
	runes := []rune(output)
	if len(runes) > scheduledShellDiagnosticLimit {
		output = "…" + string(runes[len(runes)-scheduledShellDiagnosticLimit:])
	}
	return output
}

func scheduledShellFailure(exitCode int, output string) error {
	diagnostic := scheduledShellDiagnostic(output)
	if diagnostic == "" {
		return fmt.Errorf("shell: exit code %d", exitCode)
	}
	return fmt.Errorf("shell: exit code %d: %s", exitCode, diagnostic)
}

func (e *Engine) finishInterruptedScheduledShell(p Platform, replyCtx any, cause error, cmdLabel string, mu *sync.Mutex, buf *bytes.Buffer, useUpdate bool, previewHandle any) error {
	mu.Lock()
	output := buf.String()
	mu.Unlock()
	status := e.i18n.T(MsgScheduledShellCanceled)
	if errors.Is(cause, context.DeadlineExceeded) {
		status = e.i18n.T(MsgScheduledShellTimeout)
	}
	msg := fmt.Sprintf("⏰ ⚠️ %s: `%s`", status, cmdLabel)
	if output != "" {
		msg += "\n\n" + truncateStr(output, 3000)
	}
	if useUpdate && previewHandle != nil {
		_ = updaterFor(p).UpdateMessage(e.ctx, previewHandle, msg)
	} else {
		e.send(p, replyCtx, msg)
	}
	diagnostic := scheduledShellDiagnostic(output)
	if diagnostic == "" {
		return fmt.Errorf("shell command: %w", cause)
	}
	return fmt.Errorf("shell command: %w: %s", cause, diagnostic)
}
