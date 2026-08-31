package core

// Natural-language update flow.
//
// The update notice used to demand typed commands ("Send `/upgrade confirm`
// to update now"), which is exactly backwards for a chat product: the user
// is already talking to the bot in natural language. This file lets plain
// messages like 更新 / 升级到最新版 / "update" / "确认" drive the same
// upgrade pipeline, with two safety properties:
//
//  1. No hijacking. A message is only treated as update intent when the
//     ENTIRE message matches a conservative phrase list. "更新" alone is
//     ambiguous (it could be a coding instruction), so bare phrases are
//     honored only in an update context — after this session received an
//     update notice or an upgrade prompt. Everything else falls through to
//     the agent untouched.
//  2. No gate bypass. Matched intents are rewritten to the equivalent
//     /upgrade invocation and dispatched through handleCommand, so the
//     privileged-command (admin_from) and disabled-command gates apply
//     exactly as if the user had typed the command.

import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type updateIntent int

const (
	updateIntentNone updateIntent = iota
	// updateIntentBare is a self-contained update verb ("更新", "update"):
	// clear consent inside an update conversation, ambiguous outside one.
	updateIntentBare
	// updateIntentStrong references the latest version, an explicit version
	// tag, or the bot itself — unambiguous in any context.
	updateIntentStrong
	// updateIntentConfirm is an assent word ("确认", "yes"); only meaningful
	// right after an upgrade prompt.
	updateIntentConfirm
)

const (
	// updateNoticeConsentTTL is how long after a delivered update notice a
	// bare "更新" in that session still reads as consent to upgrade.
	updateNoticeConsentTTL = 48 * time.Hour
	// updateAskConsentTTL is how long after an upgrade prompt ("reply
	// confirm to install") an assent word still reads as confirmation.
	updateAskConsentTTL = 10 * time.Minute
	// updateIntentMaxRunes bounds matching to short messages; anything
	// longer is a real instruction for the agent.
	updateIntentMaxRunes          = 24
	updateReleaseBodyPreviewRunes = 3000
)

var (
	// Bare: an update verb with only politeness/particles around it.
	updateBareRe = regexp.MustCompile(`^(?:请|麻烦|帮我|给我)?(?:更新|升级|update|upgrade)(?:一下|一波| ?now)?(?:吧|呗)?$`)

	// Strong, form 1: update verb aimed at the latest version or an
	// explicit version tag ("更新到最新版", "upgrade to latest", "升级到 v0.2.0").
	updateStrongVersionRe = regexp.MustCompile(`^(?:请|麻烦|帮我|给我)?(?:把)?(?:你(?:自己)?|机器人|bot|cc-?connect(?:-?next)?)?(?:更新|升级|update|upgrade)(?:一下)?(?:到|至| to )? ?(?:最新版本|最新版|最新|新版本|latest(?: version)?|newest(?: version)?|v?\d+\.\d+[0-9a-z.\-]*)(?:吧|呗)?$`)

	// Strong, form 2: update verb aimed at the bot itself, in either order
	// ("把你自己升级一下", "更新你自己", "升级 cc-connect-next", "update yourself").
	updateStrongSelfRe = regexp.MustCompile(`^(?:请|麻烦|帮我|给我)?(?:把)?(?:你自己|你|机器人|bot|cc-?connect(?:-?next)?)(?:更新|升级)(?:一下)?(?:吧|呗)?$` +
		`|^(?:请|麻烦|帮我|给我)?(?:更新|升级)(?:一下)? ?(?:你自己|机器人|bot|cc-?connect(?:-?next)?)(?:吧|呗)?$` +
		`|^(?:update|upgrade) ?(?:yourself|cc-?connect(?:-?next)?|the bot)$`)

	updateConfirmRe = regexp.MustCompile(`^(?:确认|确定|確認|確定|是的|是|好的|好|嗯|可以|行|安装|开始吧|yes|y|ok|okay|confirm|sure|do it|go ahead)$`)
)

// matchUpdateIntent classifies a message. It requires the whole (normalized)
// message to match: any extra words mean the message has another object and
// must reach the agent untouched.
func matchUpdateIntent(content string) updateIntent {
	s := strings.TrimSpace(content)
	if s == "" || utf8.RuneCountInString(s) > updateIntentMaxRunes {
		return updateIntentNone
	}
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), " ") // collapse whitespace runs
	s = strings.TrimRight(s, "。！!？?～~.…,，")

	switch {
	case updateStrongVersionRe.MatchString(s), updateStrongSelfRe.MatchString(s):
		return updateIntentStrong
	case updateBareRe.MatchString(s):
		return updateIntentBare
	case updateConfirmRe.MatchString(s):
		return updateIntentConfirm
	}
	return updateIntentNone
}

// updateIntentState tracks, per session key, when an update notice or an
// upgrade prompt was last shown. In-memory only: a daemon restart (which an
// upgrade performs anyway) simply closes the consent windows.
type updateIntentState struct {
	mu       sync.Mutex
	noticeAt map[string]time.Time
	askAt    map[string]time.Time
}

func (s *updateIntentState) recordNotice(sessionKey string) {
	if sessionKey == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.noticeAt == nil {
		s.noticeAt = make(map[string]time.Time)
	}
	s.noticeAt[sessionKey] = time.Now()
}

func directUpdateNoticeKey(platform, userID string) string {
	platform = strings.TrimSpace(platform)
	userID = strings.TrimSpace(userID)
	if platform == "" || userID == "" {
		return ""
	}
	return "direct-user:" + platform + ":" + userID
}

func (s *updateIntentState) recordDirectNotice(platform, userID string) {
	s.recordNotice(directUpdateNoticeKey(platform, userID))
}

func (s *updateIntentState) recordAsk(sessionKey string) {
	if sessionKey == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.askAt == nil {
		s.askAt = make(map[string]time.Time)
	}
	s.askAt[sessionKey] = time.Now()
}

func (s *updateIntentState) noticeActive(sessionKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.noticeAt[sessionKey]
	return ok && time.Since(t) < updateNoticeConsentTTL
}

func (s *updateIntentState) askActive(sessionKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.askAt[sessionKey]
	return ok && time.Since(t) < updateAskConsentTTL
}

func (e *Engine) updateNoticeActiveForMessage(msg *Message) bool {
	if msg == nil {
		return false
	}
	if e.updateIntents.noticeActive(msg.SessionKey) {
		return true
	}
	return msg.IsDirect && e.updateIntents.noticeActive(directUpdateNoticeKey(msg.Platform, msg.UserID))
}

// maybeHandleUpdateIntent routes natural-language update requests into the
// /upgrade pipeline. Returns true when the message was consumed.
func (e *Engine) maybeHandleUpdateIntent(p Platform, msg *Message, content string) bool {
	if len(msg.Images) > 0 || len(msg.Files) > 0 || msg.Audio != nil {
		return false
	}
	intent := matchUpdateIntent(content)
	if intent == updateIntentNone {
		return false
	}

	switch intent {
	case updateIntentStrong:
		// A discovery notice is not an immutable update Plan. First prepare and
		// render the exact release/assets/checksum; only a reply after that Plan
		// is shown can confirm installation.
		if e.updateIntents.askActive(msg.SessionKey) {
			return e.handleCommand(p, msg, "/upgrade confirm")
		}
		return e.handleCommand(p, msg, "/upgrade")
	case updateIntentBare:
		if !e.updateNoticeActiveForMessage(msg) && !e.updateIntents.askActive(msg.SessionKey) {
			return false // ambiguous outside an update conversation → agent
		}
		if e.updateIntents.askActive(msg.SessionKey) {
			return e.handleCommand(p, msg, "/upgrade confirm")
		}
		return e.handleCommand(p, msg, "/upgrade")
	case updateIntentConfirm:
		if !e.updateIntents.askActive(msg.SessionKey) {
			return false // generic assent belongs to whatever else is pending
		}
		return e.handleCommand(p, msg, "/upgrade confirm")
	}
	return false
}

// withUpdateHint appends the natural-language hint as its own line, for
// surfaces that render buttons but have no footnote element.
func withUpdateHint(body, hint string) string {
	if hint == "" {
		return body
	}
	return body + "\n\n" + hint
}

func previewReleaseBody(body string) string {
	runes := []rune(body)
	if len(runes) > updateReleaseBodyPreviewRunes {
		return string(runes[:updateReleaseBodyPreviewRunes]) + "…"
	}
	return body
}

func releaseBodyForLanguage(body string, lang Language) string {
	heading := "English"
	if lang == LangChinese || lang == LangTraditionalChinese {
		heading = "中文"
	}
	marker := "\n## " + heading + "\n"
	content := "\n" + strings.ReplaceAll(body, "\r\n", "\n")
	start := strings.Index(content, marker)
	if start < 0 {
		return body
	}
	section := content[start+len(marker):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return strings.TrimSpace(section)
}

func localizedReleaseBodyPreview(body string, lang Language) string {
	return previewReleaseBody(releaseBodyForLanguage(body, lang))
}

// replyUpdateActionable delivers the exact release Plan with a confirm action
// action on platforms that support cards or inline buttons, falling back to
// plain text elsewhere. The buttons reuse the cmd:/ scheme, so a tap goes
// through exactly the gates a typed command would.
//
// Three pieces of copy, one rule — a message carries exactly one *primary*
// call to action:
//
//   - actionCopy: the facts, shown next to a button. The button is the CTA.
//   - hint: a footnote (small grey on cards) telling the user a plain reply
//     works too. Subordinate on purpose — discoverable, never competing.
//   - textCopy: for surfaces with no button at all, where the typed reply
//     *is* the CTA and therefore must be stated outright. Also used when a
//     card or button send fails, so delivery never degrades into
//     button-copy without a button.
func (e *Engine) replyUpdateActionable(p Platform, replyCtx any, token, actionCopy, textCopy string) {
	btnNow := e.i18n.T(MsgUpdateBtnNow)
	hint := e.i18n.T(MsgUpdateHintReplyConfirm)
	confirmCommand := "/upgrade confirm " + token
	if cs, ok := p.(CardSender); ok {
		card := e.renderCardForPlatform(p, NewCard().
			Markdown(actionCopy).
			Buttons(CardButton{Text: btnNow, Type: "primary", Value: "cmd:" + confirmCommand}).
			Note(hint).
			Build())
		if err := cs.ReplyCard(e.ctx, replyCtx, card); err == nil {
			return
		}
	}
	if bs, ok := p.(InlineButtonSender); ok {
		row := []ButtonOption{{Text: btnNow, Data: "cmd:" + confirmCommand}}
		if err := bs.SendWithButtons(e.ctx, replyCtx, withUpdateHint(actionCopy, hint), [][]ButtonOption{row}); err == nil {
			return
		}
	}
	e.reply(p, replyCtx, textCopy+"\n\n`"+confirmCommand+"`")
}
