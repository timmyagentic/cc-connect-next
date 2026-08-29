package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

type localCapabilityClientFactory func(socketPath string) *http.Client

func runCapabilities(args []string) {
	if code := runCapabilitiesCommand(args, os.Stdout, os.Stderr, newLocalAPIClient); code != 0 {
		os.Exit(code)
	}
}

func runCapabilitiesCommand(args []string, stdout, stderr io.Writer, clientFactory localCapabilityClientFactory) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", "", "project name (defaults to CC_PROJECT or the only configured project)")
	sessionKey := fs.String("session-key", "", "session key (defaults to CC_SESSION_KEY or the only active session)")
	search := fs.String("search", "", "natural-language keywords across configuration, tools, commands, Skills, and runtime adapters")
	format := fs.String("format", "markdown", "output format: markdown or json")
	lang := fs.String("lang", "", "Markdown language: en or zh (auto-detected from search when omitted)")
	all := fs.Bool("all", false, "include configuration and activation entries for every compiled adapter")
	dataDir := fs.String("data-dir", "", "cc-connect-next data directory containing run/api.sock")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printCapabilitiesUsage(stdout)
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "Error: unexpected argument %q; use --search\n", fs.Arg(0))
		return 2
	}
	if strings.TrimSpace(*project) == "" {
		*project = strings.TrimSpace(os.Getenv("CC_PROJECT"))
	}
	if strings.TrimSpace(*sessionKey) == "" {
		*sessionKey = strings.TrimSpace(os.Getenv("CC_SESSION_KEY"))
	}

	query := url.Values{}
	if value := strings.TrimSpace(*project); value != "" {
		query.Set("project", value)
	}
	if value := strings.TrimSpace(*sessionKey); value != "" {
		query.Set("session_key", value)
	}
	if value := strings.TrimSpace(*search); value != "" {
		query.Set("search", value)
	}
	if *all {
		query.Set("all", "true")
	}
	endpoint := "http://unix/capabilities"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: build capability request: %v\n", err)
		return 1
	}
	client := clientFactory(resolveSocketPath(*dataDir))
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: query running cc-connect-next capability manifest: %v\n", err)
		_, _ = fmt.Fprintln(stderr, "The unified manifest requires a running daemon; use `cc-connect-next config capabilities` for the static configuration contract.")
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		_, _ = fmt.Fprintf(stderr, "Error: capability manifest returned HTTP %d: %s\n", resp.StatusCode, strings.TrimSpace(string(body)))
		return 1
	}
	var manifest core.AgentCapabilityManifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&manifest); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: decode capability manifest: %v\n", err)
		return 1
	}
	if strings.TrimSpace(*search) != "" && !core.AgentCapabilityManifestHasMatches(manifest) {
		language := normalizedCatalogLanguage(*lang, *search)
		if language == "zh" {
			_, _ = fmt.Fprintf(stdout, "当前运行态 `%s` 没有声明与该查询匹配的能力。不要猜测命令或配置；可以使用 `/feedback <描述>` 提交需求。\n", manifest.Version)
		} else {
			_, _ = fmt.Fprintf(stdout, "Runtime `%s` declares no capability matching that query. Do not guess a command or config key; offer `/feedback <description>`.\n", manifest.Version)
		}
		return 0
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(manifest); err != nil {
			_, _ = fmt.Fprintf(stderr, "Error: encode capability manifest: %v\n", err)
			return 1
		}
	case "markdown", "md", "":
		_, _ = fmt.Fprintln(stdout, core.RenderAgentCapabilityManifestMarkdown(manifest, normalizedCatalogLanguage(*lang, *search)))
	default:
		_, _ = fmt.Fprintf(stderr, "Error: unsupported format %q (want markdown or json)\n", *format)
		return 2
	}
	return 0
}

func printCapabilitiesUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage: cc-connect-next capabilities [options]

Query the running project's unified, read-only Agent Capability Manifest.
The response describes configuration contracts, Agent CLI tools, chat commands,
Skills, runtime Agent/Platform capabilities, parameters, permissions, write and
external side effects, fallbacks, and truthful availability reasons. It never
prints configured secret values, Skill bodies, or custom Prompt/Exec bodies.

Options:
  --search <text>       Filter every manifest section with natural-language keywords
  --project <name>      Project name (defaults to CC_PROJECT or the only project)
  --session-key <key>   Session context (defaults to CC_SESSION_KEY or the only active session)
  --format <format>     markdown (default) or json
  --lang <lang>         Markdown language: en or zh
  --all                 Include every compiled Agent/Platform adapter
  --data-dir <path>     Data directory containing run/api.sock

Examples:
  cc-connect-next capabilities --search "switch model"
  cc-connect-next capabilities --search "已有话题" --lang zh
  cc-connect-next capabilities --all --search "Slack"
  cc-connect-next capabilities --format json
`)
}
