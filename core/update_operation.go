package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

const updateWaitLimit = 30 * time.Minute
const updateInstallLimit = 10 * time.Minute
const updateRenderInterval = 3 * time.Second

type updateRecord struct {
	ID                 string          `json:"id"`
	Project            string          `json:"project"`
	Platform           string          `json:"platform"`
	SessionKey         string          `json:"session_key"`
	UserID             string          `json:"user_id"`
	FromVersion        string          `json:"from_version"`
	TargetVersion      string          `json:"target_version"`
	Installation       string          `json:"installation,omitempty"`
	Stage              string          `json:"stage"`
	Detail             string          `json:"detail,omitempty"`
	Started            time.Time       `json:"started"`
	Finished           time.Time       `json:"finished,omitempty"`
	Updated            time.Time       `json:"updated"`
	Card               json.RawMessage `json:"card,omitempty"`
	ConnectionsPending bool            `json:"connections_pending,omitempty"`
	ReplacementSent    bool            `json:"replacement_sent,omitempty"`
	ReceiptDelivered   bool            `json:"receipt_delivered"`
}

type updateOperation struct {
	mu          sync.Mutex
	record      updateRecord
	engine      *Engine
	view        *updateView
	cancel      context.CancelFunc
	cancellable bool
	changed     chan struct{}
	touched     bool
	retry       bool
}

func (op *updateOperation) snapshot() updateRecord {
	op.mu.Lock()
	defer op.mu.Unlock()
	r := op.record
	r.Card = append(json.RawMessage(nil), r.Card...)
	return r
}
func (op *updateOperation) stage(stage, detail string) {
	op.mu.Lock()
	op.record.Stage = stage
	op.record.Detail = detail
	op.record.Updated = time.Now()
	op.mu.Unlock()
	select {
	case op.changed <- struct{}{}:
	default:
	}
}
func (op *updateOperation) progress(event appfeatures.UpdateEvent) {
	stage := ""
	switch event.Stage {
	case appfeatures.StageDownloadingArchive, appfeatures.StageDownloadingChecksums:
		stage = "downloading"
	case appfeatures.StageInstalling:
		stage = "installing"
		op.mu.Lock()
		op.touched = true
		op.mu.Unlock()
	case appfeatures.StageChecksumVerified, appfeatures.StageStagedVerified:
		stage = "verifying-package"
	case appfeatures.StageInstalledVerified, appfeatures.StageVerifyingInstalled:
		stage = "verifying"
	}
	if stage != "" {
		op.stage(stage, "")
	}
}
func (op *updateOperation) persist() error {
	op.mu.Lock()
	op.record.Card = op.view.savedReference()
	op.view.mu.Lock()
	op.record.ReplacementSent = op.view.fallback
	op.view.mu.Unlock()
	r := op.record
	op.mu.Unlock()
	return writeUpdateRecord(op.engine.dataDir, r)
}
func (op *updateOperation) render() error {
	r := op.snapshot()
	op.mu.Lock()
	cancellable, retry := op.cancellable, op.retry
	op.mu.Unlock()
	detail := r.Detail
	if detail == "" && time.Since(r.Updated) >= 15*time.Second {
		detail = op.engine.i18n.T(MsgUpdateStillWaiting)
	}
	started := r.Started
	if !r.Finished.IsZero() {
		started = time.Now().Add(-r.Finished.Sub(r.Started))
	}
	card := op.engine.updateStateCard(r.Stage, r.TargetVersion, detail, started, r.ID, cancellable)
	if retry {
		card.Elements = append(card.Elements, CardActions{Buttons: []CardButton{PrimaryBtn(op.engine.i18n.T(MsgUpdateRetry), "cmd:/upgrade confirm "+r.ID)}})
	}
	transient := r.Stage != "completed" && r.Stage != "failed" && r.Stage != "cancelled"
	return op.view.render(card, transient)
}
func (op *updateOperation) present(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(updateRenderInterval)
	defer ticker.Stop()
	lastStage := ""
	lastRender := time.Time{}
	for {
		select {
		case <-stop:
			return
		case <-op.engine.ctx.Done():
			return
		case <-ticker.C:
		case <-op.changed:
		}
		r := op.snapshot()
		if time.Since(lastRender) < time.Second {
			continue
		}
		if !op.view.editable() && r.Stage == lastStage {
			continue
		}
		if err := op.render(); err != nil {
			slog.Warn("update progress presentation failed", "error", err)
		}
		if r.Stage != lastStage {
			if err := op.persist(); err != nil {
				slog.Error("update progress persistence failed", "error", err)
			}
		}
		lastStage = r.Stage
		lastRender = time.Now()
	}
}

func writeUpdateRecord(dataDir string, r updateRecord) error {
	if dataDir == "" {
		return nil
	} // in-memory Engines used by embedders/tests
	if !validUpdateID(r.ID) {
		return fmt.Errorf("invalid update result identifier")
	}
	dir := filepath.Join(dataDir, "run", "update-results")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".update-result-")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() { _ = os.Remove(name) }()
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(dir, r.ID+".json")); err != nil {
		return err
	}
	return nil
}
func validUpdateID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

// Keep each result separately so a later attempt cannot erase an undelivered
// receipt or turn an old completed card into an installation action.
func readUpdateRecords(dataDir string) ([]updateRecord, error) {
	if dataDir == "" {
		return nil, os.ErrNotExist
	}
	dir := filepath.Join(dataDir, "run", "update-results")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var records []updateRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		var r updateRecord
		if err != nil || len(data) > 64*1024 || json.Unmarshal(data, &r) != nil || !validUpdateID(r.ID) || r.ID+".json" != entry.Name() || r.Project == "" || r.TargetVersion == "" {
			slog.Warn("invalid update result retained for local inspection", "file", entry.Name())
			continue
		}
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Started.After(records[j].Started) })
	return records, nil
}
func readUpdateRecord(dataDir string) (updateRecord, error) {
	records, err := readUpdateRecords(dataDir)
	if err != nil {
		return updateRecord{}, err
	}
	if len(records) == 0 {
		return updateRecord{}, os.ErrNotExist
	}
	return records[0], nil
}

func (e *Engine) installReviewedUpdate(p Platform, msg *Message, token string, plan PreparedUpdate, service UpdateService, view *updateView) {
	if view == nil {
		view = newUpdateView(e, p, msg)
	}
	restartPath, pathErr := os.Executable()
	if target, ok := service.(interface{ UpdateExecutablePath() string }); ok {
		restartPath = target.UpdateExecutablePath()
	}
	if pathErr != nil || restartPath == "" {
		view.show(e.updateStateCard("failed", "", e.i18n.T(MsgUpdateSafeFailure), time.Time{}, "", false), false)
		return
	}
	waitCtx, cancel := context.WithTimeout(e.ctx, updateWaitLimit)
	defer cancel()
	op := &updateOperation{engine: e, view: view, cancel: cancel, cancellable: true, changed: make(chan struct{}, 1), record: updateRecord{
		ID: token, Project: e.name, Platform: p.Name(), SessionKey: msg.SessionKey, UserID: msg.UserID,
		FromVersion: CurrentVersion, TargetVersion: plan.Release.TagName, Stage: "waiting", Started: time.Now(), Updated: time.Now(),
	}}
	if info, ok := service.(interface{ UpdateInstallationKind() string }); ok {
		op.record.Installation = info.UpdateInstallationKind()
	}
	coordinator := e.updates()
	if existing, ok := coordinator.reserve(op); !ok {
		if existing == nil {
			view.show(e.updateStateCard("restarting", "", e.i18n.T(MsgRestarting), time.Time{}, "", false), false)
			return
		}
		r := existing.snapshot()
		if r.Project == e.name && r.SessionKey == msg.SessionKey && r.UserID == msg.UserID {
			if err := existing.render(); err != nil {
				slog.Warn("update status delivery failed", "error", err)
			}
		} else {
			view.show(e.updateStateCard("waiting", plan.Release.TagName, e.i18n.T(MsgUpdateBusyState), time.Time{}, "", false), false)
		}
		return
	}
	restarting := false
	defer func() {
		if !restarting {
			coordinator.release(op)
		}
	}()
	if err := op.persist(); err != nil {
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgError, err))
		return
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go op.present(stop, done)
	stopPresentation := sync.OnceFunc(func() { close(stop); <-done })
	defer stopPresentation()
	err := coordinator.waitIdle(waitCtx, func(n int) { op.stage("waiting", e.i18n.Tf(MsgUpdateActiveTasks, n)) })
	op.mu.Lock()
	op.cancellable = false
	if err == nil {
		err = waitCtx.Err()
	}
	op.mu.Unlock()
	if err != nil {
		stopPresentation()
		if errors.Is(err, context.DeadlineExceeded) {
			op.stage("failed", e.i18n.T(MsgUpdateWaitingExpired))
		} else {
			op.stage("cancelled", "")
		}
		e.clearUpdatePlan(msg.SessionKey, msg.UserID, token)
		e.finishUpdatePresentation(op)
		return
	}
	op.stage("installing", "")
	if err := op.persist(); err != nil {
		stopPresentation()
		op.stage("failed", e.i18n.T(MsgUpdateSafeFailure))
		e.finishUpdatePresentation(op)
		return
	}
	applyCtx, applyCancel := context.WithTimeout(e.ctx, updateInstallLimit)
	var result UpdateResult
	if progress, ok := service.(UpdateProgressService); ok {
		result, err = progress.ApplyWithProgress(applyCtx, plan, op.progress)
	} else {
		op.mu.Lock()
		op.touched = true
		op.mu.Unlock()
		result, err = service.Apply(applyCtx, plan)
	}
	applyCancel()
	stopPresentation()
	if err != nil {
		op.mu.Lock()
		touched := op.touched
		op.retry = !touched && !terminalUpdatePlanError(err)
		op.mu.Unlock()
		if touched || terminalUpdatePlanError(err) {
			e.clearUpdatePlan(msg.SessionKey, msg.UserID, token)
		}
		detail := e.i18n.T(MsgUpdateSafeFailure)
		if touched {
			detail = e.i18n.T(MsgUpdateUncertainFailure)
		}
		if errors.Is(err, ErrUpdateInstalledUnverified) {
			detail = e.i18n.Tf(MsgUpgradeInstalledUnverified, appfeatures.RedactFeedbackText(err.Error()))
		}
		slog.Warn("update apply failed", "error", appfeatures.RedactFeedbackText(err.Error()))
		op.stage("failed", detail)
		e.finishUpdatePresentation(op)
		return
	}
	e.clearUpdatePlan(msg.SessionKey, msg.UserID, token)
	if !result.Updated {
		op.stage("completed", e.i18n.Tf(MsgUpgradeUpToDateChannel, plan.Release.Channel, CurrentVersion))
		e.finishUpdatePresentation(op)
		return
	}
	if result.Release.TagName != plan.Release.TagName {
		op.stage("failed", e.i18n.Tf(MsgUpdateMismatch, plan.Release.TagName, result.Release.TagName))
		e.finishUpdatePresentation(op)
		return
	}
	op.stage("restarting", "")
	// Persist the exact expected version and card BEFORE replacing the process.
	if err := op.persist(); err != nil {
		op.stage("failed", e.i18n.T(MsgUpdateUncertainFailure))
		e.finishUpdatePresentation(op)
		return
	}
	if err := op.render(); err != nil {
		slog.Warn("update restart presentation failed", "error", err)
	}
	if err := op.persist(); err != nil {
		op.stage("failed", e.i18n.T(MsgUpdateUncertainFailure))
		e.finishUpdatePresentation(op)
		return
	}
	select {
	case RestartCh <- RestartRequest{SessionKey: msg.SessionKey, Platform: p.Name(), UpdateID: token, ExecutablePath: restartPath}:
		restarting = true
	default:
		op.stage("failed", e.i18n.T(MsgUpdateRecoveryHint))
		e.finishUpdatePresentation(op)
	}
}

func (e *Engine) finishUpdatePresentation(op *updateOperation) {
	op.mu.Lock()
	op.record.Finished = time.Now()
	op.mu.Unlock()
	if err := op.persist(); err != nil {
		slog.Error("update result persistence failed", "error", err)
	}
	if err := op.render(); err != nil {
		slog.Warn("update result presentation failed", "error", err)
		return
	}
	op.mu.Lock()
	op.record.ReceiptDelivered = true
	op.mu.Unlock()
	if err := op.persist(); err != nil {
		slog.Error("update receipt persistence failed", "error", err)
	}
}

func (e *Engine) cancelUpdate(p Platform, msg *Message, token string) {
	if e.cancelUpdateCheck(msg, token) {
		return
	}
	c := e.updates()
	c.mu.Lock()
	op := c.operation
	c.mu.Unlock()
	if op != nil {
		op.mu.Lock()
		own := op.record.Project == e.name && op.record.SessionKey == msg.SessionKey && op.record.UserID == msg.UserID && (token == "" || op.record.ID == token)
		if own && op.cancellable {
			op.cancel()
			op.mu.Unlock()
			return
		}
		op.mu.Unlock()
		if own {
			if err := op.render(); err != nil {
				slog.Warn("update status delivery failed", "error", err)
			}
			return
		}
	}
	if _, ok := e.pendingUpdatePlan(msg.SessionKey, msg.UserID, token); ok {
		e.updateServiceMu.Lock()
		key := token
		if key == "" {
			key = e.uniqueUpdateTokenLocked(msg.SessionKey, msg.UserID)
		}
		view := e.updatePlans[key].View
		e.updateServiceMu.Unlock()
		if view == nil {
			view = newUpdateView(e, p, msg)
		}
		e.clearUpdatePlan(msg.SessionKey, msg.UserID, token)
		view.show(e.updateStateCard("cancelled", "", "", time.Time{}, "", false), false)
		return
	}
	e.showUpdateStatus(p, msg, token)
}

func (e *Engine) showUpdateStatus(p Platform, msg *Message, token string) {
	c := e.updates()
	c.mu.Lock()
	op := c.operation
	c.mu.Unlock()
	if op != nil {
		r := op.snapshot()
		if r.Project == e.name && r.SessionKey == msg.SessionKey && r.UserID == msg.UserID {
			if err := op.render(); err != nil {
				slog.Warn("update status delivery failed", "error", err)
			}
			return
		}
		newUpdateView(e, p, msg).show(e.updateStateCard(r.Stage, r.TargetVersion, e.i18n.T(MsgUpdateBusyState), r.Started, "", false), false)
		return
	}
	records, _ := readUpdateRecords(e.dataDir)
	for _, r := range records {
		if r.Project == e.name && r.SessionKey == msg.SessionKey && r.UserID == msg.UserID && (token == "" || r.ID == token) {
			started := r.Started
			if !r.Finished.IsZero() {
				started = time.Now().Add(-r.Finished.Sub(r.Started))
			}
			newUpdateView(e, p, msg).show(e.updateStateCard(r.Stage, r.TargetVersion, r.Detail, started, r.ID, false), false)
			return
		}
	}
	newUpdateView(e, p, msg).show(e.updateStateCard("expired", "", e.i18n.T(MsgUpdateNoResult), time.Time{}, "", false), false)
}

// RecoverUpdateResult is called at startup and on platform recovery. A failed
// delivery stays on disk and can also be inspected with /upgrade status.
func (e *Engine) RecoverUpdateResult() {
	e.updateReceiptMu.Lock()
	defer e.updateReceiptMu.Unlock()
	c := e.updates()
	c.mu.Lock()
	busy := c.operation != nil
	c.mu.Unlock()
	if busy {
		return
	}
	records, err := readUpdateRecords(e.dataDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("update results unavailable", "error", err)
		}
		return
	}
	for _, r := range records {
		if r.Project == e.name && (!r.ReceiptDelivered || r.ConnectionsPending) {
			e.recoverUpdateRecord(r)
		}
	}
}

func (e *Engine) recoverUpdateRecord(r updateRecord) {
	p := e.lookupReadyPlatform(r.Platform)
	if p == nil {
		return
	}
	rctx, err := e.reconstructReplyContext(p, r.SessionKey)
	if err != nil {
		slog.Warn("update receipt target unavailable", "error", appfeatures.RedactFeedbackText(err.Error()))
		return
	}
	if r.Stage == "restarting" || r.ConnectionsPending {
		if strings.TrimPrefix(CurrentVersion, "v") != strings.TrimPrefix(r.TargetVersion, "v") {
			r.Stage = "failed"
			r.ConnectionsPending = false
			r.Detail = e.i18n.Tf(MsgUpdateMismatch, r.TargetVersion, CurrentVersion)
		} else {
			r.Stage = "completed"
			r.Detail = e.i18n.Tf(MsgUpdateConnected, CurrentVersion)
			pending := !e.updates().connectionsReady()
			if r.ReceiptDelivered && r.ConnectionsPending && pending {
				return
			}
			r.ConnectionsPending = pending
			if pending {
				r.Detail += "\n" + e.i18n.T(MsgUpdatePartial)
			}
		}
	} else if r.Stage != "completed" && r.Stage != "failed" && r.Stage != "cancelled" {
		r.Stage = "failed"
		r.Detail = e.i18n.T(MsgUpdateInterrupted)
	}
	if r.Finished.IsZero() {
		r.Finished = time.Now()
	}
	view := &updateView{engine: e, platform: p, replyCtx: rctx, reference: r.Card, fallback: r.ReplacementSent}
	op := &updateOperation{engine: e, view: view, record: r, changed: make(chan struct{}, 1)}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(attempt) * time.Second)
			select {
			case <-e.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		if err := op.render(); err == nil {
			op.mu.Lock()
			op.record.ReceiptDelivered = true
			op.mu.Unlock()
			if err := op.persist(); err != nil {
				slog.Error("update receipt persistence failed", "error", err)
			}
			return
		} else {
			slog.Warn("update recovery delivery failed", "error", err)
		}
	}
	if err := op.persist(); err != nil {
		slog.Error("update recovery persistence failed", "error", err)
	}
}
