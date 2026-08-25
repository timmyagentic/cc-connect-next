package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
	"github.com/timmyagentic/cc-connect-next/daemon"
)

const (
	migrationRuntimeReadyTimeout  = 45 * time.Second
	migrationRuntimeDisarmTimeout = 10 * time.Second
	migrationDaemonPollInterval   = 100 * time.Millisecond
	migrationHealthProbeTimeout   = 500 * time.Millisecond
)

// migrationCutoverDeps keeps service mutation behind testable boundaries. A
// production --switch creates a fresh value, so the probe captured by the
// closures is scoped to one cutover and cannot leak between concurrent calls.
type migrationCutoverDeps struct {
	PrepareMigration    func(migrationOptions) (*preparedMigration, error)
	RunMigration        func(migrationOptions) (migrationReport, error)
	CheckNextUnits      func(string) error
	NewDaemonManager    func() (daemon.Manager, error)
	ResolveDaemonConfig func(*daemon.Config) error
	SaveDaemonMeta      func(*daemon.Meta) error
	WaitRuntimeReady    func(daemon.Manager, string) (*daemon.Status, core.RuntimeHealth, error)
	WaitRuntimeDisarmed func(daemon.Manager, string) error
	ProbeOfficial       func(*preparedMigration) officialInstallState
	SwitchOfficial      func(officialInstallState, func(string, ...any) bool) error
	RestoreOfficial     func(officialInstallState, func(string, ...any) bool) error
}

type migrationCutoverResult struct {
	Report        migrationReport
	DaemonStatus  *daemon.Status
	RuntimeHealth core.RuntimeHealth
	ConfigPath    string
	WorkDir       string
}

func defaultMigrationCutoverDeps() migrationCutoverDeps {
	probe := defaultOfficialProbe()
	return migrationCutoverDeps{
		PrepareMigration:    prepareLegacyMigration,
		RunMigration:        migrateLegacyDataWithOptions,
		CheckNextUnits:      checkMigrationSuccessorRegistrations,
		NewDaemonManager:    daemon.NewManager,
		ResolveDaemonConfig: daemon.Resolve,
		SaveDaemonMeta:      daemon.SaveMeta,
		WaitRuntimeReady:    waitForMigrationRuntimeReady,
		WaitRuntimeDisarmed: waitForMigrationRuntimeDisarmed,
		ProbeOfficial: func(plan *preparedMigration) officialInstallState {
			probe = officialProbeForMigration(probe, plan)
			return probeOfficialInstall(probe)
		},
		SwitchOfficial: func(state officialInstallState, out func(string, ...any) bool) error {
			return runOfficialSwitchover(probe, state, out)
		},
		RestoreOfficial: func(state officialInstallState, out func(string, ...any) bool) error {
			if err := restoreOfficialInstall(probe, state, out); err != nil {
				return err
			}
			return waitForOfficialRestored(probe, state)
		},
	}
}

// runDirectMigrationCutover is the normal first-time production migration.
// Existing successor services are rejected before official CC Connect is
// touched; upgrading or replacing an existing Next install is a different
// operation and does not belong in this migration transaction.
func runDirectMigrationCutover(opts migrationOptions, deps migrationCutoverDeps, out func(string, ...any) bool) (migrationCutoverResult, error) {
	var result migrationCutoverResult
	opts.Force = true
	opts.DryRun = false

	plan, err := deps.PrepareMigration(opts)
	if err != nil {
		return result, fmt.Errorf("migration preflight: %w", err)
	}
	if plan.Main == nil {
		return result, fmt.Errorf("migration preflight: main target is missing")
	}
	if err := deps.CheckNextUnits(opts.Home); err != nil {
		return result, fmt.Errorf("inspect cc-connect-next service registrations: %w", err)
	}

	mgr, err := deps.NewDaemonManager()
	if err != nil {
		return result, fmt.Errorf("prepare cc-connect-next daemon: %w", err)
	}
	nextBefore, err := mgr.Status()
	if err != nil {
		return result, fmt.Errorf("inspect cc-connect-next daemon: %w", err)
	}
	if nextBefore == nil {
		nextBefore = &daemon.Status{Platform: mgr.Platform()}
	}
	if nextBefore.Installed || nextBefore.Running {
		return result, fmt.Errorf("an existing cc-connect-next service is installed; run `cc-connect-next daemon uninstall`, verify it is removed, then rerun the migration")
	}

	officialBefore := deps.ProbeOfficial(plan)
	if officialBefore.ProbeErr != nil {
		return result, fmt.Errorf("inspect official CC Connect before cutover: %w", officialBefore.ProbeErr)
	}
	if err := deps.SwitchOfficial(officialBefore, out); err != nil {
		recoveryErr := deps.RestoreOfficial(officialBefore, out)
		return result, errors.Join(fmt.Errorf("stop and disable official CC Connect: %w", err), recoveryErr)
	}

	report, err := deps.RunMigration(opts)
	result.Report = report
	if err != nil {
		recoveryErr := deps.RestoreOfficial(officialBefore, out)
		return result, errors.Join(fmt.Errorf("final migration sync: %w", err), recoveryErr)
	}

	result.ConfigPath = filepath.Join(plan.Main.Target, "config.toml")
	result.WorkDir = report.SourceWorkDir
	if result.WorkDir == "" {
		result.WorkDir = plan.SourceWorkDir
	}
	cfg := daemon.Config{
		ConfigPath:       result.ConfigPath,
		WorkDir:          result.WorkDir,
		NoCaptureSecrets: isTruthyEnv(os.Getenv("CC_DAEMON_NO_CAPTURE_SECRETS")),
	}
	if err := deps.ResolveDaemonConfig(&cfg); err != nil {
		recoveryErr := recoverAfterMigration(deps, mgr, officialBefore, result.ConfigPath, out)
		return result, errors.Join(fmt.Errorf("prepare cc-connect-next daemon config: %w", err), recoveryErr)
	}

	out("Installing and starting cc-connect-next with config %s and work_dir %s…\n", cfg.ConfigPath, cfg.WorkDir)
	if err := mgr.Install(cfg); err != nil {
		recoveryErr := recoverAfterMigration(deps, mgr, officialBefore, result.ConfigPath, out)
		return result, errors.Join(fmt.Errorf("install and start cc-connect-next daemon: %w", err), recoveryErr)
	}
	if err := deps.SaveDaemonMeta(&daemon.Meta{
		LogFile:       cfg.LogFile,
		LogMaxSize:    cfg.LogMaxSize,
		LogMaxBackups: cfg.LogMaxBackups,
		WorkDir:       cfg.WorkDir,
		ConfigPath:    cfg.ConfigPath,
		DataDir:       filepath.Dir(cfg.ConfigPath),
		BinaryPath:    cfg.BinaryPath,
		InstalledAt:   daemon.NowISO(),
	}); err != nil {
		out("Warning: cc-connect-next is installed, but daemon metadata could not be saved: %v\n", err)
	}

	status, health, err := deps.WaitRuntimeReady(mgr, filepath.Dir(result.ConfigPath))
	if err != nil {
		recoveryErr := recoverAfterMigration(deps, mgr, officialBefore, result.ConfigPath, out)
		return result, errors.Join(fmt.Errorf("wait for cc-connect-next runtime readiness: %w", err), recoveryErr)
	}
	result.DaemonStatus = status
	result.RuntimeHealth = health
	return result, nil
}

func checkMigrationSuccessorRegistrations(home string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if strings.TrimSpace(home) == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
	}
	return checkMigrationSuccessorUnitPaths([]string{
		filepath.Join(home, ".config", "systemd", "user", "cc-connect-next.service"),
		"/etc/systemd/system/cc-connect-next.service",
	})
}

func checkMigrationSuccessorUnitPaths(paths []string) error {
	var registered []string
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			registered = append(registered, path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
	}
	if len(registered) == 0 {
		return nil
	}
	return fmt.Errorf("an existing cc-connect-next service is registered at %s; run `cc-connect-next daemon uninstall` in each registration's owning scope, verify it is removed, then rerun the migration", strings.Join(registered, ", "))
}

func recoverAfterMigration(deps migrationCutoverDeps, mgr daemon.Manager, officialBefore officialInstallState, configPath string, out func(string, ...any) bool) error {
	var recoveryErrors []error
	out("cc-connect-next activation failed; disarming the successor before restoring official CC Connect…\n")
	if err := mgr.Stop(); err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("stop failed successor: %w", err))
	}
	uninstallErr := mgr.Uninstall()
	if uninstallErr != nil {
		recoveryErrors = append(recoveryErrors,
			fmt.Errorf("uninstall failed successor service: %w", uninstallErr),
			errors.New("official CC Connect remains disabled because the failed successor could not be disarmed"))
		return errors.Join(recoveryErrors...)
	}
	if err := deps.WaitRuntimeDisarmed(mgr, configPath); err != nil {
		recoveryErrors = append(recoveryErrors,
			fmt.Errorf("prove failed successor is disarmed: %w", err),
			errors.New("official CC Connect remains disabled because the failed successor could not be proven disarmed"))
		return errors.Join(recoveryErrors...)
	}
	if err := deps.RestoreOfficial(officialBefore, out); err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("restore official CC Connect: %w", err))
	}
	return errors.Join(recoveryErrors...)
}

func waitForMigrationRuntimeReady(mgr daemon.Manager, dataDir string) (*daemon.Status, core.RuntimeHealth, error) {
	return waitForMigrationRuntimeReadyWithin(mgr, dataDir, migrationRuntimeReadyTimeout, migrationDaemonPollInterval)
}

func waitForMigrationRuntimeReadyWithin(mgr daemon.Manager, dataDir string, timeout, poll time.Duration) (*daemon.Status, core.RuntimeHealth, error) {
	deadline := time.Now().Add(timeout)
	var last *daemon.Status
	var lastHealth core.RuntimeHealth
	var lastErr error
	for {
		status, err := mgr.Status()
		if err == nil && status != nil {
			last = status
			if status.Installed && status.Running {
				ctx, cancel := context.WithTimeout(context.Background(), migrationHealthProbeTimeout)
				health, healthErr := readRuntimeHealth(ctx, resolveSocketPath(dataDir))
				cancel()
				if healthErr == nil {
					lastHealth = health
					if health.Ready {
						return status, health, nil
					}
					state, platforms := summarizeRuntimeHealth(health)
					lastErr = fmt.Errorf("runtime is %s: %s", strings.ToLower(state), strings.Join(platforms, "; "))
				} else {
					lastErr = healthErr
				}
			}
		} else if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return last, lastHealth, lastErr
			}
			return last, lastHealth, fmt.Errorf("timed out after %s waiting for a ready runtime (last status: %+v)", timeout, last)
		}
		time.Sleep(poll)
	}
}

func waitForMigrationRuntimeDisarmed(mgr daemon.Manager, configPath string) error {
	return waitForMigrationRuntimeDisarmedWithin(mgr, configPath, migrationRuntimeDisarmTimeout, migrationDaemonPollInterval)
}

func waitForMigrationRuntimeDisarmedWithin(mgr daemon.Manager, configPath string, timeout, poll time.Duration) error {
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("migrated config path is empty")
	}
	deadline := time.Now().Add(timeout)
	dataDir := filepath.Dir(configPath)
	var lastErr error
	for {
		status, err := mgr.Status()
		switch {
		case err != nil:
			lastErr = err
		case status == nil:
			lastErr = fmt.Errorf("daemon status is unavailable")
		case status.Installed || status.Running:
			lastErr = fmt.Errorf("service is still installed or running: %+v", status)
		default:
			ctx, cancel := context.WithTimeout(context.Background(), migrationHealthProbeTimeout)
			reachable, socketErr := runtimeAPIReachable(ctx, resolveSocketPath(dataDir))
			cancel()
			if socketErr != nil {
				lastErr = socketErr
			} else if reachable {
				lastErr = fmt.Errorf("runtime API socket still answers")
			} else if lock, lockErr := AcquireInstanceLock(configPath); lockErr != nil {
				lastErr = lockErr
			} else {
				lock.Release()
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s: %w", timeout, lastErr)
		}
		time.Sleep(poll)
	}
}
