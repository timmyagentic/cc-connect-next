package core

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
)

// AnswerProfileName identifies a configured one-shot answer profile.
// Empty means the ordinary project default; there is deliberately no named
// "default" or "balanced" profile because ordinary messages already carry
// that meaning.
type AnswerProfileName string

const (
	AnswerProfileFast    AnswerProfileName = "fast"
	AnswerProfileQuality AnswerProfileName = "quality"
)

// AnswerProfileOptions contains the settings a named profile overrides. Empty
// fields inherit the current agent defaults when the turn starts.
type AnswerProfileOptions struct {
	Model           string
	ReasoningEffort string
	ServiceTier     string
}

// AnswerProfiles is the complete profile surface supported by the chat parser.
// A nil entry means that profile was not configured for this project.
type AnswerProfiles struct {
	Fast    *AnswerProfileOptions
	Quality *AnswerProfileOptions
}

var ErrTurnOptionsUnsupported = errors.New("agent session does not support one-shot answer profiles")

// SetAnswerProfiles configures the two named one-shot profiles. It is expected
// to run during engine bootstrap before messages are accepted.
func (e *Engine) SetAnswerProfiles(profiles AnswerProfiles) {
	clone := func(options *AnswerProfileOptions) *AnswerProfileOptions {
		if options == nil {
			return nil
		}
		copy := *options
		copy.Model = strings.TrimSpace(copy.Model)
		copy.ReasoningEffort = strings.TrimSpace(copy.ReasoningEffort)
		copy.ServiceTier = strings.TrimSpace(copy.ServiceTier)
		return &copy
	}
	e.answerProfilesMu.Lock()
	e.answerProfiles = AnswerProfiles{
		Fast:    clone(profiles.Fast),
		Quality: clone(profiles.Quality),
	}
	e.answerProfilesMu.Unlock()
}

func (e *Engine) answerProfilesConfigured() bool {
	e.answerProfilesMu.RLock()
	defer e.answerProfilesMu.RUnlock()
	return e.answerProfiles.configured()
}

func (e *Engine) answerProfile(name AnswerProfileName) (*AnswerProfileOptions, bool) {
	e.answerProfilesMu.RLock()
	defer e.answerProfilesMu.RUnlock()
	return e.answerProfiles.get(name)
}

func defaultTurnOptions(agent Agent) TurnOptions {
	options := TurnOptions{}
	if getter, ok := agent.(interface{ GetModel() string }); ok {
		options.Model = strings.TrimSpace(getter.GetModel())
	}
	if getter, ok := agent.(interface{ GetReasoningEffort() string }); ok {
		options.ReasoningEffort = strings.TrimSpace(getter.GetReasoningEffort())
	}
	if getter, ok := agent.(ServiceTierProvider); ok {
		options.ServiceTier = strings.TrimSpace(getter.GetServiceTier())
	}
	return options
}

func (e *Engine) resolveTurnOptions(agent Agent, profile AnswerProfileName) (TurnOptions, error) {
	options := defaultTurnOptions(agent)
	if profile == "" {
		return options, nil
	}
	override, ok := e.answerProfile(profile)
	if !ok {
		return TurnOptions{}, fmt.Errorf("answer profile %q is not configured", profile)
	}
	options.AnswerProfile = profile
	if value := strings.TrimSpace(override.Model); value != "" {
		options.Model = value
	}
	if value := strings.TrimSpace(override.ReasoningEffort); value != "" {
		options.ReasoningEffort = value
	}
	if value := strings.TrimSpace(override.ServiceTier); value != "" {
		options.ServiceTier = value
	}
	return options, nil
}

func (e *Engine) sendAgentTurn(agent Agent, session AgentSession, prompt string, images []ImageAttachment, files []FileAttachment, profile AnswerProfileName) error {
	sender, supportsOptions := session.(TurnOptionsSession)
	if profile == "" && (!e.answerProfilesConfigured() || !supportsOptions) {
		return session.Send(prompt, images, files)
	}
	if !supportsOptions {
		return ErrTurnOptionsUnsupported
	}
	options, err := e.resolveTurnOptions(agent, profile)
	if err != nil {
		return err
	}
	if profile != "" {
		slog.Info("using one-shot answer profile",
			"project", e.name,
			"profile", profile,
			"model", options.Model,
			"reasoning_effort", options.ReasoningEffort,
			"service_tier", options.ServiceTier,
		)
	} else {
		slog.Debug("restoring default turn options", "project", e.name)
	}
	return sender.SendWithTurnOptions(prompt, images, files, options)
}

func (p AnswerProfiles) configured() bool {
	return p.Fast != nil || p.Quality != nil
}

func (p AnswerProfiles) get(name AnswerProfileName) (*AnswerProfileOptions, bool) {
	var options *AnswerProfileOptions
	switch name {
	case AnswerProfileFast:
		options = p.Fast
	case AnswerProfileQuality:
		options = p.Quality
	}
	if options == nil {
		return nil, false
	}
	copy := *options
	return &copy, true
}

// parseAnswerProfilePrefix recognizes only explicit leading directives. It
// intentionally does not scan the rest of the message, infer intent with an
// LLM, accept abbreviations, or provide a default/balanced directive.
func parseAnswerProfilePrefix(content string) (AnswerProfileName, string, bool) {
	trimmed := strings.TrimSpace(content)
	if profile, prompt, ok := parseAnswerProfileCommand(trimmed); ok {
		return profile, prompt, true
	}
	if profile, prompt, ok := parseAnswerProfileLabel(trimmed); ok {
		return profile, prompt, true
	}
	if profile, prompt, ok := parseAnswerProfileNaturalDirective(trimmed); ok {
		return profile, prompt, true
	}
	return "", content, false
}

func parseAnswerProfileCommand(content string) (AnswerProfileName, string, bool) {
	end := strings.IndexFunc(content, unicode.IsSpace)
	token := content
	rest := ""
	if end >= 0 {
		token = content[:end]
		rest = strings.TrimSpace(content[end:])
	}
	switch strings.ToLower(token) {
	case "/fast":
		return AnswerProfileFast, rest, true
	case "/quality":
		return AnswerProfileQuality, rest, true
	default:
		return "", "", false
	}
}

func parseAnswerProfileLabel(content string) (AnswerProfileName, string, bool) {
	for _, candidate := range []struct {
		prefix  string
		profile AnswerProfileName
	}{
		{prefix: "高质量模式", profile: AnswerProfileQuality},
		{prefix: "快速模式", profile: AnswerProfileFast},
	} {
		if !strings.HasPrefix(content, candidate.prefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(content, candidate.prefix))
		if rest == "" {
			continue
		}
		if rest[0] != ':' && !strings.HasPrefix(rest, "：") {
			continue
		}
		rest = strings.TrimSpace(strings.TrimLeft(rest, ":："))
		return candidate.profile, rest, true
	}
	return "", "", false
}

func parseAnswerProfileNaturalDirective(content string) (AnswerProfileName, string, bool) {
	for _, candidate := range []struct {
		prefix  string
		profile AnswerProfileName
	}{
		{prefix: "请使用高质量模式", profile: AnswerProfileQuality},
		{prefix: "请用高质量模式", profile: AnswerProfileQuality},
		{prefix: "使用高质量模式", profile: AnswerProfileQuality},
		{prefix: "用高质量模式", profile: AnswerProfileQuality},
		{prefix: "请使用快速模式", profile: AnswerProfileFast},
		{prefix: "请用快速模式", profile: AnswerProfileFast},
		{prefix: "使用快速模式", profile: AnswerProfileFast},
		{prefix: "用快速模式", profile: AnswerProfileFast},
	} {
		if !strings.HasPrefix(content, candidate.prefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(content, candidate.prefix))
		// "来" links the profile directive to the task ("用…模式来完成").
		// Keep the task verb itself: stripping bare words such as "处理" would
		// corrupt legitimate prompts like "处理器是什么".
		if strings.HasPrefix(rest, "来") {
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "来"))
		}
		rest = strings.TrimSpace(strings.TrimLeft(rest, ":：,，。.!！?？"))
		return candidate.profile, rest, true
	}
	return "", "", false
}
