package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

type stubCoreUpdateService struct {
	plan         PreparedUpdate
	result       UpdateResult
	prepareErr   error
	applyErr     error
	prepareCalls int
	applyCalls   int
	appliedPlan  PreparedUpdate
}

func (service *stubCoreUpdateService) Prepare(context.Context) (PreparedUpdate, error) {
	service.prepareCalls++
	return service.plan, service.prepareErr
}

func (service *stubCoreUpdateService) Apply(_ context.Context, plan PreparedUpdate) (UpdateResult, error) {
	service.applyCalls++
	service.appliedPlan = plan
	return service.result, service.applyErr
}

func TestEngineUpdatePlanExpiresBeforeApply(t *testing.T) {
	engine := NewEngine("test", &stubAgent{}, nil, "", LangEnglish)
	plan := PreparedUpdate{Release: ReleaseInfo{TagName: "v1.2.3"}, Available: true, token: "exact"}
	engine.rememberUpdatePlan("test:user", plan)
	engine.updateServiceMu.Lock()
	pending := engine.updatePlans["test:user"]
	pending.At = time.Now().Add(-pendingUpdatePlanTTL - time.Second)
	engine.updatePlans["test:user"] = pending
	engine.updateServiceMu.Unlock()

	if _, ok := engine.pendingUpdatePlan("test:user"); ok {
		t.Fatal("expired update plan remained available")
	}
}

func TestFoundationUpdateServiceRejectsForeignToken(t *testing.T) {
	service := &foundationUpdateService{}
	_, err := service.Apply(context.Background(), PreparedUpdate{token: "not a foundation plan"})
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("foreign plan error = %v", err)
	}
}

func TestCmdUpgradeConfirmAppliesExactPreparedPlanWithoutSecondLookup(t *testing.T) {
	drainTestRestartRequests()
	t.Cleanup(drainTestRestartRequests)
	platform := &updateIntentStubPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	engine.SetAdminFrom("user1")
	previous := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = previous })
	service := &stubCoreUpdateService{
		plan: PreparedUpdate{
			Release:      ReleaseInfo{TagName: "v1.1.0", Body: "reviewed notes"},
			ArchiveAsset: "reviewed-asset.tar.gz",
			Available:    true,
			token:        "reviewed-plan",
		},
		result: UpdateResult{Release: ReleaseInfo{TagName: "v1.1.0"}, Updated: true},
	}
	engine.SetUpdateService(service)
	message := &Message{SessionKey: "test:user1", Platform: "test", UserID: "user1", ReplyCtx: "rc"}

	engine.cmdUpgrade(platform, message, nil)
	service.plan = PreparedUpdate{Release: ReleaseInfo{TagName: "v9.9.9"}, Available: true, token: "moving-latest"}
	engine.cmdUpgrade(platform, message, []string{"confirm"})

	if service.prepareCalls != 1 || service.applyCalls != 1 {
		t.Fatalf("prepare/apply calls = %d/%d", service.prepareCalls, service.applyCalls)
	}
	if service.appliedPlan.token != "reviewed-plan" || service.appliedPlan.Release.TagName != "v1.1.0" {
		t.Fatalf("applied plan drifted: %#v", service.appliedPlan)
	}
}

func TestCmdUpgradeConfirmWithoutReviewedPlanNeverApplies(t *testing.T) {
	drainTestRestartRequests()
	t.Cleanup(drainTestRestartRequests)
	platform := &updateIntentStubPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	previous := CurrentVersion
	CurrentVersion = "v1.0.0"
	t.Cleanup(func() { CurrentVersion = previous })
	service := &stubCoreUpdateService{}
	engine.SetUpdateService(service)

	engine.cmdUpgradeConfirm(platform, &Message{SessionKey: "test:user1", ReplyCtx: "rc"})
	if service.prepareCalls != 0 || service.applyCalls != 0 {
		t.Fatalf("confirm without review called service: prepare/apply=%d/%d", service.prepareCalls, service.applyCalls)
	}
	if sent := strings.Join(platform.getSent(), "\n"); !strings.Contains(sent, "reviewed update plan") {
		t.Fatalf("missing review guidance: %s", sent)
	}
}
