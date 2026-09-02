package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
)

// ReleaseInfo is the host projection used by notices and localized cards.
// Body is publisher-controlled text from the exact GitHub Release.
type ReleaseInfo struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	HTMLURL    string `json:"html_url"`
	Prerelease bool   `json:"prerelease"`
	Channel    string `json:"channel"`
	CreatedAt  string `json:"created_at"`
}

// PreparedUpdate is the complete presentation-safe view of one immutable
// plan. token remains opaque to product UI and can only be consumed by the
// UpdateService that created it.
type PreparedUpdate struct {
	Release      ReleaseInfo
	ArchiveAsset string
	Available    bool
	token        any
}

type UpdateResult struct {
	Release          ReleaseInfo
	Updated          bool
	ArchiveAsset     string
	BackupRetainedAt string
}

// UpdateService separates the product confirmation flow from installation.
// Implementations must apply exactly the plan returned by Prepare.
type UpdateService interface {
	Prepare(context.Context) (PreparedUpdate, error)
	Apply(context.Context, PreparedUpdate) (UpdateResult, error)
}

type foundationUpdateService struct {
	service *appfeatures.UpdateService
	channel updatechannel.Channel
}

func newFoundationUpdateService(channel updatechannel.Channel) (*foundationUpdateService, error) {
	channel = channel.Effective()
	service, err := appfeatures.NewUpdateService(appfeatures.UpdateConfig{
		CurrentVersion: CurrentVersion,
		Channel:        channel,
		Progress: func(event appfeatures.UpdateEvent) {
			slog.Info("updater: progress",
				"stage", event.Stage,
				"current", event.CurrentVersion,
				"target", event.TargetVersion,
				"asset", event.Asset,
				"bytes", event.Bytes,
			)
		},
	})
	if err != nil {
		return nil, err
	}
	return &foundationUpdateService{service: service, channel: channel}, nil
}

func (service *foundationUpdateService) Prepare(ctx context.Context) (PreparedUpdate, error) {
	plan, err := service.service.Prepare(ctx)
	if err != nil {
		return PreparedUpdate{}, err
	}
	release := plan.Release()
	return PreparedUpdate{
		Release:      releaseInfo(release, service.channel),
		ArchiveAsset: plan.ArchiveAsset().Name,
		Available:    plan.Available(),
		token:        plan,
	}, nil
}

func (service *foundationUpdateService) Apply(ctx context.Context, prepared PreparedUpdate) (UpdateResult, error) {
	plan, ok := prepared.token.(appfeatures.UpdatePlan)
	if !ok {
		return UpdateResult{}, fmt.Errorf("update plan is invalid")
	}
	result, err := service.service.Apply(ctx, plan)
	return UpdateResult{
		Release:          releaseInfo(result.Release, service.channel),
		Updated:          result.Updated,
		ArchiveAsset:     result.ArchiveAsset,
		BackupRetainedAt: result.BackupRetainedAt,
	}, err
}

func releaseInfo(release appfeatures.UpdateRelease, channel updatechannel.Channel) ReleaseInfo {
	return ReleaseInfo{
		TagName:    release.Tag,
		Name:       release.Tag,
		Body:       release.Notes,
		HTMLURL:    release.URL,
		Prerelease: release.Prerelease,
		Channel:    string(channel.Effective()),
	}
}

// CheckForUpdate preserves the historical Stable discovery API. Channel-aware
// callers use CheckForUpdateInChannel; interactive flows still call Prepare
// before showing an approval prompt.
func CheckForUpdate(currentVersion string, _ bool) (*ReleaseInfo, error) {
	return CheckForUpdateInChannel(currentVersion, updatechannel.Stable)
}

func CheckForUpdateInChannel(currentVersion string, channel updatechannel.Channel) (*ReleaseInfo, error) {
	channel = channel.Effective()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	release, err := appfeatures.CheckForUpdateChannel(ctx, currentVersion, channel, nil)
	if err != nil || release == nil {
		return nil, err
	}
	value := releaseInfo(*release, channel)
	return &value, nil
}

// SelfUpdate remains for callers that already represent an authorized,
// non-interactive action. Interactive chat uses Prepare/Apply directly.
func SelfUpdate(tag string, prerelease bool) error {
	channel := updatechannel.Stable
	if prerelease {
		channel = updatechannel.Beta
	}
	service, err := newFoundationUpdateService(channel)
	if err != nil {
		return err
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		return err
	}
	if plan.Release.TagName != tag {
		return fmt.Errorf("prepared release %s does not match requested %s", plan.Release.TagName, tag)
	}
	_, err = service.Apply(context.Background(), plan)
	return err
}

func (e *Engine) SetUpdateService(service UpdateService) {
	e.updateServiceMu.Lock()
	defer e.updateServiceMu.Unlock()
	if e.updateServices == nil {
		e.updateServices = make(map[updatechannel.Channel]UpdateService)
	}
	e.updateServices[e.updateChannel.Effective()] = service
	e.updatePlans = nil
}

func (e *Engine) SetUpdateChannel(channel string) {
	e.updateServiceMu.Lock()
	defer e.updateServiceMu.Unlock()
	e.updateChannel = updatechannel.Channel(channel).Effective()
	e.updatePlans = nil
}

func (e *Engine) currentUpdateService(channel updatechannel.Channel) (UpdateService, error) {
	e.updateServiceMu.Lock()
	defer e.updateServiceMu.Unlock()
	channel = channel.Effective()
	if e.updateServices == nil {
		e.updateServices = make(map[updatechannel.Channel]UpdateService)
	}
	if service := e.updateServices[channel]; service != nil {
		return service, nil
	}
	service, err := newFoundationUpdateService(channel)
	if err != nil {
		return nil, err
	}
	e.updateServices[channel] = service
	return service, nil
}

const (
	pendingUpdatePlanTTL = 10 * time.Minute
	pendingUpdatePlanMax = 64
)

type pendingUpdatePlan struct {
	Plan       PreparedUpdate
	At         time.Time
	SessionKey string
	UserID     string
}

func (e *Engine) rememberUpdatePlan(sessionKey, userID string, plan PreparedUpdate) (string, error) {
	if sessionKey == "" {
		return "", fmt.Errorf("update session is required")
	}
	token, err := newPendingActionToken()
	if err != nil {
		return "", err
	}
	e.updateServiceMu.Lock()
	defer e.updateServiceMu.Unlock()
	if e.updatePlans == nil {
		e.updatePlans = make(map[string]pendingUpdatePlan)
	}
	now := time.Now()
	e.pruneUpdatePlansLocked(now)
	for len(e.updatePlans) >= pendingUpdatePlanMax {
		e.evictOldestUpdatePlanLocked()
	}
	e.updatePlans[token] = pendingUpdatePlan{Plan: plan, At: now, SessionKey: sessionKey, UserID: userID}
	return token, nil
}

func (e *Engine) pendingUpdatePlan(sessionKey, userID, token string) (PreparedUpdate, bool) {
	e.updateServiceMu.Lock()
	defer e.updateServiceMu.Unlock()
	e.pruneUpdatePlansLocked(time.Now())
	if token == "" {
		token = e.uniqueUpdateTokenLocked(sessionKey, userID)
	}
	pending, exists := e.updatePlans[token]
	if !exists {
		return PreparedUpdate{}, false
	}
	if pending.SessionKey != sessionKey || pending.UserID != userID {
		return PreparedUpdate{}, false
	}
	return pending.Plan, true
}

func (e *Engine) clearUpdatePlan(sessionKey, userID, token string) {
	e.updateServiceMu.Lock()
	defer e.updateServiceMu.Unlock()
	if token == "" {
		token = e.uniqueUpdateTokenLocked(sessionKey, userID)
	}
	if pending, exists := e.updatePlans[token]; exists && pending.SessionKey == sessionKey && pending.UserID == userID {
		e.deleteUpdatePlanLocked(token)
	}
}

func (e *Engine) uniqueUpdateTokenLocked(sessionKey, userID string) string {
	match := ""
	for token, pending := range e.updatePlans {
		if pending.SessionKey != sessionKey || pending.UserID != userID {
			continue
		}
		if match != "" {
			return ""
		}
		match = token
	}
	return match
}

func (e *Engine) deleteUpdatePlanLocked(token string) {
	delete(e.updatePlans, token)
}

func (e *Engine) pruneUpdatePlansLocked(now time.Time) {
	for token, pending := range e.updatePlans {
		if now.Sub(pending.At) > pendingUpdatePlanTTL {
			e.deleteUpdatePlanLocked(token)
		}
	}
}

func (e *Engine) evictOldestUpdatePlanLocked() {
	oldestToken := ""
	var oldest pendingUpdatePlan
	for token, pending := range e.updatePlans {
		if oldestToken == "" || pending.At.Before(oldest.At) {
			oldestToken, oldest = token, pending
		}
	}
	if oldestToken != "" {
		e.deleteUpdatePlanLocked(oldestToken)
	}
}

func terminalUpdatePlanError(err error) bool {
	return appfeatures.IsTerminalUpdatePlanError(err)
}
