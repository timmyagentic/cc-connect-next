package appfeatures

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
)

func TestIssue166StandaloneUpdatePreservesSignedReleaseBytes(t *testing.T) {
	dist, version := os.Getenv("CCN_TEST_MACOS_RELEASE_DIR"), os.Getenv("CCN_TEST_MACOS_RELEASE_VERSION")
	if runtime.GOOS != "darwin" || dist == "" || version == "" {
		t.Skip("requires an isolated signed macOS release fixture")
	}
	target, source := updateFixture(t, "v0.0.0", version)
	archive := archiveName(version)
	payload, err := os.ReadFile(filepath.Join(dist, archive))
	if err != nil {
		t.Fatal(err)
	}
	source.release.Prerelease = true
	source.release.Assets[0].Size = int64(len(payload))
	source.payloads[archive] = payload
	source.payloads["checksums.txt"] = []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), archive))
	service, err := NewUpdateService(UpdateConfig{
		CurrentVersion: "v0.0.0", ExecutablePath: target, Source: source, Channel: updatechannel.Beta,
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Apply(context.Background(), plan)
	if err != nil || !result.Updated {
		t.Fatalf("signed update failed: result=%+v err=%v", result, err)
	}
	want, err := os.ReadFile(filepath.Join(dist, releaseBinaryName(version, runtime.GOOS, runtime.GOARCH)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("standalone update changed signed bytes: %v", err)
	}
	verifier, err := filepath.Abs(filepath.Join("..", "..", "scripts", "macos-signing.js"))
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("node", "-e", "require(process.argv[1]).verifyMacOSBinary(process.argv[2])", verifier, target).CombinedOutput(); err != nil {
		t.Fatalf("installed signature failed: %s (%v)", output, err)
	}
}
