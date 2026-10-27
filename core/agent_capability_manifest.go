package core

import (
	"fmt"
	"sort"
	"strings"
)

const AgentCapabilityManifestSchema = "cc-connect-next.agent-capabilities/v1"

type CapabilityAvailabilityState string

const (
	CapabilityAvailable   CapabilityAvailabilityState = "available"
	CapabilityConditional CapabilityAvailabilityState = "conditional"
	CapabilityUnavailable CapabilityAvailabilityState = "unavailable"
)

type CapabilityPermissionLevel string

const (
	CapabilityPermissionMember      CapabilityPermissionLevel = "member"
	CapabilityPermissionAdmin       CapabilityPermissionLevel = "admin"
	CapabilityPermissionConditional CapabilityPermissionLevel = "conditional"
	CapabilityPermissionLocalAgent  CapabilityPermissionLevel = "local-agent"
)

type CapabilityAvailability struct {
	State    CapabilityAvailabilityState `json:"state"`
	Reason   string                      `json:"reason"`
	ReasonZH string                      `json:"reason_zh"`
}

type CapabilityParameter struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Required      bool     `json:"required"`
	Repeatable    bool     `json:"repeatable,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
	Description   string   `json:"description"`
	DescriptionZH string   `json:"description_zh"`
}

type CapabilitySideEffect struct {
	Kind          string `json:"kind"`
	Description   string `json:"description"`
	DescriptionZH string `json:"description_zh"`
}

type CapabilityFallback struct {
	Mode          string `json:"mode"`
	Description   string `json:"description"`
	DescriptionZH string `json:"description_zh"`
}

type AgentToolCapability struct {
	ID            string                    `json:"id"`
	Invocation    string                    `json:"invocation"`
	Description   string                    `json:"description"`
	DescriptionZH string                    `json:"description_zh"`
	Parameters    []CapabilityParameter     `json:"parameters,omitempty"`
	Permission    CapabilityPermissionLevel `json:"permission"`
	ReadOnly      bool                      `json:"read_only"`
	SideEffects   []CapabilitySideEffect    `json:"side_effects,omitempty"`
	Fallback      CapabilityFallback        `json:"fallback"`
	Availability  CapabilityAvailability    `json:"availability"`
}

type AgentCommandCapability struct {
	ID             string                    `json:"id"`
	Invocation     string                    `json:"invocation"`
	Aliases        []string                  `json:"aliases,omitempty"`
	Source         string                    `json:"source"`
	Category       string                    `json:"category"`
	Usage          string                    `json:"usage"`
	Description    string                    `json:"description"`
	DescriptionZH  string                    `json:"description_zh"`
	Parameters     []CapabilityParameter     `json:"parameters,omitempty"`
	Permission     CapabilityPermissionLevel `json:"permission"`
	PrivilegedWhen []string                  `json:"privileged_when,omitempty"`
	ReadOnly       bool                      `json:"read_only"`
	SideEffects    []CapabilitySideEffect    `json:"side_effects,omitempty"`
	Fallback       CapabilityFallback        `json:"fallback"`
	Availability   CapabilityAvailability    `json:"availability"`
}

type AgentSkillCapability struct {
	Name         string                    `json:"name"`
	DisplayName  string                    `json:"display_name,omitempty"`
	Invocation   string                    `json:"invocation"`
	Description  string                    `json:"description"`
	Parameters   []CapabilityParameter     `json:"parameters,omitempty"`
	Permission   CapabilityPermissionLevel `json:"permission"`
	ReadOnly     bool                      `json:"read_only"`
	SideEffects  []CapabilitySideEffect    `json:"side_effects,omitempty"`
	Availability CapabilityAvailability    `json:"availability"`
}

type RuntimeFeatureCapability struct {
	ID            string                 `json:"id"`
	Description   string                 `json:"description"`
	DescriptionZH string                 `json:"description_zh"`
	Availability  CapabilityAvailability `json:"availability"`
	Fallback      CapabilityFallback     `json:"fallback"`
	// Optional action contract for automatic operational capabilities.
	Permission  CapabilityPermissionLevel `json:"permission,omitempty"`
	ReadOnly    *bool                     `json:"read_only,omitempty"`
	SideEffects []CapabilitySideEffect    `json:"side_effects,omitempty"`
}

type RuntimeAdapterCapabilities struct {
	Kind         string                      `json:"kind"`
	Name         string                      `json:"name"`
	State        CapabilityAvailabilityState `json:"state"`
	Reason       string                      `json:"reason,omitempty"`
	Capabilities []RuntimeFeatureCapability  `json:"capabilities"`
}

// RuntimeCapabilityAvailabilityProvider lets adapters whose concrete Go type
// exposes a superset of dynamically negotiated features refine structural
// interface checks for one session target. BridgePlatform uses it to report
// the connected external adapter's declared capabilities.
type RuntimeCapabilityAvailabilityProvider interface {
	RuntimeCapabilityAvailability(sessionKey string, replyCtx any) map[string]CapabilityAvailability
}

type AgentCapabilityManifest struct {
	Schema            string                       `json:"schema"`
	Version           string                       `json:"version"`
	Commit            string                       `json:"commit,omitempty"`
	BuildTime         string                       `json:"build_time,omitempty"`
	Project           string                       `json:"project"`
	SessionBound      bool                         `json:"session_bound"`
	AllAdapters       bool                         `json:"all_adapters"`
	ActiveAgent       string                       `json:"active_agent"`
	ActivePlatforms   []string                     `json:"active_platforms"`
	CompiledAgents    []string                     `json:"compiled_agents"`
	CompiledPlatforms []string                     `json:"compiled_platforms"`
	ReadOnly          bool                         `json:"read_only"`
	SecurityNote      string                       `json:"security_note"`
	Query             string                       `json:"query,omitempty"`
	Configuration     ConfigCatalog                `json:"configuration"`
	Tools             []AgentToolCapability        `json:"tools"`
	Commands          []AgentCommandCapability     `json:"commands"`
	Skills            []AgentSkillCapability       `json:"skills"`
	Runtime           []RuntimeAdapterCapabilities `json:"runtime"`
}

type builtinCommandDefinition struct {
	id             string
	aliases        []string
	category       string
	usage          string
	parameters     []CapabilityParameter
	readOnly       bool
	effects        []string
	admin          bool
	subcommands    []string
	privilegedWhen []string
	fallback       CapabilityFallback
	probe          string
}

func capabilityParam(name, typeName string, required bool, description, zh string, values ...string) CapabilityParameter {
	return CapabilityParameter{Name: name, Type: typeName, Required: required, AllowedValues: values, Description: description, DescriptionZH: zh}
}

func defaultRejectFallback() CapabilityFallback {
	return CapabilityFallback{
		Mode:          "reject",
		Description:   "If a prerequisite is missing, the invocation returns a localized error without pretending the operation succeeded.",
		DescriptionZH: "缺少前置条件时会返回本地化错误，不会把操作伪装成成功。",
	}
}

var builtinCommands = []builtinCommandDefinition{
	{id: "new", category: "session", usage: "/new [prompt]", parameters: []CapabilityParameter{capabilityParam("prompt", "string", false, "Optional first request for the new session.", "新会话可立即处理的首个请求。")}, effects: []string{"session_state", "agent_process"}},
	{id: "list", aliases: []string{"sessions"}, category: "session", usage: "/list", readOnly: true},
	{id: "switch", category: "session", usage: "/switch <number|name>", parameters: []CapabilityParameter{capabilityParam("session", "string", true, "Session list number or name.", "会话列表序号或名称。")}, effects: []string{"session_state", "agent_process"}},
	{id: "name", aliases: []string{"rename"}, category: "session", usage: "/name [number] <text>", parameters: []CapabilityParameter{capabilityParam("session", "integer", false, "Optional session list number; omission selects the current session.", "可选会话序号；省略时使用当前会话。"), capabilityParam("text", "string", true, "New session title.", "新的会话标题。")}, effects: []string{"session_state", "persistent_state"}},
	{id: "current", category: "session", usage: "/current", readOnly: true},
	{id: "status", category: "system", usage: "/status", readOnly: true},
	{id: "usage", aliases: []string{"quota"}, category: "agent", usage: "/usage", readOnly: true, probe: "usage"},
	{id: "history", category: "session", usage: "/history [n]", parameters: []CapabilityParameter{capabilityParam("limit", "integer", false, "Maximum number of recent messages; defaults to 10.", "最近消息数量上限；默认 10。")}, readOnly: true},
	{id: "allow", category: "agent", usage: "/allow <tool>", parameters: []CapabilityParameter{capabilityParam("tool", "string", true, "Tool name to allow for the next Agent session.", "为下一个 Agent 会话预授权的工具名。")}, effects: []string{"agent_permission_state"}, probe: "tool_authorizer"},
	{
		id: "model", category: "agent", usage: "/model [switch <name>|group [<name>|reset]]",
		parameters: []CapabilityParameter{
			capabilityParam("action", "string", false,
				"Omit to inspect models. switch (or legacy /model <name>) changes the project/workspace default for private chats and groups without a fixed model. group queries, pins, or resets this verified group's model for all members and existing/new topics. Group selections require ModelSwitcher and PerTurnModelSupporter on the configured backend; no Provider change.",
				"省略时查看模型。switch（或旧写法 /model <name>）更改项目/工作区默认模型，影响私聊和未固定模型的群。group 查询、固定或重置已验证群的模型，覆盖全部成员和新旧话题。群固定要求配置的后端具备 ModelSwitcher 和 PerTurnModelSupporter，不改变 Provider。", "switch", "group"),
			capabilityParam("name", "string", false,
				"Model identifier, alias, or displayed choice; group accepts only the current model list, or reset to follow defaults. The group binding persists within the project by platform + stable group ID, not topic or workspace, across /reset, /new, and restarts. Resolve it at execution on all three send paths, including queued requests; changes affect the next new turn without interrupting a running reply. An explicit one-shot /fast or task model takes precedence. Omit after group to query.",
				"模型标识、别名或显示选项；group 仅接受当前模型列表可选项，或 reset 恢复跟随默认。群绑定在项目内按平台 + 稳定群 ID 持久化，不按话题或工作区划分，跨 /reset、/new 和重启保留。全部三条发送路径（包括排队请求）在执行时解析；变更在下次新回合生效，不打断正在回复的回合。/fast 或任务的显式单次模型优先。group 后省略时查询。"),
		},
		effects: []string{"configuration", "agent_process", "persistent_state"}, probe: "model",
		fallback: CapabilityFallback{
			Mode:          "reject",
			Description:   "Group pinning rejects unverified group identity, unavailable storage, unsupported per-turn models, or an unlisted selection. A pinned group's ordinary turn fails on backend/Provider mismatch or model/application errors without silently falling back or switching Provider; explicitly reselect the group model or reset it. Legacy project/workspace switching remains supported by ModelSwitcher alone.",
			DescriptionZH: "群固定会拒绝未验证群身份、不可用存储、不支持逐回合模型或不在当前列表的选择。固定群的普通回合遇到后端/Provider 不匹配、模型或应用设置错误时失败，不会静默回退或切换 Provider；请显式重新设置群模型或 reset。原项目/工作区模型切换仍只需 ModelSwitcher。",
		},
	},
	{id: "reasoning", aliases: []string{"effort"}, category: "agent", usage: "/reasoning [<level>|group [<level>|reset]]", parameters: []CapabilityParameter{capabilityParam("level-or-group", "string", false, "A bare level changes the project/workspace Agent default and resets this conversation. group queries, pins, or resets only this verified group effort for all members and topics, independently of the group model.", "直接指定等级修改项目/工作区 Agent 默认值并重置本对话；group 查询、固定或重置已验证本群全部成员和话题的 effort，独立于群模型。"), capabilityParam("level", "string", false, "Group level from the Agent effort catalog, or reset to inherit. Persists across /new, /reset and restarts. Explicit one-shot/profile/task effort takes precedence; queued requests resolve the latest binding without interrupting a running reply. Requires per-turn reasoning support and matching backend/Provider; invalid or unsupported settings fail without fallback.", "群等级取自 Agent 强度列表，reset 恢复继承。跨 /new、/reset 和重启保留；显式单次/配置档/任务 effort 优先；排队请求执行时读取最新绑定，不打断当前回复。需逐回合强度支持且后端/Provider 一致；无效或不支持的设置明确失败，不回退。")}, effects: []string{"configuration", "agent_process", "persistent_state"}, probe: "reasoning"},
	{id: "speed", category: "agent", usage: "/speed [tier]", parameters: []CapabilityParameter{capabilityParam("tier", "string", false, "Omit to query. Choose default, fast, or another tier declared by the active model catalog. Saved for this conversation; affects subsequent turns without changing model/effort or interrupting a running turn. One-shot answer profiles take precedence.", "省略时只查询。选择 default、fast 或当前模型目录声明的其他档位。选择保存到当前会话；后续回合生效，不改变模型/推理强度或打断当前回合；单次回答预设优先。")}, effects: []string{"session_state", "persistent_state"}, probe: "speed"},
	{id: "mode", category: "agent", usage: "/mode [name]", parameters: []CapabilityParameter{capabilityParam("name", "string", false, "Permission mode supported by the active Agent.", "当前 Agent 支持的权限模式。")}, effects: []string{"configuration", "agent_permission_state"}, probe: "mode"},
	{id: "lang", category: "agent", usage: "/lang [en|zh|zh-TW|ja|es|auto]", parameters: []CapabilityParameter{capabilityParam("language", "string", false, "Reply language.", "回复语言。", "en", "zh", "zh-TW", "ja", "es", "auto")}, effects: []string{"configuration"}},
	{id: "quiet", category: "agent", usage: "/quiet [on|off]", parameters: []CapabilityParameter{capabilityParam("state", "string", false, "Enable or disable quiet display mode.", "开启或关闭安静显示模式。", "on", "off")}, effects: []string{"configuration"}},
	{id: "provider", category: "agent", usage: "/provider [list|add|remove|switch|clear] [arguments]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "Provider operation.", "Provider 操作。", "list", "add", "remove", "switch", "clear"), capabilityParam("arguments", "string", false, "Provider name and operation-specific values.", "Provider 名称及操作所需参数。")}, effects: []string{"configuration", "agent_process"}, probe: "provider"},
	{id: "memory", category: "agent", usage: "/memory [add|global|global add] [text]", parameters: []CapabilityParameter{capabilityParam("scope", "string", false, "Project or global memory operation.", "项目或全局记忆操作。", "add", "global", "global add"), capabilityParam("text", "string", false, "Instruction text appended by an add operation.", "add 操作要追加的指令文本。")}, effects: []string{"filesystem_read", "filesystem_write"}, probe: "memory"},
	{id: "cron", category: "automation", usage: "/cron [add|addexec|list|exec|del|enable|disable|mute|unmute|setup] [arguments]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "Recurring-job operation.", "周期任务操作。", "add", "addexec", "list", "exec", "del", "enable", "disable", "mute", "unmute", "setup"), capabilityParam("arguments", "string", false, "Schedule, prompt, command, job ID, or setting required by the action.", "操作所需的计划、Prompt、命令、任务 ID 或设置。")}, effects: []string{"scheduled_state", "persistent_state", "agent_turn", "shell_execution"}, subcommands: []string{"add", "addexec", "list", "del", "delete", "rm", "remove", "enable", "disable", "mute", "unmute", "setup"}, privilegedWhen: []string{"addexec"}, probe: "cron"},
	{id: "timer", aliases: []string{"at", "remind"}, category: "automation", usage: "/timer [add|addexec|list|del|mute|unmute] [arguments]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "One-shot timer operation.", "一次性定时器操作。", "add", "addexec", "list", "del", "mute", "unmute"), capabilityParam("arguments", "string", false, "Delay/time, prompt, command, timer ID, or setting required by the action.", "操作所需的延迟/时间、Prompt、命令、定时器 ID 或设置。")}, effects: []string{"scheduled_state", "persistent_state", "agent_turn", "shell_execution"}, subcommands: []string{"add", "addexec", "list", "del", "delete", "rm", "remove", "mute", "unmute"}, privilegedWhen: []string{"addexec"}, probe: "timer"},
	{id: "heartbeat", aliases: []string{"hb"}, category: "automation", usage: "/heartbeat [status|pause|resume|run|interval] [minutes]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "Heartbeat operation.", "Heartbeat 操作。", "status", "pause", "resume", "run", "interval"), capabilityParam("minutes", "integer", false, "New interval for the interval action.", "interval 操作的新间隔分钟数。")}, effects: []string{"scheduled_state", "persistent_state", "agent_turn"}, probe: "heartbeat"},
	{id: "compress", aliases: []string{"compact"}, category: "agent", usage: "/compress", effects: []string{"agent_context"}, probe: "compress"},
	{id: "stop", category: "session", usage: "/stop", effects: []string{"agent_process"}, probe: "active_session"},
	{id: "cancel", category: "session", usage: "/cancel", effects: []string{"session_state", "agent_process"}, probe: "active_session"},
	{id: "help", category: "system", usage: "/help", readOnly: true},
	{id: "version", category: "system", usage: "/version", readOnly: true},
	{id: "commands", aliases: []string{"command", "cmd"}, category: "system", usage: "/commands [list|add|addexec|del] [arguments]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "Custom-command registry operation.", "自定义命令注册表操作。", "list", "add", "addexec", "del"), capabilityParam("arguments", "string", false, "Name, description, prompt, shell command, or working directory required by the action.", "操作所需的名称、说明、Prompt、Shell 命令或工作目录。")}, effects: []string{"configuration", "persistent_state"}, subcommands: []string{"list", "add", "addexec", "del", "delete", "rm", "remove"}, privilegedWhen: []string{"addexec"}},
	{id: "skills", aliases: []string{"skill"}, category: "system", usage: "/skills", readOnly: true},
	{id: "config", category: "system", usage: "/config [get|set|reload] [key] [value]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "Runtime configuration operation.", "运行时配置操作。", "get", "set", "reload"), capabilityParam("key", "string", false, "Supported runtime display key.", "支持的运行时显示配置键。"), capabilityParam("value", "string", false, "New value for set.", "set 操作的新值。")}, effects: []string{"configuration"}},
	{id: "doctor", category: "system", usage: "/doctor", readOnly: true},
	{id: "feedback", aliases: []string{"fb"}, category: "system", usage: "/feedback <description>", parameters: []CapabilityParameter{capabilityParam("description", "string", true, "Problem or missing capability submitted immediately by this explicit chat action.", "由这次明确聊天动作立即提交的问题或缺失能力。")}, effects: []string{"external_service", "network"}, probe: "feedback"},
	{id: "upgrade", aliases: []string{"update"}, category: "system", usage: "/upgrade [stable|beta] [check|confirm|cancel|status]", parameters: []CapabilityParameter{capabilityParam("channel", "string", false, "Explicit stable or beta release channel; defaults to configured update_channel.", "显式 stable 或 beta 发布通道；默认使用 update_channel 配置。"), capabilityParam("confirmation", "string", false, "Review, confirm, cancel a pending check/wait, or inspect the saved update result. Confirmation installs the reviewed version after running tasks finish.", "查看、确认、取消检查或等待，或查询已保存的更新结果。确认后等待运行任务结束，再安装已审阅的版本。")}, effects: []string{"network", "filesystem_write", "process_control"}, admin: true},
	{id: "restart", category: "system", usage: "/restart", effects: []string{"process_control"}, admin: true},
	{id: "alias", category: "system", usage: "/alias [add|del] [trigger] [command]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "Alias registry operation.", "别名注册表操作。", "add", "del"), capabilityParam("trigger", "string", false, "Natural-language alias trigger.", "自然语言别名触发词。"), capabilityParam("command", "string", false, "Target slash command.", "目标 Slash 命令。")}, effects: []string{"configuration", "persistent_state"}},
	{id: "delete", aliases: []string{"del", "rm"}, category: "session", usage: "/delete <selection>", parameters: []CapabilityParameter{capabilityParam("selection", "string", true, "Session number, comma list, or range.", "会话序号、逗号列表或范围。")}, effects: []string{"session_state", "persistent_state", "agent_process"}},
	{id: "bind", category: "automation", usage: "/bind [project|-project|remove|status|setup]", parameters: []CapabilityParameter{capabilityParam("operation", "string", false, "Relay project or binding operation.", "Relay 项目或绑定操作。")}, effects: []string{"relay_state", "persistent_state"}, probe: "relay"},
	{id: "search", aliases: []string{"find"}, category: "session", usage: "/search <keyword>", parameters: []CapabilityParameter{capabilityParam("keyword", "string", true, "Session name or ID search text.", "会话名称或 ID 搜索文本。")}, readOnly: true},
	{id: "shell", aliases: []string{"sh", "exec", "run"}, category: "tools", usage: "/shell [--timeout <seconds>] <command>", parameters: []CapabilityParameter{capabilityParam("timeout", "integer", false, "Execution timeout in seconds.", "执行超时秒数。"), capabilityParam("command", "string", true, "Shell command executed with the configured shell/profile.", "使用已配置 Shell/Profile 执行的命令。")}, effects: []string{"shell_execution", "process_control", "filesystem_read", "filesystem_write", "network"}, admin: true},
	{id: "show", category: "tools", usage: "/show <reference>", parameters: []CapabilityParameter{capabilityParam("reference", "string", true, "Local file, directory, or code reference.", "本地文件、目录或代码引用。")}, readOnly: true, effects: []string{"filesystem_read"}, admin: true},
	{id: "dir", aliases: []string{"cd", "chdir", "workdir"}, category: "tools", usage: "/dir [path|reset]", parameters: []CapabilityParameter{capabilityParam("path", "string", false, "Absolute/local path or reset.", "绝对/本地路径或 reset。")}, effects: []string{"configuration", "agent_process"}, admin: true, probe: "workdir"},
	{id: "tts", category: "agent", usage: "/tts [always|voice_only]", parameters: []CapabilityParameter{capabilityParam("mode", "string", false, "Text-to-speech mode.", "文字转语音模式。", "always", "voice_only")}, effects: []string{"configuration", "external_message"}, probe: "tts"},
	{id: "workspace", aliases: []string{"ws"}, category: "tools", usage: "/workspace [init|bind|route|info|unbind] [arguments]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "Multi-workspace operation.", "多工作区操作。", "init", "bind", "route", "info", "unbind"), capabilityParam("arguments", "string", false, "Repository URL, workspace name, or local path required by the action.", "操作所需的仓库 URL、工作区名称或本地路径。")}, effects: []string{"workspace_state", "filesystem_write", "network"}, probe: "workspace"},
	{id: "whoami", aliases: []string{"myid"}, category: "system", usage: "/whoami", readOnly: true},
	{id: "web", category: "system", usage: "/web [status|setup]", parameters: []CapabilityParameter{capabilityParam("action", "string", false, "Web management operation.", "Web 管理操作。", "status", "setup")}, effects: []string{"configuration", "process_control"}, admin: true, probe: "web"},
	{id: "diff", category: "tools", usage: "/diff [git arguments]", parameters: []CapabilityParameter{capabilityParam("arguments", "string", false, "Optional git diff arguments.", "可选 git diff 参数。")}, readOnly: true, effects: []string{"filesystem_read", "process_control"}, admin: true},
	{id: "ps", aliases: []string{"btw"}, category: "session", usage: "/ps <message>", parameters: []CapabilityParameter{capabilityParam("message", "string", true, "Supplement appended to the active turn.", "并入当前回合的补充内容。")}, effects: []string{"agent_turn"}, probe: "ps", fallback: CapabilityFallback{Mode: "legacy-send", Description: "Native steer is used when supported; persistent-process Agents otherwise receive the supplement through their established stdin Send path. No active turn is rejected rather than queued.", DescriptionZH: "支持时使用原生 steer；否则持久进程 Agent 通过既有 stdin Send 路径接收补充。没有活动回合时会拒绝，不会偷偷排队。"}},
}

func builtinCommandNames(definition builtinCommandDefinition) []string {
	return append([]string{definition.id}, definition.aliases...)
}

func builtinCommandByID(id string) (builtinCommandDefinition, bool) {
	for _, definition := range builtinCommands {
		if definition.id == id {
			return definition, true
		}
	}
	return builtinCommandDefinition{}, false
}

var sideEffectDescriptions = map[string]CapabilitySideEffect{
	"agent_context":          {Kind: "agent_context", Description: "Mutates the Agent's conversation context.", DescriptionZH: "修改 Agent 会话上下文。"},
	"agent_permission_state": {Kind: "agent_permission_state", Description: "Changes Agent tool or permission state.", DescriptionZH: "修改 Agent 工具或权限状态。"},
	"agent_process":          {Kind: "agent_process", Description: "Starts, resumes, interrupts, or replaces an Agent process/session.", DescriptionZH: "启动、恢复、中断或替换 Agent 进程/会话。"},
	"agent_turn":             {Kind: "agent_turn", Description: "Starts or modifies an Agent turn; the Agent may use tools under its configured permissions.", DescriptionZH: "启动或修改 Agent 回合；Agent 可能在既有权限下使用工具。"},
	"configuration":          {Kind: "configuration", Description: "Changes runtime or persisted cc-connect-next configuration.", DescriptionZH: "修改 cc-connect-next 运行态或持久化配置。"},
	"external_message":       {Kind: "external_message", Description: "Sends user-visible content to a messaging platform.", DescriptionZH: "向消息平台发送用户可见内容。"},
	"external_service":       {Kind: "external_service", Description: "Creates or mutates data in an external service.", DescriptionZH: "在外部服务中创建或修改数据。"},
	"filesystem_read":        {Kind: "filesystem_read", Description: "Reads local filesystem content or metadata.", DescriptionZH: "读取本地文件系统内容或元数据。"},
	"filesystem_write":       {Kind: "filesystem_write", Description: "Creates or changes local filesystem content.", DescriptionZH: "创建或修改本地文件系统内容。"},
	"network":                {Kind: "network", Description: "May make outbound network requests.", DescriptionZH: "可能发起出站网络请求。"},
	"persistent_state":       {Kind: "persistent_state", Description: "Writes cc-connect-next persistent state.", DescriptionZH: "写入 cc-connect-next 持久化状态。"},
	"process_control":        {Kind: "process_control", Description: "Starts, stops, restarts, or replaces a local process.", DescriptionZH: "启动、停止、重启或替换本地进程。"},
	"relay_state":            {Kind: "relay_state", Description: "Changes cross-project relay bindings or sessions.", DescriptionZH: "修改跨项目 Relay 绑定或会话。"},
	"scheduled_state":        {Kind: "scheduled_state", Description: "Creates or changes durable scheduled execution.", DescriptionZH: "创建或修改持久化调度执行。"},
	"session_state":          {Kind: "session_state", Description: "Changes session selection, identity, title, or lifecycle state.", DescriptionZH: "修改会话选择、身份、标题或生命周期状态。"},
	"shell_execution":        {Kind: "shell_execution", Description: "Executes an operator-provided shell command with the daemon's configured OS identity.", DescriptionZH: "使用 daemon 配置的操作系统身份执行操作者提供的 Shell 命令。"},
	"workspace_state":        {Kind: "workspace_state", Description: "Changes workspace routing or bindings.", DescriptionZH: "修改工作区路由或绑定。"},
}

func expandSideEffects(kinds []string) []CapabilitySideEffect {
	result := make([]CapabilitySideEffect, 0, len(kinds))
	for _, kind := range kinds {
		if effect, ok := sideEffectDescriptions[kind]; ok {
			result = append(result, effect)
		}
	}
	return result
}

func available(reason, zh string) CapabilityAvailability {
	return CapabilityAvailability{State: CapabilityAvailable, Reason: reason, ReasonZH: zh}
}

func conditional(reason, zh string) CapabilityAvailability {
	return CapabilityAvailability{State: CapabilityConditional, Reason: reason, ReasonZH: zh}
}

func unavailable(reason, zh string) CapabilityAvailability {
	return CapabilityAvailability{State: CapabilityUnavailable, Reason: reason, ReasonZH: zh}
}

func (e *Engine) SetConfigCatalog(catalog ConfigCatalog) {
	// The bootstrap catalog is immutable and shared by every project Engine;
	// retain its slices instead of duplicating hundreds of option contracts per
	// project. Manifest projections clone before returning mutable slices.
	e.configCatalog = catalog
}

// QueryAgentCapabilityManifest builds one runtime snapshot and optionally
// filters it. Empty search text returns the complete selected-adapter view.
func (e *Engine) QueryAgentCapabilityManifest(sessionKey, search string, includeAll bool) AgentCapabilityManifest {
	manifest := e.agentCapabilityManifest(sessionKey, includeAll)
	return SearchAgentCapabilityManifest(manifest, search)
}

func cloneConfigCatalog(catalog ConfigCatalog) ConfigCatalog {
	result := catalog
	result.Agents = append([]string(nil), catalog.Agents...)
	result.Platforms = append([]string(nil), catalog.Platforms...)
	result.Capabilities = cloneConfigCapabilities(catalog.Capabilities)
	result.Options = cloneConfigOptions(catalog.Options)
	return result
}

func cloneConfigCapabilities(capabilities []ConfigCapability) []ConfigCapability {
	result := append([]ConfigCapability(nil), capabilities...)
	for i := range result {
		result[i].Keywords = append([]string(nil), result[i].Keywords...)
		result[i].Paths = append([]string(nil), result[i].Paths...)
	}
	return result
}

func filterRuntimeConfigCatalog(catalog ConfigCatalog, agent string, platforms []string) ConfigCatalog {
	platformSet := make(map[string]bool, len(platforms))
	for _, name := range platforms {
		platformSet[name] = true
	}
	result := ConfigCatalog{Version: catalog.Version}
	if agent != "" {
		result.Agents = []string{agent}
	}
	result.Platforms = append(result.Platforms, platforms...)
	paths := make(map[string]bool)
	for _, option := range catalog.Options {
		include := option.Owner == ""
		switch option.Scope {
		case ConfigScopeAgent:
			include = option.Owner == "" || option.Owner == agent
		case ConfigScopePlatform:
			include = option.Owner == "" || platformSet[option.Owner]
		}
		if include {
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
	result.Options = cloneConfigOptions(result.Options)
	result.Capabilities = cloneConfigCapabilities(result.Capabilities)
	return result
}

func (e *Engine) agentCapabilityManifest(sessionKey string, includeAll bool) AgentCapabilityManifest {
	snapshot := e.captureCapabilitySnapshot(sessionKey)
	manifestVersion := CurrentVersion
	if manifestVersion == "" {
		manifestVersion = e.configCatalog.Version
	}
	platformNames := make([]string, 0, len(e.platforms))
	for _, platform := range e.platforms {
		platformNames = append(platformNames, platform.Name())
	}
	sort.Strings(platformNames)
	manifest := AgentCapabilityManifest{
		Schema:            AgentCapabilityManifestSchema,
		Version:           manifestVersion,
		Commit:            CurrentCommit,
		BuildTime:         CurrentBuildTime,
		Project:           e.name,
		SessionBound:      strings.TrimSpace(sessionKey) != "",
		AllAdapters:       includeAll,
		ActiveAgent:       e.agent.Name(),
		ActivePlatforms:   append([]string(nil), platformNames...),
		CompiledAgents:    append([]string(nil), e.configCatalog.Agents...),
		CompiledPlatforms: append([]string(nil), e.configCatalog.Platforms...),
		ReadOnly:          true,
		SecurityNote:      "Capability metadata only: no configured secret values, Skill instructions, custom command bodies, or shell command bodies are included.",
		Configuration:     filterRuntimeConfigCatalog(e.configCatalog, e.agent.Name(), platformNames),
	}
	if includeAll {
		manifest.Configuration = cloneConfigCatalog(e.configCatalog)
	}
	manifest.Tools = e.agentToolCapabilities(snapshot)
	manifest.Commands = e.agentCommandCapabilities(snapshot)
	manifest.Skills = e.agentSkillCapabilities(sessionKey)
	manifest.Runtime = e.runtimeAdapterCapabilities(snapshot, includeAll)
	return manifest
}

func (e *Engine) capabilityProjectPolicy() (map[string]bool, string) {
	e.userRolesMu.RLock()
	disabled := e.disabledCmds
	adminFrom := e.adminFrom
	e.userRolesMu.RUnlock()
	copyDisabled := make(map[string]bool, len(disabled))
	for key, value := range disabled {
		copyDisabled[key] = value
	}
	return copyDisabled, adminFrom
}

type capabilitySnapshot struct {
	sessionKey string
	session    AgentSession
	platform   Platform
	replyCtx   any
}

func (e *Engine) captureCapabilitySnapshot(sessionKey string) capabilitySnapshot {
	snapshot := capabilitySnapshot{sessionKey: strings.TrimSpace(sessionKey)}
	if snapshot.sessionKey == "" {
		return snapshot
	}
	key := e.interactiveKeyForSessionKey(snapshot.sessionKey)
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	if state == nil {
		return snapshot
	}
	state.mu.Lock()
	snapshot.session = state.agentSession
	snapshot.platform = state.platform
	snapshot.replyCtx = state.replyCtx
	state.mu.Unlock()
	return snapshot
}

func commandPermission(definition builtinCommandDefinition) CapabilityPermissionLevel {
	if definition.admin {
		return CapabilityPermissionAdmin
	}
	if len(definition.privilegedWhen) > 0 {
		return CapabilityPermissionConditional
	}
	return CapabilityPermissionMember
}

func (e *Engine) agentCommandCapabilities(snapshot capabilitySnapshot) []AgentCommandCapability {
	disabled, adminFrom := e.capabilityProjectPolicy()
	english := NewI18n(LangEnglish)
	chinese := NewI18n(LangChinese)
	result := make([]AgentCommandCapability, 0, len(builtinCommands)+len(e.commands.ListAll()))
	builtinIDs := make(map[string]bool, len(builtinCommands))
	for _, definition := range builtinCommands {
		builtinIDs[definition.id] = true
		permission := commandPermission(definition)
		availability := e.commandCapabilityAvailability(definition.id, definition.probe, snapshot)
		if disabled[definition.id] {
			availability = unavailable("Disabled by the project-level command policy; user-role policy is checked at invocation time.", "已被项目级命令策略禁用；用户角色策略在真实调用时检查。")
		} else if permission == CapabilityPermissionAdmin {
			switch {
			case strings.TrimSpace(adminFrom) == "":
				availability = unavailable("Requires admin permission, but projects.admin_from is not configured.", "需要管理员权限，但 projects.admin_from 尚未配置。")
			default:
				availability = conditional("Requires projects.admin_from authorization at invocation time; caller identity is checked by the real command dispatch.", "调用时需要 projects.admin_from 授权；调用者身份由真实命令分发路径检查。")
			}
		}
		fallback := definition.fallback
		if fallback.Mode == "" {
			fallback = defaultRejectFallback()
		}
		aliases := builtinCommandNames(definition)
		result = append(result, AgentCommandCapability{
			ID: definition.id, Invocation: "/" + definition.id, Aliases: aliases, Source: "builtin",
			Category: definition.category, Usage: definition.usage,
			Description: english.T(MsgKey(definition.id)), DescriptionZH: chinese.T(MsgKey(definition.id)),
			Parameters: append([]CapabilityParameter(nil), definition.parameters...), Permission: permission,
			PrivilegedWhen: append([]string(nil), definition.privilegedWhen...), ReadOnly: definition.readOnly,
			SideEffects: expandSideEffects(definition.effects), Fallback: fallback, Availability: availability,
		})
	}

	customs := e.commands.ListAll()
	sort.Slice(customs, func(i, j int) bool { return strings.ToLower(customs[i].Name) < strings.ToLower(customs[j].Name) })
	for _, custom := range customs {
		id := strings.ToLower(strings.TrimSpace(custom.Name))
		availability := available("Registered for the active project.", "已为当前项目注册。")
		permission := CapabilityPermissionMember
		if builtinIDs[id] {
			availability = unavailable("Shadowed by a built-in command with the same name.", "被同名内置命令遮蔽。")
		} else if disabled[id] {
			availability = unavailable("Disabled by the project-level command policy; user-role policy is checked at invocation time.", "已被项目级命令策略禁用；用户角色策略在真实调用时检查。")
		}
		description := strings.TrimSpace(redactFeedbackText(custom.Description))
		if description == "" {
			description = "Project-defined custom command"
		}
		effects := []string{"agent_turn"}
		kind := "prompt"
		if strings.TrimSpace(custom.Exec) != "" {
			kind = "exec"
			permission = CapabilityPermissionAdmin
			effects = []string{"shell_execution", "process_control", "filesystem_read", "filesystem_write", "network"}
			if availability.State == CapabilityAvailable {
				if strings.TrimSpace(adminFrom) == "" {
					availability = unavailable("Exec-backed custom commands require admin permission, but projects.admin_from is not configured.", "Exec 自定义命令需要管理员权限，但 projects.admin_from 尚未配置。")
				} else {
					availability = conditional("Exec-backed custom commands require projects.admin_from authorization at invocation time.", "Exec 自定义命令在调用时需要 projects.admin_from 授权。")
				}
			}
		}
		result = append(result, AgentCommandCapability{
			ID: id, Invocation: "/" + custom.Name, Source: "custom-" + custom.Source, Category: "custom",
			Usage: "/" + custom.Name + " [arguments]", Description: description, DescriptionZH: description,
			Parameters: []CapabilityParameter{capabilityParam("arguments", "string", false, "Arguments expanded into the registered "+kind+" template.", "展开到已注册 "+kind+" 模板中的参数。")},
			Permission: permission, ReadOnly: false, SideEffects: expandSideEffects(effects),
			Fallback: defaultRejectFallback(), Availability: availability,
		})
	}
	return result
}

func (e *Engine) commandCapabilityAvailability(id, probe string, snapshot capabilitySnapshot) CapabilityAvailability {
	switch probe {
	case "send":
		if snapshot.platform == nil {
			return conditional("Requires a session-bound messaging platform target.", "需要绑定会话的消息平台目标。")
		}
		if !e.attachmentSendEnabled {
			return conditional("A session target exists, but attachment_send disables image/file/audio/video side-channel delivery.", "已有会话目标，但 attachment_send 禁用了图片/文件/音频/视频旁路投递。")
		}
		return available("A session-bound messaging platform target is available.", "已有绑定会话的消息平台目标。")
	case "usage":
		if _, ok := e.agent.(UsageReporter); ok {
			return available("The active Agent exposes provider usage.", "当前 Agent 提供 Provider 用量能力。")
		}
		return unavailable("The active Agent does not implement usage reporting.", "当前 Agent 未实现用量报告。")
	case "tool_authorizer":
		if _, ok := e.agent.(ToolAuthorizer); ok {
			return available("The active Agent supports dynamic tool authorization.", "当前 Agent 支持动态工具授权。")
		}
		return unavailable("The active Agent does not implement dynamic tool authorization.", "当前 Agent 未实现动态工具授权。")
	case "model":
		if _, ok := e.agent.(ModelSwitcher); ok {
			if supporter, ok := e.agent.(PerTurnModelSupporter); ok && supporter.SupportsPerTurnModel() {
				return available("ModelSwitcher supports legacy project/workspace default switching. Group pinning additionally has PerTurnModelSupporter support; verified group identity, persistent storage, the current model list, and concrete session support are checked at invocation/execution time. Existing member and command policies apply.", "ModelSwitcher 支持原项目/工作区默认模型切换。群固定另已具备 PerTurnModelSupporter 支持；已验证群身份、持久化存储、当前模型列表和实际会话能力在调用/执行时检查。沿用现有成员权限和命令策略。")
			}
			return available("ModelSwitcher supports legacy project/workspace default switching. Group pinning is unavailable because the configured backend lacks PerTurnModelSupporter support; this does not disable legacy switching. Group reset can clear a saved binding when group identity and storage are available. Existing member and command policies apply.", "ModelSwitcher 支持原项目/工作区默认模型切换。配置的后端缺少 PerTurnModelSupporter 支持，群固定不可用，但不影响原切换能力。群身份已验证且存储可用时，group reset 可清除已有绑定。沿用现有成员权限和命令策略。")
		}
		return unavailable("The active Agent does not implement model switching.", "当前 Agent 未实现模型切换。")
	case "reasoning":
		if _, ok := e.agent.(ReasoningEffortSwitcher); ok {
			if supportsGroupReasoning(e.agent) {
				return available("Legacy project/workspace effort switching is available. Group effort additionally supports per-turn reasoning; verified group identity, persistent storage, effort catalog and concrete session/provider compatibility are checked at invocation/execution time.", "支持原项目/工作区推理强度切换；群 effort 另具备逐回合强度能力，已验证群身份、持久化存储、强度列表和实际会话/Provider 兼容性在调用/执行时检查。")
			}
			return available("Legacy project/workspace effort switching is available. Group effort is unavailable because per-turn reasoning support or a non-empty effort catalog is missing. A saved group binding can still be reset.", "支持原项目/工作区推理强度切换；缺少逐回合强度支持或非空强度列表，群 effort 不可用；仍可重置已有群绑定。")
		}
		return unavailable("The active Agent does not implement reasoning-effort switching.", "当前 Agent 未实现推理强度切换。")
	case "speed":
		if _, ok := e.agent.(ServiceTierCatalog); !ok {
			return unavailable("The active Agent does not implement model-aware speed switching.", "当前 Agent 未实现依据模型目录的速度切换。")
		}
		if snapshot.session != nil {
			if _, ok := snapshot.session.(TurnOptionsSession); !ok {
				return unavailable("The active session cannot apply per-turn settings.", "当前会话无法逐回合应用设置。")
			}
		}
		return conditional("Choices are validated against the current model catalog at invocation time; query-only without arguments. Requires member access and a session supporting turn options. Missing catalog or persistence errors are reported without success.", "调用时依据当前模型目录校验可选值；无参数时只查询。需要成员访问权限及支持回合设置的会话。目录缺失或保存失败会明确报错，不会误报成功。")
	case "mode":
		if _, ok := e.agent.(ModeSwitcher); ok {
			return available("The active Agent supports permission modes.", "当前 Agent 支持权限模式。")
		}
		return unavailable("The active Agent does not implement permission modes.", "当前 Agent 未实现权限模式。")
	case "provider":
		if _, ok := e.agent.(ProviderSwitcher); ok {
			return available("The active Agent supports provider switching.", "当前 Agent 支持 Provider 切换。")
		}
		return unavailable("The active Agent does not implement provider switching.", "当前 Agent 未实现 Provider 切换。")
	case "memory":
		if _, ok := e.agent.(MemoryFileProvider); ok {
			return available("The active Agent declares project/global memory files.", "当前 Agent 声明了项目/全局记忆文件。")
		}
		return unavailable("The active Agent does not expose a writable memory-file contract.", "当前 Agent 未提供可写记忆文件契约。")
	case "cron":
		if e.cronScheduler != nil {
			return available("The recurring scheduler is configured in this runtime.", "当前运行态已配置周期调度器。")
		}
		return unavailable("The recurring scheduler is not available in this runtime.", "当前运行态没有可用的周期调度器。")
	case "timer":
		if e.timerScheduler != nil {
			return available("The one-shot timer scheduler is configured in this runtime.", "当前运行态已配置一次性定时器。")
		}
		return unavailable("The one-shot timer scheduler is not available in this runtime.", "当前运行态没有可用的一次性定时器。")
	case "heartbeat":
		if e.heartbeatScheduler != nil {
			return available("Heartbeat scheduling is configured in this runtime.", "当前运行态已配置 Heartbeat 调度。")
		}
		return unavailable("Heartbeat scheduling is not available in this runtime.", "当前运行态没有可用的 Heartbeat 调度。")
	case "compress":
		if _, ok := e.agent.(ContextCompressor); ok {
			return available("The active Agent exposes a native context-compression command.", "当前 Agent 提供原生上下文压缩命令。")
		}
		return unavailable("The active Agent does not implement context compression.", "当前 Agent 未实现上下文压缩。")
	case "active_session":
		if snapshot.session != nil && snapshot.session.Alive() {
			return available("An active Agent session is bound to this query.", "当前查询已绑定活动 Agent 会话。")
		}
		return conditional("Requires an active Agent session.", "需要活动 Agent 会话。")
	case "feedback":
		e.feedbackMu.Lock()
		enabled := e.feedbackEnabled
		e.feedbackMu.Unlock()
		if enabled {
			return available("The chat feedback channel is enabled; an explicit /feedback command or Feedback card action directly submits a bounded, redacted adjacent diagnostic context with no preview or second confirmation.", "聊天反馈通道已启用；明确的 /feedback 命令或 Feedback 卡片动作会直接提交有界、脱敏的相邻诊断上下文，无需预览或二次确认。")
		}
		return unavailable("The feedback channel is disabled by configuration.", "反馈通道已被配置禁用。")
	case "agent_feedback":
		if !e.feedbackActive() {
			return unavailable("The feedback channel is disabled or has no valid Relay endpoint.", "反馈通道已禁用或没有有效的 Relay 端点。")
		}
		return conditional("Requires an exact active Agent turn credential and a separate one-time approval token bound to the trusted project, session, user, and immutable Draft.", "需要精确的活动 Agent 回合凭证，以及绑定可信项目、会话、用户和不可变 Draft 的独立一次性 approval token。")
	case "relay":
		if e.relayManager != nil {
			return available("The cross-project relay manager is configured.", "已配置跨项目 Relay 管理器。")
		}
		return unavailable("Cross-project relay is not configured.", "未配置跨项目 Relay。")
	case "workdir":
		if _, ok := e.agent.(WorkDirSwitcher); ok {
			return available("The active Agent supports working-directory switching.", "当前 Agent 支持工作目录切换。")
		}
		return unavailable("The active Agent does not implement working-directory switching.", "当前 Agent 未实现工作目录切换。")
	case "tts":
		if e.tts != nil && e.tts.Enabled && e.tts.TTS != nil {
			return available("A text-to-speech provider is configured.", "已配置文字转语音 Provider。")
		}
		return unavailable("Text-to-speech is not configured for this project.", "当前项目未配置文字转语音。")
	case "workspace":
		if e.multiWorkspace {
			return available("Multi-workspace mode is enabled for this project.", "当前项目已启用多工作区模式。")
		}
		return unavailable("Multi-workspace mode is not enabled for this project.", "当前项目未启用多工作区模式。")
	case "web":
		if e.webSetupFunc != nil || e.webStatusFunc != nil {
			return available("Web management setup/status callbacks are configured.", "已配置 Web 管理 setup/status 回调。")
		}
		return unavailable("Web management is not configured for chat setup.", "未配置聊天内 Web 管理设置能力。")
	case "ps":
		if snapshot.session != nil && snapshot.session.Alive() {
			return conditional("Available only while that session has a turn in flight; native steer or the documented legacy Send path is selected at invocation.", "仅在该会话有进行中回合时可用；调用时选择原生 steer 或已记录的旧 Send 路径。")
		}
		return conditional("Requires a live session with a turn in flight.", "需要存在进行中回合的活动会话。")
	case "deferred_restart":
		if snapshot.session != nil && snapshot.session.Alive() {
			return conditional("Requires that this exact session still has an Agent turn in flight when invoked.", "调用时要求该精确会话仍有进行中的 Agent 回合。")
		}
		return conditional("Requires a session-bound Agent turn; external terminals keep the ordinary supervisor restart behavior.", "需要绑定会话的 Agent 回合；外部终端保持普通 supervisor 重启行为。")
	default:
		return available(fmt.Sprintf("Built-in command %s is compiled into this runtime.", id), fmt.Sprintf("内置命令 %s 已编译进当前运行态。", id))
	}
}

func (e *Engine) agentSkillCapabilities(_ string) []AgentSkillCapability {
	disabled, _ := e.capabilityProjectPolicy()
	builtin := make(map[string]bool, len(builtinCommands))
	custom := make(map[string]bool)
	for _, definition := range builtinCommands {
		builtin[normalizeCommandName(definition.id)] = true
	}
	for _, command := range e.commands.ListAll() {
		custom[normalizeCommandName(command.Name)] = true
	}
	skills := e.skills.ListAll()
	sort.Slice(skills, func(i, j int) bool { return strings.ToLower(skills[i].Name) < strings.ToLower(skills[j].Name) })
	result := make([]AgentSkillCapability, 0, len(skills))
	for _, skill := range skills {
		id := normalizeCommandName(skill.Name)
		availability := available("Discovered from the active Agent's configured Skill directories.", "已从当前 Agent 配置的 Skill 目录发现。")
		switch {
		case builtin[id]:
			availability = unavailable("Shadowed by a built-in command with the same normalized name.", "被规范化名称相同的内置命令遮蔽。")
		case custom[id]:
			availability = unavailable("Shadowed by a project custom command with the same normalized name.", "被规范化名称相同的项目自定义命令遮蔽。")
		case disabled[strings.ToLower(skill.Name)]:
			availability = unavailable("Disabled by the project-level command policy; user-role policy is checked at invocation time.", "已被项目级命令策略禁用；用户角色策略在真实调用时检查。")
		}
		result = append(result, AgentSkillCapability{
			Name: skill.Name, DisplayName: redactFeedbackText(skill.DisplayName), Invocation: "/" + skill.Name + " [arguments]",
			Description: redactFeedbackText(skill.Description),
			Parameters:  []CapabilityParameter{capabilityParam("arguments", "string", false, "User arguments passed after the Skill instructions.", "追加在 Skill 指令之后的用户参数。")},
			Permission:  CapabilityPermissionMember, ReadOnly: false, SideEffects: expandSideEffects([]string{"agent_turn"}), Availability: availability,
		})
	}
	return result
}
