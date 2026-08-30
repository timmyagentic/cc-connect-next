package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

var cachedLatestVersion struct {
	version   string
	timestamp time.Time
	mu        sync.RWMutex
}

const versionCheckTTL = time.Hour

func fetchLatestStableReleaseAsync() {
	current := version
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		release, err := appfeatures.CheckForUpdate(ctx, current, nil)
		if err != nil || release == nil {
			return
		}
		cachedLatestVersion.mu.Lock()
		cachedLatestVersion.version = release.Tag
		cachedLatestVersion.timestamp = time.Now()
		cachedLatestVersion.mu.Unlock()
	}()
}

func checkUpdateAsync() {
	if version == "dev" || version == "" {
		return
	}
	fetchLatestStableReleaseAsync()
}

// getUpdateHintIfAvailable reads only the cache and never blocks on network.
func getUpdateHintIfAvailable() string {
	if version == "dev" || version == "" {
		return ""
	}
	cachedLatestVersion.mu.RLock()
	cachedVersion := cachedLatestVersion.version
	cachedAt := cachedLatestVersion.timestamp
	cachedLatestVersion.mu.RUnlock()
	if cachedVersion == "" || time.Since(cachedAt) > versionCheckTTL {
		fetchLatestStableReleaseAsync()
		return ""
	}
	newer, err := appfeatures.IsNewerStable(cachedVersion, version)
	if err != nil || !newer {
		return ""
	}
	return fmt.Sprintf("\n📦 Update available: %s → %s  (run: cc-connect-next update)\n", version, cachedVersion)
}

func runUpdate() {
	if len(os.Args) == 3 && (os.Args[2] == "--help" || os.Args[2] == "-h") {
		fmt.Println("Usage: cc-connect-next update\n\nUpdates to the latest stable release. The installation method is detected automatically.")
		return
	}
	if err := runStableUpdate(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
}

func runStableUpdate(args []string) error {
	if err := validateStableUpdateArgs(args); err != nil {
		return err
	}
	fmt.Printf("cc-connect-next %s\n", version)
	fmt.Println("Checking for stable updates...")

	service, err := appfeatures.NewUpdateService(appfeatures.UpdateConfig{
		CurrentVersion: version,
		Progress:       printUpdateProgress,
	})
	if err != nil {
		return fmt.Errorf("initialize update service: %w", err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		return fmt.Errorf("prepare stable update: %w", err)
	}
	latest := plan.Release().Tag
	if !plan.Available() {
		fmt.Printf("Already up to date (%s >= %s).\n", version, latest)
		return nil
	}

	installation := service.Installation()
	fmt.Printf("New stable version available: %s → %s\n", version, latest)
	switch installation.Kind {
	case appfeatures.InstallNPM:
		fmt.Printf("Detected npm installation at %s.\n", installation.PackageDir)
	case appfeatures.InstallStandalone:
		fmt.Printf("Detected standalone binary at %s.\n", installation.ExecutablePath)
	default:
		return fmt.Errorf("unsupported installation method %q", installation.Kind)
	}
	fmt.Printf("Selected exact artifact: %s\n", plan.ArchiveAsset().Name)
	result, err := service.Apply(context.Background(), plan)
	if err != nil {
		return err
	}
	if !result.Updated {
		fmt.Printf("Already up to date (%s).\n", version)
		return nil
	}
	fmt.Printf("Updated to %s\n", result.Release.Tag)
	if result.BackupRetainedAt != "" {
		fmt.Printf("Recovery backup retained at %s\n", result.BackupRetainedAt)
	}
	fmt.Println("Restart cc-connect-next to use the new version.")
	return nil
}

func validateStableUpdateArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	for _, argument := range args {
		if argument == "--pre" || argument == "--beta" {
			return fmt.Errorf("cc-connect-next update installs stable releases only; use npm install -g cc-connect-next@beta to opt into prereleases")
		}
	}
	return fmt.Errorf("unknown update option %q; cc-connect-next update accepts no options and installs the latest stable release", args[0])
}

func printUpdateProgress(event appfeatures.UpdateEvent) {
	switch event.Stage {
	case appfeatures.StageDownloadingChecksums:
		fmt.Printf("Downloading %s ...\n", event.Asset)
	case appfeatures.StageDownloadingArchive:
		fmt.Printf("Downloading %s ...\n", event.Asset)
	case appfeatures.StageChecksumVerified:
		fmt.Printf("Verified SHA-256 for %s\n", event.Asset)
	case appfeatures.StageStagedVerified:
		fmt.Printf("Verified staged %s binary.\n", event.TargetVersion)
	case appfeatures.StageInstalling:
		fmt.Printf("Installing %s ...\n", event.TargetVersion)
	case appfeatures.StageInstalledVerified:
		fmt.Printf("Verified installed %s binary.\n", event.TargetVersion)
	}
}

func checkUpdate() {
	if version == "dev" || version == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	release, err := appfeatures.CheckForUpdate(ctx, version, nil)
	if err == nil && release != nil {
		fmt.Fprintf(os.Stderr, "Stable update available: %s → %s (run: cc-connect-next update)\n", version, release.Tag)
	}
}
