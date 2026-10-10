package feishu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

type mediaContractSession struct {
	dir      string
	received chan *core.Message
	events   chan core.Event
}

func (s *mediaContractSession) Send(prompt string, images []core.ImageAttachment, files []core.FileAttachment) error {
	paths, err := core.StageFilesToDisk(s.dir, files)
	if err != nil {
		return err
	}
	s.received <- &core.Message{Content: core.AppendFileRefs(prompt, paths), Images: images, Files: files}
	s.events <- core.Event{Type: core.EventResult, Content: "fixture done", Done: true}
	return nil
}
func (*mediaContractSession) RespondPermission(string, core.PermissionResult) error { return nil }
func (s *mediaContractSession) Events() <-chan core.Event                           { return s.events }
func (*mediaContractSession) CurrentSessionID() string                              { return "fixture-session" }
func (*mediaContractSession) Alive() bool                                           { return true }
func (*mediaContractSession) Close() error                                          { return nil }

type mediaContractAgent struct{ s *mediaContractSession }

func (*mediaContractAgent) Name() string { return "fixture" }
func (a *mediaContractAgent) StartSession(context.Context, string) (core.AgentSession, error) {
	return a.s, nil
}
func (*mediaContractAgent) ListSessions(context.Context) ([]core.AgentSessionInfo, error) {
	return nil, nil
}
func (*mediaContractAgent) Stop() error { return nil }

type mediaContractPlatform struct{}

func (*mediaContractPlatform) Name() string                             { return "feishu" }
func (*mediaContractPlatform) Start(core.MessageHandler) error          { return nil }
func (*mediaContractPlatform) Stop() error                              { return nil }
func (*mediaContractPlatform) Send(context.Context, any, string) error  { return nil }
func (*mediaContractPlatform) Reply(context.Context, any, string) error { return nil }

func TestIssue146_PostThroughEngineToStagedAgentInput(t *testing.T) {
	image := []byte("\xff\xd8\xffimage")
	video := []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveResourceTestToken(t, w, r) {
			return
		}
		if strings.HasSuffix(r.URL.Path, "/bad") {
			http.NotFound(w, r)
			return
		}
		payload := video
		if strings.HasSuffix(r.URL.Path, "/img") {
			payload = image
		}
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusPartialContent)
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	s := &mediaContractSession{dir: t.TempDir(), received: make(chan *core.Message, 1), events: make(chan core.Event, 2)}
	sink := &mediaContractPlatform{}
	engine := core.NewEngine("media-contract", &mediaContractAgent{s}, []core.Platform{sink}, filepath.Join(t.TempDir(), "sessions.json"), core.LangEnglish)
	defer func() { _ = engine.Stop() }()
	p := resourceTestPlatform(server)
	p.handler = func(_ core.Platform, m *core.Message) { engine.ReceiveMessage(sink, m) }
	p.dispatchMessage(context.Background(), "post", `{"content":[[{"tag":"text","text":"inspect"},{"tag":"img","image_key":"img"},{"tag":"media","file_key":"video"},{"tag":"img","image_key":"bad"}]]}`, nil, "source", "feishu:fixture:user", "", "", replyContext{}, "", 0)
	select {
	case got := <-s.received:
		if len(got.Images) != 1 || string(got.Images[0].Data) != string(image) || len(got.Files) != 1 || !strings.Contains(got.Content, "unavailable") {
			t.Fatalf("pipeline lost media/failure: %+v", got)
		}
		path := filepath.Join(s.dir, ".cc-connect-next", "attachments", "source", "video.mp4")
		data, err := os.ReadFile(path)
		if err != nil || string(data) != string(video) || !strings.Contains(got.Content, path) {
			t.Fatalf("video not fully accessible: %q %v", data, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("media did not reach agent")
	}
}
