package core

import (
	"sort"
	"strings"
)

type agentToolDefinition struct {
	id          string
	invocation  string
	description string
	zh          string
	parameters  []CapabilityParameter
	permission  CapabilityPermissionLevel
	readOnly    bool
	effects     []string
	fallback    CapabilityFallback
	probe       string
}

var agentToolDefinitions = []agentToolDefinition{
	{
		id: "capability-manifest", invocation: `cc-connect-next capabilities [--search "keywords"] [--format markdown|json]`,
		description: "Query this runtime's unified, read-only Agent Capability Manifest.", zh: "查询当前运行态统一、只读的 Agent Capability Manifest。", readOnly: true,
		parameters: []CapabilityParameter{
			capabilityParam("search", "string", false, "Natural-language keywords used to filter every manifest section.", "用于过滤 Manifest 全部区段的自然语言关键词。"),
			capabilityParam("format", "string", false, "Output format.", "输出格式。", "markdown", "json"),
			capabilityParam("project", "string", false, "Project name; defaults to CC_PROJECT or the only configured project.", "项目名；默认使用 CC_PROJECT 或唯一项目。"),
			capabilityParam("session-key", "string", false, "Session context; defaults to CC_SESSION_KEY or the only active session.", "会话上下文；默认使用 CC_SESSION_KEY 或唯一活动会话。"),
		},
	},
	{
		id: "configuration-catalog", invocation: `cc-connect-next config capabilities [--search "keywords"] [--key path] [--format markdown|json]`,
		description: "Query the compiled configuration contract without reading current configured values.", zh: "查询已编译的配置契约，不读取当前配置值。", readOnly: true,
		parameters: []CapabilityParameter{
			capabilityParam("search", "string", false, "Natural-language configuration keywords.", "自然语言配置关键词。"),
			capabilityParam("key", "string", false, "Exact configuration path or option key.", "精确配置路径或配置键。"),
			capabilityParam("format", "string", false, "Output format.", "输出格式。", "markdown", "json"),
		},
	},
	{
		id: "send", invocation: "cc-connect-next send [options]",
		description: "Send generated attachments, native media, TTS, or an explicit side-channel message to the current chat.", zh: "向当前聊天发送生成附件、原生媒体、TTS 或明确的旁路消息。", probe: "send",
		parameters: []CapabilityParameter{
			capabilityParam("message", "string", false, "Optional visible text; ordinary Agent replies must not use this tool.", "可选可见文本；普通 Agent 回复不得使用此工具。"),
			{Name: "image", Type: "absolute-path", Repeatable: true, Description: "Generated image path.", DescriptionZH: "生成图片路径。"},
			{Name: "file", Type: "absolute-path", Repeatable: true, Description: "Generated file path.", DescriptionZH: "生成文件路径。"},
			{Name: "audio", Type: "absolute-path", Repeatable: true, Description: "Audio rendered natively when supported.", DescriptionZH: "支持时以原生方式渲染的音频。"},
			{Name: "video", Type: "absolute-path", Repeatable: true, Description: "Video rendered natively when supported.", DescriptionZH: "支持时以原生方式渲染的视频。"},
			capabilityParam("tts", "string", false, "Text to synthesize and send as speech.", "要合成并以语音发送的文本。"),
		},
		effects:  []string{"filesystem_read", "external_message", "network"},
		fallback: CapabilityFallback{Mode: "media-to-file", Description: "Audio/video fall back to file delivery when the active platform lacks a native media interface; unsupported attachment delivery returns an error.", DescriptionZH: "当前平台缺少原生媒体接口时，音频/视频退化为文件投递；不支持的附件投递会返回错误。"},
	},
	{
		id: "cron", invocation: "cc-connect-next cron <add|list|info|edit|exec|del> [options]",
		description: "Create and manage recurring prompt or shell jobs for the current project/session.", zh: "为当前项目/会话创建和管理周期 Prompt 或 Shell 任务。", probe: "cron",
		parameters: []CapabilityParameter{
			capabilityParam("action", "string", true, "Recurring-job operation.", "周期任务操作。", "add", "list", "info", "edit", "exec", "del"),
			capabilityParam("cron", "cron-expression", false, "Five-field schedule for add/edit.", "add/edit 使用的五段 Cron 表达式。"),
			capabilityParam("prompt", "string", false, "Agent request executed on schedule; mutually exclusive with exec.", "按计划执行的 Agent 请求；与 exec 互斥。"),
			capabilityParam("exec", "string", false, "Shell command executed on schedule; mutually exclusive with prompt.", "按计划执行的 Shell 命令；与 prompt 互斥。"),
			capabilityParam("job-id", "string", false, "Existing job identifier for info/edit/exec/del.", "info/edit/exec/del 使用的现有任务 ID。"),
		},
		effects: []string{"scheduled_state", "persistent_state", "agent_turn", "shell_execution", "external_message", "network"}, fallback: defaultRejectFallback(),
	},
	{
		id: "timer", invocation: "cc-connect-next timer <add|list|info|del> [options]",
		description: "Create and manage one-shot delayed prompt or shell jobs.", zh: "创建和管理一次性延迟 Prompt 或 Shell 任务。", probe: "timer",
		parameters: []CapabilityParameter{
			capabilityParam("action", "string", true, "One-shot timer operation.", "一次性定时器操作。", "add", "list", "info", "del"),
			capabilityParam("delay-or-at", "duration|timestamp", false, "Relative delay or absolute local/offset timestamp.", "相对延迟或带本地/时区偏移的绝对时间。"),
			capabilityParam("prompt", "string", false, "Agent request; mutually exclusive with exec.", "Agent 请求；与 exec 互斥。"),
			capabilityParam("exec", "string", false, "Shell command; mutually exclusive with prompt.", "Shell 命令；与 prompt 互斥。"),
			capabilityParam("timer-id", "string", false, "Existing timer identifier for info/del.", "info/del 使用的现有定时器 ID。"),
		},
		effects: []string{"scheduled_state", "persistent_state", "agent_turn", "shell_execution", "external_message", "network"}, fallback: defaultRejectFallback(),
	},
	{
		id: "relay", invocation: "cc-connect-next relay send --to <project> <message>",
		description: "Send a request to another configured project Agent and wait for its response.", zh: "向另一个已配置项目的 Agent 发送请求并等待响应。", probe: "relay",
		parameters: []CapabilityParameter{
			capabilityParam("to", "project-name", true, "Exact target project name from the relay binding.", "Relay 绑定中的精确目标项目名。"),
			capabilityParam("message", "string", true, "Request delivered to the target Agent.", "发送给目标 Agent 的请求。"),
		},
		effects: []string{"agent_turn", "relay_state", "external_message"}, fallback: defaultRejectFallback(),
	},
	{
		id: "agent-session-id", invocation: "cc-connect-next agent-sid",
		description: "Read the backing Agent session ID for the current project/session.", zh: "读取当前项目/会话对应的 Agent session ID。", readOnly: true, probe: "active_session",
	},
	{
		id: "doctor", invocation: "cc-connect-next doctor [--project <name>]",
		description: "Run side-effect-free configuration, dependency, Agent CLI, and platform preflight checks.", zh: "执行无副作用的配置、依赖、Agent CLI 与平台预检。", readOnly: true,
		parameters: []CapabilityParameter{capabilityParam("project", "string", false, "Optional project filter.", "可选项目过滤。")},
	},
	{
		id: "feedback", invocation: "cc-connect-next feedback <preview|submit> [options]",
		description: "Preview one complete redacted Foundation Draft, then submit that exact Draft once only when the current user explicitly authorized it.", zh: "先预览一份完整脱敏的 Foundation Draft；仅当当前用户明确授权后，才能一次性提交该精确 Draft。",
		permission: CapabilityPermissionLocalAgent, effects: []string{"external_service", "network"}, probe: "agent_feedback",
		parameters: []CapabilityParameter{
			capabilityParam("action", "string", true, "Feedback operation; preview never performs network I/O.", "反馈操作；preview 绝不发出网络请求。", "preview", "submit"),
			capabilityParam("description", "string", false, "Problem or missing capability for preview.", "用于生成预览的问题或缺失能力。"),
			capabilityParam("approval-token", "opaque-token", false, "One-time token returned by preview and accepted only for the initiating user/session from a live Agent turn.", "preview 返回的一次性 token，仅可由发起用户在同一会话的活动 Agent 回合中使用。"),
		},
		fallback: CapabilityFallback{
			Mode:          "chat-command",
			Description:   "The Agent path fails closed without a live turn credential or exact approval token; an explicit /feedback action in chat submits directly without a preview or second confirmation.",
			DescriptionZH: "缺少活动回合凭证或精确 approval token 时 Agent 路径会失败关闭；聊天内明确触发 /feedback 会直接提交，无需预览或二次确认。",
		},
	},
	{
		id: "daemon-restart", invocation: "cc-connect-next daemon restart",
		description: "Use a random turn-bound credential to schedule a graceful daemon restart after the current Agent turn and accepted queued messages finish.", zh: "使用随机回合凭证，在当前 Agent 回合及已接收排队消息完成后安排 daemon 优雅重启。",
		permission: CapabilityPermissionAdmin, effects: []string{"process_control"}, probe: "deferred_restart",
		fallback: CapabilityFallback{
			Mode:          "chat-command",
			Description:   "The Agent path fails closed when the active-turn runtime endpoint or authorization is unavailable; use the chat /restart command instead. It never falls back to an immediate supervisor restart.",
			DescriptionZH: "活动回合运行态端点或授权不可用时，Agent 路径会变更前失败；请改用聊天 /restart。绝不会退化成立即调用 supervisor 重启。",
		},
	},
}

func (e *Engine) agentToolCapabilities(snapshot capabilitySnapshot) []AgentToolCapability {
	result := make([]AgentToolCapability, 0, len(agentToolDefinitions))
	for _, definition := range agentToolDefinitions {
		availability := available("This Agent tool is compiled into the current build.", "该 Agent 工具已编译进当前构建。")
		if definition.probe != "" {
			availability = e.commandCapabilityAvailability(definition.id, definition.probe, snapshot)
		}
		permission := definition.permission
		if permission == "" {
			permission = CapabilityPermissionLocalAgent
		}
		if permission == CapabilityPermissionAdmin {
			_, adminFrom := e.capabilityProjectPolicy()
			if strings.TrimSpace(adminFrom) == "" {
				availability = unavailable("Requires admin permission, but projects.admin_from is not configured.", "需要管理员权限，但 projects.admin_from 尚未配置。")
			} else if availability.State != CapabilityUnavailable {
				availability = conditional("Requires an exact active Agent turn credential and projects.admin_from authorization; routing and caller identity are read from trusted Engine state at invocation time.", "需要精确的活动 Agent 回合凭证及 projects.admin_from 授权；路由与调用者身份在执行时从可信 Engine 状态读取。")
			}
		}
		fallback := definition.fallback
		if fallback.Mode == "" {
			fallback = defaultRejectFallback()
		}
		result = append(result, AgentToolCapability{
			ID: definition.id, Invocation: definition.invocation, Description: definition.description, DescriptionZH: definition.zh,
			Parameters: append([]CapabilityParameter(nil), definition.parameters...), Permission: permission,
			ReadOnly: definition.readOnly, SideEffects: expandSideEffects(definition.effects), Fallback: fallback, Availability: availability,
		})
	}
	return result
}

func (e *Engine) runtimeAdapterCapabilities(snapshot capabilitySnapshot, includeAll bool) []RuntimeAdapterCapabilities {
	result := []RuntimeAdapterCapabilities{e.agentRuntimeCapabilities()}
	if snapshot.session != nil && snapshot.session.Alive() {
		result = append(result, e.sessionRuntimeCapabilities(snapshot.session))
	} else {
		result = append(result, e.unboundSessionRuntimeCapabilities())
	}
	statuses := e.PlatformStatuses()
	statusByName := make(map[string]PlatformStatus, len(statuses))
	for _, status := range statuses {
		statusByName[status.Name] = status
	}
	platforms := append([]Platform(nil), e.platforms...)
	sort.Slice(platforms, func(i, j int) bool { return platforms[i].Name() < platforms[j].Name() })
	for _, platform := range platforms {
		status := statusByName[platform.Name()]
		result = append(result, e.platformRuntimeCapabilities(platform, status, snapshot))
	}
	if includeAll {
		activeAgents := map[string]bool{e.agent.Name(): true}
		for _, name := range e.configCatalog.Agents {
			if activeAgents[name] {
				continue
			}
			result = append(result, inactiveCompiledAdapter("agent", name))
		}
		activePlatforms := make(map[string]bool, len(platforms))
		for _, platform := range platforms {
			activePlatforms[platform.Name()] = true
		}
		for _, name := range e.configCatalog.Platforms {
			if activePlatforms[name] {
				continue
			}
			result = append(result, inactiveCompiledAdapter("platform", name))
		}
	}
	return result
}

func inactiveCompiledAdapter(kind, name string) RuntimeAdapterCapabilities {
	return RuntimeAdapterCapabilities{
		Kind: kind, Name: name, State: CapabilityUnavailable,
		Reason: "compiled into this build but not active in the current project",
		Capabilities: []RuntimeFeatureCapability{feature(
			"activation",
			"Configure this compiled adapter for the project.",
			"为项目配置该已编译适配器。",
			unavailable("The adapter is not configured in the current project.", "该适配器未配置到当前项目。"),
			CapabilityFallback{Mode: "configure-and-restart", Description: "Inspect its configuration with --all, add it to the project, and restart the runtime.", DescriptionZH: "使用 --all 查看其配置，把它加入项目并重启运行态。"},
		)},
	}
}

func feature(id, description, zh string, availability CapabilityAvailability, fallback CapabilityFallback) RuntimeFeatureCapability {
	if fallback.Mode == "" {
		fallback = CapabilityFallback{Mode: "none", Description: "No alternative behavior is declared.", DescriptionZH: "未声明替代行为。"}
	}
	return RuntimeFeatureCapability{ID: id, Description: description, DescriptionZH: zh, Availability: availability, Fallback: fallback}
}

func interfaceAvailability(ok bool, supported, supportedZH, unsupported, unsupportedZH string) CapabilityAvailability {
	if ok {
		return available(supported, supportedZH)
	}
	return unavailable(unsupported, unsupportedZH)
}

func (e *Engine) agentRuntimeCapabilities() RuntimeAdapterCapabilities {
	name := e.agent.Name()
	result := RuntimeAdapterCapabilities{Kind: "agent", Name: name, State: CapabilityAvailable}
	result.Capabilities = append(result.Capabilities,
		feature("persistent_sessions", "Start and resume persistent interactive Agent sessions.", "启动和恢复持久交互式 Agent 会话。", available("Required by the base Agent interface.", "由基础 Agent 接口保证。"), CapabilityFallback{}),
	)

	nativePrompt := false
	if supporter, ok := e.agent.(SystemPromptSupporter); ok {
		nativePrompt = supporter.HasSystemPromptSupport()
	}
	promptAvailability := unavailable("The Agent has neither native prompt injection nor a declared memory file.", "Agent 既不支持原生 Prompt 注入，也未声明记忆文件。")
	if nativePrompt {
		promptAvailability = available("The Agent natively injects cc-connect-next instructions.", "Agent 原生注入 cc-connect-next 指令。")
	} else if _, ok := e.agent.(MemoryFileProvider); ok {
		promptAvailability = conditional("Instructions can be installed into the Agent memory file with /cron setup or /bind setup.", "可通过 /cron setup 或 /bind setup 把指令安装到 Agent 记忆文件。")
	}
	result.Capabilities = append(result.Capabilities, feature("cc_connect_instructions", "Receive cc-connect-next tool and delivery instructions.", "接收 cc-connect-next 工具和投递指令。", promptAvailability, defaultRejectFallback()))

	type agentProbe struct {
		id, description, zh, fallback, fallbackZH string
		ok                                        bool
	}
	_, history := e.agent.(HistoryProvider)
	_, authorizer := e.agent.(ToolAuthorizer)
	_, providers := e.agent.(ProviderSwitcher)
	_, memory := e.agent.(MemoryFileProvider)
	_, models := e.agent.(ModelSwitcher)
	_, reasoning := e.agent.(ReasoningEffortSwitcher)
	_, usage := e.agent.(UsageReporter)
	_, compressor := e.agent.(ContextCompressor)
	_, commands := e.agent.(CommandProvider)
	_, skills := e.agent.(SkillProvider)
	_, deleter := e.agent.(SessionDeleter)
	_, titles := e.agent.(SessionTitleProvider)
	_, workdir := e.agent.(WorkDirSwitcher)
	_, modes := e.agent.(ModeSwitcher)
	probes := []agentProbe{
		{"history", "Read backing Agent conversation history.", "读取 Agent 后端会话历史。", "Engine-local history remains available.", "仍可使用 Engine 本地历史。", history},
		{"tool_authorization", "Dynamically pre-authorize Agent tools.", "动态预授权 Agent 工具。", "Use the Agent's configured permission mode.", "使用 Agent 已配置的权限模式。", authorizer},
		{"provider_switching", "Switch configured API providers.", "切换已配置 API Provider。", "No runtime provider switch is available.", "无法在运行态切换 Provider。", providers},
		{"memory_files", "Read or append project/global Agent memory files.", "读取或追加项目/全局 Agent 记忆文件。", "Memory commands are unavailable.", "记忆命令不可用。", memory},
		{"model_switching", "Switch the Agent model.", "切换 Agent 模型。", "Keep the constructor-configured model.", "保留构造时配置的模型。", models},
		{"reasoning_effort", "Switch reasoning effort.", "切换推理强度。", "Keep the Agent's configured/default effort.", "保留 Agent 已配置/默认的推理强度。", reasoning},
		{"usage_reporting", "Report provider quota usage.", "报告 Provider 配额用量。", "Usage reporting is unavailable.", "用量报告不可用。", usage},
		{"context_compression", "Compress active conversation context.", "压缩活动会话上下文。", "Start a new session when context must be reset.", "需要重置上下文时创建新会话。", compressor},
		{"custom_command_discovery", "Discover Agent-native command files.", "发现 Agent 原生命令文件。", "Only config-defined custom commands are available.", "仅可使用配置定义的自定义命令。", commands},
		{"skill_discovery", "Discover Agent Skills from SKILL.md.", "从 SKILL.md 发现 Agent Skills。", "No Agent Skill directories are declared.", "未声明 Agent Skill 目录。", skills},
		{"session_deletion", "Delete backing Agent sessions.", "删除 Agent 后端会话。", "Only cc-connect-next session metadata can be removed.", "只能移除 cc-connect-next 会话元数据。", deleter},
		{"session_titles", "Read backing Agent session titles.", "读取 Agent 后端会话标题。", "Use cc-connect-next local session names.", "使用 cc-connect-next 本地会话名。", titles},
		{"workdir_switching", "Switch the Agent working directory.", "切换 Agent 工作目录。", "Keep the configured working directory.", "保留已配置工作目录。", workdir},
		{"permission_modes", "Switch Agent permission modes.", "切换 Agent 权限模式。", "Keep the constructor-configured permission behavior.", "保留构造时配置的权限行为。", modes},
	}
	for _, probe := range probes {
		availability := interfaceAvailability(probe.ok,
			"The active Agent implements this capability.", "当前 Agent 实现了该能力。",
			"The active Agent does not implement this capability.", "当前 Agent 未实现该能力。")
		result.Capabilities = append(result.Capabilities, feature(probe.id, probe.description, probe.zh, availability,
			CapabilityFallback{Mode: "degrade", Description: probe.fallback, DescriptionZH: probe.fallbackZH}))
	}

	steerAvailability := unavailable("The active Agent does not declare native steer.", "当前 Agent 未声明原生 steer。")
	steerFallback := CapabilityFallback{Mode: "queue", Description: "Busy messages fall back to the session FIFO.", DescriptionZH: "忙时消息退化到会话 FIFO。"}
	if info, ok := e.agent.(NativeSteerDoctorInfo); ok {
		availableSteer, detail := info.NativeSteerStatus()
		if availableSteer {
			steerAvailability = available(detail, "当前 Agent 后端声明原生 steer 可用。")
		} else {
			steerAvailability = unavailable(detail, "当前 Agent 后端不支持原生 steer；忙时消息退化到 FIFO。")
		}
	}
	result.Capabilities = append(result.Capabilities, feature("native_steer", "Append input to the turn already in flight.", "把输入并入正在执行的回合。", steerAvailability, steerFallback))
	return result
}

type sessionRuntimeCapabilitySpec struct {
	id, description, zh                    string
	unboundDescription, unboundZH          string
	fallback, fallbackZH                   string
	unavailableReason, unavailableReasonZH string
	supported                              func(AgentSession) bool
}

var sessionRuntimeCapabilitySpecs = []sessionRuntimeCapabilitySpec{
	{id: "turn_options", description: "Apply model, reasoning, service tier, or answer profile to one turn.", zh: "为单个回合应用模型、推理强度、服务等级或回答档位。", unboundDescription: "Apply runtime settings to one turn.", unboundZH: "为单个回合应用运行时设置。", fallback: "Use persistent Agent defaults.", fallbackZH: "使用 Agent 持久默认值。", supported: func(session AgentSession) bool { _, ok := session.(TurnOptionsSession); return ok }},
	{id: "steer", description: "Append input to the active turn.", zh: "把输入并入当前回合。", unboundDescription: "Append input to the active turn.", unboundZH: "把输入并入当前回合。", fallback: "Use FIFO for ordinary busy messages; /ps rejects when it cannot safely supplement.", fallbackZH: "普通忙时消息使用 FIFO；/ps 无法安全补充时会拒绝。", supported: func(session AgentSession) bool { _, ok := session.(SteerableSession); return ok }},
	{id: "context_usage", description: "Report live context-window usage.", zh: "报告实时上下文窗口用量。", unboundDescription: "Report live context-window usage.", unboundZH: "报告实时上下文窗口用量。", fallback: "No live context percentage is shown.", fallbackZH: "不显示实时上下文百分比。", unavailableReason: "Session context usage is intentionally unavailable because no production tool or API reads a session-level value.", unavailableReasonZH: "会话上下文用量已明确不可用，因为当前没有生产工具或 API 读取会话级数值。"},
	{id: "cancel_turn", description: "Cancel the current turn without destroying the process.", zh: "取消当前回合而不销毁进程。", unboundDescription: "Cancel the active turn.", unboundZH: "取消活动回合。", fallback: "Close and recreate the Agent session when cancellation is required.", fallbackZH: "需要取消时关闭并重建 Agent 会话。", supported: func(session AgentSession) bool { _, ok := session.(AgentSessionCanceller); return ok }},
	{id: "live_mode", description: "Apply a permission-mode change to the live process.", zh: "把权限模式变更应用到活动进程。", unboundDescription: "Apply a permission-mode change live.", unboundZH: "实时应用权限模式变更。", fallback: "Apply the mode on the next session.", fallbackZH: "在下一个会话中应用模式。", supported: func(session AgentSession) bool { _, ok := session.(LiveModeSwitcher); return ok }},
	{id: "set_session_title", description: "Persist a title in the backing Agent session.", zh: "在 Agent 后端会话中持久化标题。", unboundDescription: "Persist a backing Agent session title.", unboundZH: "持久化 Agent 后端会话标题。", fallback: "Keep the title in cc-connect-next session metadata.", fallbackZH: "仅在 cc-connect-next 会话元数据中保存标题。", supported: func(session AgentSession) bool { _, ok := session.(SessionTitleSetter); return ok }},
	{id: "initial_session_title", description: "Initialize the backing Agent title from the first real request.", zh: "根据首个真实请求初始化 Agent 后端标题。", unboundDescription: "Initialize a fresh backing Agent title.", unboundZH: "初始化新 Agent 后端会话标题。", fallback: "Use the local fallback session title.", fallbackZH: "使用本地回退会话标题。", supported: func(session AgentSession) bool { _, ok := session.(InitialSessionTitleSetter); return ok }},
}

func (e *Engine) sessionRuntimeCapabilities(session AgentSession) RuntimeAdapterCapabilities {
	result := RuntimeAdapterCapabilities{Kind: "session", Name: e.agent.Name() + ":active", State: CapabilityAvailable}
	for _, item := range sessionRuntimeCapabilitySpecs {
		availability := unavailable(item.unavailableReason, item.unavailableReasonZH)
		if item.unavailableReason == "" {
			availability = interfaceAvailability(item.supported(session), "The active session implements this capability.", "当前活动会话实现了该能力。", "The active session does not implement this capability.", "当前活动会话未实现该能力。")
		}
		if item.id == "steer" {
			if info, ok := e.agent.(NativeSteerDoctorInfo); ok {
				native, detail := info.NativeSteerStatus()
				if native {
					availability = available(detail, "当前 Agent 后端声明原生 steer 可用。")
				} else {
					availability = unavailable(detail, "当前 Agent 后端不支持原生 steer。")
				}
			}
		}
		result.Capabilities = append(result.Capabilities, feature(item.id, item.description, item.zh, availability, CapabilityFallback{Mode: "degrade", Description: item.fallback, DescriptionZH: item.fallbackZH}))
	}
	return result
}

func (e *Engine) unboundSessionRuntimeCapabilities() RuntimeAdapterCapabilities {
	result := RuntimeAdapterCapabilities{Kind: "session", Name: e.agent.Name() + ":unbound", State: CapabilityConditional, Reason: "No active Agent session is bound to this query."}
	for _, item := range sessionRuntimeCapabilitySpecs {
		availability := conditional("Availability is resolved after an Agent session is active.", "Agent 会话启动后才能判断可用性。")
		fallback := defaultRejectFallback()
		if item.unavailableReason != "" {
			availability = unavailable(item.unavailableReason, item.unavailableReasonZH)
			fallback = CapabilityFallback{Mode: "degrade", Description: item.fallback, DescriptionZH: item.fallbackZH}
		}
		result.Capabilities = append(result.Capabilities, feature(item.id, item.unboundDescription, item.unboundZH,
			availability, fallback))
	}
	return result
}

func (e *Engine) platformRuntimeCapabilities(platform Platform, status PlatformStatus, snapshot capabilitySnapshot) RuntimeAdapterCapabilities {
	state := CapabilityAvailable
	reason := ""
	if status.Err != nil {
		state = CapabilityUnavailable
		reason = strings.TrimSpace(redactFeedbackText(status.Err.Error()))
	} else if !status.Usable() {
		state = CapabilityConditional
		reason = "waiting for platform readiness"
	}
	result := RuntimeAdapterCapabilities{Kind: "platform", Name: platform.Name(), State: state, Reason: reason}
	platformAvailability := func(ok bool) CapabilityAvailability {
		if state == CapabilityUnavailable {
			return unavailable("The platform transport is unavailable: "+reason, "平台传输当前不可用。")
		}
		if state == CapabilityConditional {
			return conditional("The adapter implements this capability but is still waiting for readiness.", "适配器实现了该能力，但仍在等待 Ready。")
		}
		return interfaceAvailability(ok, "The active platform adapter implements this capability.", "当前平台适配器实现了该能力。", "The active platform adapter does not implement this capability.", "当前平台适配器未实现该能力。")
	}
	result.Capabilities = append(result.Capabilities, feature("text", "Send and reply with text.", "发送和回复文本。", platformAvailability(true), CapabilityFallback{}))

	type platformProbe struct {
		id, description, zh, fallbackMode, fallback, fallbackZH string
		ok                                                      bool
	}
	_, rich := platform.(RichCardSupporter)
	_, cards := platform.(CardSender)
	_, streamingCards := platform.(StreamingCardPlatform)
	_, buttons := platform.(InlineButtonSender)
	_, images := platform.(ImageSender)
	_, files := platform.(FileSender)
	_, audio := platform.(AudioSender)
	_, video := platform.(VideoSender)
	_, update := platform.(MessageUpdater)
	_, typing := platform.(TypingIndicator)
	_, preview := platform.(PreviewStarter)
	_, mentions := platform.(AtMentionSender)
	_, reconstruct := platform.(ReplyContextReconstructor)
	_, replySnapshot := platform.(ReplyContextSnapshotter)
	_, directUser := platform.(DirectUserSender)
	_, directUserCard := platform.(DirectUserCardSender)
	_, recall := platform.(MessageRecallDetector)
	_, navigation := platform.(CardNavigable)
	var reported map[string]CapabilityAvailability
	if provider, ok := platform.(RuntimeCapabilityAvailabilityProvider); ok {
		var replyCtx any
		if snapshot.platform == platform {
			replyCtx = snapshot.replyCtx
		}
		reported = provider.RuntimeCapabilityAvailability(snapshot.sessionKey, replyCtx)
	}
	probes := []platformProbe{
		{"rich_answer_lifecycle", "Render the native rich answer lifecycle.", "渲染原生富回答生命周期。", "text", "Fall back to ordinary text/progress delivery.", "退化为普通文本/进度投递。", rich},
		{"structured_cards", "Send structured cards.", "发送结构化卡片。", "text", "Render the card as plain text.", "把卡片渲染为纯文本。", cards},
		{"streaming_cards", "Aggregate a turn into one updatable streaming card.", "把整个回合聚合到一张可更新流式卡片。", "messages", "Use ordinary progress and final messages.", "使用普通进度消息和最终消息。", streamingCards},
		{"buttons", "Send interactive buttons.", "发送交互按钮。", "text", "Send textual instructions without buttons.", "发送不含按钮的文本说明。", buttons},
		{"images", "Upload and send images.", "上传并发送图片。", "reject", "Return an unsupported attachment error.", "返回不支持附件的错误。", images},
		{"files", "Upload and send files.", "上传并发送文件。", "reject", "Return an unsupported attachment error.", "返回不支持附件的错误。", files},
		{"audio", "Send native audio/voice messages.", "发送原生音频/语音消息。", "file", "Use file delivery when FileSender is available.", "FileSender 可用时退化为文件投递。", audio},
		{"video", "Send native inline video messages.", "发送原生内联视频消息。", "file", "Use file delivery when FileSender is available.", "FileSender 可用时退化为文件投递。", video},
		{"message_update", "Update an existing message in place.", "原地更新已有消息。", "new-message", "Send a new/final message instead of patching.", "不原地更新，改为发送新消息/最终消息。", update},
		{"typing", "Show a typing or processing indicator.", "显示输入中或处理中指示。", "silent", "Continue without a typing indicator.", "不显示输入指示但继续处理。", typing},
		{"preview", "Start and update a streaming preview.", "启动并更新流式预览。", "final-only", "Deliver the final answer without a live preview.", "不显示实时预览，仅投递最终回答。", preview},
		{"mentions", "Send native mention notifications.", "发送原生 @ 通知。", "visible-text", "Render mention text without guaranteed notification semantics.", "显示 @ 文本，但不保证通知语义。", mentions},
		{"reply_context_reconstruction", "Reconstruct a proactive reply target from a session key.", "根据 session key 重建主动回复目标。", "reject", "Persistent proactive delivery is unavailable.", "持久主动投递不可用。", reconstruct},
		{"reply_target_snapshot", "Persist and restore a concrete reply target independently from session identity.", "将具体回复目标与 session identity 分离并持久恢复。", "session-key", "Fall back to session-key reconstruction when the target shape permits it.", "目标结构允许时退化为 session-key 重建。", replySnapshot},
		{"direct_user_messages", "Send proactive private messages to an explicit platform user ID without borrowing a recent chat/session.", "按明确的平台 user ID 主动私聊，不借用最近 chat/session。", "reject", "Reject when no explicit direct-user target exists; never fall back to a recent group or topic.", "没有明确私聊目标时拒绝；绝不回退最近群聊或话题。", directUser},
		{"direct_user_cards", "Send actionable structured cards to an explicit private user target.", "向明确的私聊用户目标发送可操作结构化卡片。", "direct-text", "Fall back to a private text message for the same explicit user.", "退化为发送给同一明确用户的私聊文本。", directUserCard},
		{"message_recall_detection", "Detect whether the triggering message was recalled.", "检测触发消息是否已撤回。", "best-effort", "Continue without recall detection.", "无法检测撤回时按尽力而为继续。", recall},
		{"card_navigation", "Navigate interactive cards in place.", "原地导航交互卡片。", "new-card", "Send a replacement card or text response.", "发送替换卡片或文本回复。", navigation},
	}
	for _, item := range probes {
		availability := platformAvailability(item.ok)
		if state == CapabilityAvailable {
			if dynamic, ok := reported[item.id]; ok {
				availability = dynamic
			}
		}
		result.Capabilities = append(result.Capabilities, feature(item.id, item.description, item.zh, availability, CapabilityFallback{Mode: item.fallbackMode, Description: item.fallback, DescriptionZH: item.fallbackZH}))
	}

	proactiveAvailability := platformAvailability(reconstruct)
	_, hasValidator := platform.(PersistentProactiveTargetValidator)
	_, requiresSnapshot := platform.(PersistentReplyTargetRequirer)
	if hasValidator || requiresSnapshot {
		if snapshot.sessionKey == "" {
			proactiveAvailability = conditional("A session key is required to validate persistent proactive delivery.", "需要 session key 才能验证持久主动投递。")
		} else if snapshot.platform == platform {
			if err := e.ValidatePersistentProactiveTarget(snapshot.sessionKey); err != nil {
				proactiveAvailability = unavailable(strings.TrimSpace(redactFeedbackText(err.Error())), "当前会话目标不支持持久主动投递。")
			}
		} else {
			proactiveAvailability = conditional("Select an active session on this platform to validate persistent proactive delivery.", "请选择该平台的活动会话以验证持久主动投递。")
		}
	}
	result.Capabilities = append(result.Capabilities, feature("persistent_proactive_delivery", "Persist a Cron/Timer target that remains deliverable after transport reconnection.", "持久化 Cron/Timer 目标，并在传输重连后仍可投递。", proactiveAvailability, CapabilityFallback{Mode: "reject", Description: "Job creation fails before persistence when the target cannot be reconstructed safely.", DescriptionZH: "目标无法安全重建时，在持久化前拒绝创建任务。"}))
	return result
}
