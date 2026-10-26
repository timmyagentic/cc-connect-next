package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

type updateExperiencePlatform struct {
	stubCardPlatform
	edits []*Card
}

func (p *updateExperiencePlatform) UpdateCard(_ context.Context, _ any, card *Card) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.edits = append(p.edits, card)
	return nil
}

func TestUpdateExperience_ReviewEditsClickedCardAndKeepsDecisionCompact(t *testing.T) {
	withCurrentVersion(t, "v1.0.0")
	p := &updateExperiencePlatform{stubCardPlatform: stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
	e.SetAdminFrom("admin")
	e.SetUpdateService(&stubCoreUpdateService{plan: PreparedUpdate{
		Release:      ReleaseInfo{TagName: "v1.1.0", Body: "## 中文\n\n### 更新亮点\n- 更可靠的更新进度\n\n### 安装\n" + strings.Repeat("npm install technical details ", 150), HTMLURL: "https://github.com/example/product/releases/tag/v1.1.0"},
		ArchiveAsset: "cc-connect-next@1.1.0", Available: true, token: "exact-plan",
	}})
	e.cmdUpgrade(p, &Message{SessionKey: "test:admin", UserID: "admin", Platform: "test", ReplyCtx: "card-1", IsCardAction: true}, nil)
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.repliedCards) != 0 || len(p.sent) != 0 {
		t.Fatalf("review appended messages instead of editing clicked card: cards=%d text=%v", len(p.repliedCards), p.sent)
	}
	if len(p.edits) == 0 {
		t.Fatal("clicked update card was never updated")
	}
	card := p.edits[len(p.edits)-1]
	text := card.RenderText()
	if utf8.RuneCountInString(text) > 650 || strings.Contains(text, "npm install") {
		t.Fatalf("confirmation contains release implementation details: %s", text)
	}
	if !strings.Contains(text, "v1.0.0") || !strings.Contains(text, "v1.1.0") || !strings.Contains(text, "重启") {
		t.Fatalf("confirmation omitted version or restart impact: %s", text)
	}
	if labels := strings.Join(cardButtonLabels(card), "|"); !strings.Contains(labels, "更新并重启") || strings.Contains(labels, "查看并更新") {
		t.Fatalf("ambiguous installation action: %s", labels)
	}
}

type progressUpdateService struct {
	stubCoreUpdateService
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (s *progressUpdateService) ApplyWithProgress(ctx context.Context, p PreparedUpdate, progress func(appfeatures.UpdateEvent)) (UpdateResult, error) {
	s.calls.Add(1)
	progress(appfeatures.UpdateEvent{Stage: appfeatures.StageInstalling, TargetVersion: p.Release.TagName})
	close(s.started)
	select {
	case <-ctx.Done():
		return UpdateResult{}, ctx.Err()
	case <-s.release:
	}
	progress(appfeatures.UpdateEvent{Stage: appfeatures.StageVerifyingInstalled, TargetVersion: p.Release.TagName})
	return s.result, s.applyErr
}
func updateTestPlatform() *updateExperiencePlatform {
	return &updateExperiencePlatform{stubCardPlatform: stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}}
}
func reviewedUpdate(t *testing.T, e *Engine, p *updateExperiencePlatform) (*Message, string) {
	t.Helper()
	m := &Message{SessionKey: "test:admin", UserID: "admin", Platform: "test", ReplyCtx: "card-1", IsCardAction: true}
	e.cmdUpgrade(p, m, nil)
	p.mu.Lock()
	defer p.mu.Unlock()
	card := p.edits[len(p.edits)-1]
	for _, el := range card.Elements {
		if row, ok := el.(CardActions); ok {
			for _, b := range row.Buttons {
				if strings.HasPrefix(b.Value, "cmd:/upgrade confirm ") {
					return m, strings.TrimPrefix(b.Value, "cmd:/upgrade confirm ")
				}
			}
		}
	}
	t.Fatal("review has no confirmation action")
	return nil, ""
}
func waitUpdate(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal("update state did not arrive")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestUpdateExperience_SameCardProgressRejectsDoubleClickAndKeepsRestartPending(t *testing.T) {
	withCurrentVersion(t, "v1.0.0")
	drainTestRestartRequests()
	t.Cleanup(drainTestRestartRequests)
	p := updateTestPlatform()
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
	e.SetAdminFrom("admin")
	e.dataDir = t.TempDir()
	service := &progressUpdateService{stubCoreUpdateService: stubCoreUpdateService{plan: PreparedUpdate{Release: ReleaseInfo{TagName: "v1.1.0", Body: "notes"}, Available: true, token: "exact"}, result: UpdateResult{Updated: true, Release: ReleaseInfo{TagName: "v1.1.0"}}}, started: make(chan struct{}), release: make(chan struct{})}
	e.SetUpdateService(service)
	msg, token := reviewedUpdate(t, e, p)
	finished := make(chan struct{})
	go func() { e.cmdUpgradeConfirm(p, msg, token); close(finished) }()
	<-service.started
	e.cmdUpgradeConfirm(p, msg, token)
	e.cancelUpdate(p, msg, token) // Installation is not cancellable halfway through.
	if service.calls.Load() != 1 {
		t.Fatal("duplicate install")
	}
	close(service.release)
	<-finished
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.repliedCards) != 0 || len(p.sent) != 0 {
		t.Fatalf("progress appended messages: %v", p.sent)
	}
	last := p.edits[len(p.edits)-1]
	if last.Header.Title != e.i18n.T(MsgUpdateRestartingState) || len(cardButtonLabels(last)) != 0 {
		t.Fatalf("restart is not a non-actionable pending state: %s", last.RenderText())
	}
	if strings.Contains(last.RenderText(), e.i18n.T(MsgUpdateCompleteState)) {
		t.Fatal("success before restart")
	}
	r, err := readUpdateRecord(e.dataDir)
	if err != nil || r.TargetVersion != "v1.1.0" || r.Stage != "restarting" || r.ReceiptDelivered {
		t.Fatalf("invalid restart checkpoint: %+v %v", r, err)
	}
	if request := <-RestartCh; request.UpdateID != token {
		t.Fatal("restart lost operation identity")
	}
}
func TestUpdateExperience_WaitsForOtherProjectAndCancellationReopensAdmission(t *testing.T) {
	withCurrentVersion(t, "v1.0.0")
	c := NewUpdateCoordinator()
	other := NewEngine("other", &stubAgent{}, nil, "", LangEnglish)
	other.SetUpdateCoordinator(c)
	taskDone, err := other.beginUpdateWork()
	if err != nil {
		t.Fatal(err)
	}
	defer taskDone()
	p := updateTestPlatform()
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
	e.SetUpdateCoordinator(c)
	e.SetAdminFrom("admin")
	service := &stubCoreUpdateService{plan: PreparedUpdate{Release: ReleaseInfo{TagName: "v1.1.0"}, Available: true, token: "exact"}}
	e.SetUpdateService(service)
	msg, token := reviewedUpdate(t, e, p)
	finished := make(chan struct{})
	go func() { e.cmdUpgradeConfirm(p, msg, token); close(finished) }()
	waitUpdate(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.draining })
	if done, ok := c.beginWork(); ok {
		done()
		t.Fatal("new work admitted during drain")
	}
	e.cancelUpdate(p, &Message{SessionKey: msg.SessionKey, UserID: "different-admin", ReplyCtx: "different-card", IsCardAction: true}, token)
	c.mu.Lock()
	stillWaiting := c.draining
	c.mu.Unlock()
	if !stillWaiting {
		t.Fatal("another admin cancelled the operation")
	}
	e.cancelUpdate(p, msg, token)
	<-finished
	if service.applyCalls != 0 {
		t.Fatal("cancellation installed anyway")
	}
	if done, ok := c.beginWork(); !ok {
		t.Fatal("cancel left admission closed")
	} else {
		done()
	}
}

type persistentUpdatePlatform struct {
	updateExperiencePlatform
	failEdits    bool
	failSends    bool
	replacements int
	references   []string
}

func (p *persistentUpdatePlatform) ReconstructReplyCtx(key string) (any, error) { return key, nil }
func (p *persistentUpdatePlatform) CardReference(rctx any) (json.RawMessage, error) {
	return json.Marshal(rctx)
}
func (p *persistentUpdatePlatform) ReplyCardWithReference(ctx context.Context, rctx any, c *Card) (json.RawMessage, error) {
	if p.failSends {
		return nil, errors.New("send unavailable")
	}
	p.replacements++
	if err := p.ReplyCard(ctx, rctx, c); err != nil {
		return nil, err
	}
	return json.Marshal("replacement-card")
}
func (p *persistentUpdatePlatform) UpdateCardByReference(ctx context.Context, ref json.RawMessage, c *Card) error {
	if p.failEdits {
		return errors.New("offline")
	}
	p.references = append(p.references, string(ref))
	return p.UpdateCard(ctx, nil, c)
}
func TestUpdateExperience_RecoveryVerifiesRunningVersionAndEditsOriginalCardOnce(t *testing.T) {
	for _, version := range []string{"v1.0.0", "v1.1.0"} {
		t.Run(version, func(t *testing.T) {
			withCurrentVersion(t, version)
			p := &persistentUpdatePlatform{updateExperiencePlatform: *updateTestPlatform()}
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
			e.dataDir = t.TempDir()
			original := updateRecord{ID: "op", Project: "test", Platform: "test", SessionKey: "test:admin", UserID: "admin", FromVersion: "v1.0.0", TargetVersion: "v1.1.0", Stage: "restarting", Started: time.Now().Add(-time.Minute), Card: json.RawMessage(`"original-card"`)}
			if err := writeUpdateRecord(e.dataDir, original); err != nil {
				t.Fatal(err)
			}
			e.RecoverUpdateResult()
			if len(p.edits) != 0 {
				t.Fatal("receipt delivered before platform ready")
			}
			e.markPlatformReady(p)
			e.RecoverUpdateResult()
			e.RecoverUpdateResult()
			r, err := readUpdateRecord(e.dataDir)
			if err != nil || !r.ReceiptDelivered {
				t.Fatalf("receipt not retained: %v %+v", err, r)
			}
			want := "completed"
			if version != "v1.1.0" {
				want = "failed"
			}
			if r.Stage != want || len(p.references) != 1 || p.references[0] != `"original-card"` {
				t.Fatalf("wrong recovered state: %+v refs=%v", r, p.references)
			}
		})
	}
}
func TestUpdateSummary_PreservesExplicitUpgradeWarnings(t *testing.T) {
	notes := "## 中文\n### 更新亮点\n- 进度更清晰\n### 升级注意\n必须先备份数据库。\n### 安装\nnpm install ..."
	if got := updateSummary(notes, LangChinese); got != "• 进度更清晰" {
		t.Fatal(got)
	}
	if got := updateImportantNotes(notes, LangChinese); got != "必须先备份数据库。" {
		t.Fatal(got)
	}
}

// A transient network error is not proof that a card was deleted. Progress
// must keep retrying the exact card; only the final result may replace it.
func TestUpdateExperience_DeletedCardGetsOneFinalReplacement(t *testing.T) {
	p := &persistentUpdatePlatform{updateExperiencePlatform: *updateTestPlatform(), failEdits: true}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	view := &updateView{engine: e, platform: p, replyCtx: "chat", reference: json.RawMessage(`"deleted"`)}
	for i := 0; i < 3; i++ {
		_ = view.render(e.updateStateCard("installing", "v1.1", "", time.Now(), "op", false), true)
	}
	if p.replacements != 0 {
		t.Fatal("progress created replacement messages")
	}
	card := e.updateStateCard("completed", "v1.1", "", time.Now(), "op", false)
	if err := view.render(card, false); err != nil {
		t.Fatal(err)
	}
	if err := view.render(card, false); err == nil {
		t.Fatal("deleted replacement unexpectedly edited")
	}
	if p.replacements != 1 {
		t.Fatalf("replacement count=%d", p.replacements)
	}
}

func TestUpdateExperience_UndeliveredResultsSurviveAnotherAttemptAndRecoverOnce(t *testing.T) {
	withCurrentVersion(t, "v1.1.0")
	p := &persistentUpdatePlatform{updateExperiencePlatform: *updateTestPlatform(), failEdits: true, failSends: true}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.dataDir = t.TempDir()
	e.markPlatformReady(p)
	for _, id := range []string{"older", "newer"} {
		if err := writeUpdateRecord(e.dataDir, updateRecord{ID: id, Project: "test", Platform: "test", SessionKey: "test:admin", UserID: "admin", TargetVersion: "v1.1.0", Stage: "restarting", Started: time.Now(), Card: json.RawMessage(`"` + id + `"`)}); err != nil {
			t.Fatal(err)
		}
	}
	// A failed edit AND failed replacement must not claim delivery.
	records, _ := readUpdateRecords(e.dataDir)
	e.recoverUpdateRecord(records[0])
	records, _ = readUpdateRecords(e.dataDir)
	if len(records) != 2 || records[0].ReceiptDelivered || records[1].ReceiptDelivered {
		t.Fatal("undelivered result lost")
	}
	p.failEdits = false
	p.failSends = false
	e.RecoverUpdateResult()
	e.RecoverUpdateResult()
	if len(p.references) != 2 {
		t.Fatalf("recoveries=%v", p.references)
	}
	for _, r := range records {
		info, err := os.Stat(filepath.Join(e.dataDir, "run", "update-results", r.ID+".json"))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("result permissions: %v %v", info, err)
		}
	}
}

func TestUpdateExperience_PartialConnectionsUpdateWhenOtherProjectRecovers(t *testing.T) {
	withCurrentVersion(t, "v1.1.0")
	c := NewUpdateCoordinator()
	p := &persistentUpdatePlatform{updateExperiencePlatform: *updateTestPlatform()}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.dataDir = t.TempDir()
	e.SetUpdateCoordinator(c)
	e.markPlatformReady(p)
	bs := NewBridgeServerInsecure(0, "", "/bridge/ws", nil)
	bp := bs.NewPlatform("test")
	bs.RegisterEngine("test", e, bp)
	e.AddPlatform(bp)
	otherPlatform := &stubPlatformEngine{n: "other-platform"}
	other := NewEngine("other", &stubAgent{}, []Platform{otherPlatform}, "", LangEnglish)
	other.SetUpdateCoordinator(c)
	if err := writeUpdateRecord(e.dataDir, updateRecord{ID: "op", Project: "test", Platform: "test", SessionKey: "test:admin", TargetVersion: "v1.1.0", Stage: "restarting", Started: time.Now().Add(-time.Minute), Card: json.RawMessage(`"card"`)}); err != nil {
		t.Fatal(err)
	}
	e.RecoverUpdateResult()
	e.RecoverUpdateResult()
	r, _ := readUpdateRecord(e.dataDir)
	if !r.ConnectionsPending || len(p.references) != 1 {
		t.Fatalf("partial state=%+v", r)
	}
	finished := r.Finished
	other.markPlatformReady(otherPlatform)
	c.recoverReceipts()
	r, _ = readUpdateRecord(e.dataDir)
	if r.ConnectionsPending || len(p.references) != 2 || !r.Finished.Equal(finished) {
		t.Fatalf("ready state=%+v refs=%v", r, p.references)
	}
}

type blockingCheckService struct {
	started chan struct{}
	calls   atomic.Int32
}

func (s *blockingCheckService) Prepare(ctx context.Context) (PreparedUpdate, error) {
	s.calls.Add(1)
	close(s.started)
	<-ctx.Done()
	return PreparedUpdate{}, ctx.Err()
}
func (*blockingCheckService) Apply(context.Context, PreparedUpdate) (UpdateResult, error) {
	return UpdateResult{}, errors.New("unexpected installation")
}
func TestUpdateExperience_CheckCanCancelWithoutDuplicatePrepare(t *testing.T) {
	withCurrentVersion(t, "v1.0.0")
	p := updateTestPlatform()
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangChinese)
	s := &blockingCheckService{started: make(chan struct{})}
	e.SetUpdateService(s)
	m := &Message{SessionKey: "test:admin", UserID: "admin", ReplyCtx: "card", IsCardAction: true}
	done := make(chan struct{})
	go func() { e.cmdUpgrade(p, m, nil); close(done) }()
	<-s.started
	e.cmdUpgrade(p, m, nil)
	e.updateServiceMu.Lock()
	var token string
	for key := range e.updateChecks {
		token = key
	}
	e.updateServiceMu.Unlock()
	e.cancelUpdate(p, m, token)
	<-done
	p.mu.Lock()
	defer p.mu.Unlock()
	if s.calls.Load() != 1 || p.edits[len(p.edits)-1].Header.Title != e.i18n.T(MsgUpdateCancelledState) {
		t.Fatal("cancelled check was replaced by a duplicate review")
	}
}

func TestUpdateExperience_PersistenceFailureNeverInstalls(t *testing.T) {
	withCurrentVersion(t, "v1.0.0")
	p := updateTestPlatform()
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	dir := t.TempDir()
	e.dataDir = filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(e.dataDir, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &stubCoreUpdateService{plan: PreparedUpdate{Release: ReleaseInfo{TagName: "v1.1.0"}, Available: true}}
	e.SetUpdateService(s)
	m, token := reviewedUpdate(t, e, p)
	e.cmdUpgradeConfirm(p, m, token)
	if s.applyCalls != 0 {
		t.Fatal("install began without durable recovery state")
	}
	if done, ok := e.updates().beginWork(); !ok {
		t.Fatal("persistence failure kept admission closed")
	} else {
		done()
	}
}

func testCUJUpdateDrainAndRecovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell task; admission and recovery tests are platform-independent")
	}
	withCurrentVersion(t, "v1.0.0")
	drainTestRestartRequests()
	t.Cleanup(drainTestRestartRequests)
	c := NewUpdateCoordinator()
	p := &persistentUpdatePlatform{updateExperiencePlatform: *updateTestPlatform()}
	dir := t.TempDir()
	e := NewEngine("test", &stubAgent{}, []Platform{p}, filepath.Join(dir, "sessions.json"), LangEnglish)
	e.SetDataDir(dir)
	e.SetAdminFrom("admin")
	e.SetUpdateCoordinator(c)
	t.Cleanup(func() { _ = e.Stop() })
	other := newCUJEnv(t)
	other.engine.SetAdminFrom("admin")
	other.engine.SetUpdateCoordinator(c)
	release := filepath.Join(t.TempDir(), "release")
	t.Cleanup(func() { _ = os.WriteFile(release, nil, 0600) })
	// 1. Another project accepts actual shell work.
	other.userSends("admin", "/shell while [ ! -f '"+release+"' ]; do sleep 0.02; done; echo task-finished")
	waitUpdate(t, func() bool { return c.activeCount() > 0 })
	service := &stubCoreUpdateService{plan: PreparedUpdate{Release: ReleaseInfo{TagName: "v1.1.0", Body: "Short highlights"}, Available: true, token: "reviewed"}, result: UpdateResult{Updated: true, Release: ReleaseInfo{TagName: "v1.1.0"}}}
	e.SetUpdateService(service)
	send := func(engine *Engine, content string) {
		engine.ReceiveMessage(p, &Message{SessionKey: "test:admin", Platform: "test", MessageID: content, UserID: "admin", Content: content, ReplyCtx: "original-card", IsCardAction: true})
	}
	// 2. Review, 3. confirm; the card must show waiting and allow cancellation.
	send(e, "/upgrade")
	done := make(chan struct{})
	go func() { send(e, "/upgrade confirm"); close(done) }()
	waitUpdate(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.edits) > 0 && p.edits[len(p.edits)-1].Header.Title == e.i18n.T(MsgUpdateWaitingState)
	})
	// 4. New work is rejected explicitly, leaving accepted work intact.
	other.userSends("admin", "new request while updating")
	if !other.sentContains(other.engine.i18n.T(MsgUpdateAdmissionClosed)) {
		t.Fatalf("new work not explained: %v", other.plat.getSent())
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	<-done
	if !other.sentContains("task-finished") {
		t.Fatalf("accepted work lost: %v", other.plat.getSent())
	}
	request := <-RestartCh
	if request.UpdateID == "" || service.applyCalls != 1 {
		t.Fatal("missing unique update handoff")
	}
	// Restart boundary: only the new process, at the target version and with a
	// usable platform, may replace the pending state with completion.
	CurrentVersion = "v1.1.0"
	restored := NewEngine("test", &stubAgent{}, []Platform{p}, filepath.Join(dir, "sessions.json"), LangEnglish)
	restored.SetDataDir(dir)
	restored.SetAdminFrom("admin")
	restored.markPlatformReady(p)
	t.Cleanup(func() { _ = restored.Stop() })
	restored.RecoverUpdateResult()
	// 5. Clicking the old consumed confirmation shows the saved completion.
	send(restored, "/upgrade confirm "+request.UpdateID)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.edits[len(p.edits)-1].Header.Title != restored.i18n.T(MsgUpdateCompleteState) || p.replacements != 0 || len(p.repliedCards) != 0 {
		t.Fatal("same-card completion was not preserved")
	}
	for _, ref := range p.references {
		if ref != `"original-card"` {
			t.Fatalf("wrong card edited: %s", ref)
		}
	}
}

func TestUpdateExperience_RestartCannotInterruptInstallation(t *testing.T) {
	drainTestRestartRequests()
	t.Cleanup(drainTestRestartRequests)
	e, state := prepareDeferredRestartTurn(t, "admin", "test:admin")
	e.SetAdminFrom("admin")
	c := e.updates()
	op := &updateOperation{}
	if _, ok := c.reserve(op); !ok {
		t.Fatal("could not start update")
	}
	if e.queueServiceRestart(RestartRequest{SessionKey: "test:admin", Platform: "test"}) {
		t.Fatal("manual restart interrupted update")
	}
	state.mu.Lock()
	credential := state.restartTurnToken
	state.mu.Unlock()
	if _, err := e.RequestDeferredRestart(credential); !errors.Is(err, ErrDeferredRestartAlreadyPending) {
		t.Fatalf("deferred restart admitted: %v", err)
	}
	select {
	case <-RestartCh:
		t.Fatal("restart was queued during installation")
	default:
	}
	c.release(op)
	if !c.reserveRestart() {
		t.Fatal("restart not admitted after update released")
	}
	if _, ok := c.reserve(&updateOperation{}); ok {
		t.Fatal("update admitted after restart was accepted")
	}
}

func TestUpdateExperience_TextFallbackProvidesExecutableCancelAndRetryCommands(t *testing.T) {
	p := &stubPlatformEngine{n: "text"}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	view := newUpdateView(e, p, &Message{ReplyCtx: "chat"})
	if err := view.render(e.updateStateCard("waiting", "v1.1", "", time.Time{}, "bound-token", true), true); err != nil {
		t.Fatal(err)
	}
	if err := view.render(e.updateStateCard("expired", "", "", time.Time{}, "", false), false); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(p.getSent(), "\n")
	if !strings.Contains(text, "/upgrade cancel bound-token") || !strings.Contains(text, "Check again: /upgrade") {
		t.Fatalf("text controls are not executable: %s", text)
	}
}
