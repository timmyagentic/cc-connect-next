package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

const generatedConfigDocNotice = "<!-- Code generated from the compiled configuration catalog. DO NOT EDIT. -->\n\n"

func generatedConfigCapabilityDocs() map[string]string {
	catalog := config.CapabilityCatalog("source")
	return map[string]string{
		"configuration.md":       generatedConfigDocNotice + core.RenderConfigCatalogMarkdown(catalog, "en"),
		"configuration.zh-CN.md": generatedConfigDocNotice + core.RenderConfigCatalogMarkdown(catalog, "zh"),
	}
}

func runConfigDocs(args []string) error {
	fs := flag.NewFlagSet("config docs", flag.ContinueOnError)
	outputDir := fs.String("output-dir", "", "directory for generated configuration.md files")
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
	return nil
}
