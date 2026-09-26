package appfeatures

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
)

func TestNPMUpdateReportsCompletedInstallWhenVerificationFails(t *testing.T) {
	for _, failure := range []string{"metadata", "binary"} {
		t.Run(failure, func(t *testing.T) {
			prefix := t.TempDir()
			packageDir := filepath.Join(prefix, "lib", "node_modules", ProductName)
			executable := filepath.Join(packageDir, "bin", ProductName)
			if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
				t.Fatal(err)
			}
			packageJSON := filepath.Join(packageDir, "package.json")
			writePackageMetadata(t, packageJSON, "1.0.0")
			if err := os.WriteFile(executable, versionScript("v1.0.0"), 0o755); err != nil {
				t.Fatal(err)
			}
			installed := false
			var stages []featureupdater.Stage
			service, err := NewUpdateService(UpdateConfig{
				CurrentVersion: "v1.0.0", ExecutablePath: executable,
				Source: &memoryUpdateSource{release: featureupdater.Release{Tag: "v1.2.3"}},
				Runner: func(context.Context, string, ...string) error {
					installed = true
					if failure == "binary" {
						writePackageMetadata(t, packageJSON, "1.2.3")
					}
					return nil
				},
				Progress: func(event featureupdater.Event) { stages = append(stages, event.Stage) },
			})
			if err != nil {
				t.Fatal(err)
			}
			plan, err := service.Prepare(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Apply(context.Background(), plan)
			if err == nil || !errors.Is(err, ErrUpdateInstalledUnverified) || !strings.Contains(err.Error(), "npm install cc-connect-next@1.2.3 completed") || !strings.Contains(err.Error(), "verification failed") {
				t.Fatalf("error hides completed install: %v", err)
			}
			if !installed || result.Updated || containsUpdateStage(stages, featureupdater.StageComplete) || containsUpdateStage(stages, featureupdater.StageInstalledVerified) {
				t.Fatalf("unverified installation reported as success: installed=%t result=%+v stages=%v", installed, result, stages)
			}
			if failure == "binary" {
				metadata, err := readPackageMetadata(packageJSON)
				if err != nil || metadata.Version != "1.2.3" {
					t.Fatalf("installed metadata=%+v err=%v", metadata, err)
				}
				if verifyErr := service.verifier.Verify(context.Background(), executable, "v1.2.3"); verifyErr == nil || !strings.Contains(verifyErr.Error(), "does not equal") {
					t.Fatal("wrong installed binary was not rejected by exact verification")
				}
			}
		})
	}
}
