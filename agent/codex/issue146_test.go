package codex

import (
	"context"
	"encoding/json"
	"github.com/timmyagentic/cc-connect-next/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssue146_FileStagingFailureStopsSendAndSteer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cc-connect-next"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	files := []core.FileAttachment{{FileName: "video.mp4", Data: []byte("video")}}
	execSession := &codexSession{workDir: dir, ctx: context.Background()}
	execSession.alive.Store(true)
	app := &appServerSession{workDir: dir}
	app.alive.Store(true)
	for name, send := range map[string]func(string, []core.ImageAttachment, []core.FileAttachment) error{"exec": execSession.Send, "app": app.Send, "steer": app.Steer} {
		t.Run(name, func(t *testing.T) {
			// Baseline app-server proceeds to unavailable transport after silently dropping files.
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("staging failure reached transport: %v", r)
				}
			}()
			err := send("look", nil, files)
			if err == nil || !strings.Contains(err.Error(), "stage files") {
				t.Fatalf("error=%v, want explicit staging failure", err)
			}
		})
	}
}

func TestIssue146_StageFilesPreservesBytesAndMessageIsolation(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"first", "second"} {
		data := []byte("complete-video-" + id)
		prompt, err := stageCodexFiles(dir, "look", []core.FileAttachment{{FileName: "../../video.mp4", MessageID: id, MimeType: "video/mp4", Data: data}})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, ".cc-connect-next", "attachments", id, "video.mp4")
		got, err := os.ReadFile(path)
		if err != nil || string(got) != string(data) || !strings.Contains(prompt, path) {
			t.Fatalf("unusable video path: %q %v", prompt, err)
		}
	}
	// Partial staging must not turn a multi-file request into a silent subset.
	if err := os.WriteFile(filepath.Join(dir, ".cc-connect-next", "attachments", "blocked"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	prompt, err := stageCodexFiles(dir, "look", []core.FileAttachment{{FileName: "ok.mp4", MessageID: "ok", Data: []byte("ok")}, {FileName: "bad.mp4", MessageID: "blocked", Data: []byte("bad")}})
	if err == nil || prompt != "" {
		t.Fatalf("partial files forwarded: %q %v", prompt, err)
	}
}

func TestIssue146_AppServerMediaInput(t *testing.T) {
	for _, steer := range []bool{false, true} {
		t.Run(map[bool]string{false: "send", true: "steer"}[steer], func(t *testing.T) {
			stdin := &steerFakeStdin{}
			s := newSteerTestSession(t, stdin, "thread-1", "turn-1")
			stdin.respond = func(id int64, _ map[string]any) {
				s.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(`{"turn":{"id":"turn-1"}}`)})
			}
			images := []core.ImageAttachment{{MimeType: "image/jpeg", Data: []byte("jpeg"), FileName: "image.jpg"}}
			files := []core.FileAttachment{{MimeType: "video/mp4", Data: []byte("full video"), FileName: "video.mp4", MessageID: "source"}}
			send := s.Send
			if steer {
				send = s.Steer
			}
			if err := send("look", images, files); err != nil {
				t.Fatal(err)
			}
			req := stdin.lastRequest()
			params := req["params"].(map[string]any)
			input := params["input"].([]any)
			if len(input) != 2 {
				t.Fatalf("input=%v", input)
			}
			text := input[0].(map[string]any)["text"].(string)
			videoPath := filepath.Join(s.workDir, ".cc-connect-next", "attachments", "source", "video.mp4")
			data, err := os.ReadFile(videoPath)
			if err != nil || string(data) != "full video" || !strings.Contains(text, videoPath) {
				t.Fatalf("video path missing: %q %v", text, err)
			}
			image := input[1].(map[string]any)
			if image["type"] != "localImage" {
				t.Fatalf("image is not native input: %v", image)
			}
			data, err = os.ReadFile(image["path"].(string))
			if err != nil || string(data) != "jpeg" {
				t.Fatalf("image bytes lost: %q %v", data, err)
			}
		})
	}
}
