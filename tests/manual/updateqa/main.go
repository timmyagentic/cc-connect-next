//go:build updateqa && !windows

// updateqa is an opt-in real-client harness, never part of the product binary.
// It runs the production Engine, Feishu adapter and installer against an exact
// loopback release fixture and an isolated executable/npm prefix. No user event
// is synthesized: confirmation must arrive from the real Feishu client.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
	"github.com/timmyagentic/cc-connect-next/core"
	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
	_ "github.com/timmyagentic/cc-connect-next/platform/feishu"
)

var version = "v0.0.0-qa.1"

type qaAgent struct{}

func (*qaAgent) Name() string { return "qa" }
func (*qaAgent) StartSession(context.Context, string) (core.AgentSession, error) {
	return nil, fmt.Errorf("this isolated update QA harness supports /version, /upgrade and /shell, not model requests")
}
func (*qaAgent) ListSessions(context.Context) ([]core.AgentSessionInfo, error) { return nil, nil }
func (*qaAgent) Stop() error                                                   { return nil }

type source struct{ base string }

func (s source) LatestStable(ctx context.Context) (featureupdater.Release, error) {
	return s.release(ctx)
}
func (s source) LatestPrerelease(ctx context.Context) (featureupdater.Release, error) {
	return s.release(ctx)
}
func (s source) release(ctx context.Context) (featureupdater.Release, error) {
	var r featureupdater.Release
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/release.json", nil)
	if err != nil {
		return r, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return r, fmt.Errorf("fixture release HTTP %d", resp.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&r)
	return r, err
}
func (s source) Download(ctx context.Context, a featureupdater.Asset, w io.Writer) error {
	u, err := url.Parse(a.DownloadURL)
	if err != nil || !strings.HasPrefix(a.DownloadURL, s.base+"/") {
		return fmt.Errorf("fixture asset escaped its loopback origin")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("fixture asset HTTP %d", resp.StatusCode)
	}
	_, err = io.Copy(w, io.LimitReader(resp.Body, 256*1024*1024))
	return err
}

type configuration struct {
	DataDir  string `toml:"data_dir"`
	Projects []struct {
		Name      string `toml:"name"`
		AdminFrom string `toml:"admin_from"`
		Platforms []struct {
			Type    string         `toml:"type"`
			Options map[string]any `toml:"options"`
		} `toml:"platforms"`
	} `toml:"projects"`
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("cc-connect-next " + version)
		return
	}
	configPath := flag.String("config", "", "private isolated QA config")
	fixture := flag.String("fixture", "", "loopback release fixture origin")
	notice := flag.Bool("notice", false, "send the real update notice to the configured QA administrator")
	flag.Parse()
	u, err := url.Parse(*fixture)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		panic("QA requires a loopback fixture")
	}
	var cfg configuration
	if _, err = toml.DecodeFile(*configPath, &cfg); err != nil {
		panic("QA config cannot be read")
	}
	if len(cfg.Projects) != 1 || cfg.Projects[0].Name != "update-experience-qa" || cfg.DataDir == "" || !strings.Contains(cfg.DataDir, "cc-update-qa-") {
		panic("QA isolation guard rejected config")
	}
	exe, err := os.Executable()
	if err != nil || !strings.Contains(exe, "cc-update-qa-") {
		panic("QA executable must be in an isolated cc-update-qa directory")
	}
	if cfg.Projects[0].AdminFrom == "" || cfg.Projects[0].AdminFrom == "*" {
		panic("QA requires an explicit administrator")
	}
	core.CurrentVersion = version
	core.VersionInfo = "cc-connect-next " + version
	var platforms []core.Platform
	for _, pc := range cfg.Projects[0].Platforms {
		p, err := core.CreatePlatform(pc.Type, pc.Options)
		if err != nil {
			panic("QA platform configuration failed")
		}
		platforms = append(platforms, p)
	}
	e := core.NewEngine(cfg.Projects[0].Name, &qaAgent{}, platforms, filepath.Join(cfg.DataDir, "sessions.json"), core.LangChinese)
	e.SetAdminFrom(cfg.Projects[0].AdminFrom)
	e.SetDataDir(cfg.DataDir)
	e.SetUpdateChannel("beta")
	service, err := appfeatures.NewUpdateService(appfeatures.UpdateConfig{CurrentVersion: version, Channel: updatechannel.Beta, ExecutablePath: exe, Source: source{*fixture}, Progress: func(event appfeatures.UpdateEvent) {
		slog.Info("QA_INSTALL_STAGE", "stage", event.Stage, "target", event.TargetVersion, "bytes", event.Bytes)
	}})
	if err != nil {
		panic(err)
	}
	e.SetUpdateService(core.NewFixtureUpdateService(service, updatechannel.Beta))
	if err = e.Start(); err != nil {
		panic("QA platform could not start")
	}
	go e.RecoverUpdateResult()
	slog.Info("QA_READY", "version", version, "pid", os.Getpid())
	if *notice && version == "v0.0.0-qa.1" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		r, err := (source{*fixture}).release(ctx)
		cancel()
		if err != nil || !e.NotifyUpdateAvailable(&core.ReleaseInfo{TagName: r.Tag, Body: r.Notes, HTMLURL: r.URL, Channel: "beta", Prerelease: true}) {
			slog.Warn("QA_NOTICE_NOT_DELIVERED")
		}
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-sig:
		_ = e.Stop()
	case req := <-core.RestartCh:
		slog.Info("QA_RESTART", "target_operation", req.UpdateID)
		if err = core.SaveRestartNotify(cfg.DataDir, req); err != nil {
			panic(err)
		}
		if err = e.Stop(); err != nil {
			slog.Warn("QA_STOP", "error", err)
		}
		time.Sleep(250 * time.Millisecond)
		if req.ExecutablePath != exe {
			panic("QA restart target escaped the isolated installation")
		}
		if err = syscall.Exec(req.ExecutablePath, os.Args, os.Environ()); err != nil {
			panic(err)
		}
	}
}
