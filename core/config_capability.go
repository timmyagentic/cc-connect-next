package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

const maxCapabilityBriefOptions = 64

// ConfigScope identifies where an option is configured.
type ConfigScope string

const (
	ConfigScopeGlobal   ConfigScope = "global"
	ConfigScopeProject  ConfigScope = "project"
	ConfigScopeAgent    ConfigScope = "agent"
	ConfigScopePlatform ConfigScope = "platform"
)

// ConfigApplyMode tells an operator when a changed value takes effect.
type ConfigApplyMode string

const (
	ConfigApplyLive       ConfigApplyMode = "live"
	ConfigApplyReload     ConfigApplyMode = "reload"
	ConfigApplyNewSession ConfigApplyMode = "new-session"
	ConfigApplyRestart    ConfigApplyMode = "restart"
)

// ConfigRequirement describes whether an operator must provide an option.
// Conditional requirements are accompanied by RequiredWhen expressions.
type ConfigRequirement string

const (
	ConfigRequirementOptional    ConfigRequirement = "optional"
	ConfigRequirementRequired    ConfigRequirement = "required"
	ConfigRequirementConditional ConfigRequirement = "conditional"
)

// ConfigDefaultSource identifies who chooses an omitted value. This keeps an
// honest distinction between a cc-connect-next builtin, runtime inheritance,
// an adapter/upstream CLI default, and a field with no default at all.
type ConfigDefaultSource string

const (
	ConfigDefaultBuiltin ConfigDefaultSource = "builtin"
	ConfigDefaultUnset   ConfigDefaultSource = "unset"
	ConfigDefaultRuntime ConfigDefaultSource = "runtime"
	ConfigDefaultAdapter ConfigDefaultSource = "adapter"
	ConfigDefaultInherit ConfigDefaultSource = "inherit"
	ConfigDefaultNone    ConfigDefaultSource = "none"
)

// ConfigSource identifies the operator-facing configuration mechanism.
type ConfigSource string

const (
	ConfigSourceTOML        ConfigSource = "toml"
	ConfigSourceEnvironment ConfigSource = "environment"
	ConfigSourceCLI         ConfigSource = "cli"
)

// ConfigPresetValue records an explicit generated or recommended value that
// differs from (or intentionally pins) the omitted runtime default.
type ConfigPresetValue struct {
	Preset        string `json:"preset"`
	Value         string `json:"value"`
	Description   string `json:"description,omitempty"`
	DescriptionZH string `json:"description_zh,omitempty"`
}

// ConfigOption is the machine-readable, Agent-friendly description of one
// supported configuration key. It deliberately describes capability rather
// than the current value, so catalog queries never need to read credentials.
type ConfigOption struct {
	Path             string              `json:"path"`
	Key              string              `json:"key"`
	Scope            ConfigScope         `json:"scope"`
	Owner            string              `json:"owner,omitempty"`
	Source           ConfigSource        `json:"source"`
	Placement        string              `json:"placement"`
	Type             string              `json:"type"`
	Default          string              `json:"default"`
	DefaultSource    ConfigDefaultSource `json:"default_source"`
	Requirement      ConfigRequirement   `json:"requirement"`
	RequiredWhen     []string            `json:"required_when,omitempty"`
	Requires         []string            `json:"requires,omitempty"`
	ConflictsWith    []string            `json:"conflicts_with,omitempty"`
	Minimum          *float64            `json:"minimum,omitempty"`
	Maximum          *float64            `json:"maximum,omitempty"`
	Unit             string              `json:"unit,omitempty"`
	Values           []string            `json:"allowed_values,omitempty"`
	ClosedValues     bool                `json:"closed_values,omitempty"`
	OpenValues       bool                `json:"open_values,omitempty"`
	Description      string              `json:"description"`
	DescriptionZH    string              `json:"description_zh"`
	Keywords         []string            `json:"keywords,omitempty"`
	ApplyMode        ConfigApplyMode     `json:"apply_mode"`
	Sensitive        bool                `json:"sensitive,omitempty"`
	Deprecated       bool                `json:"deprecated,omitempty"`
	Internal         bool                `json:"internal,omitempty"`
	Example          string              `json:"example,omitempty"`
	PresetValues     []ConfigPresetValue `json:"preset_values,omitempty"`
	exampleGenerated bool
}

// ConfigCapability maps natural-language user intent to one or more exact
// config paths. A capability may require several coordinated options.
type ConfigCapability struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	TitleZH       string   `json:"title_zh"`
	Description   string   `json:"description"`
	DescriptionZH string   `json:"description_zh"`
	Keywords      []string `json:"keywords,omitempty"`
	Paths         []string `json:"paths,omitempty"`
}

// ConfigCatalog is the version/build-specific view returned to humans and
// Agents. Agent and platform lists are populated by compiled plugin packages,
// so selective builds cannot advertise adapters they do not contain.
type ConfigCatalog struct {
	Version      string             `json:"version"`
	Capabilities []ConfigCapability `json:"capabilities"`
	Options      []ConfigOption     `json:"options"`
	Agents       []string           `json:"agents,omitempty"`
	Platforms    []string           `json:"platforms,omitempty"`
}

var configOptionRegistry = struct {
	sync.RWMutex
	agents    map[string][]ConfigOption
	platforms map[string][]ConfigOption
}{agents: make(map[string][]ConfigOption), platforms: make(map[string][]ConfigOption)}

// RegisterAgentConfigOptions declares the complete public option surface for
// one compiled Agent adapter.
func RegisterAgentConfigOptions(name string, options []ConfigOption) {
	options = normalizePluginOptions(ConfigScopeAgent, name, cloneConfigOptions(options))
	configOptionRegistry.Lock()
	configOptionRegistry.agents[name] = options
	configOptionRegistry.Unlock()
}

// RegisterPlatformConfigOptions declares the complete public option surface
// for one compiled messaging-platform adapter.
func RegisterPlatformConfigOptions(name string, options []ConfigOption) {
	options = normalizePluginOptions(ConfigScopePlatform, name, cloneConfigOptions(options))
	configOptionRegistry.Lock()
	configOptionRegistry.platforms[name] = options
	configOptionRegistry.Unlock()
}

func AgentConfigOptions(name string) []ConfigOption {
	configOptionRegistry.RLock()
	defer configOptionRegistry.RUnlock()
	return cloneConfigOptions(configOptionRegistry.agents[name])
}

func PlatformConfigOptions(name string) []ConfigOption {
	configOptionRegistry.RLock()
	defer configOptionRegistry.RUnlock()
	return cloneConfigOptions(configOptionRegistry.platforms[name])
}

func RegisteredConfigOptions() (map[string][]ConfigOption, map[string][]ConfigOption) {
	configOptionRegistry.RLock()
	defer configOptionRegistry.RUnlock()
	agents := make(map[string][]ConfigOption, len(configOptionRegistry.agents))
	for name, options := range configOptionRegistry.agents {
		agents[name] = cloneConfigOptions(options)
	}
	platforms := make(map[string][]ConfigOption, len(configOptionRegistry.platforms))
	for name, options := range configOptionRegistry.platforms {
		platforms[name] = cloneConfigOptions(options)
	}
	return agents, platforms
}

func normalizePluginOptions(scope ConfigScope, owner string, options []ConfigOption) []ConfigOption {
	byKey := make(map[string]ConfigOption, len(options))
	for _, option := range options {
		option.Key = strings.TrimSpace(option.Key)
		if option.Key == "" {
			continue
		}
		option.Scope = scope
		option.Owner = owner
		if option.Path == "" {
			if scope == ConfigScopeAgent {
				option.Path = "projects.agent.options." + option.Key
			} else {
				option.Path = "projects.platforms.options." + option.Key
			}
		}
		if option.Type == "" {
			option.Type = "string"
		}
		if option.Default == "" {
			option.Default = "unset"
		}
		if option.ApplyMode == "" {
			option.ApplyMode = ConfigApplyRestart
		}
		// documentedOption runs before owner/path normalization, so recompute the
		// derived placement now that the final adapter context is known.
		option.Placement = ""
		option = FinalizeConfigOption(option)
		byKey[option.Key] = option
	}
	out := make([]ConfigOption, 0, len(byKey))
	for _, option := range byKey {
		out = append(out, option)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// ConfigureOption refines adapter-local defaults and accepted values without
// teaching core which concrete Agent or Platform owns them.
func ConfigureOption(options []ConfigOption, key, defaultValue string, values ...string) []ConfigOption {
	for i := range options {
		if options[i].Key != key {
			continue
		}
		if defaultValue != "" {
			options[i].Default = defaultValue
			options[i].DefaultSource = ""
		}
		options[i].Values = append([]string(nil), values...)
		options[i] = FinalizeConfigOption(options[i])
		break
	}
	return options
}

// RefineConfigOption lets one adapter attach owner-specific contract facts
// while keeping the shared bilingual option vocabulary in core.
func RefineConfigOption(options []ConfigOption, key string, refine func(*ConfigOption)) []ConfigOption {
	for i := range options {
		if options[i].Key != key {
			continue
		}
		previousExample := options[i].Example
		refine(&options[i])
		if options[i].Example != previousExample {
			options[i].exampleGenerated = false
		}
		options[i] = FinalizeConfigOption(options[i])
		break
	}
	return options
}

// RequireConfigOptions marks adapter-local credentials or identifiers that
// have no usable runtime fallback.
func RequireConfigOptions(options []ConfigOption, keys ...string) []ConfigOption {
	for _, key := range keys {
		options = RefineConfigOption(options, key, func(option *ConfigOption) {
			option.Requirement = ConfigRequirementRequired
			option.Default = "none"
			option.DefaultSource = ConfigDefaultNone
		})
	}
	return options
}

// ConfigureConditionalOption records an adapter-local conditional contract.
func ConfigureConditionalOption(options []ConfigOption, key string, requiredWhen ...string) []ConfigOption {
	return RefineConfigOption(options, key, func(option *ConfigOption) {
		option.Requirement = ConfigRequirementConditional
		option.RequiredWhen = append([]string(nil), requiredWhen...)
	})
}

// ConfigureOptionExample pins an adapter-reviewed example instead of the
// generic type-derived fallback.
func ConfigureOptionExample(options []ConfigOption, key, example string) []ConfigOption {
	for i := range options {
		if options[i].Key != key {
			continue
		}
		options[i].Example = example
		options[i].exampleGenerated = false
		options[i] = FinalizeConfigOption(options[i])
		break
	}
	return options
}

// ConfigurePermissionModeOption derives the catalog's mode enum from the
// adapter's own user-facing permission-mode capability. The first mode is the
// adapter default, matching every built-in implementation.
func ConfigurePermissionModeOption(options []ConfigOption, modes []PermissionModeInfo) []ConfigOption {
	if len(modes) == 0 {
		return options
	}
	values := make([]string, 0, len(modes))
	for _, mode := range modes {
		values = append(values, mode.Key)
	}
	return ConfigureOption(options, "mode", modes[0].Key, values...)
}

func cloneConfigOptions(options []ConfigOption) []ConfigOption {
	out := append([]ConfigOption(nil), options...)
	for i := range out {
		out[i].Values = append([]string(nil), out[i].Values...)
		out[i].Keywords = append([]string(nil), out[i].Keywords...)
		out[i].RequiredWhen = append([]string(nil), out[i].RequiredWhen...)
		out[i].Requires = append([]string(nil), out[i].Requires...)
		out[i].ConflictsWith = append([]string(nil), out[i].ConflictsWith...)
		out[i].PresetValues = append([]ConfigPresetValue(nil), out[i].PresetValues...)
		if out[i].Minimum != nil {
			value := *out[i].Minimum
			out[i].Minimum = &value
		}
		if out[i].Maximum != nil {
			value := *out[i].Maximum
			out[i].Maximum = &value
		}
	}
	return out
}

// FinalizeConfigOption fills the contract fields shared by typed and dynamic
// options. Adapter-local schemas should set their precise facts first and let
// registration call this function once more after owner/path normalization.
func FinalizeConfigOption(option ConfigOption) ConfigOption {
	if option.Source == "" {
		option.Source = ConfigSourceTOML
	}
	if option.Placement == "" {
		option.Placement = defaultConfigPlacement(option)
	}
	if option.Requirement == "" {
		option.Requirement = ConfigRequirementOptional
	}
	if option.Type == "" {
		option.Type = "string"
	}
	switch option.Type {
	case "bool":
		option.Type = "boolean"
	case "int":
		option.Type = "integer"
	}

	switch option.Default {
	case "":
		option.Default = "unset"
	case "unset / runtime default":
		option.Default = "unset"
		if option.DefaultSource == "" {
			option.DefaultSource = ConfigDefaultRuntime
		}
	case "unset / adapter default", "adapter default":
		option.Default = "unset"
		if option.DefaultSource == "" {
			option.DefaultSource = ConfigDefaultAdapter
		}
	case "required":
		option.Default = "none"
		option.Requirement = ConfigRequirementRequired
		option.DefaultSource = ConfigDefaultNone
	}
	if option.DefaultSource == "" {
		switch {
		case option.Requirement == ConfigRequirementRequired && (option.Default == "none" || option.Default == "unset"):
			option.DefaultSource = ConfigDefaultNone
		case option.Default == "unset":
			option.DefaultSource = ConfigDefaultUnset
		case strings.HasPrefix(option.Default, "inherit"):
			option.DefaultSource = ConfigDefaultInherit
		default:
			option.DefaultSource = ConfigDefaultBuiltin
		}
	}
	if (option.Example == "" || option.exampleGenerated) && !option.Internal {
		option.Example = defaultConfigOptionExample(option)
		option.exampleGenerated = true
	}
	if option.Unit == "" {
		option.Unit = inferredConfigUnit(option.Key)
	}
	if len(option.Values) > 0 && !option.ClosedValues && !option.OpenValues {
		option.ClosedValues = true
		for _, value := range option.Values {
			if strings.TrimSpace(value) == "" || strings.ContainsAny(value, " ()") {
				option.ClosedValues = false
				break
			}
		}
	}
	return option
}

func defaultConfigPlacement(option ConfigOption) string {
	if option.Source == ConfigSourceEnvironment {
		return "process environment"
	}
	if option.Source == ConfigSourceCLI {
		return "command line"
	}
	if option.Owner != "" {
		switch option.Scope {
		case ConfigScopeAgent:
			return "[projects.agent.options] (inside one [[projects]])"
		case ConfigScopePlatform:
			return "[projects.platforms.options] (inside one [[projects.platforms]])"
		}
	}
	path := option.Path
	switch {
	case strings.HasPrefix(path, "aliases."):
		return "[[aliases]]"
	case strings.HasPrefix(path, "commands."):
		return "[[commands]]"
	case strings.HasPrefix(path, "hooks."):
		return "[[hooks]]"
	case strings.HasPrefix(path, "providers.models."):
		return "[[providers.models]] (inside one [[providers]])"
	case strings.HasPrefix(path, "providers.agent_model_lists.<name>."):
		return "[[providers.agent_model_lists.<name>]] (inside one [[providers]])"
	case strings.HasPrefix(path, "providers."):
		return "[[providers]]"
	case strings.HasPrefix(path, "projects.agent.providers.models."):
		return "[[projects.agent.providers.models]] (inside one [[projects.agent.providers]])"
	case strings.HasPrefix(path, "projects.agent.providers.agent_model_lists.<name>."):
		return "[[projects.agent.providers.agent_model_lists.<name>]] (inside one [[projects.agent.providers]])"
	case strings.HasPrefix(path, "projects.agent.providers."):
		return "[[projects.agent.providers]] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.agent.answer_profiles.fast."):
		return "[projects.agent.answer_profiles.fast] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.agent.answer_profiles.quality."):
		return "[projects.agent.answer_profiles.quality] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.agent."):
		return "[projects.agent] (inside one [[projects]])"
	case path == "projects.platforms.type":
		return "[[projects.platforms]] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.users.roles.<name>.rate_limit."):
		return "[projects.users.roles.<name>.rate_limit] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.users.roles.<name>."):
		return "[projects.users.roles.<name>] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.users."):
		return "[projects.users] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.display."):
		return "[projects.display] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.references."):
		return "[projects.references] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.heartbeat."):
		return "[projects.heartbeat] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects.auto_compress."):
		return "[projects.auto_compress] (inside one [[projects]])"
	case strings.HasPrefix(path, "projects."):
		return "[[projects]]"
	case strings.Contains(path, "."):
		return "[" + path[:strings.LastIndex(path, ".")] + "]"
	default:
		return "config.toml root"
	}
}

func defaultConfigOptionExample(option ConfigOption) string {
	key := option.Key
	if key == "" || key == "<name>" {
		key = "example"
	}
	typeName := strings.TrimSpace(strings.Split(option.Type, "|")[0])
	value := ""
	for _, candidate := range option.Values {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && !strings.Contains(candidate, " ") && !strings.Contains(candidate, "(") {
			value = candidate
			break
		}
	}
	switch typeName {
	case "boolean":
		if option.Default == "true" || option.Default == "false" {
			return key + " = " + option.Default
		}
		return key + " = true"
	case "integer":
		if _, err := strconv.ParseInt(option.Default, 10, 64); err == nil {
			return key + " = " + option.Default
		}
		return key + " = 1"
	case "number":
		return key + " = 1.0"
	case "string[]":
		if value == "" {
			value = "value"
		}
		return fmt.Sprintf(`%s = [%q]`, key, value)
	case "table":
		return key + ` = { example = "value" }`
	default:
		if option.Sensitive {
			value = "${" + strings.ToUpper(strings.ReplaceAll(key, "-", "_")) + "}"
		} else if value == "" {
			if option.DefaultSource == ConfigDefaultBuiltin && option.Default != "unset" && option.Default != "none" && len(option.Default) <= 80 && !strings.ContainsAny(option.Default, " \t;()") {
				value = option.Default
			} else {
				value = "value"
			}
		}
		return fmt.Sprintf(`%s = %q`, key, value)
	}
}

func inferredConfigUnit(key string) string {
	switch {
	case strings.HasSuffix(key, "_ms"):
		return "milliseconds"
	case strings.HasSuffix(key, "_secs") || strings.HasSuffix(key, "_seconds"):
		return "seconds"
	case strings.HasSuffix(key, "_mins") || strings.HasSuffix(key, "_minutes"):
		return "minutes"
	case strings.HasSuffix(key, "_mb"):
		return "MiB"
	default:
		return ""
	}
}

// DescribeAgentOptions attaches stable bilingual semantics to an adapter's
// exact key list. Adapter-specific schemas stay beside their implementation;
// shared concepts such as work_dir and model are described only once here.
func DescribeAgentOptions(keys []string) []ConfigOption {
	options := make([]ConfigOption, 0, len(keys))
	for _, key := range keys {
		options = append(options, agentOption(key))
	}
	return options
}

// DescribePlatformOptions is the platform counterpart to
// DescribeAgentOptions.
func DescribePlatformOptions(keys []string) []ConfigOption {
	options := make([]ConfigOption, 0, len(keys))
	for _, key := range keys {
		options = append(options, platformOption(key))
	}
	return options
}

type optionDoc struct {
	typeName, defaultValue string
	values                 []string
	openValues             bool
	description, zh        string
	keywords               []string
	apply                  ConfigApplyMode
	defaultSource          ConfigDefaultSource
	requirement            ConfigRequirement
	requiredWhen           []string
	requires               []string
	conflictsWith          []string
	minimum, maximum       *float64
	unit                   string
	sensitive, internal    bool
	example                string
	presetValues           []ConfigPresetValue
}

func configFloat64(value float64) *float64 { return &value }

func documentedOption(key string, doc optionDoc) ConfigOption {
	if doc.typeName == "" {
		doc.typeName = "string"
	}
	switch doc.typeName {
	case "bool":
		doc.typeName = "boolean"
	case "int":
		doc.typeName = "integer"
	}
	if doc.defaultValue == "" {
		if doc.sensitive {
			doc.defaultValue = "unset"
		} else {
			doc.defaultValue = "unset / adapter default"
		}
	}
	if doc.apply == "" {
		doc.apply = ConfigApplyRestart
	}
	if doc.description == "" {
		doc.description = fmt.Sprintf("Configure the %s adapter option.", strings.ReplaceAll(key, "_", " "))
	}
	if doc.zh == "" {
		doc.zh = fmt.Sprintf("配置适配器的 %s 选项。", key)
	}
	return FinalizeConfigOption(ConfigOption{
		Key: key, Type: doc.typeName, Default: doc.defaultValue, Values: append([]string(nil), doc.values...), OpenValues: doc.openValues,
		Description: doc.description, DescriptionZH: doc.zh, Keywords: append([]string(nil), doc.keywords...),
		ApplyMode: doc.apply, Sensitive: doc.sensitive, Internal: doc.internal, Example: doc.example,
		DefaultSource: doc.defaultSource, Requirement: doc.requirement,
		RequiredWhen: append([]string(nil), doc.requiredWhen...), Requires: append([]string(nil), doc.requires...),
		ConflictsWith: append([]string(nil), doc.conflictsWith...), Minimum: doc.minimum, Maximum: doc.maximum,
		Unit: doc.unit, PresetValues: append([]ConfigPresetValue(nil), doc.presetValues...),
	})
}

func agentOption(key string) ConfigOption {
	docs := map[string]optionDoc{
		"agent":                {description: "Select the named sub-agent or profile exposed by the CLI.", zh: "选择 CLI 暴露的子 Agent 或配置档。", keywords: []string{"subagent", "子agent"}},
		"allowed_tools":        {typeName: "string[]", description: "Pre-approve selected Claude Code tools in approval-based modes.", zh: "在需要审批的模式中预先允许指定 Claude Code 工具。", keywords: []string{"preapprove", "预授权", "工具权限"}},
		"append_system_prompt": {description: "Append project instructions while preserving the Agent's default system prompt.", zh: "保留 Agent 默认系统提示并追加项目指令。", keywords: []string{"追加提示词", "instructions"}},
		"app_server_url":       {defaultValue: "stdio", description: "Choose the Codex app-server transport endpoint.", zh: "选择 Codex app-server 的传输端点。", keywords: []string{"app server", "stdio"}},
		"args":                 {typeName: "string[]", description: "Pass additional arguments to the configured Agent command.", zh: "向配置的 Agent 命令传递额外参数。", keywords: []string{"arguments", "启动参数"}},
		"auth_method":          {description: "Select the authentication method used by an ACP Agent.", zh: "选择 ACP Agent 使用的认证方式。", keywords: []string{"authentication", "认证"}},
		"auto_create":          {typeName: "bool", defaultValue: "true", description: "Create the configured tmux session when it does not exist.", zh: "配置的 tmux 会话不存在时自动创建。"},
		"backend":              {defaultValue: "app_server", values: []string{"app_server", "exec"}, description: "Select the Codex execution backend; app_server supports native steering and approvals.", zh: "选择 Codex 执行后端；app_server 支持原生 steer 与审批。", keywords: []string{"steer", "执行后端"}},
		"cc_data_dir":          {description: "Internal cc-connect-next data directory injected by the host.", zh: "由宿主注入的 cc-connect-next 内部数据目录。", internal: true},
		"cc_project":           {description: "Internal cc-connect-next project name injected by the host.", zh: "由宿主注入的 cc-connect-next 内部项目名。", internal: true},
		"cli_args_flag":        {description: "Name the wrapper flag that accepts Agent CLI arguments.", zh: "指定包装器接收 Agent CLI 参数的标志名。"},
		"cli_path":             {defaultValue: "adapter default", description: "Override the Agent CLI executable path.", zh: "覆盖 Agent CLI 可执行文件路径。", keywords: []string{"binary", "二进制路径"}},
		"cmd":                  {defaultValue: "adapter default", description: "Override the Agent command, optionally including global arguments.", zh: "覆盖 Agent 命令，可同时包含全局参数。", keywords: []string{"command", "命令"}},
		"cmd_args_flag":        {description: "Name the wrapper flag used to forward command arguments.", zh: "指定包装器转发命令参数所使用的标志名。"},
		"codex_home":           {description: "Override CODEX_HOME for this project without changing the user's global Codex home.", zh: "为当前项目覆盖 CODEX_HOME，不修改用户全局 Codex 目录。", keywords: []string{"CODEX_HOME"}},
		"command":              {defaultValue: "adapter default", description: "Set the Agent executable; an alias used by several adapters.", zh: "设置 Agent 可执行命令；多个适配器使用的别名。"},
		"disallowed_tools":     {typeName: "string[]", description: "Deny selected Claude Code tools even when the mode would otherwise allow them.", zh: "即使当前模式允许，也禁止指定 Claude Code 工具。", keywords: []string{"deny tools", "禁用工具"}},
		"display_name":         {description: "Set the user-facing name of a generic or ACP Agent.", zh: "设置通用或 ACP Agent 的用户可见名称。"},
		"env":                  {typeName: "table", description: "Inject project-scoped environment variables into Agent processes.", zh: "向 Agent 进程注入项目级环境变量。", sensitive: true, keywords: []string{"环境变量", "provider env"}},
		"init_command":         {description: "Run a shell initialization command before tmux prompts are sent.", zh: "发送 tmux 提示前运行初始化命令。"},
		"max_context_tokens":   {typeName: "integer", minimum: configFloat64(1), description: "Override the maximum context-token budget accepted by Claude Code.", zh: "覆盖 Claude Code 可使用的最大上下文 token 数。", keywords: []string{"context window", "上下文窗口"}},
		"mode":                 {defaultValue: "adapter default", description: "Choose the Agent approval, sandbox, or planning mode.", zh: "选择 Agent 的审批、沙箱或规划模式。", keywords: []string{"yolo", "权限模式", "plan"}},
		"model":                {description: "Select the default model for new Agent sessions.", zh: "选择新 Agent 会话的默认模型。", keywords: []string{"模型"}, example: `model = "provider/model-name"`},
		"model_context_window": {typeName: "integer", minimum: configFloat64(1), description: "Declare the Codex model context window used for usage reporting and compaction decisions.", zh: "声明 Codex 模型上下文窗口，用于用量展示和压缩决策。", keywords: []string{"context", "上下文窗口"}},
		"pane":                 {defaultValue: "0", description: "Select the tmux pane used for Agent input and output.", zh: "选择用于 Agent 输入输出的 tmux pane。"},
		"plugin_dir":           {typeName: "string | string[]", description: "Load one or more Claude Code plugin directories.", zh: "加载一个或多个 Claude Code 插件目录。", keywords: []string{"plugins", "插件"}},
		"poll_interval_ms":     {typeName: "integer", defaultValue: "200", minimum: configFloat64(1), unit: "milliseconds", description: "Set the tmux output polling interval in milliseconds.", zh: "设置 tmux 输出轮询间隔（毫秒）。"},
		"prompt_pattern":       {description: "Regular expression used to recognize the tmux Agent prompt.", zh: "用于识别 tmux Agent 提示符的正则表达式。"},
		"provider":             {description: "Select the active configured provider for this project.", zh: "选择当前项目使用的已配置 Provider。", apply: ConfigApplyReload, keywords: []string{"服务商", "provider switch"}, example: `provider = "provider-name"`},
		"reasoning_effort":     {description: "Set the default reasoning-effort level for new turns.", zh: "设置新回合默认推理强度。", keywords: []string{"effort", "推理强度", "max"}},
		"router_api_key":       {description: "Authenticate to the configured Claude Code Router.", zh: "用于认证已配置的 Claude Code Router。", sensitive: true},
		"router_url":           {description: "Route Claude Code requests through the specified router URL.", zh: "将 Claude Code 请求路由到指定 Router 地址。"},
		"run_as_env":           {typeName: "string[]", description: "Extend the environment allowlist passed across OS-user isolation.", zh: "扩展跨 OS 用户隔离传递的环境变量白名单。"},
		"run_as_user":          {description: "Run Claude Code under another non-root OS user.", zh: "以另一个非 root 操作系统用户运行 Claude Code。", keywords: []string{"用户隔离", "sudo"}},
		"service_tier":         {description: "Select the model-catalog service tier; common Codex values include default and fast.", zh: "选择模型目录声明的服务档位；Codex 常见取值包括 default 和 fast。", values: []string{"model-catalog-driven (for example: default, fast)"}, keywords: []string{"fast", "服务档位"}},
		"session":              {requirement: ConfigRequirementRequired, description: "Name the tmux session that hosts the Agent.", zh: "指定承载 Agent 的 tmux 会话名。"},
		"session_title_model":  {description: "Optionally use an isolated local Codex model to generate concise Codex App titles.", zh: "可选使用隔离的本地 Codex 模型生成简洁的 Codex App 标题。", keywords: []string{"标题模型", "title"}},
		"session_title_prefix": {defaultValue: "[飞书]", description: "Prefix Codex App session titles with a configurable source label.", zh: "为 Codex App 会话标题添加可配置的来源前缀。", keywords: []string{"标题前缀", "source label"}},
		"shell":                {description: "Select the shell used by the tmux adapter.", zh: "选择 tmux 适配器使用的 shell。"},
		"startup_wait_ms":      {typeName: "integer", defaultValue: "0 (or 2000 when init_command is set)", minimum: configFloat64(0), unit: "milliseconds", description: "Wait this many milliseconds after creating a tmux session.", zh: "创建 tmux 会话后等待指定毫秒数。"},
		"strip_input_block":    {typeName: "bool", defaultValue: "true", description: "Remove the echoed input block from captured tmux output.", zh: "从捕获的 tmux 输出中移除回显输入块。"},
		"strip_patterns":       {typeName: "string[]", description: "Remove output lines matching the listed patterns.", zh: "移除匹配指定模式的输出行。"},
		"system_prompt":        {description: "Replace the Agent's default system prompt for this project.", zh: "替换当前项目中 Agent 的默认系统提示。", keywords: []string{"系统提示词"}},
		"thinking":             {description: "Configure the pi Agent's thinking mode or level.", zh: "配置 pi Agent 的思考模式或级别。"},
		"timeout_mins":         {typeName: "integer", minimum: configFloat64(0), unit: "minutes", description: "Set the adapter process timeout in minutes; zero uses its default.", zh: "设置适配器进程超时分钟数；0 使用默认值。"},
		"tool_timeout_secs":    {typeName: "integer", minimum: configFloat64(0), unit: "seconds", description: "Set the maximum wait for an iFlow tool call in seconds.", zh: "设置 iFlow 工具调用最大等待秒数。"},
		"window_per_session":   {typeName: "bool", defaultValue: "false", description: "Use a separate tmux window for every cc-connect-next session.", zh: "为每个 cc-connect-next 会话使用独立 tmux window。"},
		"work_dir":             {defaultValue: ".", conflictsWith: []string{"projects.mode = multi-workspace"}, description: "Set the project working directory used by the Agent.", zh: "设置 Agent 使用的项目工作目录。", keywords: []string{"cwd", "工作目录"}, example: `work_dir = "/absolute/path/to/project"`},
	}
	return documentedOption(key, docs[key])
}

func platformOption(key string) ConfigOption {
	docs := map[string]optionDoc{
		"access_token":                    {description: "Authenticate the Matrix bot account.", zh: "认证 Matrix 机器人账号。", sensitive: true},
		"account_id":                      {defaultValue: "default", description: "Separate persistent Weixin state for multiple accounts.", zh: "为多个微信账号隔离持久化状态。"},
		"agent_id":                        {description: "Identify the bot application Agent in the platform tenant.", zh: "指定平台租户中的机器人应用 Agent ID。", keywords: []string{"钉钉工作通知 Agent ID"}},
		"allow_chat":                      {description: "Restrict Feishu access to selected chat IDs.", zh: "将飞书访问限制在指定会话 ID。", keywords: []string{"群白名单", "chat allowlist"}, example: `allow_chat = "oc_chat_id"`},
		"allow_from":                      {defaultValue: "empty", description: "Restrict bot access to selected platform user IDs; empty or '*' allows every platform user.", zh: "将机器人访问限制在指定平台用户 ID；留空或 '*' 表示允许所有平台用户。", keywords: []string{"只允许我", "飞书只允许我使用", "用户白名单", "access control"}, example: `allow_from = "user-id-1,user-id-2"`},
		"api_base":                        {description: "Override the platform REST API base URL.", zh: "覆盖平台 REST API 基础地址。"},
		"api_base_url":                    {description: "Override the platform API base URL.", zh: "覆盖平台 API 基础地址。"},
		"app_id":                          {description: "Identify the bot application.", zh: "标识机器人应用。"},
		"app_secret":                      {description: "Authenticate the bot application.", zh: "认证机器人应用。", sensitive: true},
		"app_token":                       {description: "Authenticate Slack Socket Mode.", zh: "认证 Slack Socket Mode。", sensitive: true},
		"auto_join":                       {typeName: "bool", defaultValue: "true", description: "Automatically join invited Matrix rooms.", zh: "自动加入受邀的 Matrix 房间。"},
		"auto_verify":                     {typeName: "bool", defaultValue: "true", description: "Automatically accept Matrix SAS device verification.", zh: "自动接受 Matrix SAS 设备验证。"},
		"base_url":                        {description: "Override the platform service base URL.", zh: "覆盖平台服务基础地址。"},
		"bot_id":                          {description: "Identify a WeCom WebSocket bot.", zh: "标识企业微信 WebSocket 机器人。", keywords: []string{"企业微信 websocket 模式需要什么凭证"}},
		"bot_secret":                      {description: "Authenticate a WeCom WebSocket bot.", zh: "认证企业微信 WebSocket 机器人。", sensitive: true},
		"bot_token":                       {description: "Authenticate the Slack bot user.", zh: "认证 Slack 机器人用户。", sensitive: true},
		"burst_limit":                     {typeName: "integer", defaultValue: "4", description: "Limit separate Weixin outbound messages within one burst window.", zh: "限制一个窗口内独立发送的微信消息数量。", keywords: []string{"微信发送次数限制"}},
		"burst_window_secs":               {typeName: "integer", defaultValue: "86400", description: "Set the Weixin outbound burst window length in seconds.", zh: "设置微信出站突发窗口长度（秒）。"},
		"callback_aes_key":                {description: "Decrypt encrypted WeCom callback payloads.", zh: "解密企业微信回调负载。", sensitive: true},
		"callback_path":                   {description: "Set the inbound webhook callback path.", zh: "设置入站 Webhook 回调路径。"},
		"callback_token":                  {description: "Verify WeCom callback requests.", zh: "验证企业微信回调请求。", sensitive: true},
		"card_template_id":                {description: "Select the DingTalk interactive-card template ID.", zh: "选择钉钉互动卡片模板 ID。"},
		"card_template_key":               {description: "Select the DingTalk card-template key.", zh: "选择钉钉卡片模板 Key。"},
		"card_throttle_ms":                {typeName: "integer", minimum: configFloat64(1), unit: "milliseconds", description: "Throttle DingTalk card updates in milliseconds.", zh: "限制钉钉卡片更新频率（毫秒）。"},
		"cc_data_dir":                     {description: "Internal cc-connect-next data directory injected by the host.", zh: "由宿主注入的 cc-connect-next 内部数据目录。", internal: true},
		"cc_project":                      {description: "Internal cc-connect-next project name injected by the host.", zh: "由宿主注入的 cc-connect-next 内部项目名。", internal: true},
		"cdn_base_url":                    {description: "Override the Weixin CDN download/upload base URL.", zh: "覆盖微信 CDN 下载/上传基础地址。"},
		"channel_secret":                  {description: "Verify LINE webhook signatures.", zh: "验证 LINE Webhook 签名。", sensitive: true},
		"channel_token":                   {description: "Authenticate LINE Messaging API requests.", zh: "认证 LINE Messaging API 请求。", sensitive: true},
		"clean_reply":                     {typeName: "bool", defaultValue: "false", description: "Strip thinking and tool-progress lines from WPS replies.", zh: "从 WPS 回复中移除思考和工具进度行。"},
		"client_id":                       {description: "Identify the DingTalk application client.", zh: "标识钉钉应用客户端。"},
		"client_secret":                   {description: "Authenticate the DingTalk application client.", zh: "认证钉钉应用客户端。", sensitive: true},
		"corp_id":                         {description: "Identify the WeCom enterprise.", zh: "标识企业微信企业。"},
		"corp_secret":                     {description: "Authenticate the WeCom application.", zh: "认证企业微信应用。", sensitive: true},
		"cross_signing_password":          {description: "Initialize Matrix cross-signing when the server requires the account password.", zh: "服务器需要账号密码时初始化 Matrix 跨签名。", sensitive: true},
		"domain":                          {description: "Override the Feishu/Lark API and WebSocket domain.", zh: "覆盖飞书/Lark API 与 WebSocket 域名。"},
		"done_emoji":                      {defaultValue: "Done", description: "Choose the completion reaction; 'none' disables it.", zh: "选择完成表情；'none' 表示关闭。"},
		"enable_feishu_card":              {typeName: "bool", defaultValue: "true", description: "Enable Feishu interactive-card replies.", zh: "启用飞书互动卡片回复。"},
		"enable_markdown":                 {typeName: "bool", description: "Enable Markdown formatting for WeCom replies.", zh: "启用企业微信回复的 Markdown 格式。"},
		"enable_reactions":                {typeName: "bool", defaultValue: "false", description: "Add a processing reaction to incoming messages.", zh: "为收到的消息添加处理中表情。"},
		"encrypt_key":                     {description: "Decrypt encrypted Feishu webhook events.", zh: "解密飞书 Webhook 事件。", sensitive: true},
		"group_only":                      {typeName: "bool", defaultValue: "false", description: "Accept Feishu messages only from group chats.", zh: "仅接受飞书群聊消息。"},
		"group_reply_all":                 {typeName: "bool", defaultValue: "false", description: "Reply to every group message without requiring a mention.", zh: "无需 @ 即回复所有群消息。", keywords: []string{"无需@", "群聊全部回复", "群里不 @ 也回复"}},
		"group_reply_all_chats":           {typeName: "string[]", description: "Enable mention-free replies only in selected Feishu chats.", zh: "仅在指定飞书会话中启用无需 @ 的回复。"},
		"group_reply_all_guilds":          {description: "Enable mention-free replies for a comma-separated list of Discord guild IDs.", zh: "为逗号分隔的 Discord 服务器 ID 列表启用无需 @ 的回复。"},
		"guild_id":                        {description: "Limit Discord command registration to one guild for faster propagation.", zh: "将 Discord 命令注册限制到单个服务器以加快生效。"},
		"homeserver":                      {description: "Set the Matrix homeserver URL.", zh: "设置 Matrix Homeserver 地址。"},
		"http_url":                        {description: "Set the NapCat/QQ HTTP API endpoint.", zh: "设置 NapCat/QQ HTTP API 地址。"},
		"image_batch_window_ms":           {typeName: "integer", description: "Batch Feishu images arriving close together into one turn.", zh: "将在短时间内连续到达的飞书图片合并为一个回合。"},
		"intents":                         {typeName: "integer", minimum: configFloat64(1), description: "Set the QQ Bot gateway intent bitmask.", zh: "设置 QQ Bot Gateway Intent 位掩码。"},
		"long_poll_timeout_ms":            {typeName: "integer", defaultValue: "35000", minimum: configFloat64(0), unit: "milliseconds", description: "Set the Weixin long-poll timeout in milliseconds.", zh: "设置微信长轮询超时（毫秒）。"},
		"markdown_support":                {typeName: "bool", description: "Enable QQ Bot Markdown message support.", zh: "启用 QQ Bot Markdown 消息。"},
		"mention_map":                     {typeName: "table", description: "Map Feishu mention identities to replacement text or Agent handles.", zh: "将飞书 @ 身份映射为替换文本或 Agent 标识。"},
		"mode":                            {description: "Select the platform connection mode, such as WebSocket or callback.", zh: "选择平台连接模式，例如 WebSocket 或回调。"},
		"name":                            {description: "Set the account display name used by the platform adapter.", zh: "设置平台适配器使用的账号显示名称。"},
		"peer_bots":                       {typeName: "string[]", description: "Recognize selected Feishu bot identities as relay peers.", zh: "将指定飞书机器人身份识别为 Relay 对端。"},
		"port":                            {typeName: "string", defaultValue: "adapter default", description: "Set the inbound webhook listening port as a quoted string.", zh: "以带引号的字符串设置入站 Webhook 监听端口。"},
		"progress_style":                  {defaultValue: "compact", values: []string{"legacy", "compact", "card"}, description: "Choose how progress is rendered on the messaging platform.", zh: "选择消息平台上的进度展示样式。", keywords: []string{"进度卡片", "card"}},
		"proxy":                           {description: "Route platform HTTP/WebSocket traffic through an HTTP or SOCKS5 proxy.", zh: "通过 HTTP 或 SOCKS5 代理转发平台 HTTP/WebSocket 流量。"},
		"proxy_password":                  {description: "Authenticate to the configured platform proxy.", zh: "认证已配置的平台代理。", sensitive: true, requires: []string{"proxy"}},
		"proxy_username":                  {description: "Set the username for platform proxy authentication.", zh: "设置平台代理认证用户名。", requires: []string{"proxy"}},
		"reaction_emoji":                  {description: "Choose the processing reaction emoji.", zh: "选择处理中表情。", keywords: []string{"飞书机器人不要发表情"}},
		"reply_to_trigger":                {typeName: "bool", description: "Reply in Feishu using the triggering message as the reply target.", zh: "在飞书中回复到触发消息。", keywords: []string{"飞书回复不要引用原消息"}},
		"require_mention":                 {typeName: "bool", description: "Require an explicit mention before replying in group chats.", zh: "群聊中必须明确 @ 机器人才回复。"},
		"resolve_mentions":                {typeName: "bool", description: "Resolve Feishu mentions to readable names before sending text to the Agent.", zh: "发送给 Agent 前将飞书 @ 解析为可读名称。"},
		"respond_to_at_everyone_and_here": {typeName: "bool", defaultValue: "false", description: "Treat @everyone/@here as a valid bot mention.", zh: "将 @everyone/@here 视为有效的机器人提及。"},
		"robot_code":                      {description: "Identify the DingTalk robot used for outbound messages.", zh: "标识用于出站消息的钉钉机器人。"},
		"route_tag":                       {description: "Set the optional Weixin SKRouteTag request header.", zh: "设置可选的微信 SKRouteTag 请求头。"},
		"sandbox":                         {typeName: "bool", description: "Use the QQ Bot sandbox environment.", zh: "使用 QQ Bot 沙箱环境。"},
		"session_scope":                   {defaultValue: "user", defaultSource: ConfigDefaultRuntime, values: []string{"user", "channel", "thread"}, description: "Choose whether Slack sessions are isolated by user, channel, or thread. When omitted, legacy share_session_in_channel=true changes the effective default to channel.", zh: "选择 Slack 会话按用户、频道还是线程隔离。省略时，旧 share_session_in_channel=true 会把有效默认值改为 channel。"},
		"share_session_in_channel":        {typeName: "bool", defaultValue: "false", description: "Share one Agent session among all users in a channel or room.", zh: "让频道或房间内所有用户共享同一个 Agent 会话。", keywords: []string{"共享会话"}},
		"state_dir":                       {description: "Override the directory used for persistent platform state.", zh: "覆盖平台持久化状态目录。"},
		"thread_isolation":                {typeName: "bool", defaultValue: "false", description: "Use a separate Agent session for each platform thread or topic.", zh: "为每个话题或线程使用独立 Agent 会话。"},
		"token":                           {description: "Authenticate the platform bot or gateway.", zh: "认证平台机器人或网关。", sensitive: true},
		"token_endpoint":                  {description: "Override the endpoint used to obtain Weibo access tokens.", zh: "覆盖获取微博访问令牌的端点。"},
		"user_id":                         {description: "Set or override the Matrix bot user ID.", zh: "设置或覆盖 Matrix 机器人用户 ID。"},
		"webhook_listen":                  {description: "Set the local listen address for MAX webhook delivery.", zh: "设置 MAX Webhook 的本地监听地址。"},
		"webhook_path":                    {description: "Set the MAX webhook URL path.", zh: "设置 MAX Webhook URL 路径。"},
		"webhook_resubscribe_interval":    {typeName: "string", defaultValue: "5m", values: []string{"Go duration string (for example: 30s, 5m, 1h)"}, description: "Periodically refresh the MAX webhook subscription using a Go duration string.", zh: "使用 Go duration 字符串定期刷新 MAX Webhook 订阅。", example: `webhook_resubscribe_interval = "5m"`},
		"webhook_secret":                  {description: "Verify MAX webhook requests.", zh: "验证 MAX Webhook 请求。", sensitive: true},
		"webhook_url":                     {description: "Publish the externally reachable MAX webhook URL.", zh: "设置外部可访问的 MAX Webhook 地址。"},
		"ws_endpoint":                     {description: "Override the platform WebSocket endpoint.", zh: "覆盖平台 WebSocket 地址。"},
		"ws_url":                          {description: "Set the NapCat/QQ forward WebSocket endpoint.", zh: "设置 NapCat/QQ 正向 WebSocket 地址。"},
	}
	return documentedOption(key, docs[key])
}

// SearchConfigCatalog performs deterministic bilingual substring matching.
// It intentionally leaves semantic interpretation to the Agent while making
// common natural-language phrases such as “怎么隐藏思考” discover exact paths.
func SearchConfigCatalog(catalog ConfigCatalog, query string) ConfigCatalog {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return catalog
	}
	result := ConfigCatalog{Version: catalog.Version, Agents: append([]string(nil), catalog.Agents...), Platforms: append([]string(nil), catalog.Platforms...)}
	type scoredCapability struct {
		value ConfigCapability
		score int
	}
	type scoredOption struct {
		value ConfigOption
		score int
	}
	paths := make(map[string]int)
	var matchedCapabilities []scoredCapability
	for _, capability := range catalog.Capabilities {
		values := []string{capability.ID, capability.Title, capability.TitleZH, capability.Description, capability.DescriptionZH}
		values = append(values, capability.Keywords...)
		if score := catalogTextMatchScore(query, values...); score > 0 {
			matchedCapabilities = append(matchedCapabilities, scoredCapability{value: capability, score: score})
			for _, path := range capability.Paths {
				paths[path] = max(paths[path], score/10+1)
			}
		}
	}
	sort.SliceStable(matchedCapabilities, func(i, j int) bool { return matchedCapabilities[i].score > matchedCapabilities[j].score })
	for _, matched := range matchedCapabilities {
		result.Capabilities = append(result.Capabilities, matched.value)
	}
	var matchedOptions []scoredOption
	for _, option := range catalog.Options {
		values := []string{
			option.Path, option.Key, option.Owner, string(option.Source), option.Placement, option.Type, option.Default,
			string(option.DefaultSource), string(option.Requirement), option.Description, option.DescriptionZH,
			option.Example, option.Unit,
		}
		values = append(values, option.Keywords...)
		values = append(values, option.Values...)
		values = append(values, option.RequiredWhen...)
		values = append(values, option.Requires...)
		values = append(values, option.ConflictsWith...)
		for _, preset := range option.PresetValues {
			values = append(values, preset.Preset, preset.Value, preset.Description, preset.DescriptionZH)
		}
		score := catalogTextMatchScore(query, values...)
		if strings.Contains(query, strings.ToLower(option.Path)) {
			score += 5000
		}
		if capabilityScore := paths[option.Path]; capabilityScore > score {
			score = capabilityScore
		}
		if score > 0 {
			matchedOptions = append(matchedOptions, scoredOption{value: option, score: score})
		}
	}
	sort.SliceStable(matchedOptions, func(i, j int) bool {
		if matchedOptions[i].score == matchedOptions[j].score {
			if matchedOptions[i].value.Path == matchedOptions[j].value.Path {
				return matchedOptions[i].value.Owner < matchedOptions[j].value.Owner
			}
			return matchedOptions[i].value.Path < matchedOptions[j].value.Path
		}
		return matchedOptions[i].score > matchedOptions[j].score
	})
	for _, matched := range matchedOptions {
		result.Options = append(result.Options, matched.value)
	}
	return result
}

func catalogTextMatchScore(query string, values ...string) int {
	haystack := strings.ToLower(strings.Join(values, " "))
	if strings.Contains(haystack, query) {
		return 900 + len([]rune(query))
	}
	tokens := catalogQueryTokens(query)
	if len(tokens) > 1 {
		all := true
		for _, token := range tokens {
			if !strings.Contains(haystack, token) {
				all = false
				break
			}
		}
		if all {
			return 700 + len(tokens)
		}
	}
	for _, value := range values {
		value = strings.ToLower(value)
		if len([]rune(value)) >= 2 && strings.Contains(query, value) {
			return 800 + len([]rune(value))
		}
	}
	queryBigrams := catalogHanBigrams(query)
	if len(queryBigrams) >= 2 {
		haystackBigrams := catalogHanBigrams(haystack)
		matches := 0
		for bigram := range queryBigrams {
			if haystackBigrams[bigram] {
				matches++
			}
		}
		if matches >= 2 {
			return 400 + matches
		}
	}
	return 0
}

func catalogHanBigrams(text string) map[string]bool {
	var runes []rune
	result := make(map[string]bool)
	flush := func() {
		for i := 0; i+1 < len(runes); i++ {
			result[string(runes[i:i+2])] = true
		}
		runes = runes[:0]
	}
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			runes = append(runes, r)
			continue
		}
		flush()
	}
	flush()
	return result
}

func catalogQueryTokens(query string) []string {
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "can": true, "cc": true, "cc-connect": true,
		"configure": true, "configuration": true, "do": true, "does": true, "for": true, "how": true,
		"i": true, "is": true, "it": true, "next": true, "of": true, "please": true, "the": true,
		"to": true, "what": true,
	}
	parts := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	var tokens []string
	for _, part := range parts {
		if part == "" || stop[part] {
			continue
		}
		tokens = append(tokens, part)
	}
	return tokens
}

// RenderConfigCatalogMarkdown produces a stable human/Agent-readable view.
func RenderConfigCatalogMarkdown(catalog ConfigCatalog, lang string) string {
	zh := strings.HasPrefix(strings.ToLower(lang), "zh")
	var b strings.Builder
	if zh {
		fmt.Fprintf(&b, "# cc-connect-next 配置能力参考\n\n当前目录版本：`%s`。本参考描述能力，不读取或显示本机配置值。\n\n", catalog.Version)
		b.WriteString("生效方式：`live` 表示当前运行态立即生效；`reload` 表示保存后可用 `/config reload` 应用；`new-session` 表示新 Agent 会话生效；`restart` 表示需要重启 cc-connect-next。\n\n")
		b.WriteString("配置来源：`toml` 表示持久化在 config.toml；`environment` 表示进程环境变量；`cli` 表示启动或安装参数。TOML 字符串支持 `${VAR_NAME}` 环境变量占位符。点路径描述语义位置；Agent/Platform 选项分别写入 `[projects.agent.options]` / `[projects.platforms.options]`，并在相邻表中设置适配器 `type`。\n\n")
		b.WriteString("优先级：显式 CLI 覆盖对应环境变量，环境覆盖项覆盖对应 TOML，项目级字段逐项覆盖全局字段，之后才使用运行时默认值。预设值只表示生成器/Profile 会显式写入什么，不会覆盖用户已有显式配置。\n\n")
		b.WriteString("## 能力概览\n\n")
	} else {
		fmt.Fprintf(&b, "# cc-connect-next Configuration Capabilities\n\nCatalog version: `%s`. This reference describes capability and never reads or prints local configuration values.\n\n", catalog.Version)
		b.WriteString("Apply modes: `live` takes effect in the running process; `reload` can be applied with `/config reload` after saving; `new-session` affects newly started Agent sessions; `restart` requires restarting cc-connect-next.\n\n")
		b.WriteString("Configuration sources: `toml` persists in config.toml; `environment` is read from the process environment; `cli` is a startup or installation flag. TOML strings support `${VAR_NAME}` environment placeholders. Dotted paths describe semantic placement. Agent and Platform options go under `[projects.agent.options]` / `[projects.platforms.options]` with the adjacent adapter `type` selected.\n\n")
		b.WriteString("Precedence: an explicit CLI flag overrides its corresponding environment variable, an environment override wins over its corresponding TOML option, project fields override global fields per value, and runtime defaults apply last. Preset values describe what a generator/Profile writes explicitly; they never override an existing explicit user value.\n\n")
		b.WriteString("## Capability overview\n\n")
	}
	for _, capability := range catalog.Capabilities {
		title, description := capability.Title, capability.Description
		if zh {
			title, description = capability.TitleZH, capability.DescriptionZH
		}
		fmt.Fprintf(&b, "### %s (`%s`)\n\n%s\n", title, capability.ID, description)
		if len(capability.Paths) > 0 {
			fmt.Fprintf(&b, "\n%s `%s`\n", map[bool]string{true: "相关配置：", false: "Related configuration:"}[zh], strings.Join(capability.Paths, "`, `"))
		}
		b.WriteString("\n")
	}
	if zh {
		b.WriteString("## 配置项参考\n\n")
	} else {
		b.WriteString("## Option reference\n\n")
	}
	for _, option := range coalescedConfigOptions(catalog.Options) {
		description := option.Description
		if zh {
			description = option.DescriptionZH
		}
		heading := fmt.Sprintf("`%s`", option.Path)
		if option.Owner != "" {
			heading += fmt.Sprintf(" — `%s`", option.Owner)
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", heading, description)
		if zh {
			fmt.Fprintf(&b, "- 来源：`%s`\n- 配置位置：`%s`\n- 作用域：`%s`%s\n- 类型：`%s`\n- 要求：`%s`\n- 默认值：`%s`\n- 默认值来源：`%s`\n- 生效方式：`%s`\n", option.Source, option.Placement, option.Scope, ownerSuffix(option), option.Type, localizedRequirement(option.Requirement, true), option.Default, option.DefaultSource, option.ApplyMode)
		} else {
			fmt.Fprintf(&b, "- Source: `%s`\n- Placement: `%s`\n- Scope: `%s`%s\n- Type: `%s`\n- Requirement: `%s`\n- Default: `%s`\n- Default source: `%s`\n- Takes effect: `%s`\n", option.Source, option.Placement, option.Scope, ownerSuffix(option), option.Type, option.Requirement, option.Default, option.DefaultSource, option.ApplyMode)
		}
		if len(option.RequiredWhen) > 0 {
			fmt.Fprintf(&b, "- %s: `%s`\n", map[bool]string{true: "必填条件", false: "Required when"}[zh], strings.Join(option.RequiredWhen, "`, `"))
		}
		if len(option.Requires) > 0 {
			fmt.Fprintf(&b, "- %s: `%s`\n", map[bool]string{true: "依赖", false: "Requires"}[zh], strings.Join(option.Requires, "`, `"))
		}
		if len(option.ConflictsWith) > 0 {
			fmt.Fprintf(&b, "- %s: `%s`\n", map[bool]string{true: "冲突", false: "Conflicts with"}[zh], strings.Join(option.ConflictsWith, "`, `"))
		}
		if option.Minimum != nil || option.Maximum != nil {
			minimum, maximum := "-∞", "+∞"
			if option.Minimum != nil {
				minimum = formatConfigContractNumber(*option.Minimum)
			}
			if option.Maximum != nil {
				maximum = formatConfigContractNumber(*option.Maximum)
			}
			fmt.Fprintf(&b, "- %s: `%s` %s `%s`", map[bool]string{true: "范围", false: "Range"}[zh], minimum, map[bool]string{true: "到", false: "to"}[zh], maximum)
			if option.Unit != "" {
				fmt.Fprintf(&b, " `%s`", option.Unit)
			}
			b.WriteString("\n")
		} else if option.Unit != "" {
			fmt.Fprintf(&b, "- %s: `%s`\n", map[bool]string{true: "单位", false: "Unit"}[zh], option.Unit)
		}
		if len(option.Values) > 0 {
			label := map[bool]string{true: "允许值", false: "Allowed values"}[zh]
			if option.OpenValues {
				label = map[bool]string{true: "规范值（也接受文档化别名）", false: "Canonical values (documented aliases also accepted)"}[zh]
			}
			fmt.Fprintf(&b, "- %s: `%s`\n", label, strings.Join(option.Values, "`, `"))
		}
		if option.Sensitive {
			fmt.Fprintf(&b, "- %s\n", map[bool]string{true: "敏感信息：是；优先使用环境变量占位符。", false: "Sensitive: yes; prefer an environment-variable placeholder."}[zh])
		}
		if option.Deprecated {
			fmt.Fprintf(&b, "- %s\n", map[bool]string{true: "状态：已废弃，仅保留兼容。", false: "Status: deprecated compatibility option."}[zh])
		}
		if option.Example != "" {
			fmt.Fprintf(&b, "- %s: `%s`\n", map[bool]string{true: "示例", false: "Example"}[zh], option.Example)
		}
		for _, preset := range option.PresetValues {
			description := preset.Description
			if zh {
				description = preset.DescriptionZH
			}
			fmt.Fprintf(&b, "- %s `%s`: `%s`", map[bool]string{true: "预设", false: "Preset"}[zh], preset.Preset, preset.Value)
			if description != "" {
				fmt.Fprintf(&b, " — %s", description)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func localizedRequirement(requirement ConfigRequirement, zh bool) string {
	if !zh {
		return string(requirement)
	}
	switch requirement {
	case ConfigRequirementRequired:
		return "必填"
	case ConfigRequirementConditional:
		return "条件必填"
	default:
		return "可选"
	}
}

func formatConfigContractNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func coalescedConfigOptions(options []ConfigOption) []ConfigOption {
	type group struct {
		option ConfigOption
		owners []string
	}
	groups := make(map[string]*group)
	var order []string
	for _, option := range options {
		option = FinalizeConfigOption(option)
		if option.Internal {
			continue
		}
		identityOption := option
		identityOption.Owner = ""
		identity := fmt.Sprintf("%#v", identityOption)
		if identityJSON, err := json.Marshal(identityOption); err == nil {
			identity = string(identityJSON)
		}
		current := groups[identity]
		if current == nil {
			copy := option
			copy.Owner = ""
			current = &group{option: copy}
			groups[identity] = current
			order = append(order, identity)
		}
		if option.Owner != "" && !containsString(current.owners, option.Owner) {
			current.owners = append(current.owners, option.Owner)
		}
	}
	out := make([]ConfigOption, 0, len(order))
	for _, identity := range order {
		current := groups[identity]
		sort.Strings(current.owners)
		current.option.Owner = strings.Join(current.owners, ", ")
		out = append(out, current.option)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Owner < out[j].Owner
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func ownerSuffix(option ConfigOption) string {
	if option.Owner == "" {
		return ""
	}
	return fmt.Sprintf(" (`%s`)", option.Owner)
}

// BuildAgentCapabilityBrief creates the bounded standing contract injected
// once per Agent session. Exact configuration, tools, commands, Skills, and
// runtime adapter availability remain on demand through the local unified
// manifest command.
func BuildAgentCapabilityBrief(catalog ConfigCatalog, agent string, platforms []string) string {
	platforms = append([]string(nil), platforms...)
	sort.Strings(platforms)
	var active []ConfigOption
	for _, option := range catalog.Options {
		if (option.Scope == ConfigScopeAgent && option.Owner == agent) || (option.Scope == ConfigScopePlatform && containsString(platforms, option.Owner)) {
			active = append(active, option)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].Owner == active[j].Owner {
			return active[i].Key < active[j].Key
		}
		return active[i].Owner < active[j].Owner
	})

	cmd := "cc-connect-next capabilities --search \"<2-4 keywords from the user's question>\""

	var b strings.Builder
	fmt.Fprintf(&b, "[cc-connect-next capability brief]\nThis conversation is bridged through cc-connect-next %s. The active Agent is %q and the active platform adapters are %q.\n", catalog.Version, agent, strings.Join(platforms, ", "))
	b.WriteString("Users will ask natural-language questions about what cc-connect-next can do or configure. Before answering a specific capability question or invoking a cc-connect-next operation, query the local version/project/session-matched, read-only Agent Capability Manifest:\n  ")
	b.WriteString(cmd)
	b.WriteString("\nThe Manifest contains configuration contracts, Agent CLI tools, chat commands, discovered Skills, and active Agent/Platform runtime capabilities. Add `--all` when the user asks about compiled adapters beyond the active project. It describes parameters, permission, read-only status, write/external side effects, fallback behavior, and truthful availability reasons; it never returns configured secret values, Skill bodies, or custom Prompt/Exec bodies. Use separate searches for unrelated wishes. Treat a related result without an exact action/option as unsupported. For configuration, explain exact source/location, requirements, omitted default versus preset values, accepted values/range, scope, apply mode, security caveats, and the validated example; do not invent commands or config keys. If there is no exact match, say this runtime does not declare that capability and offer `/feedback <description>`. Do not invoke a mutating action, edit config, or restart the service unless the user explicitly asks.\n")
	if len(catalog.Capabilities) > 0 {
		b.WriteString("Configuration areas: ")
		limit := len(catalog.Capabilities)
		if limit > 16 {
			limit = 16
		}
		for i, capability := range catalog.Capabilities[:limit] {
			if i > 0 {
				b.WriteString("; ")
			}
			fmt.Fprintf(&b, "%s (%s)", capability.Title, capability.ID)
		}
		b.WriteString(".\n")
	}
	if len(active) > 0 {
		b.WriteString("Active adapter options: ")
		visible := 0
		for _, option := range active {
			if option.Internal {
				continue
			}
			if visible == maxCapabilityBriefOptions {
				break
			}
			if visible > 0 {
				b.WriteString("; ")
			}
			fmt.Fprintf(&b, "%s.%s — %s", option.Owner, option.Key, option.Description)
			visible++
		}
		publicCount := 0
		for _, option := range active {
			if !option.Internal {
				publicCount++
			}
		}
		if remaining := publicCount - visible; remaining > 0 {
			fmt.Fprintf(&b, "; … %d more are available through the Manifest command", remaining)
		}
	}
	return strings.TrimSpace(b.String())
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
