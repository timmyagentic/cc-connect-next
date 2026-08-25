package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type migrationReport struct {
	CopiedFiles        int
	ProjectDirectories int
	SkippedRuntime     int
	SkippedSymlinks    int
	SkippedProjects    []migrationSkippedProjectRecord
	SkippedDataEntries []string
	SourceReferences   []string
	SourceRoot         string
	SourceDataDir      string
	SourceWorkDir      string
	SourceVersion      string
	BackupDir          string
	Backups            []migrationBackupRecord
	ManifestPath       string
	DryRun             bool
}

var legacyRuntimeEntries = map[string]struct{}{
	"run":                   {},
	"logs":                  {},
	"daemon.json":           {},
	"cc-connect-daemon.ps1": {},
	"restart_notify":        {},
	"instance.lock":         {},
	"config.toml.lock":      {},
}

func runMigrate(args []string) int {
	return runMigrateCommand(args, os.Stdout, os.Stderr)
}

func runMigrateCommand(args []string, stdout, stderr io.Writer) int {
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "migrate: resolve home directory: %v\n", err)
		return 1
	}
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", filepath.Join(home, ".cc-connect"), "official CC Connect configuration directory")
	target := flags.String("target", filepath.Join(home, ".cc-connect-next"), "cc-connect-next data directory")
	force := flags.Bool("force", false, "merge into an existing target and overwrite matching files")
	dryRun := flags.Bool("dry-run", false, "validate and report without writing files")
	skipProjectData := flags.Bool("skip-project-data", false, "do not copy project-local .cc-connect images and attachments")
	runtimeWorkDir := flags.String("runtime-work-dir", "", "official runtime working directory for resolving relative config paths (auto-detected)")
	sourceVersion := flags.String("source-version", "auto", "official CC Connect release (auto, v1.4.1, or v1.5.0-beta.1 through v1.5.0)")
	switchOver := flags.Bool("switch", false, "require no installed Next service, stop and disable official CC Connect, final-sync, then install and start Next")
	notifyProject := flags.String("notify-project", "", "project whose Feishu/Lark bot sends the private completion message")
	notifyPlatform := flags.String("notify-platform", "", "choose feishu or lark when both are configured")
	notifyUser := flags.String("notify-user", "", "Feishu/Lark open_id to receive the private completion message")
	withLarkCLI := flags.Bool("lark-cli", false, "install/bind official lark-cli after migration and make the selected bot its default bot profile")
	withoutLarkCLI := flags.Bool("no-lark-cli", false, "skip lark-cli installation/binding without asking")
	larkCLIProject := flags.String("lark-cli-project", "", "project whose Feishu/Lark bot should become the default lark-cli profile")
	larkCLIPlatformIndex := flags.Int("lark-cli-platform-index", 0, "1-based index among Feishu/Lark platforms in the selected project")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: cc-connect-next migrate [--source DIR] [--target DIR] [--source-version RELEASE] [--runtime-work-dir DIR] [--dry-run] [--force] [--switch] [--notify-project NAME] [--notify-platform TYPE] [--notify-user OPEN_ID] [--lark-cli] [--no-lark-cli] [--lark-cli-project NAME] [--lark-cli-platform-index N] [--skip-project-data]")
		_, _ = fmt.Fprintln(flags.Output(), "Copies configuration, the effective data_dir, and project-local state while excluding daemon, logs, locks, and sockets.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *switchOver && *dryRun {
		_, _ = fmt.Fprintln(stderr, "migrate: --switch cannot be combined with --dry-run (the switchover stops a live daemon)")
		return 2
	}
	if _, _, err := resolveLarkCLICompanionFlags(*withLarkCLI, *withoutLarkCLI); err != nil {
		_, _ = fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 2
	}
	if *larkCLIPlatformIndex < 0 {
		_, _ = fmt.Fprintln(stderr, "migrate: --lark-cli-platform-index must be >= 0")
		return 2
	}
	if *switchOver && strings.TrimSpace(os.Getenv("CC_SESSION_KEY")) != "" {
		_, _ = fmt.Fprintln(stderr, "migrate: --switch must run from an external terminal; stopping the official daemon would also terminate an Agent-launched migration")
		_, _ = fmt.Fprintln(stderr, "Rerun outside the connected Agent session; use --notify-project and --notify-user when the private-message target is not unique.")
		return 2
	}

	writeStdout := func(format string, args ...any) bool {
		if _, err := fmt.Fprintf(stdout, format, args...); err != nil {
			_, _ = fmt.Fprintf(stderr, "migrate: write output: %v\n", err)
			return false
		}
		return true
	}
	migrationOpts := migrationOptions{
		Source:             expandMigrationPath(*source, home),
		Target:             expandMigrationPath(*target, home),
		Home:               home,
		RuntimeWorkDir:     expandMigrationPath(*runtimeWorkDir, home),
		SourceVersion:      *sourceVersion,
		Force:              *force,
		DryRun:             *dryRun,
		IncludeProjectData: !*skipProjectData,
	}
	var notifyTarget *migrationNotifyTarget
	notifyReason := ""
	if *switchOver {
		notifyTarget, notifyReason, err = resolveMigrationNotifyTarget(filepath.Join(migrationOpts.Source, "config.toml"), migrationNotifyHints{
			Project: *notifyProject, Platform: *notifyPlatform, UserID: *notifyUser,
		})
		if err != nil {
			notifyReason = err.Error()
		}
		if notifyTarget == nil {
			if !writeStdout("Completion notification will be skipped: %s.\n", notifyReason) {
				return 1
			}
		}
	}
	var report migrationReport
	var cutover migrationCutoverResult
	if *switchOver {
		cutover, err = runDirectMigrationCutover(migrationOpts, defaultMigrationCutoverDeps(), writeStdout)
		report = cutover.Report
	} else {
		report, err = migrateLegacyDataWithOptions(migrationOpts)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 1
	}
	writeOutput := func(format string, args ...any) bool {
		if _, err := fmt.Fprintf(stdout, format, args...); err != nil {
			_, _ = fmt.Fprintf(stderr, "migrate: write output: %v\n", err)
			return false
		}
		return true
	}
	verb := "Migrated"
	if report.DryRun {
		verb = "Would migrate"
	}
	// --source names the directory holding config.toml, which is not always
	// where the state lives: a config that omits data_dir uses the official
	// default no matter where it was loaded from. Naming one root for both
	// would misattribute every copied file.
	sourceRoot := report.SourceRoot
	if sourceRoot == "" {
		sourceRoot = *source
	}
	if report.SourceDataDir != "" && report.SourceDataDir != sourceRoot {
		if !writeOutput("%s %d persistent files to %s: config from %s, state from %s.\n", verb, report.CopiedFiles, *target, sourceRoot, report.SourceDataDir) {
			return 1
		}
	} else if !writeOutput("%s %d persistent files from %s to %s.\n", verb, report.CopiedFiles, *source, *target) {
		return 1
	}
	if report.SkippedRuntime > 0 || report.SkippedSymlinks > 0 {
		if !writeOutput("Skipped %d runtime entries and %d symlinks.\n", report.SkippedRuntime, report.SkippedSymlinks) {
			return 1
		}
	}
	if len(report.SkippedDataEntries) > 0 {
		if !writeOutput("Skipped %d entries under the configured data_dir that this build does not recognize as CC Connect state; they were not copied.\n", len(report.SkippedDataEntries)) {
			return 1
		}
		for _, entry := range report.SkippedDataEntries {
			if !writeOutput("  - %s\n", entry) {
				return 1
			}
		}
	}
	if len(report.SourceReferences) > 0 {
		if !writeOutput("%d configuration values still point at %s. Migration preserves them byte-for-byte, so the migrated installation keeps using the old directory until you update them (the source cannot be removed before you do):\n", len(report.SourceReferences), sourceRoot) {
			return 1
		}
		for _, reference := range report.SourceReferences {
			if !writeOutput("  - %s\n", reference) {
				return 1
			}
		}
	}
	if len(report.SkippedProjects) > 0 {
		if !writeOutput("Skipped %d optional project-data discovery entries; grant access or repair malformed metadata and rerun before relying on project-local completeness.\n", len(report.SkippedProjects)) {
			return 1
		}
		for _, skipped := range report.SkippedProjects {
			if !writeOutput("  - %s (%s)\n", skipped.Source, skipped.Reason) {
				return 1
			}
		}
	}
	if !writeOutput("Official runtime work_dir: %s. Effective data_dir: %s. Project-local directories: %d.\n", report.SourceWorkDir, report.SourceDataDir, report.ProjectDirectories) {
		return 1
	}
	if !writeOutput("Source compatibility: %s.\n", report.SourceVersion) {
		return 1
	}
	if !report.DryRun {
		for _, backup := range report.Backups {
			if !writeOutput("Previous target backup: %s -> %s\n", backup.Target, backup.Backup) {
				return 1
			}
		}
		if !writeOutput("Verification manifest: %s\n", report.ManifestPath) {
			return 1
		}
		if *switchOver {
			if !writeOutput("Production cutover complete: official CC Connect is stopped and disabled; cc-connect-next is installed and running.\n") {
				return 1
			}
			if cutover.DaemonStatus != nil && cutover.DaemonStatus.PID > 0 {
				if !writeOutput("cc-connect-next daemon: running (PID %d, platform %s).\n", cutover.DaemonStatus.PID, cutover.DaemonStatus.Platform) {
					return 1
				}
			} else if cutover.DaemonStatus != nil {
				if !writeOutput("cc-connect-next daemon: running (platform %s).\n", cutover.DaemonStatus.Platform) {
					return 1
				}
			}
			if !writeOutput("Daemon config: %s\nDaemon work_dir: %s\n", cutover.ConfigPath, cutover.WorkDir) {
				return 1
			}
			runtimeState, runtimePlatforms := summarizeRuntimeHealth(cutover.RuntimeHealth)
			if !writeOutput("cc-connect-next runtime: %s.\n", runtimeState) {
				return 1
			}
			for _, platform := range runtimePlatforms {
				if !writeOutput("  - %s\n", platform) {
					return 1
				}
			}
			if notifyTarget != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				notifyErr := sendMigrationComplete(ctx, notifyTarget, filepath.Dir(cutover.ConfigPath))
				cancel()
				if notifyErr != nil {
					if !writeOutput("Migration is complete, but the private Feishu/Lark notification failed: %v\n", notifyErr) {
						return 1
					}
				} else if !writeOutput("Private migration-completion message sent to the selected Feishu/Lark operator.\n") {
					return 1
				}
			}
		} else {
			targetConfig := filepath.Join(expandMigrationPath(*target, home), "config.toml")
			probe := defaultOfficialProbe()
			coexistState := probeOfficialInstall(probe)
			overlap := officialCredentialOverlap(coexistState, extractAppIDsFromConfigFile(targetConfig))
			if !writeOutput("%s", renderOfficialCoexistenceGuidance(coexistState, overlap)) {
				return 1
			}
			if !writeOutput("Next: cc-connect-next --config %s\n", targetConfig) {
				return 1
			}
			if !writeOutput("Daemon config: %s\n", filepath.Join(expandMigrationPath(*target, home), "config.toml")) {
				return 1
			}
			if !writeOutput("Daemon work_dir: %s\n", report.SourceWorkDir) {
				return 1
			}
		}
	}

	larkConfigPath := filepath.Join(expandMigrationPath(*target, home), "config.toml")
	if *switchOver {
		larkConfigPath = cutover.ConfigPath
	}
	larkProjectHint := strings.TrimSpace(*larkCLIProject)
	if larkProjectHint == "" {
		larkProjectHint = strings.TrimSpace(*notifyProject)
	}
	if err := maybeSetupMigratedLarkCLI(
		stdout,
		os.Stdin,
		*withLarkCLI,
		*withoutLarkCLI,
		stdinIsInteractive(),
		report.DryRun,
		larkConfigPath,
		larkProjectHint,
		*larkCLIPlatformIndex,
		func(target larkCLITarget) error {
			result, err := setupLarkCLICompanion(context.Background(), target, larkCLISetupOptions{InstallIfMissing: true}, realLarkCLIProcess{})
			if err != nil {
				return err
			}
			reportLarkCLISetupResult(stdout, result)
			return nil
		},
	); err != nil {
		if !writeOutput("Migration succeeded, but the lark-cli companion was not configured: %v\n", err) {
			return 1
		}
		if !writeOutput("Run `cc-connect-next lark-cli setup --config %s --project <name>` after choosing the intended bot.\n", larkConfigPath) {
			return 1
		}
	}
	return 0
}

func migrateLegacyData(source, target string, force, dryRun bool) (migrationReport, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return migrationReport{DryRun: dryRun}, fmt.Errorf("resolve home directory: %w", err)
	}
	if filepath.Base(filepath.Clean(source)) == ".cc-connect" {
		// Keep this compatibility helper deterministic for callers that pass a
		// conventional legacy root outside the current process home. The real
		// CLI supplies os.UserHomeDir directly through migrationOptions.
		home = filepath.Dir(filepath.Clean(source))
	}
	return migrateLegacyDataWithOptions(migrationOptions{
		Source:             source,
		Target:             target,
		Home:               home,
		Force:              force,
		DryRun:             dryRun,
		IncludeProjectData: true,
		SourceVersion:      "auto",
	})
}

func directoryHasEntries(path string) (bool, error) {
	dir, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect target: %w", err)
	}
	defer func() { _ = dir.Close() }()
	if info, err := dir.Stat(); err != nil {
		return false, err
	} else if !info.IsDir() {
		return false, fmt.Errorf("target is not a directory: %s", path)
	}
	_, err = dir.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return false, nil
	}
	return err == nil, err
}

func rewriteMigratedDataDir(configBytes []byte, target string) ([]byte, error) {
	configText := string(configBytes)
	replacement := strconv.Quote(filepath.ToSlash(target))
	valueStart, valueEnd, found, err := findTopLevelTOMLStringValue(configText, "data_dir")
	if err != nil {
		return nil, err
	}
	if found {
		return []byte(configText[:valueStart] + replacement + configText[valueEnd:]), nil
	}

	// Distinguish an absent key from a valid spelling the locator failed to
	// understand. Never prepend a duplicate logical key: that would make an
	// otherwise valid legacy config fail only after the migration starts.
	var root map[string]any
	if _, err := toml.Decode(configText, &root); err != nil {
		return nil, fmt.Errorf("decode config while locating data_dir: %w", err)
	}
	if _, exists := root["data_dir"]; exists {
		return nil, fmt.Errorf("locate existing top-level data_dir assignment")
	}
	return []byte("data_dir = " + replacement + "\n\n" + configText), nil
}

func findTopLevelTOMLStringValue(src, wantedKey string) (int, int, bool, error) {
	for pos := 0; pos < len(src); {
		for pos < len(src) {
			switch src[pos] {
			case ' ', '\t', '\r', '\n':
				pos++
			case '#':
				pos = skipTOMLLine(src, pos)
			default:
				goto statement
			}
		}
		break

	statement:
		// Once a table header begins, subsequent simple keys belong to that
		// table; TOML has no syntax for returning to the root table.
		if src[pos] == '[' {
			return 0, 0, false, nil
		}

		equals, err := findTOMLKeyEquals(src, pos)
		if err != nil {
			return 0, 0, false, err
		}
		keyText := strings.TrimSpace(src[pos:equals])
		valueStart := equals + 1
		for valueStart < len(src) && (src[valueStart] == ' ' || src[valueStart] == '\t') {
			valueStart++
		}
		if tomlKeyEquals(keyText, wantedKey) {
			valueEnd, err := scanTOMLString(src, valueStart)
			if err != nil {
				return 0, 0, false, fmt.Errorf("scan top-level %s value: %w", wantedKey, err)
			}
			return valueStart, valueEnd, true, nil
		}

		pos, err = skipTOMLValue(src, valueStart)
		if err != nil {
			return 0, 0, false, err
		}
	}
	return 0, 0, false, nil
}

func findTOMLKeyEquals(src string, start int) (int, error) {
	var quote byte
	escaped := false
	for i := start; i < len(src); i++ {
		ch := src[i]
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '=':
			return i, nil
		case '\n', '#':
			return 0, fmt.Errorf("top-level TOML statement has no assignment")
		}
	}
	return 0, fmt.Errorf("unterminated top-level TOML assignment")
}

func tomlKeyEquals(keyText, wanted string) bool {
	var decoded map[string]any
	probe := keyText + " = \"__cc_connect_next_key_probe__\""
	if _, err := toml.Decode(probe, &decoded); err != nil {
		return false
	}
	value, ok := decoded[wanted]
	return ok && value == "__cc_connect_next_key_probe__"
}

func scanTOMLString(src string, start int) (int, error) {
	if start >= len(src) || (src[start] != '"' && src[start] != '\'') {
		return 0, fmt.Errorf("value is not a TOML string")
	}
	quote := src[start]
	triple := start+2 < len(src) && src[start+1] == quote && src[start+2] == quote
	i := start + 1
	if triple {
		i = start + 3
	}
	for i < len(src) {
		if quote == '"' && src[i] == '\\' {
			if i+1 >= len(src) {
				return 0, fmt.Errorf("unterminated escape sequence")
			}
			i += 2
			continue
		}
		if src[i] == quote {
			if !triple {
				return i + 1, nil
			}
			runStart := i
			for i < len(src) && src[i] == quote {
				i++
			}
			if i-runStart >= 3 {
				return i, nil
			}
			continue
		}
		if !triple && (src[i] == '\n' || src[i] == '\r') {
			return 0, fmt.Errorf("unterminated single-line string")
		}
		i++
	}
	return 0, fmt.Errorf("unterminated TOML string")
}

func skipTOMLValue(src string, start int) (int, error) {
	squareDepth := 0
	curlyDepth := 0
	for i := start; i < len(src); {
		switch src[i] {
		case '"', '\'':
			end, err := scanTOMLString(src, i)
			if err != nil {
				return 0, err
			}
			i = end
		case '[':
			squareDepth++
			i++
		case ']':
			if squareDepth > 0 {
				squareDepth--
			}
			i++
		case '{':
			curlyDepth++
			i++
		case '}':
			if curlyDepth > 0 {
				curlyDepth--
			}
			i++
		case '#':
			i = skipTOMLLine(src, i)
			if squareDepth == 0 && curlyDepth == 0 {
				return i, nil
			}
		case '\n':
			i++
			if squareDepth == 0 && curlyDepth == 0 {
				return i, nil
			}
		default:
			i++
		}
	}
	return len(src), nil
}

func skipTOMLLine(src string, start int) int {
	if newline := strings.IndexByte(src[start:], '\n'); newline >= 0 {
		return start + newline + 1
	}
	return len(src)
}

func copyMigrationFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(target, 0o600)
}

func writeMigrationFile(target string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".config.toml.migrate-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, target)
}

func expandMigrationPath(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(home, path[2:])
	}
	return path
}
