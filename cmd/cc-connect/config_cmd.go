package main

//go:generate go run . config docs --output-dir ../../docs

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"

	ccconnect "github.com/timmyagentic/cc-connect-next"
	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

func runConfig(args []string) {
	if len(args) == 0 {
		printConfigUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "example":
		fmt.Print(ccconnect.ConfigExampleTOML)
	case "capabilities", "capability", "catalog":
		if err := writeConfigCapabilities(os.Stdout, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(2)
		}
	case "docs":
		if err := runConfigDocs(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(2)
		}
	case "format", "fmt":
		runConfigFormat(args[1:])
	case "path":
		fmt.Println(resolveConfigPath(""))
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", args[0])
		printConfigUsage()
		os.Exit(1)
	}
}

func writeConfigCapabilities(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("config capabilities", flag.ContinueOnError)
	fs.SetOutput(w)
	search := fs.String("search", "", "search paths, descriptions, and natural-language keywords")
	key := fs.String("key", "", "show one exact config path or option key")
	agent := fs.String("agent", "", "comma-separated Agent adapters to include (defaults to CC_AGENT_TYPE)")
	platform := fs.String("platform", "", "comma-separated platform adapters to include (defaults to CC_PLATFORM_TYPES)")
	format := fs.String("format", "markdown", "output format: markdown or json")
	lang := fs.String("lang", "", "Markdown language: en or zh (auto-detected from the query when omitted)")
	all := fs.Bool("all", false, "ignore active-adapter environment filters and include every compiled adapter")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q; use --search or --key", fs.Arg(0))
	}

	catalog := config.CapabilityCatalog(version)
	agents := splitCatalogFilter(*agent)
	platforms := splitCatalogFilter(*platform)
	if !*all {
		if len(agents) == 0 {
			agents = splitCatalogFilter(os.Getenv("CC_AGENT_TYPE"))
		}
		if len(platforms) == 0 {
			platforms = splitCatalogFilter(os.Getenv("CC_PLATFORM_TYPES"))
		}
		if err := validateCatalogFilter("Agent", agents, catalog.Agents); err != nil {
			return err
		}
		if err := validateCatalogFilter("platform", platforms, catalog.Platforms); err != nil {
			return err
		}
	}
	catalog = filterConfigCatalogAdapters(catalog, agents, platforms, *all)

	if exact := strings.TrimSpace(*key); exact != "" {
		catalog = exactConfigCatalogMatch(catalog, exact)
	} else if query := strings.TrimSpace(*search); query != "" {
		catalog = config.SearchCapabilities(catalog, query)
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		return encoder.Encode(catalog)
	case "markdown", "md", "":
		language := normalizedCatalogLanguage(*lang, *search)
		if len(catalog.Capabilities) == 0 && len(catalog.Options) == 0 {
			if language == "zh" {
				_, err := fmt.Fprintf(w, "当前构建没有声明与该查询匹配的配置能力（版本 `%s`）。不要猜测配置项；可以使用 `/feedback <描述>` 提交需求。\n", catalog.Version)
				return err
			} else {
				_, err := fmt.Fprintf(w, "This build `%s` declares no configuration capability matching that query. Do not guess a key; offer `/feedback <description>`.\n", catalog.Version)
				return err
			}
		}
		_, err := io.WriteString(w, core.RenderConfigCatalogMarkdown(catalog, language))
		return err
	default:
		return fmt.Errorf("unsupported format %q (want markdown or json)", *format)
	}
}

func validateCatalogFilter(kind string, selected, available []string) error {
	availableSet := sliceSet(available)
	for _, value := range selected {
		if !availableSet[value] {
			return fmt.Errorf("unknown %s adapter %q; compiled adapters: %s", kind, value, strings.Join(available, ", "))
		}
	}
	return nil
}

func splitCatalogFilter(raw string) []string {
	seen := make(map[string]bool)
	var values []string
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func filterConfigCatalogAdapters(catalog core.ConfigCatalog, agents, platforms []string, all bool) core.ConfigCatalog {
	if all || (len(agents) == 0 && len(platforms) == 0) {
		return catalog
	}
	agentSet := sliceSet(agents)
	platformSet := sliceSet(platforms)
	filtered := catalog
	filtered.Agents = intersectCatalogNames(catalog.Agents, agentSet)
	filtered.Platforms = intersectCatalogNames(catalog.Platforms, platformSet)
	filtered.Options = nil
	for _, option := range catalog.Options {
		switch option.Scope {
		case core.ConfigScopeAgent:
			if option.Owner == "" || agentSet[option.Owner] {
				filtered.Options = append(filtered.Options, option)
			}
		case core.ConfigScopePlatform:
			if option.Owner == "" || platformSet[option.Owner] {
				filtered.Options = append(filtered.Options, option)
			}
		default:
			filtered.Options = append(filtered.Options, option)
		}
	}
	return filtered
}

func sliceSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func intersectCatalogNames(values []string, selected map[string]bool) []string {
	var result []string
	for _, value := range values {
		if selected[value] {
			result = append(result, value)
		}
	}
	return result
}

func exactConfigCatalogMatch(catalog core.ConfigCatalog, key string) core.ConfigCatalog {
	result := core.ConfigCatalog{Version: catalog.Version, Agents: catalog.Agents, Platforms: catalog.Platforms}
	paths := make(map[string]bool)
	for _, option := range catalog.Options {
		if option.Path == key || option.Key == key {
			result.Options = append(result.Options, option)
			paths[option.Path] = true
		}
	}
	for _, capability := range catalog.Capabilities {
		for _, path := range capability.Paths {
			if paths[path] {
				result.Capabilities = append(result.Capabilities, capability)
				break
			}
		}
	}
	return result
}

func normalizedCatalogLanguage(explicit, query string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(explicit)), "zh") {
		return "zh"
	}
	if strings.EqualFold(strings.TrimSpace(explicit), "en") {
		return "en"
	}
	for _, r := range query {
		if unicode.Is(unicode.Han, r) {
			return "zh"
		}
	}
	return "en"
}

func runConfigFormat(args []string) {
	fs := flag.NewFlagSet("config format", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file (default: auto-detect)")
	_ = fs.Parse(args)

	path := resolveConfigPath(*configPath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Config file not found: %s\n", path)
		os.Exit(1)
	}

	if err := config.FormatConfigFile(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Formatted %s\n", path)
}

func printConfigUsage() {
	fmt.Fprintf(os.Stderr, `Usage: cc-connect-next config <subcommand>

Subcommands:
  example    Print a complete annotated config.toml example
  capabilities  Explain supported configuration in Markdown or JSON
  docs       Generate the checked-in bilingual configuration reference
  format     Format the config file (alias: fmt)
  path       Print the resolved config file path

Flags for 'format':
  --config <path>   Path to config file (default: auto-detect)

Examples:
  cc-connect-next config example              Print example config
  cc-connect-next config example > config.toml  Save example config
  cc-connect-next config capabilities --search "hide reasoning"
  cc-connect-next config capabilities --agent codex --platform feishu --format json
  cc-connect-next config docs --output-dir ./docs
  cc-connect-next config format               Format default config file
  cc-connect-next config fmt --config /path/to/config.toml
`)
}
