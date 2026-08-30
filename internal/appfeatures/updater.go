package appfeatures

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
	featuregithub "github.com/timmyagentic/awesome-agent-app-features/updater/github"
)

const releaseRepository = "timmyagentic/cc-connect-next"

type UpdateRelease = featureupdater.Release
type UpdateAsset = featureupdater.Asset
type UpdateEvent = featureupdater.Event
type UpdateStage = featureupdater.Stage
type UpdateResult = featureupdater.Result

const (
	StageDownloadingChecksums = featureupdater.StageDownloadingChecksums
	StageDownloadingArchive   = featureupdater.StageDownloadingArchive
	StageChecksumVerified     = featureupdater.StageChecksumVerified
	StageStagedVerified       = featureupdater.StageStagedVerified
	StageInstalling           = featureupdater.StageInstalling
	StageInstalledVerified    = featureupdater.StageInstalledVerified
)

type InstallKind string

const (
	InstallStandalone InstallKind = "standalone"
	InstallNPM        InstallKind = "npm"
)

type Installation struct {
	Kind           InstallKind
	ExecutablePath string
	PackageDir     string
	NPMPrefix      string
}

type CommandRunner func(context.Context, string, ...string) error

type UpdateConfig struct {
	CurrentVersion string
	ExecutablePath string
	Source         featureupdater.Source
	Verifier       featureupdater.VersionVerifier
	Runner         CommandRunner
	Progress       func(featureupdater.Event)
}

type UpdateService struct {
	config       UpdateConfig
	installation Installation
	source       featureupdater.Source
	verifier     featureupdater.VersionVerifier
	runner       CommandRunner
	standalone   *featureupdater.Updater
	platformOS   string
	platformArch string
	operation    sync.Mutex
	generation   atomic.Uint64
}

var updateTargetLocks sync.Map

type UpdatePlan struct {
	state *updatePlanState
}

type updatePlanState struct {
	owner      *UpdateService
	generation uint64
	status     atomic.Uint32
	available  bool
	release    featureupdater.Release
	asset      featureupdater.Asset
	binaryName string
	checksum   string
	foundation featureupdater.Plan
}

const (
	updatePlanReady uint32 = iota
	updatePlanApplying
	updatePlanConsumed
)

func (plan UpdatePlan) Available() bool {
	return plan.state != nil && plan.state.available
}

func (plan UpdatePlan) Release() UpdateRelease {
	if plan.state == nil {
		return UpdateRelease{}
	}
	return cloneUpdateRelease(plan.state.release)
}

func (plan UpdatePlan) ArchiveAsset() UpdateAsset {
	if plan.state == nil {
		return UpdateAsset{}
	}
	return plan.state.asset
}

func NewUpdateService(config UpdateConfig) (*UpdateService, error) {
	return newUpdateService(config, runtime.GOOS, runtime.GOARCH)
}

func newUpdateService(config UpdateConfig, platformOS, platformArch string) (*UpdateService, error) {
	config.CurrentVersion = strings.TrimSpace(config.CurrentVersion)
	if config.CurrentVersion == "" {
		return nil, fmt.Errorf("current version is required")
	}
	installation, err := DetectUpdateInstallation(config.ExecutablePath)
	if err != nil {
		return nil, err
	}
	source := config.Source
	if source == nil {
		source = featuregithub.Source{Repository: releaseRepository, UserAgent: ProductName + "-updater/1"}
	}
	verifier := config.Verifier
	if verifier == nil {
		verifier = featureupdater.ExactVersionLine(ProductName)
	}
	runner := config.Runner
	if runner == nil {
		runner = runUpdateCommand
	}
	service := &UpdateService{
		config:       config,
		installation: installation,
		source:       source,
		verifier:     verifier,
		runner:       runner,
		platformOS:   platformOS,
		platformArch: platformArch,
	}
	if installation.Kind == InstallStandalone && (platformOS == "darwin" || platformOS == "linux") {
		standalone, err := featureupdater.New(featureupdater.Config{
			Product:           ProductName,
			CurrentVersion:    config.CurrentVersion,
			ExecutablePath:    installation.ExecutablePath,
			ArchiveBinaryName: releaseBinaryName,
			ChecksumsAsset:    "checksums.txt",
			AssetName:         releaseArchiveName,
			Source:            source,
			Verifier:          verifier,
			Progress:          config.Progress,
		})
		if err != nil {
			return nil, err
		}
		service.standalone = standalone
	}
	return service, nil
}

func (service *UpdateService) Installation() Installation {
	if service == nil {
		return Installation{}
	}
	return service.installation
}

func (service *UpdateService) Prepare(ctx context.Context) (UpdatePlan, error) {
	if service == nil {
		return UpdatePlan{}, fmt.Errorf("update service is nil")
	}
	if !service.operation.TryLock() {
		return UpdatePlan{}, featureupdater.ErrUpdateInProgress
	}
	defer service.operation.Unlock()

	state := &updatePlanState{owner: service}
	if service.standalone != nil {
		plan, err := service.standalone.Prepare(ctx)
		if err != nil {
			return UpdatePlan{}, err
		}
		state.foundation = plan
		state.available = plan.Available()
		state.release = plan.Release()
		state.asset = plan.ArchiveAsset()
	} else {
		service.emit(featureupdater.Event{Stage: featureupdater.StageChecking})
		release, err := service.source.LatestStable(ctx)
		if err != nil {
			return UpdatePlan{}, fmt.Errorf("resolve latest stable release: %w", err)
		}
		if err := featureupdater.ValidateStableRelease(release); err != nil {
			return UpdatePlan{}, err
		}
		available, err := featureupdater.IsNewerStable(release.Tag, service.config.CurrentVersion)
		if err != nil {
			return UpdatePlan{}, err
		}
		state.available = available
		state.release = cloneUpdateRelease(release)
		if available && service.installation.Kind == InstallNPM {
			state.asset = featureupdater.Asset{Name: ProductName + "@" + strings.TrimPrefix(release.Tag, "v")}
		}
		if available && service.installation.Kind == InstallStandalone {
			archiveName := releaseArchiveName(release.Tag, service.platformOS, service.platformArch)
			binaryName := releaseBinaryName(release.Tag, service.platformOS, service.platformArch)
			archive, err := exactUpdateAsset(release, archiveName)
			if err != nil {
				return UpdatePlan{}, err
			}
			checksums, err := exactUpdateAsset(release, "checksums.txt")
			if err != nil {
				return UpdatePlan{}, err
			}
			service.emit(featureupdater.Event{Stage: featureupdater.StageDownloadingChecksums, TargetVersion: release.Tag, Asset: checksums.Name})
			manifest, err := service.downloadBytes(ctx, checksums, maxChecksumManifestBytes)
			if err != nil {
				return UpdatePlan{}, fmt.Errorf("download checksum manifest: %w", err)
			}
			checksum, err := parseUpdateChecksum(string(manifest), archiveName)
			if err != nil {
				return UpdatePlan{}, err
			}
			state.asset = archive
			state.binaryName = binaryName
			state.checksum = checksum
		}
		stage := featureupdater.StageUpToDate
		if available {
			stage = featureupdater.StageAvailable
		}
		service.emit(featureupdater.Event{Stage: stage, TargetVersion: release.Tag})
	}
	state.generation = service.generation.Add(1)
	return UpdatePlan{state: state}, nil
}

func (service *UpdateService) Apply(ctx context.Context, plan UpdatePlan) (result UpdateResult, resultErr error) {
	if service == nil {
		return result, fmt.Errorf("update service is nil")
	}
	if !service.operation.TryLock() {
		return result, featureupdater.ErrUpdateInProgress
	}
	defer service.operation.Unlock()
	targetLockValue, _ := updateTargetLocks.LoadOrStore(service.installation.ExecutablePath, &sync.Mutex{})
	targetLock := targetLockValue.(*sync.Mutex)
	if !targetLock.TryLock() {
		return result, featureupdater.ErrUpdateInProgress
	}
	defer targetLock.Unlock()
	state, err := service.validatePlan(plan)
	if err != nil {
		return result, err
	}
	if !state.status.CompareAndSwap(updatePlanReady, updatePlanApplying) {
		if state.status.Load() == updatePlanConsumed {
			return result, featureupdater.ErrPlanConsumed
		}
		return result, featureupdater.ErrUpdateInProgress
	}
	defer func() {
		if resultErr == nil {
			state.status.Store(updatePlanConsumed)
		} else {
			state.status.Store(updatePlanReady)
		}
	}()

	if service.standalone != nil {
		return service.standalone.Apply(ctx, state.foundation)
	}
	result.Release = cloneUpdateRelease(state.release)
	result.ArchiveAsset = state.asset.Name
	if !state.available {
		return result, nil
	}
	if service.installation.Kind == InstallNPM {
		if err := service.applyNPM(ctx, state.release.Tag); err != nil {
			return result, err
		}
	} else {
		backup, err := service.applyHostStandalone(ctx, state)
		result.BackupRetainedAt = backup
		if err != nil {
			return result, err
		}
	}
	result.Updated = true
	service.emit(featureupdater.Event{Stage: featureupdater.StageComplete, TargetVersion: state.release.Tag})
	return result, nil
}

func (service *UpdateService) UpdateLatest(ctx context.Context) (UpdateResult, error) {
	plan, err := service.Prepare(ctx)
	if err != nil {
		return UpdateResult{}, err
	}
	return service.Apply(ctx, plan)
}

func (service *UpdateService) validatePlan(plan UpdatePlan) (*updatePlanState, error) {
	if plan.state == nil || plan.state.owner != service {
		return nil, featureupdater.ErrInvalidPlan
	}
	if plan.state.generation != service.generation.Load() {
		return nil, featureupdater.ErrPlanSuperseded
	}
	return plan.state, nil
}

func (service *UpdateService) applyNPM(ctx context.Context, tag string) error {
	installation := service.installation
	if installation.Kind != InstallNPM || installation.PackageDir == "" || installation.NPMPrefix == "" {
		return fmt.Errorf("invalid npm installation metadata")
	}
	stableVersion := strings.TrimPrefix(tag, "v")
	packageSpec := ProductName + "@" + stableVersion
	service.emit(featureupdater.Event{Stage: featureupdater.StageInstalling, TargetVersion: tag, Asset: packageSpec})
	if err := service.runner(ctx, npmExecutableName(), "install", "--global", "--prefix", installation.NPMPrefix, packageSpec); err != nil {
		return fmt.Errorf("npm install %s: %w", packageSpec, err)
	}

	metadata, err := readPackageMetadata(filepath.Join(installation.PackageDir, "package.json"))
	if err != nil {
		return fmt.Errorf("verify updated npm package: %w", err)
	}
	if metadata.Name != ProductName || strings.TrimPrefix(metadata.Version, "v") != stableVersion {
		return fmt.Errorf("npm package metadata did not update to %s", stableVersion)
	}
	if err := service.verifier.Verify(ctx, installation.ExecutablePath, tag); err != nil {
		return fmt.Errorf("verify updated npm binary: %w", err)
	}
	service.emit(featureupdater.Event{Stage: featureupdater.StageInstalledVerified, TargetVersion: tag, Asset: packageSpec})
	return nil
}

const (
	maxChecksumManifestBytes = 1024 * 1024
	maxHostArchiveBytes      = 256 * 1024 * 1024
	maxHostBinaryBytes       = 128 * 1024 * 1024
)

// applyHostStandalone is the explicit host adapter for standalone platforms
// outside the Foundation's darwin/linux replacement guarantee (currently
// Windows). Release selection and checksum pinning still come from Prepare.
func (service *UpdateService) applyHostStandalone(ctx context.Context, state *updatePlanState) (string, error) {
	if service.platformOS != "windows" {
		return "", fmt.Errorf("standalone host adapter is unavailable on %s", service.platformOS)
	}
	target := service.installation.ExecutablePath
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("current executable is missing or unsafe")
	}

	service.emit(featureupdater.Event{Stage: featureupdater.StageDownloadingArchive, TargetVersion: state.release.Tag, Asset: state.asset.Name})
	archivePath, archiveBytes, err := service.downloadFile(ctx, state.asset, maxHostArchiveBytes, filepath.Dir(target))
	if err != nil {
		return "", fmt.Errorf("download release archive: %w", err)
	}
	defer func() { _ = os.Remove(archivePath) }()
	if err := verifyUpdateChecksum(archivePath, state.checksum); err != nil {
		return "", err
	}
	service.emit(featureupdater.Event{Stage: featureupdater.StageChecksumVerified, TargetVersion: state.release.Tag, Asset: state.asset.Name, Bytes: archiveBytes})

	staged, err := extractExactZipBinary(archivePath, state.binaryName, filepath.Dir(target), maxHostBinaryBytes)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(staged) }()
	if err := os.Chmod(staged, 0o755); err != nil {
		return "", fmt.Errorf("prepare staged binary: %w", err)
	}
	stagedHash, err := updateFileSHA256(staged)
	if err != nil {
		return "", err
	}
	if err := service.verifier.Verify(ctx, staged, state.release.Tag); err != nil {
		return "", fmt.Errorf("verify staged binary: %w", err)
	}
	if err := requireUpdateFileHash(staged, stagedHash, "staged version probe modified the binary"); err != nil {
		return "", err
	}
	service.emit(featureupdater.Event{Stage: featureupdater.StageStagedVerified, TargetVersion: state.release.Tag, Asset: state.asset.Name})

	backup := target + ".old"
	if _, err := os.Lstat(backup); err == nil {
		return backup, fmt.Errorf("refusing to overwrite existing update backup %s", backup)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect update backup: %w", err)
	}
	service.emit(featureupdater.Event{Stage: featureupdater.StageInstalling, TargetVersion: state.release.Tag, Asset: state.asset.Name})
	if err := os.Rename(target, backup); err != nil {
		return "", fmt.Errorf("backup current executable: %w", err)
	}
	rollback := func(cause error) (string, error) {
		_ = os.Remove(target)
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return backup, fmt.Errorf("%w; rollback failed: %v", cause, restoreErr)
		}
		return "", cause
	}
	if err := os.Rename(staged, target); err != nil {
		return rollback(fmt.Errorf("install staged executable: %w", err))
	}
	if err := requireUpdateFileHash(target, stagedHash, "installed binary changed before version verification"); err != nil {
		return rollback(err)
	}
	if err := service.verifier.Verify(ctx, target, state.release.Tag); err != nil {
		return rollback(fmt.Errorf("verify installed binary: %w", err))
	}
	if err := requireUpdateFileHash(target, stagedHash, "installed version probe modified the binary"); err != nil {
		return rollback(err)
	}
	service.emit(featureupdater.Event{Stage: featureupdater.StageInstalledVerified, TargetVersion: state.release.Tag, Asset: state.asset.Name})
	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return backup, nil
	}
	return "", nil
}

func exactUpdateAsset(release featureupdater.Release, name string) (featureupdater.Asset, error) {
	var match featureupdater.Asset
	found := false
	for _, asset := range release.Assets {
		if asset.Name != name {
			continue
		}
		if found {
			return featureupdater.Asset{}, fmt.Errorf("release %s contains duplicate asset %s", release.Tag, name)
		}
		match = asset
		found = true
	}
	if !found {
		return featureupdater.Asset{}, fmt.Errorf("release %s does not contain %s", release.Tag, name)
	}
	return match, nil
}

func (service *UpdateService) downloadBytes(ctx context.Context, asset featureupdater.Asset, maximum int64) ([]byte, error) {
	var buffer strings.Builder
	writer := &boundedUpdateWriter{destination: &buffer, remaining: maximum}
	if err := service.source.Download(ctx, asset, writer); err != nil {
		return nil, err
	}
	return []byte(buffer.String()), nil
}

func (service *UpdateService) downloadFile(ctx context.Context, asset featureupdater.Asset, maximum int64, directory string) (string, int64, error) {
	file, err := os.CreateTemp(directory, ".cc-connect-next-update-*")
	if err != nil {
		return "", 0, err
	}
	path := file.Name()
	writer := &boundedUpdateWriter{destination: file, remaining: maximum}
	if err := service.source.Download(ctx, asset, writer); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", writer.written, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", writer.written, err
	}
	return path, writer.written, nil
}

type boundedUpdateWriter struct {
	destination io.Writer
	remaining   int64
	written     int64
}

func (writer *boundedUpdateWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > writer.remaining {
		allowed := int(writer.remaining)
		if allowed > 0 {
			written, err := writer.destination.Write(data[:allowed])
			writer.written += int64(written)
			writer.remaining -= int64(written)
			if err != nil {
				return written, err
			}
			return written, fmt.Errorf("asset exceeds configured size limit")
		}
		return 0, fmt.Errorf("asset exceeds configured size limit")
	}
	written, err := writer.destination.Write(data)
	writer.written += int64(written)
	writer.remaining -= int64(written)
	return written, err
}

func parseUpdateChecksum(manifest, assetName string) (string, error) {
	var checksum string
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != assetName {
			continue
		}
		if checksum != "" {
			return "", fmt.Errorf("checksum manifest contains more than one entry for %s", assetName)
		}
		candidate := strings.ToLower(fields[0])
		decoded, err := hex.DecodeString(candidate)
		if err != nil || len(decoded) != sha256.Size {
			return "", fmt.Errorf("checksum manifest contains an invalid SHA-256 for %s", assetName)
		}
		checksum = candidate
	}
	if checksum == "" {
		return "", fmt.Errorf("checksum manifest does not contain %s", assetName)
	}
	return checksum, nil
}

func verifyUpdateChecksum(path, expected string) error {
	actual, err := updateFileSHA256(path)
	if err != nil {
		return err
	}
	if actual != strings.ToLower(strings.TrimSpace(expected)) {
		return fmt.Errorf("checksum mismatch for release archive; refusing to install")
	}
	return nil
}

func updateFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func requireUpdateFileHash(path, expected, message string) error {
	actual, err := updateFileSHA256(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func extractExactZipBinary(archivePath, binaryName, directory string, maximum int64) (string, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("open zip archive: %w", err)
	}
	defer func() { _ = archive.Close() }()
	var match *zip.File
	for _, entry := range archive.File {
		if entry.Name != binaryName {
			continue
		}
		if match != nil {
			return "", fmt.Errorf("archive contains duplicate binary %s", binaryName)
		}
		if entry.FileInfo().IsDir() || entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
			return "", fmt.Errorf("archive binary is not a regular file")
		}
		if int64(entry.UncompressedSize64) > maximum {
			return "", fmt.Errorf("archive binary exceeds configured size limit")
		}
		match = entry
	}
	if match == nil {
		return "", fmt.Errorf("archive does not contain exact binary %s", binaryName)
	}
	reader, err := match.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()
	file, err := os.CreateTemp(directory, ".cc-connect-next-staged-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	writer := &boundedUpdateWriter{destination: file, remaining: maximum}
	if _, err := io.Copy(writer, reader); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (service *UpdateService) emit(event featureupdater.Event) {
	if service.config.Progress == nil {
		return
	}
	if event.Product == "" {
		event.Product = ProductName
	}
	if event.CurrentVersion == "" {
		event.CurrentVersion = service.config.CurrentVersion
	}
	service.config.Progress(event)
}

// CheckForUpdate performs discovery only. Interactive installation must still
// call UpdateService.Prepare and render that exact Plan before approval.
func CheckForUpdate(ctx context.Context, currentVersion string, source featureupdater.Source) (*UpdateRelease, error) {
	if source == nil {
		source = featuregithub.Source{Repository: releaseRepository, UserAgent: ProductName + "-updater/1"}
	}
	release, err := source.LatestStable(ctx)
	if err != nil {
		return nil, err
	}
	if err := featureupdater.ValidateStableRelease(release); err != nil {
		return nil, err
	}
	newer, err := featureupdater.IsNewerStable(release.Tag, currentVersion)
	if err != nil {
		return nil, err
	}
	if !newer {
		return nil, nil
	}
	copy := cloneUpdateRelease(release)
	return &copy, nil
}

func IsNewerStable(candidate, current string) (bool, error) {
	return featureupdater.IsNewerStable(candidate, current)
}

func IsTerminalUpdatePlanError(err error) bool {
	return errors.Is(err, featureupdater.ErrInvalidPlan) ||
		errors.Is(err, featureupdater.ErrPlanSuperseded) ||
		errors.Is(err, featureupdater.ErrPlanConsumed)
}

func DetectUpdateInstallation(executablePath string) (Installation, error) {
	if strings.TrimSpace(executablePath) == "" {
		path, err := os.Executable()
		if err != nil {
			return Installation{}, fmt.Errorf("locate current binary: %w", err)
		}
		executablePath = path
	}
	resolved := executablePath
	if path, err := filepath.EvalSymlinks(executablePath); err == nil {
		resolved = path
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return Installation{}, fmt.Errorf("resolve current binary: %w", err)
	}
	installation := Installation{Kind: InstallStandalone, ExecutablePath: absolute}
	binDir := filepath.Dir(absolute)
	if filepath.Base(binDir) != "bin" {
		return installation, nil
	}
	packageDir := filepath.Dir(binDir)
	metadata, err := readPackageMetadata(filepath.Join(packageDir, "package.json"))
	if os.IsNotExist(err) {
		return installation, nil
	}
	if err != nil {
		return Installation{}, fmt.Errorf("inspect possible npm installation: %w", err)
	}
	if metadata.Name != ProductName {
		return installation, nil
	}
	nodeModulesDir := filepath.Dir(packageDir)
	if filepath.Base(nodeModulesDir) != "node_modules" {
		return Installation{}, fmt.Errorf("npm package %s is not directly inside node_modules", packageDir)
	}
	prefixParent := filepath.Dir(nodeModulesDir)
	npmPrefix := prefixParent
	if filepath.Base(prefixParent) == "lib" {
		npmPrefix = filepath.Dir(prefixParent)
	}
	installation.Kind = InstallNPM
	installation.PackageDir = packageDir
	installation.NPMPrefix = npmPrefix
	return installation, nil
}

type packageMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func readPackageMetadata(path string) (packageMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return packageMetadata{}, err
	}
	var metadata packageMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return packageMetadata{}, err
	}
	return metadata, nil
}

func runUpdateCommand(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func npmExecutableName() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

func releaseArchiveName(tag, goos, goarch string) string {
	return featureupdater.ReleaseArchiveName(ProductName)(tag, goos, goarch)
}

func releaseBinaryName(tag, goos, goarch string) string {
	name := fmt.Sprintf("%s-%s-%s-%s", ProductName, tag, goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func cloneUpdateRelease(release featureupdater.Release) featureupdater.Release {
	copy := release
	copy.Assets = append([]featureupdater.Asset(nil), release.Assets...)
	return copy
}
