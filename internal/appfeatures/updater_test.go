package appfeatures

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
)

type memoryUpdateSource struct {
	release   featureupdater.Release
	payloads  map[string][]byte
	downloads atomic.Int32
}

func (source *memoryUpdateSource) LatestStable(context.Context) (featureupdater.Release, error) {
	return source.release, nil
}

func (source *memoryUpdateSource) LatestPrerelease(context.Context) (featureupdater.Release, error) {
	return source.release, nil
}

func (source *memoryUpdateSource) Download(_ context.Context, asset featureupdater.Asset, destination io.Writer) error {
	source.downloads.Add(1)
	payload, ok := source.payloads[asset.Name]
	if !ok {
		return fmt.Errorf("missing payload %s", asset.Name)
	}
	_, err := destination.Write(payload)
	return err
}

func TestUpdateServiceStandaloneAppliesExactPreparedPlan(t *testing.T) {
	target, source := updateFixture(t, "v1.0.0", "v1.1.0")
	var stages []featureupdater.Stage
	service, err := NewUpdateService(UpdateConfig{
		CurrentVersion: "v1.0.0",
		ExecutablePath: target,
		Source:         source,
		Progress: func(event featureupdater.Event) {
			stages = append(stages, event.Stage)
		},
	})
	if err != nil {
		t.Fatalf("NewUpdateService: %v", err)
	}

	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !plan.Available() || plan.Release().Tag != "v1.1.0" || plan.ArchiveAsset().Name != archiveName("v1.1.0") {
		t.Fatalf("plan = %#v / %#v", plan.Release(), plan.ArchiveAsset())
	}
	// A moving latest endpoint after approval must not change Apply.
	source.release.Tag = "v1.2.0"
	result, err := service.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Updated || result.Release.Tag != "v1.1.0" {
		t.Fatalf("result = %#v", result)
	}
	assertTargetVersion(t, target, "v1.1.0")
	for _, required := range []featureupdater.Stage{featureupdater.StageChecksumVerified, featureupdater.StageStagedVerified, featureupdater.StageInstalledVerified, featureupdater.StageComplete} {
		if !containsUpdateStage(stages, required) {
			t.Errorf("missing stage %s in %v", required, stages)
		}
	}
}

func TestUpdateServiceWindowsHostAdapterPreservesExactPlanAndRollbackBoundaries(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, ProductName+".exe")
	if err := os.WriteFile(target, versionScript("v1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	archiveName := releaseArchiveName("v1.1.0", "windows", "amd64")
	binaryName := releaseBinaryName("v1.1.0", "windows", "amd64")
	payload := zipBinary(t, binaryName, versionScript("v1.1.0"))
	source := &memoryUpdateSource{
		release: featureupdater.Release{
			Tag: "v1.1.0",
			Assets: []featureupdater.Asset{
				{Name: archiveName, DownloadURL: "https://example.invalid/" + archiveName, Size: int64(len(payload))},
				{Name: "checksums.txt", DownloadURL: "https://example.invalid/checksums.txt"},
			},
		},
		payloads: map[string][]byte{
			archiveName:     payload,
			"checksums.txt": []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), archiveName)),
		},
	}
	service, err := newUpdateService(UpdateConfig{CurrentVersion: "v1.0.0", ExecutablePath: target, Source: source}, "windows", "amd64")
	if err != nil {
		t.Fatalf("newUpdateService: %v", err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if plan.ArchiveAsset().Name != archiveName {
		t.Fatalf("archive = %q", plan.ArchiveAsset().Name)
	}
	result, err := service.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Updated || result.BackupRetainedAt != "" {
		t.Fatalf("result = %#v", result)
	}
	assertTargetVersion(t, target, "v1.1.0")
}

func TestUpdateServiceWindowsHostAdapterRemovesVerifiedStaleBackupBeforeNextUpdate(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, ProductName+".exe")
	backup := target + ".old"
	if err := os.WriteFile(target, versionScript("v1.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, versionScript("v1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	service, err := newUpdateService(UpdateConfig{
		CurrentVersion: "v1.1.0",
		ExecutablePath: target,
		Source:         windowsUpdateSource(t, "v1.2.0"),
	}, "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply with verified stale backup: %v", err)
	}
	if !result.Updated || result.BackupRetainedAt != "" {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Lstat(backup); !os.IsNotExist(err) {
		t.Fatalf("stale backup still exists: %v", err)
	}
	assertTargetVersion(t, target, "v1.2.0")
}

func TestUpdateServiceWindowsHostAdapterPreservesBackupWhenCurrentTargetIsUnverified(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, ProductName+".exe")
	backup := target + ".old"
	if err := os.WriteFile(target, versionScript("v0.9.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, versionScript("v1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := windowsUpdateSource(t, "v1.2.0")
	service, err := newUpdateService(UpdateConfig{
		CurrentVersion: "v1.1.0",
		ExecutablePath: target,
		Source:         source,
	}, "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	retainedBackup := service.Installation().ExecutablePath + ".old"
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Apply(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "verify current executable before removing stale update backup") {
		t.Fatalf("Apply error = %v", err)
	}
	if result.BackupRetainedAt != retainedBackup {
		t.Fatalf("backup retained at %q, want %q", result.BackupRetainedAt, retainedBackup)
	}
	if source.downloads.Load() != 1 {
		t.Fatalf("downloads = %d, want checksum-only Prepare", source.downloads.Load())
	}
	assertTargetVersion(t, target, "v0.9.0")
	assertTargetVersion(t, backup, "v1.0.0")
}

func windowsUpdateSource(t *testing.T, targetVersion string) *memoryUpdateSource {
	t.Helper()
	archiveName := releaseArchiveName(targetVersion, "windows", "amd64")
	binaryName := releaseBinaryName(targetVersion, "windows", "amd64")
	payload := zipBinary(t, binaryName, versionScript(targetVersion))
	return &memoryUpdateSource{
		release: featureupdater.Release{
			Tag: targetVersion,
			Assets: []featureupdater.Asset{
				{Name: archiveName, DownloadURL: "https://example.invalid/" + archiveName, Size: int64(len(payload))},
				{Name: "checksums.txt", DownloadURL: "https://example.invalid/checksums.txt"},
			},
		},
		payloads: map[string][]byte{
			archiveName:     payload,
			"checksums.txt": []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), archiveName)),
		},
	}
}

func TestUpdateServicesSerializeTheSameExecutableAcrossHostEntryPoints(t *testing.T) {
	target, baseSource := updateFixture(t, "v1.0.0", "v1.1.0")
	started := make(chan struct{})
	proceed := make(chan struct{})
	source := &blockingArchiveUpdateSource{memoryUpdateSource: baseSource, started: started, proceed: proceed}
	first, err := NewUpdateService(UpdateConfig{CurrentVersion: "v1.0.0", ExecutablePath: target, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewUpdateService(UpdateConfig{CurrentVersion: "v1.0.0", ExecutablePath: target, Source: baseSource})
	if err != nil {
		t.Fatal(err)
	}
	firstPlan, err := first.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secondPlan, err := second.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := first.Apply(context.Background(), firstPlan)
		firstDone <- err
	}()
	<-started
	if _, err := second.Apply(context.Background(), secondPlan); !errors.Is(err, featureupdater.ErrUpdateInProgress) {
		t.Fatalf("concurrent second Apply error = %v", err)
	}
	close(proceed)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Apply: %v", err)
	}
}

type blockingArchiveUpdateSource struct {
	*memoryUpdateSource
	started chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (source *blockingArchiveUpdateSource) Download(ctx context.Context, asset featureupdater.Asset, destination io.Writer) error {
	if asset.Name != "checksums.txt" {
		source.once.Do(func() { close(source.started) })
		select {
		case <-source.proceed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return source.memoryUpdateSource.Download(ctx, asset, destination)
}

func TestUpdateServiceDetectsAndAppliesExactNPMVersion(t *testing.T) {
	prefix := t.TempDir()
	packageDir := filepath.Join(prefix, "lib", "node_modules", ProductName)
	binDir := filepath.Join(packageDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	packageJSON := filepath.Join(packageDir, "package.json")
	writePackageMetadata(t, packageJSON, "1.0.0")
	executable := filepath.Join(binDir, ProductName)
	if err := os.WriteFile(executable, versionScript("v1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := &memoryUpdateSource{release: featureupdater.Release{Tag: "v1.2.3"}}
	var command string
	var arguments []string
	service, err := NewUpdateService(UpdateConfig{
		CurrentVersion: "v1.0.0",
		ExecutablePath: executable,
		Source:         source,
		Runner: func(_ context.Context, name string, args ...string) error {
			command = name
			arguments = append([]string(nil), args...)
			writePackageMetadata(t, packageJSON, "1.2.3")
			return os.WriteFile(executable, versionScript("v1.2.3"), 0o755)
		},
	})
	if err != nil {
		t.Fatalf("NewUpdateService: %v", err)
	}
	if service.Installation().Kind != InstallNPM {
		t.Fatalf("installation = %#v", service.Installation())
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	result, err := service.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Updated || command != npmExecutableName() {
		t.Fatalf("result=%#v command=%q", result, command)
	}
	resolvedPrefix, err := filepath.EvalSymlinks(prefix)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"install", "--global", "--prefix", resolvedPrefix, ProductName + "@1.2.3"}
	if !reflect.DeepEqual(arguments, want) {
		t.Fatalf("npm arguments = %#v, want %#v", arguments, want)
	}
}

func TestUpdateServiceBetaChannelAppliesExactPrereleaseNPMVersion(t *testing.T) {
	prefix := t.TempDir()
	packageDir := filepath.Join(prefix, "lib", "node_modules", ProductName)
	binDir := filepath.Join(packageDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	packageJSON := filepath.Join(packageDir, "package.json")
	writePackageMetadata(t, packageJSON, "0.3.0-beta.1")
	executable := filepath.Join(binDir, ProductName)
	if err := os.WriteFile(executable, versionScript("v0.3.0-beta.1"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := &memoryUpdateSource{release: featureupdater.Release{Tag: "v0.3.0-beta.2", Prerelease: true}}
	var arguments []string
	service, err := NewUpdateService(UpdateConfig{
		CurrentVersion: "v0.3.0-beta.1",
		Channel:        updatechannel.Beta,
		ExecutablePath: executable,
		Source:         source,
		Runner: func(_ context.Context, _ string, args ...string) error {
			arguments = append([]string(nil), args...)
			writePackageMetadata(t, packageJSON, "0.3.0-beta.2")
			return os.WriteFile(executable, versionScript("v0.3.0-beta.2"), 0o755)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare beta: %v", err)
	}
	if !plan.Available() || plan.Release().Tag != "v0.3.0-beta.2" || service.Channel() != updatechannel.Beta {
		t.Fatalf("beta plan = %#v channel=%s", plan.Release(), service.Channel())
	}
	result, err := service.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply beta: %v", err)
	}
	if !result.Updated || !slices.Contains(arguments, ProductName+"@0.3.0-beta.2") {
		t.Fatalf("result=%#v arguments=%v", result, arguments)
	}
}

func TestUpdateServiceBetaChannelAppliesChecksumVerifiedStandalonePlan(t *testing.T) {
	target, source := updateFixture(t, "v0.3.0-beta.1", "v0.3.0-beta.2")
	source.release.Prerelease = true
	service, err := newUpdateService(UpdateConfig{
		CurrentVersion: "v0.3.0-beta.1",
		Channel:        updatechannel.Beta,
		ExecutablePath: target,
		Source:         source,
	}, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare beta standalone: %v", err)
	}
	if !plan.Available() || plan.ArchiveAsset().Name == "" {
		t.Fatalf("beta standalone plan = %#v / %#v", plan.Release(), plan.ArchiveAsset())
	}
	result, err := service.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply beta standalone: %v", err)
	}
	if !result.Updated {
		t.Fatalf("beta standalone result = %#v", result)
	}
	assertTargetVersion(t, target, "v0.3.0-beta.2")
}

func TestUpdateServiceBetaStandaloneRollsBackFailedInstalledProbe(t *testing.T) {
	target, source := updateFixture(t, "v0.3.0-beta.1", "v0.3.0-beta.2")
	source.release.Prerelease = true
	var probes atomic.Int32
	verifier := featureupdater.VersionVerifierFunc(func(_ context.Context, _, _ string) error {
		if probes.Add(1) == 2 {
			return errors.New("installed probe failed")
		}
		return nil
	})
	service, err := newUpdateService(UpdateConfig{
		CurrentVersion: "v0.3.0-beta.1", Channel: updatechannel.Beta,
		ExecutablePath: target, Source: source, Verifier: verifier,
	}, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Apply(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "installed probe failed") {
		t.Fatalf("Apply error = %v", err)
	}
	assertTargetVersion(t, target, "v0.3.0-beta.1")
	if _, err := os.Lstat(target + ".old"); !os.IsNotExist(err) {
		t.Fatalf("rollback backup remained: %v", err)
	}
}

func TestValidateUpdateReleaseForChannelRejectsMislabeledPrereleases(t *testing.T) {
	stable := featureupdater.Release{Tag: "v1.2.3"}
	beta := featureupdater.Release{Tag: "v1.3.0-beta.2", Prerelease: true}
	if err := validateUpdateReleaseForChannel(stable, updatechannel.Stable); err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateReleaseForChannel(beta, updatechannel.Beta); err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateReleaseForChannel(beta, updatechannel.Stable); err == nil {
		t.Fatal("stable channel accepted a prerelease")
	}
	if err := validateUpdateReleaseForChannel(stable, updatechannel.Beta); err == nil {
		t.Fatal("beta channel accepted a stable release")
	}
}

func TestCCConnectReleaseNamesMatchPublishedLayout(t *testing.T) {
	for _, test := range []struct {
		tag, goos, goarch, archive, binary string
	}{
		{"v0.2.1", "darwin", "arm64", "cc-connect-next-v0.2.1-darwin-arm64.tar.gz", "cc-connect-next-v0.2.1-darwin-arm64"},
		{"v0.2.1", "linux", "amd64", "cc-connect-next-v0.2.1-linux-amd64.tar.gz", "cc-connect-next-v0.2.1-linux-amd64"},
		{"v0.2.1", "windows", "amd64", "cc-connect-next-v0.2.1-windows-amd64.zip", "cc-connect-next-v0.2.1-windows-amd64.exe"},
	} {
		if got := releaseArchiveName(test.tag, test.goos, test.goarch); got != test.archive {
			t.Errorf("archive = %q, want %q", got, test.archive)
		}
		if got := releaseBinaryName(test.tag, test.goos, test.goarch); got != test.binary {
			t.Errorf("binary = %q, want %q", got, test.binary)
		}
	}
}

func updateFixture(t *testing.T, current, targetVersion string) (string, *memoryUpdateSource) {
	t.Helper()
	directory := t.TempDir()
	target := filepath.Join(directory, ProductName)
	if err := os.WriteFile(target, versionScript(current), 0o755); err != nil {
		t.Fatalf("write current binary: %v", err)
	}
	archive := archiveName(targetVersion)
	payload := releaseArchive(t, releaseBinaryName(targetVersion, runtime.GOOS, runtime.GOARCH), versionScript(targetVersion))
	return target, &memoryUpdateSource{
		release: featureupdater.Release{
			Tag:   targetVersion,
			URL:   "https://github.com/timmyagentic/cc-connect-next/releases/tag/" + targetVersion,
			Notes: "exact notes",
			Assets: []featureupdater.Asset{
				{Name: archive, DownloadURL: "https://example.invalid/" + archive, Size: int64(len(payload))},
				{Name: "checksums.txt", DownloadURL: "https://example.invalid/checksums.txt"},
			},
		},
		payloads: map[string][]byte{
			archive:         payload,
			"checksums.txt": []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), archive)),
		},
	}
}

func archiveName(tag string) string {
	return releaseArchiveName(tag, runtime.GOOS, runtime.GOARCH)
}

func releaseArchive(t *testing.T, binaryName string, body []byte) []byte {
	t.Helper()
	if runtime.GOOS == "windows" {
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		entry, err := writer.Create(binaryName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(body); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{Name: binaryName, Mode: 0o755, Size: int64(len(body))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func zipBinary(t *testing.T, binaryName string, body []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create(binaryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func versionScript(tag string) []byte {
	return []byte("#!/bin/sh\nprintf '%s\\n' '" + ProductName + " " + tag + "'\n")
}

func assertTargetVersion(t *testing.T, target, version string) {
	t.Helper()
	output, err := exec.Command(target, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("run target: %v", err)
	}
	if strings.TrimSpace(string(output)) != ProductName+" "+version {
		t.Fatalf("target output = %q, want %s", output, version)
	}
}

func containsUpdateStage(stages []featureupdater.Stage, want featureupdater.Stage) bool {
	for _, stage := range stages {
		if stage == want {
			return true
		}
	}
	return false
}

func writePackageMetadata(t *testing.T, path, version string) {
	t.Helper()
	data, err := json.Marshal(map[string]string{"name": ProductName, "version": version})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
