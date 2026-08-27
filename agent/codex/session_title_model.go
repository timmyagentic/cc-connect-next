package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

type sessionTitleGenerator func(context.Context, string) (string, error)

const sessionTitleGenerationTimeout = 10 * time.Second

var sessionTitleDisabledFeatures = []string{
	"plugins",
	"apps",
	"skill_search",
	"memories",
	"multi_agent",
	"hooks",
	"browser_use",
	"computer_use",
	"in_app_browser",
	"workspace_dependencies",
	"tool_suggest",
	"shell_tool",
	"image_generation",
	"view_image",
	"code_mode_host",
}

func (s *appServerSession) sessionTitleGenerationArgs(cwd string) []string {
	args := []string{
		"exec",
		"--ephemeral",
		"--ignore-user-config",
		"--ignore-rules",
		"--skip-git-repo-check",
		"-C", cwd,
		"-m", strings.TrimSpace(s.sessionTitleModel),
		"-s", "read-only",
	}
	for _, feature := range sessionTitleDisabledFeatures {
		args = append(args, "--disable", feature)
	}
	return append(args, "--color", "never", "--json", "-")
}

func (s *appServerSession) generateSessionTitleWithCodex(parent context.Context, candidate string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, sessionTitleGenerationTimeout)
	defer cancel()

	cwd := os.TempDir()
	cmd := exec.CommandContext(ctx, s.cliBin, s.sessionTitleGenerationArgs(cwd)...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(sessionTitleGenerationPrompt(candidate))
	cmd.Stderr = io.Discard
	cmd.Env = s.sessionTitleGenerationEnv()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("codex title generator stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start codex title generator: %w", err)
	}
	title, parseErr := parseSessionTitleGenerationOutput(stdout)
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return "", fmt.Errorf("codex title generation: %w", ctx.Err())
	}
	if waitErr != nil {
		return "", fmt.Errorf("codex title generator exited: %w", waitErr)
	}
	if parseErr != nil {
		return "", parseErr
	}
	return title, nil
}

func (s *appServerSession) sessionTitleGenerationEnv() []string {
	env := core.MergeEnv(os.Environ(), s.extraEnv)
	if s.codexHome != "" {
		env = core.MergeEnv(env, []string{"CODEX_HOME=" + s.codexHome})
	}
	return env
}

func sessionTitleGenerationPrompt(candidate string) string {
	encoded, _ := json.Marshal(candidate)
	return "Generate a concise conversation title from the JSON string below. " +
		"Use the same language, at most 24 Unicode characters. Do not call tools or add quotes or Markdown. " +
		"Return only the title: " + string(encoded)
}

func parseSessionTitleGenerationOutput(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 16*1024), 256*1024)
	var title string
	for scanner.Scan() {
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			if text := strings.TrimSpace(event.Item.Text); text != "" {
				title = text
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read codex title generator output: %w", err)
	}
	if title == "" {
		return "", fmt.Errorf("codex title generator returned no title")
	}
	return title, nil
}
