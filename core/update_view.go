package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
)

// PersistentCardSender addresses an exact card across process restarts. The
// platform owns the reference format and must validate application/project
// identity on every use. References must never contain credentials.
type PersistentCardSender interface {
	CardReference(replyCtx any) (json.RawMessage, error)
	ReplyCardWithReference(context.Context, any, *Card) (json.RawMessage, error)
	UpdateCardByReference(context.Context, json.RawMessage, *Card) error
}

type updateView struct {
	mu        sync.Mutex
	engine    *Engine
	platform  Platform
	replyCtx  any
	exact     bool
	reference json.RawMessage
	lastText  string
	fallback  bool
}

func newUpdateView(e *Engine, p Platform, msg *Message) *updateView {
	v := &updateView{engine: e, platform: p, replyCtx: msg.ReplyCtx, exact: msg.IsCardAction}
	if msg.IsCardAction {
		if sender, ok := p.(PersistentCardSender); ok {
			var err error
			v.reference, err = sender.CardReference(msg.ReplyCtx)
			if err != nil {
				slog.Warn("update card reference unavailable", "error", err)
			}
		}
	}
	return v
}

func (v *updateView) editable() bool {
	if _, ok := v.platform.(PersistentCardSender); ok {
		return true
	}
	_, ok := v.platform.(CardActionUpdater)
	return ok && v.exact
}

func (v *updateView) render(card *Card, transient bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	ctx, cancel := context.WithTimeout(v.engine.ctx, 10*time.Second)
	defer cancel()
	card = v.engine.renderCardForPlatform(v.platform, card)
	if sender, ok := v.platform.(PersistentCardSender); ok {
		if len(v.reference) > 0 {
			if err := sender.UpdateCardByReference(ctx, v.reference, card); err == nil {
				return nil
			} else if transient || v.fallback {
				return err
			}
		}
		replacement := len(v.reference) > 0
		ref, err := sender.ReplyCardWithReference(ctx, v.replyCtx, card)
		if err == nil {
			if len(ref) == 0 {
				return fmt.Errorf("update card sender returned no reference")
			}
			v.reference = ref
			v.exact = true
			v.fallback = v.fallback || replacement
		}
		return err
	}
	if updater, ok := v.platform.(CardActionUpdater); ok && v.exact {
		return updater.UpdateCard(ctx, v.replyCtx, card)
	}
	// Text-only platforms receive stage transitions, never periodic elapsed
	// updates. Card-only platforms retain the same readable fallback.
	text := card.RenderText()
	if transient && text == v.lastText {
		return nil
	}
	v.lastText = text
	if cs, ok := v.platform.(CardSender); ok && (!transient || card.HasButtons()) {
		return cs.ReplyCard(ctx, v.replyCtx, card)
	}
	if buttons, ok := v.platform.(InlineButtonSender); ok && (!transient || card.HasButtons()) {
		var rows [][]ButtonOption
		copy := *card
		copy.Elements = nil
		for _, element := range card.Elements {
			if actions, ok := element.(CardActions); ok {
				var row []ButtonOption
				for _, button := range actions.Buttons {
					row = append(row, ButtonOption{Text: button.Text, Data: button.Value})
				}
				rows = append(rows, row)
			} else {
				copy.Elements = append(copy.Elements, element)
			}
		}
		if len(rows) > 0 {
			return buttons.SendWithButtons(ctx, v.replyCtx, copy.RenderText(), rows)
		}
	}
	// Plain text has no clickable controls. Preserve the exact commands so
	// cancellation, retry and rechecking remain usable without guessing.
	var commands []string
	for _, element := range card.Elements {
		if actions, ok := element.(CardActions); ok {
			for _, button := range actions.Buttons {
				if strings.HasPrefix(button.Value, "cmd:") {
					commands = append(commands, button.Text+": "+strings.TrimPrefix(button.Value, "cmd:"))
				}
			}
		}
	}
	if len(commands) > 0 {
		text += "\n\n" + strings.Join(commands, "\n")
	}
	return v.platform.Reply(ctx, v.replyCtx, text)
}
func (v *updateView) savedReference() json.RawMessage {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append(json.RawMessage(nil), v.reference...)
}

func releaseNotesLink(release ReleaseInfo, label string) string {
	u, err := url.Parse(release.HTMLURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	return "[" + label + "](" + strings.ReplaceAll(u.String(), ")", "%29") + ")"
}

// Prefer publisher-authored highlights. Older releases degrade to short bullet
// headings, not installation commands or internal release-validation prose.
func updateSummary(body string, lang Language) string {
	body = releaseBodyForLanguage(body, lang)
	lines := strings.Split(body, "\n")
	var highlights []string
	inHighlights := false
	for _, line := range lines {
		heading := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			if inHighlights {
				break
			}
			inHighlights = strings.EqualFold(heading, "Highlights") || heading == "更新亮点" || heading == "更新亮點"
			continue
		}
		if inHighlights && strings.TrimSpace(line) != "" {
			highlights = append(highlights, line)
		}
	}
	if len(highlights) > 0 {
		lines = highlights
	}
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ") {
			continue
		}
		line = strings.TrimSpace(line[2:])
		if len(highlights) == 0 {
			if strings.HasPrefix(line, "**") {
				if end := strings.Index(line[2:], "**"); end >= 0 {
					line = line[2 : 2+end]
				}
			}
		}
		line = strings.ReplaceAll(strings.ReplaceAll(line, "**", ""), "`", "")
		runes := []rune(line)
		if len(runes) > 100 {
			line = string(runes[:100]) + "…"
		}
		out = append(out, "• "+line)
		if len(out) == 3 {
			break
		}
	}
	return strings.Join(out, "\n")
}

// Explicit upgrade notices are never silently truncated into innocuous copy.
func updateImportantNotes(body string, lang Language) string {
	lines := strings.Split(releaseBodyForLanguage(body, lang), "\n")
	var out []string
	inside := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			if inside {
				break
			}
			h := strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#")))
			inside = h == "upgrade notes" || h == "breaking changes" || h == "升级注意" || h == "升級注意" || h == "重要变更" || h == "重要變更"
			continue
		}
		if inside {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func (e *Engine) updateReviewCard(plan PreparedUpdate, token string) *Card {
	channelKey := MsgUpdateStable
	if updatechannel.Channel(plan.Release.Channel).Effective() == updatechannel.Beta {
		channelKey = MsgUpdateBeta
	}
	summary := updateSummary(plan.Release.Body, e.i18n.CurrentLang())
	if summary == "" {
		// A short plain release note remains useful for older release formats.
		body := strings.TrimSpace(releaseBodyForLanguage(plan.Release.Body, e.i18n.CurrentLang()))
		if len([]rune(body)) <= 180 && !strings.Contains(body, "\n#") && !strings.Contains(body, "```") {
			summary = body
		}
		if summary == "" {
			summary = e.i18n.T(MsgUpdateSummaryFallback)
		}
	}
	cb := NewCard().Title(e.i18n.T(MsgUpdateReviewTitle), "blue").
		Markdownf("**%s → %s** · %s", CurrentVersion, plan.Release.TagName, e.i18n.T(channelKey)).
		Markdown(summary).Markdown(updateImportantNotes(plan.Release.Body, e.i18n.CurrentLang())).
		Divider().Markdown(e.i18n.T(MsgUpdateImpact))
	label := e.i18n.T(MsgUpdateInstall)
	if n := e.updates().activeCount(); n > 0 {
		cb.Markdown(e.i18n.Tf(MsgUpdateActiveTasks, n))
		label = e.i18n.T(MsgUpdateAfterTasks)
	}
	cb.Buttons(PrimaryBtn(label, "cmd:/upgrade confirm "+token), DefaultBtn(e.i18n.T(MsgUpdateLater), "cmd:/upgrade cancel "+token)).
		Markdown(releaseNotesLink(plan.Release, e.i18n.T(MsgUpdateDetails))).Note(e.i18n.T(MsgUpdateHintReplyConfirm))
	return cb.Build()
}

func (e *Engine) updateStateCard(stage, version, detail string, started time.Time, token string, cancellable bool) *Card {
	key := MsgUpdateInstallingState
	color := "blue"
	switch stage {
	case "checking":
		key = MsgUpdateCheckingState
	case "waiting":
		key = MsgUpdateWaitingState
		color = "orange"
	case "downloading":
		key = MsgUpdateDownloadingState
	case "verifying":
		key = MsgUpdateVerifyingState
	case "verifying-package":
		key = MsgUpdateVerifyingPackage
	case "restarting":
		key = MsgUpdateRestartingState
	case "completed":
		key = MsgUpdateCompleteState
		color = "green"
	case "failed":
		key = MsgUpdateFailureState
		color = "red"
	case "cancelled":
		key = MsgUpdateCancelledState
		color = "grey"
	case "expired":
		key = MsgUpdateExpiredState
		color = "orange"
	}
	cb := NewCard().Title(e.i18n.T(key), color)
	if version != "" {
		cb.Markdown("**" + version + "**")
	}
	cb.Markdown(detail)
	if !started.IsZero() {
		cb.Note(e.i18n.Tf(MsgUpdateElapsed, formatUpdateElapsed(time.Since(started))))
	}
	if cancellable {
		cancelKey := MsgUpdateCancel
		if stage == "checking" {
			cancelKey = MsgUpdateCancelCheck
		}
		cb.Buttons(DefaultBtn(e.i18n.T(cancelKey), "cmd:/upgrade cancel "+token))
	}
	if stage == "failed" || stage == "expired" || stage == "cancelled" {
		cb.Buttons(PrimaryBtn(e.i18n.T(MsgUpdateCheckAgain), "cmd:/upgrade"))
	}
	if stage == "restarting" {
		cb.Note(e.i18n.T(MsgUpdateRecoveryHint))
	}
	return cb.Build()
}

func (v *updateView) show(card *Card, transient bool) {
	if err := v.render(card, transient); err != nil {
		slog.Warn("update card delivery failed", "error", err)
	}
}

func formatUpdateElapsed(elapsed time.Duration) string {
	if elapsed < 0 {
		elapsed = 0
	}
	seconds := int(elapsed / time.Second)
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}
