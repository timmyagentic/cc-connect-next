package feishu

import (
	"errors"
	"sync"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

type lifecycleRecorder struct {
	mu          sync.Mutex
	ready       []core.Platform
	unavailable []error
}

func (r *lifecycleRecorder) OnPlatformReady(p core.Platform) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = append(r.ready, p)
}

func (r *lifecycleRecorder) OnPlatformUnavailable(_ core.Platform, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unavailable = append(r.unavailable, err)
}

func resetSharedWSGroups(t *testing.T) {
	t.Helper()
	sharedWSMu.Lock()
	defer sharedWSMu.Unlock()
	sharedWSGroups = map[string]*sharedWSGroup{}
}

func TestPlatformImplementsAsyncRecoverableLifecycle(t *testing.T) {
	var _ core.AsyncRecoverablePlatform = (*Platform)(nil)
}

func TestSharedConnectionLifecycleReachesEveryPlatform(t *testing.T) {
	resetSharedWSGroups(t)
	t.Cleanup(func() { resetSharedWSGroups(t) })

	primaryHandler := &lifecycleRecorder{}
	secondaryHandler := &lifecycleRecorder{}
	primary := &Platform{platformName: "feishu", appID: "cli_shared", domain: "feishu.cn"}
	primary.self = primary
	primary.SetLifecycleHandler(primaryHandler)
	group, isPrimary := registerSharedWS(primary)
	primary.sharedGroup, primary.isWSPrimary = group, isPrimary

	secondary := &Platform{platformName: "feishu", appID: "cli_shared", domain: "feishu.cn"}
	secondary.self = secondary
	secondary.SetLifecycleHandler(secondaryHandler)
	group, isPrimary = registerSharedWS(secondary)
	secondary.sharedGroup, secondary.isWSPrimary = group, isPrimary

	primary.markConnectionReady()
	if len(primaryHandler.ready) != 1 || len(secondaryHandler.ready) != 1 {
		t.Fatalf("ready callbacks = primary %d secondary %d", len(primaryHandler.ready), len(secondaryHandler.ready))
	}

	wantErr := errors.New("websocket disconnected")
	primary.markConnectionUnavailable(wantErr)
	if len(primaryHandler.unavailable) != 1 || len(secondaryHandler.unavailable) != 1 {
		t.Fatalf("unavailable callbacks = primary %d secondary %d", len(primaryHandler.unavailable), len(secondaryHandler.unavailable))
	}
	if !errors.Is(primary.ConnectionError(), wantErr) || !errors.Is(secondary.ConnectionError(), wantErr) {
		t.Fatalf("connection errors = %v / %v", primary.ConnectionError(), secondary.ConnectionError())
	}
}

func TestLateSharedPlatformInheritsReadyState(t *testing.T) {
	resetSharedWSGroups(t)
	t.Cleanup(func() { resetSharedWSGroups(t) })

	primary := &Platform{platformName: "feishu", appID: "cli_late", domain: "feishu.cn"}
	primary.self = primary
	group, isPrimary := registerSharedWS(primary)
	primary.sharedGroup, primary.isWSPrimary = group, isPrimary
	primary.markConnectionReady()

	recorder := &lifecycleRecorder{}
	secondary := &Platform{platformName: "feishu", appID: "cli_late", domain: "feishu.cn"}
	secondary.self = secondary
	secondary.SetLifecycleHandler(recorder)
	group, isPrimary = registerSharedWS(secondary)
	secondary.sharedGroup, secondary.isWSPrimary = group, isPrimary
	secondary.syncSharedConnectionState()

	if len(recorder.ready) != 1 {
		t.Fatalf("late secondary ready callbacks = %d, want 1", len(recorder.ready))
	}
}
