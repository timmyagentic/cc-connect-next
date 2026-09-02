package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
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
		fmt.Println("Usage: cc-connect-next update [stable|beta]\n       cc-connect-next update --channel <stable|beta>\n\nStable is the default. Beta is an explicit prerelease opt-in. The installation method is detected automatically.")
		return
	}
	options, err := parseUpdateOptions(os.Args[2:])
	if err == nil {
		err = runChannelUpdate(options)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
}

type updateOptions struct {
	Channel updatechannel.Channel
}

func parseUpdateOptions(args []string) (updateOptions, error) {
	options := updateOptions{Channel: updatechannel.Stable}
	if len(args) == 0 {
		return options, nil
	}
	if len(args) == 1 {
		switch args[0] {
		case "--beta", "--pre":
			options.Channel = updatechannel.Beta
			return options, nil
		case "--stable":
			return options, nil
		}
		if channel, ok := updatechannel.Parse(args[0]); ok && strings.TrimSpace(args[0]) != "" {
			options.Channel = channel
			return options, nil
		}
	}
	if len(args) == 2 && args[0] == "--channel" {
		channel, ok := updatechannel.Parse(args[1])
		if !ok || strings.TrimSpace(args[1]) == "" {
			return updateOptions{}, fmt.Errorf("update channel must be stable or beta")
		}
		options.Channel = channel
		return options, nil
	}
	return updateOptions{}, fmt.Errorf("unknown update option(s) %q; choose stable or beta", strings.Join(args, " "))
}

func runChannelUpdate(options updateOptions) error {
	channel := options.Channel.Effective()
	fmt.Printf("cc-connect-next %s\n", version)
	fmt.Printf("Checking the %s update channel...\n", channel)

	service, err := appfeatures.NewUpdateService(appfeatures.UpdateConfig{
		CurrentVersion: version,
		Channel:        channel,
		Progress:       printUpdateProgress,
	})
	if err != nil {
		return fmt.Errorf("initialize update service: %w", err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		return fmt.Errorf("prepare %s update: %w", channel, err)
	}
	latest := plan.Release().Tag
	if !plan.Available() {
		fmt.Printf("Already up to date on the %s channel (%s >= %s).\n", channel, version, latest)
		return nil
	}

	installation := service.Installation()
	fmt.Printf("New %s available on the %s channel: %s → %s\n", channel.ReleaseType(plan.Release().Prerelease), channel, version, latest)
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

func checkUpdate(args []string) {
	if version == "dev" || version == "" {
		return
	}
	options, err := parseUpdateOptions(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Check update failed: %v\n", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	release, err := appfeatures.CheckForUpdateChannel(ctx, version, options.Channel, nil)
	if err == nil && release != nil {
		channel := options.Channel.Effective()
		fmt.Fprintf(os.Stderr, "%s update available on the %s channel: %s → %s (run: cc-connect-next update %s)\n", channel.ReleaseType(release.Prerelease), channel, version, release.Tag, channel)
	}
}
