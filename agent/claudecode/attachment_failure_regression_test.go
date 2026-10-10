package claudecode

import (
	"bytes"
	"context"
	"github.com/timmyagentic/cc-connect-next/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssue146_AttachmentStagingFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, kind := range []string{"file"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ".cc-connect-next"), []byte("blocked"), 0600); err != nil {
				t.Fatal(err)
			}
			s := &claudeSession{workDir: dir, ctx: context.Background()}
			s.alive.Store(true)
			var images []core.ImageAttachment
			var files []core.FileAttachment
			if kind == "file" {
				files = []core.FileAttachment{{FileName: "report.txt", Data: []byte("report")}}
			} else {
				images = []core.ImageAttachment{{MimeType: "image/png", Data: []byte("png")}}
			}
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("staging failure reached transport: %v", r)
				}
			}()
			if err := s.Send("inspect", images, files); err == nil || !strings.Contains(err.Error(), "stag") {
				t.Fatalf("missing staging error: %v", err)
			}
		})
	}
}

type issue146InlineWriter struct{ bytes.Buffer }

func (*issue146InlineWriter) Close() error { return nil }
func TestIssue146_InlineImageSurvivesOptionalLocalCopyFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cc-connect-next"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	w := &issue146InlineWriter{}
	s := &claudeSession{workDir: dir, stdin: w}
	s.alive.Store(true)
	if err := s.Send("inspect", []core.ImageAttachment{{MimeType: "image/png", Data: []byte("png")}}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.String(), "cG5n") || !strings.Contains(w.String(), "base64") {
		t.Fatalf("inline bytes lost: %s", w.String())
	}
}
