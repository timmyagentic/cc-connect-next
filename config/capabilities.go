package config

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/timmyagentic/cc-connect-next/core"
)

// CapabilityCatalog returns the complete configuration surface of this
// compiled build. Typed TOML options are discovered from the real config
// structs, while Agent and Platform options come from their build-tag-aware
// plugin registrations.
func CapabilityCatalog(version string) core.ConfigCatalog {
	catalog := core.ConfigCatalog{
		Version:      version,
		Capabilities: builtinCapabilities(),
		Options:      typedConfigOptions(),
	}
	agents, platforms := core.RegisteredConfigOptions()
	for name, options := range agents {
		catalog.Agents = append(catalog.Agents, name)
		catalog.Options = append(catalog.Options, options...)
	}
	for name, options := range platforms {
		catalog.Platforms = append(catalog.Platforms, name)
		catalog.Options = append(catalog.Options, options...)
	}
	sort.Strings(catalog.Agents)
	sort.Strings(catalog.Platforms)
	sort.Slice(catalog.Capabilities, func(i, j int) bool { return catalog.Capabilities[i].ID < catalog.Capabilities[j].ID })
	sort.Slice(catalog.Options, func(i, j int) bool {
		if catalog.Options[i].Path == catalog.Options[j].Path {
			return catalog.Options[i].Owner < catalog.Options[j].Owner
		}
		return catalog.Options[i].Path < catalog.Options[j].Path
	})
	return catalog
}

func SearchCapabilities(catalog core.ConfigCatalog, query string) core.ConfigCatalog {
	return core.SearchConfigCatalog(catalog, query)
}

func typedConfigOptions() []core.ConfigOption {
	var options []core.ConfigOption
	walkConfigType(reflect.TypeOf(Config{}), "", &options)
	return options
}

func walkConfigType(t reflect.Type, prefix string, options *[]core.ConfigOption) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := strings.Split(field.Tag.Get("toml"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		path := tag
		if prefix != "" {
			path = prefix + "." + tag
		}
		ft := field.Type
		for ft.Kind() == reflect.Pointer || ft.Kind() == reflect.Slice || ft.Kind() == reflect.Array {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Interface || (ft.Kind() == reflect.Map && ft.Elem().Kind() == reflect.Interface) {
			// Agent and Platform options are declared by their compiled adapter.
			continue
		}
		if ft.Kind() == reflect.Struct {
			walkConfigType(ft, path, options)
			continue
		}
		if ft.Kind() == reflect.Map {
			mt := ft.Elem()
			for mt.Kind() == reflect.Pointer || mt.Kind() == reflect.Slice || mt.Kind() == reflect.Array {
				mt = mt.Elem()
			}
			mapPath := path + ".<name>"
			if mt.Kind() == reflect.Struct {
				walkConfigType(mt, mapPath, options)
			} else {
				*options = append(*options, builtinConfigOption(mapPath, field.Type))
			}
			continue
		}
		*options = append(*options, builtinConfigOption(path, field.Type))
	}
}

type builtinOptionMeta struct {
	description, zh string
	defaultValue    string
	values          []string
	keywords        []string
	apply           core.ConfigApplyMode
	deprecated      bool
	example         string
}

func builtinConfigOption(path string, t reflect.Type) core.ConfigOption {
	meta := builtinMetadata(path)
	if meta.description == "" {
		meta.description, meta.zh = genericBuiltinDescription(path)
	}
	if meta.defaultValue == "" {
		if isSensitiveConfigPath(path) {
			meta.defaultValue = "unset"
		} else {
			meta.defaultValue = "unset / runtime default"
		}
	}
	if meta.apply == "" {
		meta.apply = defaultApplyMode(path)
	}
	key := path
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		key = path[idx+1:]
	}
	return core.ConfigOption{
		Path: path, Key: key, Scope: scopeForPath(path), Type: configTypeName(t), Default: meta.defaultValue,
		Values: meta.values, Description: meta.description, DescriptionZH: meta.zh, Keywords: meta.keywords,
		ApplyMode: meta.apply, Sensitive: isSensitiveConfigPath(path), Deprecated: meta.deprecated, Example: meta.example,
	}
}

func defaultApplyMode(path string) core.ConfigApplyMode {
	for _, prefix := range []string{
		"display.", "commands.", "aliases.", "projects.display.", "projects.auto_compress.",
		"projects.users.", "projects.agent.providers.",
	} {
		if strings.HasPrefix(path, prefix) {
			return core.ConfigApplyReload
		}
	}
	for _, exact := range []string{
		"attachment_send", "max_attachment_size_mb", "quiet", "banned_words", "instant_reply.enabled", "instant_reply.content",
		"projects.reset_on_idle_mins", "projects.inject_sender", "projects.filter_external_sessions",
		"projects.disabled_commands", "projects.admin_from", "projects.reply_footer", "projects.quiet",
	} {
		if path == exact {
			return core.ConfigApplyReload
		}
	}
	return core.ConfigApplyRestart
}

func scopeForPath(path string) core.ConfigScope {
	switch {
	case strings.HasPrefix(path, "projects.agent."):
		return core.ConfigScopeAgent
	case strings.HasPrefix(path, "projects.platforms."):
		return core.ConfigScopePlatform
	case strings.HasPrefix(path, "projects."):
		return core.ConfigScopeProject
	default:
		return core.ConfigScopeGlobal
	}
}

func configTypeName(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return configTypeName(t.Elem()) + "[]"
	case reflect.Map:
		return "table"
	default:
		return strings.ToLower(t.Kind().String())
	}
}

func isSensitiveConfigPath(path string) bool {
	lower := strings.ToLower(path)
	for _, marker := range []string{"api_key", "secret", "password", "token", "http_headers"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	if strings.Contains(lower, ".env.") || strings.Contains(lower, "http_headers") {
		return true
	}
	return false
}

func builtinMetadata(path string) builtinOptionMeta {
	meta := map[string]builtinOptionMeta{
		"language":                                         {description: "Choose the language used by bot messages, or detect it from the user's first message.", zh: "选择机器人消息语言，或从用户首条消息自动检测。", defaultValue: "zh", values: []string{"zh", "en", "zh-TW", "ja", "es", "auto"}, keywords: []string{"语言", "locale"}, apply: core.ConfigApplyRestart, example: `language = "zh"`},
		"data_dir":                                         {description: "Choose where cc-connect-next stores sessions, state, media, and runtime metadata.", zh: "选择 cc-connect-next 存储会话、状态、媒体和运行元数据的位置。", defaultValue: "~/.cc-connect-next", keywords: []string{"存储目录", "state"}},
		"attachment_send":                                  {description: "Allow or block Agent-initiated image and file send-back without disabling text replies.", zh: "允许或阻止 Agent 主动回传图片和文件，不影响文本回复。", defaultValue: "on", values: []string{"on", "off"}, keywords: []string{"附件回传", "send file"}, apply: core.ConfigApplyReload},
		"max_attachment_size_mb":                           {description: "Set the maximum size of one outbound attachment in MiB.", zh: "设置单个出站附件的最大大小（MiB）。", defaultValue: "50", keywords: []string{"大文件", "附件大小"}, apply: core.ConfigApplyReload},
		"update_notice":                                    {description: "Notify the most recently active chat once when a newer stable release is available.", zh: "有新稳定版时向最近活跃会话发送一次升级提醒。", defaultValue: "true", apply: core.ConfigApplyRestart},
		"provider_presets_url":                             {description: "Override the remote JSON source used for recommended provider presets.", zh: "覆盖推荐 Provider 预设使用的远程 JSON 地址。"},
		"banned_words":                                     {description: "Block messages containing any configured banned word.", zh: "阻止包含任一已配置违禁词的消息。", defaultValue: "[]", apply: core.ConfigApplyReload},
		"quiet":                                            {description: "Legacy switch that hides thinking and tool messages when newer display fields are unset.", zh: "旧版静默开关；未设置新版 Display 字段时隐藏思考和工具消息。", defaultValue: "false", apply: core.ConfigApplyReload, deprecated: true},
		"aliases.name":                                     {description: "Set the natural-language trigger for a command alias.", zh: "设置命令别名的自然语言触发词。", apply: core.ConfigApplyReload},
		"aliases.command":                                  {description: "Choose the slash command expanded by this alias.", zh: "选择该别名展开成的 Slash Command。", apply: core.ConfigApplyReload},
		"commands.name":                                    {description: "Set the custom slash-command name.", zh: "设置自定义 Slash Command 名称。", apply: core.ConfigApplyReload},
		"commands.description":                             {description: "Describe the custom command in menus and help.", zh: "在菜单和帮助中说明自定义命令。", apply: core.ConfigApplyReload},
		"commands.prompt":                                  {description: "Expand the custom command into an Agent prompt.", zh: "将自定义命令展开为 Agent Prompt。", apply: core.ConfigApplyReload},
		"commands.exec":                                    {description: "Execute a shell command instead of prompting the Agent.", zh: "执行 Shell 命令而不是向 Agent 发送 Prompt。", apply: core.ConfigApplyReload},
		"commands.work_dir":                                {description: "Override the working directory for a custom exec command.", zh: "覆盖自定义 Exec 命令的工作目录。", apply: core.ConfigApplyReload},
		"shell":                                            {description: "Choose the shell used by /shell, exec cron jobs, hooks, and webhook exec.", zh: "选择 /shell、exec Cron、Hooks 和 Webhook exec 使用的 Shell。", defaultValue: "sh on Unix; powershell.exe on Windows"},
		"shell_profile":                                    {description: "Prepend an initialization command to every configured shell command.", zh: "在每条配置的 Shell 命令前执行初始化命令。"},
		"idle_timeout_mins":                                {description: "Stop a turn when the Agent produces no events for this many minutes; zero disables it.", zh: "Agent 连续指定分钟无事件时终止回合；0 表示禁用。", defaultValue: "120", keywords: []string{"卡住", "空闲超时"}},
		"max_turn_time_mins":                               {description: "Cap the absolute wall-clock duration of one Agent turn; zero disables it.", zh: "限制单次 Agent 回合的绝对运行时长；0 表示禁用。", defaultValue: "0", keywords: []string{"最长回合", "wall clock"}},
		"workspace_idle_timeout_mins":                      {description: "Reap inactive multi-workspace engines after this many minutes; zero disables it.", zh: "多工作区引擎空闲指定分钟后回收；0 表示禁用。", defaultValue: "15"},
		"feedback.enabled":                                 {description: "Enable /feedback and capability-gap prompts; every submission still requires confirmation.", zh: "启用 /feedback 和能力缺口提示；每次提交仍需确认。", defaultValue: "true"},
		"feedback.endpoint":                                {description: "Override the author-operated anonymous feedback relay endpoint.", zh: "覆盖作者维护的匿名反馈中继地址。", defaultValue: "built-in author relay"},
		"log.level":                                        {description: "Set the minimum runtime log severity.", zh: "设置运行日志的最低严重级别。", defaultValue: "info", values: []string{"debug", "info", "warn", "error"}},
		"cron.silent":                                      {description: "Suppress the notification sent when a scheduled run starts.", zh: "禁止定时任务开始执行时的提示消息。", defaultValue: "false"},
		"cron.session_mode":                                {description: "Choose whether scheduled runs reuse a session or create a fresh session per run.", zh: "选择定时任务复用会话还是每次创建新会话。", defaultValue: "reuse", values: []string{"reuse", "new_per_run"}},
		"queue.max_depth":                                  {description: "Limit pending user messages queued behind one busy session.", zh: "限制一个忙碌会话后等待的用户消息数量。", defaultValue: "5"},
		"queue.busy_message_mode":                          {description: "Steer eligible input into the active turn or always preserve FIFO queueing.", zh: "将符合条件的新输入 steer 到当前回合，或始终保持 FIFO 排队。", defaultValue: "steer", values: []string{"steer", "queue"}, keywords: []string{"插队", "忙时消息", "追加问题"}},
		"webhook.enabled":                                  {description: "Expose the external HTTP endpoint that triggers Agent prompts or shell commands.", zh: "开放可触发 Agent 提示或 Shell 命令的外部 HTTP 端点。", defaultValue: "false"},
		"webhook.port":                                     {description: "Set the external webhook listening port.", zh: "设置外部 Webhook 监听端口。", defaultValue: "9111"},
		"webhook.path":                                     {description: "Set the external webhook URL path prefix.", zh: "设置外部 Webhook URL 路径前缀。", defaultValue: "/hook"},
		"bridge.enabled":                                   {description: "Enable the WebSocket/REST bridge for external platform adapters.", zh: "启用供外部平台适配器使用的 WebSocket/REST Bridge。", defaultValue: "false"},
		"bridge.port":                                      {description: "Set the external adapter bridge port.", zh: "设置外部适配器 Bridge 端口。", defaultValue: "9810"},
		"bridge.path":                                      {description: "Set the external adapter bridge WebSocket path.", zh: "设置外部适配器 Bridge WebSocket 路径。", defaultValue: "/bridge/ws"},
		"bridge.insecure":                                  {description: "Allow a tokenless bridge for local development only.", zh: "仅为本地开发允许无 Token Bridge。", defaultValue: "false", keywords: []string{"无认证", "insecure"}},
		"management.enabled":                               {description: "Enable the local management API and Web console backend.", zh: "启用本地管理 API 和 Web 控制台后端。", defaultValue: "false"},
		"management.port":                                  {description: "Set the management API listening port.", zh: "设置管理 API 监听端口。", defaultValue: "9820"},
		"display.mode":                                     {description: "Choose full, compact, or quiet reply presentation defaults.", zh: "选择 full、compact 或 quiet 回复展示默认模式。", defaultValue: "full (process messages remain off unless explicitly selected)", values: []string{"full", "compact", "quiet"}, keywords: []string{"隐藏思考", "安静模式"}, apply: core.ConfigApplyReload},
		"display.card_mode":                                {description: "Choose rich Card 2.0 or legacy card rendering where supported.", zh: "在支持的平台选择 Rich Card 2.0 或旧卡片渲染。", defaultValue: "rich", values: []string{"rich", "legacy"}, apply: core.ConfigApplyReload},
		"display.thinking_messages":                        {description: "Show or hide Agent reasoning progress messages.", zh: "显示或隐藏 Agent 思考进度消息。", defaultValue: "false", keywords: []string{"隐藏思考", "reasoning"}, apply: core.ConfigApplyReload},
		"display.tool_messages":                            {description: "Show or hide Agent tool-progress messages.", zh: "显示或隐藏 Agent 工具进度消息。", defaultValue: "false", keywords: []string{"隐藏工具", "tool progress"}, apply: core.ConfigApplyReload},
		"display.thinking_max_len":                         {description: "Limit reasoning-progress text length; zero disables truncation.", zh: "限制思考进度文本长度；0 表示不截断。", defaultValue: "300", apply: core.ConfigApplyReload},
		"display.tool_max_len":                             {description: "Limit tool-progress text length; zero disables truncation.", zh: "限制工具进度文本长度；0 表示不截断。", defaultValue: "500", apply: core.ConfigApplyReload},
		"display.history_max_len":                          {description: "Limit each /history entry; zero disables truncation.", zh: "限制每条 /history 记录长度；0 表示不截断。", defaultValue: "1000", apply: core.ConfigApplyReload},
		"display.reply_footer":                             {description: "Show the model, reasoning effort, and elapsed-time footer on completed replies.", zh: "在完成回复底部显示模型、推理强度和处理耗时。", defaultValue: "false", keywords: []string{"底部状态栏", "耗时", "footer"}, apply: core.ConfigApplyReload},
		"display.hide_agent_footer":                        {description: "Strip equivalent model/token/context footer lines emitted by the Agent itself.", zh: "移除 Agent 自己输出的等价模型、Token 和上下文状态行。", defaultValue: "false", apply: core.ConfigApplyReload},
		"display.show_context_indicator":                   {description: "Deprecated no-op retained only for old config compatibility.", zh: "已废弃的无效果配置，仅保留旧配置兼容。", defaultValue: "false", deprecated: true},
		"stream_preview.enabled":                           {description: "Update one preview message while the Agent is still streaming.", zh: "Agent 流式输出期间持续更新一条预览消息。", defaultValue: "true"},
		"stream_preview.interval_ms":                       {description: "Set the minimum interval between preview updates.", zh: "设置流式预览更新的最小间隔。", defaultValue: "1500"},
		"stream_preview.min_delta_chars":                   {description: "Require this many new characters before refreshing the preview.", zh: "至少新增指定字符数后才刷新预览。", defaultValue: "30"},
		"stream_preview.max_chars":                         {description: "Limit the accumulated streaming-preview length.", zh: "限制累计流式预览长度。", defaultValue: "2000"},
		"instant_reply.enabled":                            {description: "Immediately acknowledge an incoming message before Agent work begins.", zh: "收到消息后、Agent 开始工作前立即发送确认。", defaultValue: "false", apply: core.ConfigApplyReload},
		"instant_reply.content":                            {description: "Override the localized immediate acknowledgement text.", zh: "覆盖本地化即时确认文案。", apply: core.ConfigApplyReload},
		"rate_limit.max_messages":                          {description: "Limit inbound messages per user/session window; zero disables the limit.", zh: "限制每个用户/会话窗口内的入站消息数；0 表示禁用。", defaultValue: "20"},
		"rate_limit.window_secs":                           {description: "Set the inbound rate-limit window in seconds.", zh: "设置入站限流窗口秒数。", defaultValue: "60"},
		"outgoing_rate_limit.max_per_second":               {description: "Limit outgoing messages per second; zero means unlimited.", zh: "限制每秒出站消息数；0 表示不限。", defaultValue: "0"},
		"outgoing_rate_limit.burst":                        {description: "Set the maximum immediate outbound burst.", zh: "设置出站消息的最大瞬时突发数量。", defaultValue: "ceil(max_per_second)"},
		"relay.timeout_secs":                               {description: "Limit how long cross-project relay waits for a response; zero disables waiting.", zh: "限制跨项目 Relay 等待回复的时长；0 表示禁用等待。", defaultValue: "120"},
		"relay.visibility":                                 {description: "Choose how much relay activity is echoed into the group.", zh: "选择群内展示多少 Relay 活动。", defaultValue: "full", values: []string{"full", "summary", "none"}},
		"projects.name":                                    {description: "Give the project a unique name used by commands, storage, and relay routing.", zh: "设置供命令、存储和 Relay 路由使用的唯一项目名。"},
		"projects.mode":                                    {description: "Enable fixed-workspace or multi-workspace project routing.", zh: "选择固定工作区或多工作区项目路由。", defaultValue: "fixed", values: []string{"", "multi-workspace"}},
		"projects.base_dir":                                {description: "Set the parent directory for dynamically created multi-workspaces.", zh: "设置动态创建多工作区的父目录。"},
		"projects.skip_git":                                {description: "Allow multi-workspace directories that are not Git repositories.", zh: "允许多工作区目录不是 Git 仓库。", defaultValue: "false"},
		"projects.workspace_init_allow_local_paths":        {description: "Allow /workspace init to bind local directories in addition to Git URLs.", zh: "允许 /workspace init 除 Git URL 外绑定本地目录。", defaultValue: "false"},
		"projects.busy_message_mode":                       {description: "Override the process-wide busy-message policy for one project.", zh: "为单个项目覆盖进程级忙时消息策略。", defaultValue: "inherit", values: []string{"steer", "queue"}},
		"projects.reset_on_idle_mins":                      {description: "Rotate to a fresh session when the user returns after this idle period; zero disables it.", zh: "用户空闲指定时间后回来时切换到新会话；0 表示禁用。", defaultValue: "0"},
		"projects.run_as_user":                             {description: "Run this project's Agent as another non-root OS user.", zh: "以另一个非 root 操作系统用户运行当前项目 Agent。"},
		"projects.reply_footer":                            {description: "Override the reply footer for one project.", zh: "为单个项目覆盖回复底部状态栏。", defaultValue: "inherit", apply: core.ConfigApplyReload},
		"projects.show_context_indicator":                  {description: "Deprecated no-op retained only for old project config compatibility.", zh: "已废弃的无效果项目配置，仅保留旧配置兼容。", defaultValue: "false", deprecated: true},
		"projects.show_workdir_indicator":                  {description: "Deprecated no-op retained only for old project config compatibility.", zh: "已废弃的无效果项目配置，仅保留旧配置兼容。", defaultValue: "false", deprecated: true},
		"projects.workspace_idle_timeout_mins":             {description: "Deprecated project-level workspace reaper timeout; use the top-level option instead.", zh: "已废弃的项目级工作区回收超时；请改用顶层配置。", defaultValue: "inherit", deprecated: true},
		"projects.quiet":                                   {description: "Legacy per-project switch that hides thinking and tool messages when display overrides are unset.", zh: "旧版项目级静默开关；未设置 Display 覆盖时隐藏思考和工具消息。", defaultValue: "inherit", apply: core.ConfigApplyReload, deprecated: true},
		"projects.inject_sender":                           {description: "Prepend platform sender identity to prompts delivered to the Agent.", zh: "在发送给 Agent 的提示前添加平台发送者身份。", defaultValue: "false", apply: core.ConfigApplyReload},
		"projects.disabled_commands":                       {description: "Disable selected built-in commands for this project.", zh: "为当前项目禁用指定内置命令。", defaultValue: "[]", apply: core.ConfigApplyReload},
		"projects.admin_from":                              {description: "Restrict privileged commands to selected platform user IDs; unset blocks privileged commands for everyone.", zh: "将特权命令限制给指定平台用户 ID；未设置时所有人都不能执行特权命令。", defaultValue: "unset / nobody", keywords: []string{"管理员", "shell 权限"}, apply: core.ConfigApplyReload},
		"projects.filter_external_sessions":                {description: "Hide Agent sessions that were not created by cc-connect-next.", zh: "隐藏不是由 cc-connect-next 创建的 Agent 会话。", defaultValue: "false", apply: core.ConfigApplyReload},
		"projects.agent.type":                              {description: "Select the Agent adapter used by this project.", zh: "选择当前项目使用的 Agent 适配器。"},
		"projects.platforms.type":                          {description: "Select a messaging-platform adapter connected to this project.", zh: "选择连接当前项目的消息平台适配器。"},
		"projects.heartbeat.enabled":                       {description: "Wake the main session periodically for awareness or unfinished work.", zh: "定期唤醒主会话进行状态巡检或继续未完成工作。", defaultValue: "false"},
		"projects.heartbeat.interval_mins":                 {description: "Set the interval between heartbeat turns.", zh: "设置心跳回合间隔。", defaultValue: "30"},
		"projects.heartbeat.session_key":                   {description: "Choose the chat/session that receives heartbeat work.", zh: "选择接收心跳任务的会话。"},
		"projects.heartbeat.only_when_idle":                {description: "Run heartbeat only while the target session is idle.", zh: "仅在目标会话空闲时运行心跳。", defaultValue: "true"},
		"projects.heartbeat.prompt":                        {description: "Set the heartbeat prompt; empty reads HEARTBEAT.md from the Agent work directory.", zh: "设置心跳 Prompt；留空时从 Agent 工作目录读取 HEARTBEAT.md。", defaultValue: "HEARTBEAT.md"},
		"projects.heartbeat.silent":                        {description: "Suppress the heartbeat start notification.", zh: "隐藏心跳开始提示。", defaultValue: "true"},
		"projects.heartbeat.timeout_mins":                  {description: "Limit one heartbeat turn in minutes.", zh: "限制单次心跳回合的分钟数。", defaultValue: "30"},
		"projects.auto_compress.enabled":                   {description: "Automatically run context compression near the configured token threshold.", zh: "接近配置的 Token 阈值时自动执行上下文压缩。", defaultValue: "false", apply: core.ConfigApplyReload},
		"projects.auto_compress.max_tokens":                {description: "Set the estimated token threshold that triggers auto-compression.", zh: "设置触发自动压缩的估算 Token 阈值。", defaultValue: "12000", apply: core.ConfigApplyReload},
		"projects.auto_compress.min_gap_mins":              {description: "Set the minimum gap between automatic compression runs.", zh: "设置两次自动压缩之间的最小间隔分钟数。", defaultValue: "30", apply: core.ConfigApplyReload},
		"projects.users.default_role":                      {description: "Choose the role assigned to users not explicitly listed.", zh: "选择未显式列出用户的默认角色。", defaultValue: "member", apply: core.ConfigApplyReload},
		"hooks.async":                                      {description: "Run the hook asynchronously instead of blocking message handling.", zh: "异步运行 Hook，避免阻塞消息处理。", defaultValue: "true"},
		"projects.agent.answer_profiles.fast.service_tier": {description: "Override the model-catalog service tier for one-shot /fast answers.", zh: "为一次性 /fast 回答覆盖模型目录声明的服务档位。", defaultValue: "inherit", values: []string{"model-catalog-driven (for example: default, fast)"}},
		"projects.agent.answer_profiles.quality.service_tier": {description: "Override the model-catalog service tier for one-shot /quality answers.", zh: "为一次性 /quality 回答覆盖模型目录声明的服务档位。", defaultValue: "inherit", values: []string{"model-catalog-driven (for example: default, fast)"}},
		"providers.api_key":     {description: "Authenticate to a shared model provider.", zh: "认证共享模型 Provider。"},
		"providers.base_url":    {description: "Override the shared provider API base URL.", zh: "覆盖共享 Provider API 基础地址。"},
		"providers.agent_types": {description: "Restrict a shared provider to selected Agent adapter types.", zh: "将共享 Provider 限制给指定 Agent 适配器类型。"},
		"speech.enabled":        {description: "Transcribe incoming voice messages before sending them to the Agent.", zh: "将收到的语音消息转写后再发送给 Agent。", defaultValue: "false"},
		"speech.provider":       {description: "Choose the speech-to-text provider.", zh: "选择语音转文字 Provider。", values: []string{"openai", "groq", "qwen", "gemini"}},
		"tts.enabled":           {description: "Enable text-to-speech replies.", zh: "启用文字转语音回复。", defaultValue: "false"},
		"tts.provider":          {description: "Choose the text-to-speech provider.", zh: "选择文字转语音 Provider。", values: []string{"qwen", "openai", "minimax", "mimo", "espeak", "pico", "edge"}},
		"tts.tts_mode":          {description: "Choose voice-only replies or synthesize every eligible response.", zh: "选择仅语音触发时回复，或为每条符合条件的回复合成语音。", defaultValue: "voice_only", values: []string{"voice_only", "always"}},
	}
	if meta, ok := meta[path]; ok {
		return meta
	}

	// Project display fields share the same semantics as their global source.
	if strings.HasPrefix(path, "projects.display.") {
		if source, ok := meta[strings.TrimPrefix(path, "projects.")]; ok {
			source.defaultValue = "inherit"
			return source
		}
	}
	return builtinOptionMeta{}
}

func genericBuiltinDescription(path string) (string, string) {
	leaf := path
	parent := "global configuration"
	parentZH := "全局配置"
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		leaf = path[idx+1:]
		parent = strings.ReplaceAll(path[:idx], ".", " ")
		parentZH = path[:idx]
	}
	humanLeaf := strings.ReplaceAll(leaf, "_", " ")
	switch leaf {
	case "<name>":
		return fmt.Sprintf("Set one named entry in %s.", parent), fmt.Sprintf("设置 %s 中的一个命名条目。", parentZH)
	case "agent_types":
		return fmt.Sprintf("Restrict %s to the listed Agent adapter types.", parent), fmt.Sprintf("将 %s 限制给指定 Agent 适配器类型。", parentZH)
	case "alias":
		return fmt.Sprintf("Set a short user-facing alias for %s.", parent), fmt.Sprintf("为 %s 设置简短的用户可见别名。", parentZH)
	case "async":
		return fmt.Sprintf("Run %s asynchronously instead of blocking message handling.", parent), fmt.Sprintf("异步运行 %s，避免阻塞消息处理。", parentZH)
	case "burst":
		return fmt.Sprintf("Set the maximum burst size for %s.", parent), fmt.Sprintf("设置 %s 的最大突发数量。", parentZH)
	case "channel":
		return fmt.Sprintf("Choose the destination channel used by %s.", parent), fmt.Sprintf("选择 %s 使用的目标频道。", parentZH)
	case "command":
		return fmt.Sprintf("Set the shell command executed by %s.", parent), fmt.Sprintf("设置 %s 执行的 Shell 命令。", parentZH)
	case "config_file":
		return fmt.Sprintf("Override the auxiliary configuration-file path used by %s.", parent), fmt.Sprintf("覆盖 %s 使用的辅助配置文件路径。", parentZH)
	case "cors_origins":
		return fmt.Sprintf("Allow browser requests to %s from the listed CORS origins.", parent), fmt.Sprintf("允许列出的 CORS Origin 访问 %s。", parentZH)
	case "default_role":
		return fmt.Sprintf("Choose the role assigned to users not explicitly listed in %s.", parent), fmt.Sprintf("选择 %s 中未显式列出用户的默认角色。", parentZH)
	case "disabled_commands":
		return fmt.Sprintf("Disable the listed commands for %s.", parent), fmt.Sprintf("为 %s 禁用列出的命令。", parentZH)
	case "disabled_platforms":
		return fmt.Sprintf("Disable %s on the listed messaging platforms.", parent), fmt.Sprintf("在列出的消息平台上关闭 %s。", parentZH)
	case "display_path":
		return fmt.Sprintf("Choose the user-facing path rendered by %s.", parent), fmt.Sprintf("选择 %s 渲染给用户看的路径。", parentZH)
	case "enclosure_style":
		return fmt.Sprintf("Choose how %s encloses normalized references.", parent), fmt.Sprintf("选择 %s 包裹标准化引用的样式。", parentZH)
	case "enabled":
		return fmt.Sprintf("Enable or disable %s.", parent), fmt.Sprintf("启用或关闭 %s。", parentZH)
	case "env_key":
		return fmt.Sprintf("Name the environment variable from which %s reads its credential.", parent), fmt.Sprintf("指定 %s 读取凭据的环境变量名。", parentZH)
	case "event":
		return fmt.Sprintf("Choose the event that triggers %s.", parent), fmt.Sprintf("选择触发 %s 的事件。", parentZH)
	case "api_key":
		return fmt.Sprintf("Authenticate requests made by %s.", parent), fmt.Sprintf("认证 %s 发出的请求。", parentZH)
	case "base_url":
		return fmt.Sprintf("Override the service base URL for %s.", parent), fmt.Sprintf("覆盖 %s 的服务基础地址。", parentZH)
	case "model":
		return fmt.Sprintf("Select the model used by %s.", parent), fmt.Sprintf("选择 %s 使用的模型。", parentZH)
	case "language":
		return fmt.Sprintf("Set the language or locale hint used by %s.", parent), fmt.Sprintf("设置 %s 使用的语言或 Locale 提示。", parentZH)
	case "language_type":
		return fmt.Sprintf("Set the provider-specific language hint used by %s.", parent), fmt.Sprintf("设置 %s 使用的 Provider 专属语言提示。", parentZH)
	case "level":
		return fmt.Sprintf("Choose the verbosity or severity level for %s.", parent), fmt.Sprintf("选择 %s 的详细程度或严重级别。", parentZH)
	case "marker_style":
		return fmt.Sprintf("Choose the marker syntax emitted by %s.", parent), fmt.Sprintf("选择 %s 输出的标记语法。", parentZH)
	case "max_messages":
		return fmt.Sprintf("Limit how many messages %s accepts in one window.", parent), fmt.Sprintf("限制 %s 在一个窗口内接受的消息数。", parentZH)
	case "max_per_second":
		return fmt.Sprintf("Limit how many operations %s sends per second.", parent), fmt.Sprintf("限制 %s 每秒发送的操作数。", parentZH)
	case "max_text_len":
		return fmt.Sprintf("Skip or truncate %s beyond this text length; zero removes the limit.", parent), fmt.Sprintf("文本超过该长度时跳过或截断 %s；0 表示不限制。", parentZH)
	case "min_gap_mins":
		return fmt.Sprintf("Require this many minutes between repeated %s runs.", parent), fmt.Sprintf("限制两次 %s 运行之间至少间隔指定分钟。", parentZH)
	case "name":
		return fmt.Sprintf("Set the name used by %s.", parent), fmt.Sprintf("设置 %s 使用的名称。", parentZH)
	case "normalize_agents":
		return fmt.Sprintf("Apply %s normalization only to the listed Agent adapters.", parent), fmt.Sprintf("仅对列出的 Agent 适配器应用 %s 标准化。", parentZH)
	case "only_when_idle":
		return fmt.Sprintf("Run %s only while the target session is idle.", parent), fmt.Sprintf("仅在目标会话空闲时运行 %s。", parentZH)
	case "port":
		return fmt.Sprintf("Set the listening port for %s.", parent), fmt.Sprintf("设置 %s 的监听端口。", parentZH)
	case "prompt":
		return fmt.Sprintf("Set the Agent prompt used by %s.", parent), fmt.Sprintf("设置 %s 使用的 Agent Prompt。", parentZH)
	case "provider":
		return fmt.Sprintf("Select the provider used by %s.", parent), fmt.Sprintf("选择 %s 使用的 Provider。", parentZH)
	case "provider_refs":
		return fmt.Sprintf("Reference shared provider names from %s.", parent), fmt.Sprintf("从 %s 引用共享 Provider 名称。", parentZH)
	case "reasoning_effort":
		return fmt.Sprintf("Override reasoning effort for %s.", parent), fmt.Sprintf("为 %s 覆盖推理强度。", parentZH)
	case "render_platforms":
		return fmt.Sprintf("Render %s only on the listed messaging platforms.", parent), fmt.Sprintf("仅在列出的消息平台渲染 %s。", parentZH)
	case "run_as_env":
		return fmt.Sprintf("Allow the listed environment-variable names across %s user isolation.", parent), fmt.Sprintf("允许列出的环境变量名通过 %s 用户隔离边界。", parentZH)
	case "service_tier":
		return fmt.Sprintf("Override the model service tier for %s.", parent), fmt.Sprintf("为 %s 覆盖模型服务档位。", parentZH)
	case "shell":
		return fmt.Sprintf("Choose the shell used by %s.", parent), fmt.Sprintf("选择 %s 使用的 Shell。", parentZH)
	case "shell_profile":
		return fmt.Sprintf("Prepend a shell initialization command for %s.", parent), fmt.Sprintf("为 %s 添加 Shell 初始化命令。", parentZH)
	case "silent":
		return fmt.Sprintf("Suppress user-visible start notifications from %s.", parent), fmt.Sprintf("禁止 %s 的用户可见启动提示。", parentZH)
	case "speed":
		return fmt.Sprintf("Set the speech-speed multiplier used by %s.", parent), fmt.Sprintf("设置 %s 使用的语速倍率。", parentZH)
	case "thinking":
		return fmt.Sprintf("Choose the provider thinking mode used by %s.", parent), fmt.Sprintf("选择 %s 使用的 Provider 思考模式。", parentZH)
	case "timeout":
		return fmt.Sprintf("Set the execution timeout in seconds for %s.", parent), fmt.Sprintf("设置 %s 的执行超时秒数。", parentZH)
	case "timeout_mins":
		return fmt.Sprintf("Set the execution timeout in minutes for %s.", parent), fmt.Sprintf("设置 %s 的执行超时分钟数。", parentZH)
	case "token":
		return fmt.Sprintf("Authenticate %s with a shared token.", parent), fmt.Sprintf("使用共享 Token 认证 %s。", parentZH)
	case "type":
		return fmt.Sprintf("Choose the implementation type used by %s.", parent), fmt.Sprintf("选择 %s 使用的实现类型。", parentZH)
	case "url":
		return fmt.Sprintf("Set the HTTP URL called by %s.", parent), fmt.Sprintf("设置 %s 调用的 HTTP URL。", parentZH)
	case "user_ids":
		return fmt.Sprintf("Assign the listed platform user IDs to %s.", parent), fmt.Sprintf("将列出的平台用户 ID 分配给 %s。", parentZH)
	case "voice":
		return fmt.Sprintf("Choose the voice used by %s.", parent), fmt.Sprintf("选择 %s 使用的音色。", parentZH)
	case "voice_id":
		return fmt.Sprintf("Set the provider-specific voice ID used by %s.", parent), fmt.Sprintf("设置 %s 使用的 Provider 专属音色 ID。", parentZH)
	case "window_secs":
		return fmt.Sprintf("Set the rate-limit window length in seconds for %s.", parent), fmt.Sprintf("设置 %s 的限流窗口秒数。", parentZH)
	case "wire_api":
		return fmt.Sprintf("Select the wire protocol used by %s.", parent), fmt.Sprintf("选择 %s 使用的 Wire API 协议。", parentZH)
	default:
		return fmt.Sprintf("Configure %s for %s.", humanLeaf, parent), fmt.Sprintf("配置 %s 的 %s。", parentZH, leaf)
	}
}

func builtinCapabilities() []core.ConfigCapability {
	return []core.ConfigCapability{
		{ID: "access-control", Title: "Access control and roles", TitleZH: "访问控制与角色", Description: "Limit who can talk to the bot, who can run privileged commands, and how roles inherit limits.", DescriptionZH: "限制谁能与机器人对话、谁能执行特权命令，以及角色如何继承限流策略。", Keywords: []string{"只允许我", "白名单", "管理员", "权限", "roles"}, Paths: []string{"projects.platforms.options.allow_from", "projects.admin_from", "projects.users.default_role", "projects.users.roles.<name>.user_ids", "projects.users.roles.<name>.disabled_commands"}},
		{ID: "agent-execution", Title: "Agent execution", TitleZH: "Agent 执行", Description: "Choose the Agent, command, working directory, model, approval mode, prompts, and adapter-specific behavior.", DescriptionZH: "选择 Agent、命令、工作目录、模型、审批模式、提示词和适配器专属行为。", Keywords: []string{"agent", "模型", "yolo", "审批", "工作目录", "fast"}, Paths: []string{"projects.agent.type", "projects.agent.options.work_dir", "projects.agent.options.mode", "projects.agent.options.model", "projects.agent.options.reasoning_effort", "projects.agent.options.service_tier"}},
		{ID: "attachments-media", Title: "Attachments and media", TitleZH: "附件与媒体", Description: "Control file/image send-back, attachment limits, references, speech recognition, and voice replies.", DescriptionZH: "控制文件/图片回传、附件大小、引用、语音识别和语音回复。", Keywords: []string{"附件", "图片", "文件", "语音", "tts", "stt"}, Paths: []string{"attachment_send", "max_attachment_size_mb", "speech.enabled", "speech.provider", "tts.enabled", "tts.provider"}},
		{ID: "automation", Title: "Cron, timers, and heartbeat", TitleZH: "Cron、Timer 与心跳", Description: "Configure recurring defaults, one-shot task behavior, and periodic main-session awareness.", DescriptionZH: "配置周期任务默认值、一次性任务行为和主会话定期巡检。", Keywords: []string{"定时", "每天", "提醒", "heartbeat", "cron", "timer"}, Paths: []string{"cron.silent", "cron.session_mode", "projects.heartbeat.enabled", "projects.heartbeat.interval_mins", "projects.heartbeat.session_key"}},
		{ID: "customization", Title: "Commands, aliases, and content policy", TitleZH: "命令、别名与内容策略", Description: "Add prompt/exec commands, natural-language aliases, banned words, and per-project command restrictions.", DescriptionZH: "添加 Prompt/Exec 命令、自然语言别名、违禁词和项目级命令限制。", Keywords: []string{"自定义命令", "别名", "违禁词", "slash command"}, Paths: []string{"commands.name", "commands.prompt", "commands.exec", "aliases.name", "aliases.command", "banned_words", "projects.disabled_commands"}},
		{ID: "display", Title: "Message presentation", TitleZH: "消息展示", Description: "Control cards, reasoning/tool progress, streaming previews, immediate acknowledgements, history truncation, and reply footers.", DescriptionZH: "控制卡片、思考/工具进度、流式预览、即时确认、历史截断和回复底部状态栏。", Keywords: []string{"隐藏思考", "隐藏工具", "卡片模式", "卡片展示", "rich card", "card mode", "耗时", "footer", "quiet", "展示"}, Paths: []string{"display.mode", "display.card_mode", "display.thinking_messages", "display.tool_messages", "display.reply_footer", "stream_preview.enabled", "instant_reply.enabled"}},
		{ID: "external-interfaces", Title: "Webhook, Bridge, and management API", TitleZH: "Webhook、Bridge 与管理 API", Description: "Expose authenticated endpoints for automation, external adapters, and the Web management console.", DescriptionZH: "为自动化、外部适配器和 Web 管理台开放带认证的端点。", Keywords: []string{"webhook", "bridge", "管理 API", "web console", "cors"}, Paths: []string{"webhook.enabled", "bridge.enabled", "management.enabled", "hooks.event"}},
		{ID: "feedback-updates", Title: "Feedback and updates", TitleZH: "反馈与更新", Description: "Control anonymous feedback availability and one-time stable-version notifications.", DescriptionZH: "控制匿名反馈能力和稳定版一次性升级提醒。", Keywords: []string{"feedback", "反馈", "升级提醒"}, Paths: []string{"feedback.enabled", "feedback.endpoint", "update_notice"}},
		{ID: "multi-project", Title: "Projects and workspaces", TitleZH: "项目与工作区", Description: "Run multiple named projects, dynamically bind workspaces, isolate OS users, and reap idle workspaces.", DescriptionZH: "运行多个命名项目、动态绑定工作区、隔离 OS 用户并回收空闲工作区。", Keywords: []string{"多项目", "多工作区", "workspace", "用户隔离"}, Paths: []string{"projects.name", "projects.mode", "projects.base_dir", "projects.workspace_init_allow_local_paths", "projects.run_as_user", "workspace_idle_timeout_mins"}},
		{ID: "platform-connections", Title: "Messaging platforms", TitleZH: "消息平台", Description: "Connect and tune Feishu, Telegram, Discord, Slack, DingTalk, WeCom, Weixin, QQ, Matrix, and other compiled adapters.", DescriptionZH: "连接并调整飞书、Telegram、Discord、Slack、钉钉、企业微信、微信、QQ、Matrix 等当前构建内的平台适配器。", Keywords: []string{"飞书", "telegram", "discord", "slack", "代理", "群聊"}, Paths: []string{"projects.platforms.type", "projects.platforms.options.allow_from", "projects.platforms.options.proxy", "projects.platforms.options.group_reply_all", "projects.platforms.options.share_session_in_channel"}},
		{ID: "providers-models", Title: "Providers, models, and answer profiles", TitleZH: "Provider、模型与回答档位", Description: "Share provider credentials, route each Agent to endpoints/models, switch providers, and configure one-shot fast/quality answers.", DescriptionZH: "共享 Provider 凭据、为各 Agent 路由端点/模型、切换 Provider，并配置一次性 Fast/Quality 回答。", Keywords: []string{"provider", "服务商", "模型", "fast", "quality", "base url"}, Paths: []string{"providers.name", "providers.base_url", "providers.model", "projects.agent.provider_refs", "projects.agent.options.provider", "projects.agent.answer_profiles.fast.model", "projects.agent.answer_profiles.quality.reasoning_effort"}},
		{ID: "rate-limits", Title: "Inbound and outbound rate limits", TitleZH: "入站与出站限流", Description: "Protect sessions and platform APIs with global, role-based, and per-platform limits.", DescriptionZH: "通过全局、角色级和平台级限流保护会话与平台 API。", Keywords: []string{"限流", "rate limit", "频率"}, Paths: []string{"rate_limit.max_messages", "rate_limit.window_secs", "outgoing_rate_limit.max_per_second", "projects.users.roles.<name>.rate_limit.max_messages"}},
		{ID: "relay", Title: "Cross-project relay", TitleZH: "跨项目 Relay", Description: "Bind bots/projects together and control relay timeouts and group visibility.", DescriptionZH: "绑定机器人/项目，并控制 Relay 超时和群内可见性。", Keywords: []string{"relay", "跨项目", "机器人协作"}, Paths: []string{"relay.timeout_secs", "relay.visibility", "projects.platforms.options.peer_bots"}},
		{ID: "runtime-storage", Title: "Runtime, storage, and logs", TitleZH: "运行、存储与日志", Description: "Choose language, state directory, shell environment, log level, timeouts, and attachment limits.", DescriptionZH: "选择语言、状态目录、Shell 环境、日志级别、超时和附件限制。", Keywords: []string{"日志", "存储", "超时", "shell", "language"}, Paths: []string{"language", "data_dir", "log.level", "shell", "shell_profile", "idle_timeout_mins", "max_turn_time_mins"}},
		{ID: "session-lifecycle", Title: "Sessions, queueing, and context", TitleZH: "会话、排队与上下文", Description: "Control busy-message steering, queue depth, idle rotation, external-session visibility, and automatic compression.", DescriptionZH: "控制忙时 steer、排队深度、空闲轮换、外部会话可见性和自动压缩。", Keywords: []string{"忙时消息", "插队", "queue", "session", "自动压缩", "上下文"}, Paths: []string{"queue.max_depth", "queue.busy_message_mode", "projects.busy_message_mode", "projects.reset_on_idle_mins", "projects.filter_external_sessions", "projects.auto_compress.enabled"}},
		{ID: "shell-hooks", Title: "Shell, hooks, and event automation", TitleZH: "Shell、Hooks 与事件自动化", Description: "Select a shell and run command or HTTP hooks on message, session, cron, permission, and error events.", DescriptionZH: "选择 Shell，并在消息、会话、Cron、权限和错误事件上运行命令或 HTTP Hook。", Keywords: []string{"hooks", "事件", "shell command", "回调"}, Paths: []string{"shell", "shell_profile", "hooks.event", "hooks.type", "hooks.command", "hooks.url"}},
	}
}
