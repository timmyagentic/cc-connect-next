package appfeatures

import (
	"context"
	"os"
	"runtime"
	"testing"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
)

func TestBetaUpdateDoesNotOverwriteBackupCreatedDuringInstallation(t *testing.T) {
	target, source := updateFixture(t, "v0.3.0-beta.1", "v0.3.0-beta.2")
	source.release.Prerelease = true
	backup := target + ".old"
	service, err := NewUpdateService(UpdateConfig{
		CurrentVersion: "v0.3.0-beta.1", Channel: updatechannel.Beta,
		ExecutablePath: target, Source: source,
		Progress: func(event featureupdater.Event) {
			if event.Stage == featureupdater.StageInstalling {
				if err := os.WriteFile(backup, []byte("independent recovery copy"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.Apply(context.Background(), plan); err == nil || result.Updated {
		t.Fatalf("update overwrote a concurrently created backup: updated=%v err=%v", result.Updated, err)
	}
	data, err := os.ReadFile(backup)
	if err != nil || string(data) != "independent recovery copy" {
		t.Fatalf("recovery copy changed: %q, %v", data, err)
	}
	assertTargetVersion(t, target, "v0.3.0-beta.1")
}

func TestBetaUpdatePreservesExistingBackupBeforeDownloadingArchive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows separately validates and removes its stale running-image backup")
	}
	target, source := updateFixture(t, "v0.3.0-beta.1", "v0.3.0-beta.2")
	source.release.Prerelease = true
	service, err := NewUpdateService(UpdateConfig{
		CurrentVersion: "v0.3.0-beta.1", Channel: updatechannel.Beta,
		ExecutablePath: target, Source: source,
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".old", []byte("recovery copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	downloads := source.downloads.Load()
	if result, err := service.Apply(context.Background(), plan); err == nil || result.Updated {
		t.Fatalf("update discarded existing recovery copy: updated=%v err=%v", result.Updated, err)
	}
	if source.downloads.Load() != downloads {
		t.Fatal("unsafe update downloaded an archive")
	}
	data, err := os.ReadFile(target + ".old")
	if err != nil || string(data) != "recovery copy" {
		t.Fatalf("recovery copy changed: %q, %v", data, err)
	}
}

func TestBetaUpdatePreservesExecutablePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permission contract")
	}
	target, source := updateFixture(t, "v0.3.0-beta.1", "v0.3.0-beta.2")
	source.release.Prerelease = true
	if err := os.Chmod(target, 0o700); err != nil {
		t.Fatal(err)
	}
	service, err := NewUpdateService(UpdateConfig{
		CurrentVersion: "v0.3.0-beta.1", Channel: updatechannel.Beta,
		ExecutablePath: target, Source: source,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateLatest(context.Background()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("update widened executable permissions: %o", info.Mode().Perm())
	}
}
