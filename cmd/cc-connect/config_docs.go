package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

const generatedConfigDocNotice = "<!-- Code generated from the compiled configuration catalog. DO NOT EDIT. -->\n\n"
const generatedWebContractNotice = "// Code generated from the compiled configuration catalog. DO NOT EDIT.\n\n"

func generatedConfigCapabilityDocs() map[string]string {
	catalog := config.CapabilityCatalog("source")
	return map[string]string{
		"configuration.md":       generatedConfigDocNotice + core.RenderConfigCatalogMarkdown(catalog, "en"),
		"configuration.zh-CN.md": generatedConfigDocNotice + core.RenderConfigCatalogMarkdown(catalog, "zh"),
	}
}

func generatedWebConfigContract() string {
	catalog := config.CapabilityCatalog("source")
	byPath := make(map[string]core.ConfigOption, len(catalog.Options))
	for _, option := range catalog.Options {
		if option.Owner == "" {
			byPath[option.Path] = option
		}
	}
	fields := []struct {
		name string
		path string
	}{
		{name: "language", path: "language"},
		{name: "attachmentSend", path: "attachment_send"},
		{name: "logLevel", path: "log.level"},
		{name: "idleTimeoutMins", path: "idle_timeout_mins"},
		{name: "thinkingMessages", path: "display.thinking_messages"},
		{name: "thinkingMaxLen", path: "display.thinking_max_len"},
		{name: "toolMessages", path: "display.tool_messages"},
		{name: "toolMaxLen", path: "display.tool_max_len"},
		{name: "streamPreviewEnabled", path: "stream_preview.enabled"},
		{name: "streamPreviewIntervalMs", path: "stream_preview.interval_ms"},
		{name: "rateLimitMaxMessages", path: "rate_limit.max_messages"},
		{name: "rateLimitWindowSecs", path: "rate_limit.window_secs"},
	}
	var b strings.Builder
	b.WriteString(generatedWebContractNotice)
	b.WriteString("export const globalSettingsContract = {\n")
	for _, field := range fields {
		option, ok := byPath[field.path]
		if !ok {
			panic("web config contract path missing: " + field.path)
		}
		fmt.Fprintf(&b, "  %s: { defaultValue: %s", field.name, configContractJavaScriptValue(option))
		if len(option.Values) > 0 {
			encoded, err := json.Marshal(option.Values)
			if err != nil {
				panic(err)
			}
			fmt.Fprintf(&b, ", allowedValues: %s", encoded)
		}
		b.WriteString(" },\n")
	}
	b.WriteString("} as const;\n")
	return b.String()
}

func configContractJavaScriptValue(option core.ConfigOption) string {
	switch option.Type {
	case "boolean":
		value, err := strconv.ParseBool(option.Default)
		if err != nil {
			panic(fmt.Sprintf("web contract %s default %q is not boolean", option.Path, option.Default))
		}
		return strconv.FormatBool(value)
	case "integer":
		value, err := strconv.ParseInt(option.Default, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("web contract %s default %q is not integer", option.Path, option.Default))
		}
		return strconv.FormatInt(value, 10)
	default:
		encoded, err := json.Marshal(option.Default)
		if err != nil {
			panic(err)
		}
		return string(encoded)
	}
}

func runConfigDocs(args []string) error {
	fs := flag.NewFlagSet("config docs", flag.ContinueOnError)
	outputDir := fs.String("output-dir", "", "directory for generated configuration.md files")
	webOutput := fs.String("web-output", "", "optional generated TypeScript contract path for the Web settings UI")
	check := fs.Bool("check", false, "verify generated files without changing them")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *outputDir == "" {
		return fmt.Errorf("--output-dir is required")
	}
	if !*check {
		if err := os.MkdirAll(*outputDir, 0o755); err != nil {
			return err
		}
	}
	for name, content := range generatedConfigCapabilityDocs() {
		path := filepath.Join(*outputDir, name)
		if *check {
			current, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if string(current) != content {
				return fmt.Errorf("%s is stale; regenerate configuration docs", path)
			}
			continue
		}
		if err := core.AtomicWriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if *webOutput != "" {
		content := generatedWebConfigContract()
		if *check {
			current, err := os.ReadFile(*webOutput)
			if err != nil {
				return fmt.Errorf("read %s: %w", *webOutput, err)
			}
			if string(current) != content {
				return fmt.Errorf("%s is stale; regenerate Web config contract", *webOutput)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(*webOutput), 0o755); err != nil {
				return err
			}
			if err := core.AtomicWriteFile(*webOutput, []byte(content), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", *webOutput, err)
			}
		}
	}
	return nil
}
