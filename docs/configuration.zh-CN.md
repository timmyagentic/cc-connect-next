<!-- Code generated from the compiled configuration catalog. DO NOT EDIT. -->

# cc-connect-next 配置能力参考

当前目录版本：`source`。本参考描述能力，不读取或显示本机配置值。

生效方式：`live` 表示当前运行态立即生效；`reload` 表示保存后可用 `/config reload` 应用；`new-session` 表示新 Agent 会话生效；`restart` 表示需要重启 cc-connect-next。

## 能力概览

### 访问控制与角色 (`access-control`)

限制谁能与机器人对话、谁能执行特权命令，以及角色如何继承限流策略。

相关配置： `projects.platforms.options.allow_from`, `projects.admin_from`, `projects.users.default_role`, `projects.users.roles.<name>.user_ids`, `projects.users.roles.<name>.disabled_commands`

### Agent 执行 (`agent-execution`)

选择 Agent、命令、工作目录、模型、审批模式、提示词和适配器专属行为。

相关配置： `projects.agent.type`, `projects.agent.options.work_dir`, `projects.agent.options.mode`, `projects.agent.options.model`, `projects.agent.options.reasoning_effort`, `projects.agent.options.service_tier`

### 附件与媒体 (`attachments-media`)

控制文件/图片回传、附件大小、引用、语音识别和语音回复。

相关配置： `attachment_send`, `max_attachment_size_mb`, `speech.enabled`, `speech.provider`, `tts.enabled`, `tts.provider`

### Cron、Timer 与心跳 (`automation`)

配置周期任务默认值、一次性任务行为和主会话定期巡检。

相关配置： `cron.silent`, `cron.session_mode`, `projects.heartbeat.enabled`, `projects.heartbeat.interval_mins`, `projects.heartbeat.session_key`

### 命令、别名与内容策略 (`customization`)

添加 Prompt/Exec 命令、自然语言别名、违禁词和项目级命令限制。

相关配置： `commands.name`, `commands.prompt`, `commands.exec`, `aliases.name`, `aliases.command`, `banned_words`, `projects.disabled_commands`

### 消息展示 (`display`)

控制卡片、思考/工具进度、流式预览、即时确认、历史截断和回复底部状态栏。

相关配置： `display.mode`, `display.card_mode`, `display.thinking_messages`, `display.tool_messages`, `display.reply_footer`, `stream_preview.enabled`, `instant_reply.enabled`

### Webhook、Bridge 与管理 API (`external-interfaces`)

为自动化、外部适配器和 Web 管理台开放带认证的端点。

相关配置： `webhook.enabled`, `bridge.enabled`, `management.enabled`, `hooks.event`

### 反馈与更新 (`feedback-updates`)

控制匿名反馈能力和稳定版一次性升级提醒。

相关配置： `feedback.enabled`, `feedback.endpoint`, `update_notice`

### 项目与工作区 (`multi-project`)

运行多个命名项目、动态绑定工作区、隔离 OS 用户并回收空闲工作区。

相关配置： `projects.name`, `projects.mode`, `projects.base_dir`, `projects.workspace_init_allow_local_paths`, `projects.run_as_user`, `workspace_idle_timeout_mins`

### 消息平台 (`platform-connections`)

连接并调整飞书、Telegram、Discord、Slack、钉钉、企业微信、微信、QQ、Matrix 等当前构建内的平台适配器。

相关配置： `projects.platforms.type`, `projects.platforms.options.allow_from`, `projects.platforms.options.proxy`, `projects.platforms.options.group_reply_all`, `projects.platforms.options.share_session_in_channel`

### 群聊回复与会话边界 (`platform-session-routing`)

控制群聊免 @ 回复，并选择用户、频道或平台话题是共享还是隔离 Agent 会话。

相关配置： `projects.platforms.options.group_reply_all`, `projects.platforms.options.share_session_in_channel`, `projects.platforms.options.thread_isolation`

### Provider、模型与回答档位 (`providers-models`)

共享 Provider 凭据、为各 Agent 路由端点/模型、切换 Provider，并配置一次性 Fast/Quality 回答。

相关配置： `providers.name`, `providers.base_url`, `providers.model`, `projects.agent.provider_refs`, `projects.agent.options.provider`, `projects.agent.answer_profiles.fast.model`, `projects.agent.answer_profiles.quality.reasoning_effort`

### 入站与出站限流 (`rate-limits`)

通过全局、角色级和平台级限流保护会话与平台 API。

相关配置： `rate_limit.max_messages`, `rate_limit.window_secs`, `outgoing_rate_limit.max_per_second`, `projects.users.roles.<name>.rate_limit.max_messages`

### 跨项目 Relay (`relay`)

绑定机器人/项目，并控制 Relay 超时和群内可见性。

相关配置： `relay.timeout_secs`, `relay.visibility`, `projects.platforms.options.peer_bots`

### 运行、存储与日志 (`runtime-storage`)

选择语言、状态目录、Shell 环境、日志级别、超时和附件限制。

相关配置： `language`, `data_dir`, `log.level`, `shell`, `shell_profile`, `idle_timeout_mins`, `max_turn_time_mins`

### 会话、排队与上下文 (`session-lifecycle`)

控制忙时 steer、排队深度、空闲轮换、外部会话可见性和自动压缩。

相关配置： `queue.max_depth`, `queue.busy_message_mode`, `projects.busy_message_mode`, `projects.reset_on_idle_mins`, `projects.filter_external_sessions`, `projects.auto_compress.enabled`

### Shell、Hooks 与事件自动化 (`shell-hooks`)

选择 Shell，并在消息、会话、Cron、权限和错误事件上运行命令或 HTTP Hook。

相关配置： `shell`, `shell_profile`, `hooks.event`, `hooks.type`, `hooks.command`, `hooks.url`

## 配置项参考

### `aliases.command`

选择该别名展开成的 Slash Command。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `aliases.name`

设置命令别名的自然语言触发词。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `attachment_send`

允许或阻止 Agent 主动回传图片和文件，不影响文本回复。

- 作用域：`global`
- 类型：`string`
- 默认值：`on`
- 生效方式：`reload`
- 允许值: `on`, `off`

### `banned_words`

阻止包含任一已配置违禁词的消息。

- 作用域：`global`
- 类型：`string[]`
- 默认值：`[]`
- 生效方式：`reload`

### `bridge.cors_origins`

允许列出的 CORS Origin 访问 bridge。

- 作用域：`global`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `bridge.enabled`

启用供外部平台适配器使用的 WebSocket/REST Bridge。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `bridge.insecure`

仅为本地开发允许无 Token Bridge。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `bridge.path`

设置外部适配器 Bridge WebSocket 路径。

- 作用域：`global`
- 类型：`string`
- 默认值：`/bridge/ws`
- 生效方式：`restart`

### `bridge.port`

设置外部适配器 Bridge 端口。

- 作用域：`global`
- 类型：`integer`
- 默认值：`9810`
- 生效方式：`restart`

### `bridge.token`

使用共享 Token 认证 bridge。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `commands.description`

在菜单和帮助中说明自定义命令。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `commands.exec`

执行 Shell 命令而不是向 Agent 发送 Prompt。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `commands.name`

设置自定义 Slash Command 名称。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `commands.prompt`

将自定义命令展开为 Agent Prompt。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `commands.work_dir`

覆盖自定义 Exec 命令的工作目录。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `cron.session_mode`

选择定时任务复用会话还是每次创建新会话。

- 作用域：`global`
- 类型：`string`
- 默认值：`reuse`
- 生效方式：`restart`
- 允许值: `reuse`, `new_per_run`

### `cron.silent`

禁止定时任务开始执行时的提示消息。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `data_dir`

选择 cc-connect-next 存储会话、状态、媒体和运行元数据的位置。

- 作用域：`global`
- 类型：`string`
- 默认值：`~/.cc-connect-next`
- 生效方式：`restart`

### `display.card_mode`

在支持的平台选择 Rich Card 2.0 或旧卡片渲染。

- 作用域：`global`
- 类型：`string`
- 默认值：`rich`
- 生效方式：`reload`
- 允许值: `rich`, `legacy`

### `display.hide_agent_footer`

移除 Agent 自己输出的等价模型、Token 和上下文状态行。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`

### `display.history_max_len`

限制每条 /history 记录长度；0 表示不截断。

- 作用域：`global`
- 类型：`integer`
- 默认值：`1000`
- 生效方式：`reload`

### `display.mode`

选择 full、compact 或 quiet 回复展示默认模式。

- 作用域：`global`
- 类型：`string`
- 默认值：`full (process messages remain off unless explicitly selected)`
- 生效方式：`reload`
- 允许值: `full`, `compact`, `quiet`

### `display.reply_footer`

在完成回复底部显示模型、推理强度和处理耗时。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`reload`

### `display.show_context_indicator`

已废弃的无效果配置，仅保留旧配置兼容。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`
- 状态：已废弃，仅保留兼容。

### `display.thinking_max_len`

限制思考进度文本长度；0 表示不截断。

- 作用域：`global`
- 类型：`integer`
- 默认值：`300`
- 生效方式：`reload`

### `display.thinking_messages`

显示或隐藏 Agent 思考进度消息。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`

### `display.tool_max_len`

限制工具进度文本长度；0 表示不截断。

- 作用域：`global`
- 类型：`integer`
- 默认值：`500`
- 生效方式：`reload`

### `display.tool_messages`

显示或隐藏 Agent 工具进度消息。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`

### `feedback.enabled`

启用 /feedback 和能力缺口提示；每次提交仍需确认。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `feedback.endpoint`

覆盖作者维护的匿名反馈中继地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`built-in author relay`
- 生效方式：`restart`

### `hooks.async`

异步运行 Hook，避免阻塞消息处理。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `hooks.command`

设置 hooks 执行的 Shell 命令。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `hooks.event`

选择触发 hooks 的事件。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `hooks.timeout`

设置 hooks 的执行超时秒数。

- 作用域：`global`
- 类型：`integer`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `hooks.type`

选择 hooks 使用的实现类型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `hooks.url`

设置 hooks 调用的 HTTP URL。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `idle_timeout_mins`

Agent 连续指定分钟无事件时终止回合；0 表示禁用。

- 作用域：`global`
- 类型：`integer`
- 默认值：`120`
- 生效方式：`restart`

### `instant_reply.content`

覆盖本地化即时确认文案。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `instant_reply.enabled`

收到消息后、Agent 开始工作前立即发送确认。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`

### `language`

选择机器人消息语言，或从用户首条消息自动检测。

- 作用域：`global`
- 类型：`string`
- 默认值：`zh`
- 生效方式：`restart`
- 允许值: `zh`, `en`, `zh-TW`, `ja`, `es`, `auto`
- 示例: `language = "zh"`

### `log.level`

设置运行日志的最低严重级别。

- 作用域：`global`
- 类型：`string`
- 默认值：`info`
- 生效方式：`restart`
- 允许值: `debug`, `info`, `warn`, `error`

### `management.cors_origins`

允许列出的 CORS Origin 访问 management。

- 作用域：`global`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `management.enabled`

启用本地管理 API 和 Web 控制台后端。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `management.port`

设置管理 API 监听端口。

- 作用域：`global`
- 类型：`integer`
- 默认值：`9820`
- 生效方式：`restart`

### `management.token`

使用共享 Token 认证 management。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `max_attachment_size_mb`

设置单个出站附件的最大大小（MiB）。

- 作用域：`global`
- 类型：`integer`
- 默认值：`50`
- 生效方式：`reload`

### `max_turn_time_mins`

限制单次 Agent 回合的绝对运行时长；0 表示禁用。

- 作用域：`global`
- 类型：`integer`
- 默认值：`0`
- 生效方式：`restart`

### `outgoing_rate_limit.burst`

设置出站消息的最大瞬时突发数量。

- 作用域：`global`
- 类型：`integer`
- 默认值：`ceil(max_per_second)`
- 生效方式：`restart`

### `outgoing_rate_limit.max_per_second`

限制每秒出站消息数；0 表示不限。

- 作用域：`global`
- 类型：`number`
- 默认值：`0`
- 生效方式：`restart`

### `outgoing_rate_limit.platforms.<name>.burst`

设置 outgoing_rate_limit.platforms.<name> 的最大突发数量。

- 作用域：`global`
- 类型：`integer`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `outgoing_rate_limit.platforms.<name>.max_per_second`

限制 outgoing_rate_limit.platforms.<name> 每秒发送的操作数。

- 作用域：`global`
- 类型：`number`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.admin_from`

将特权命令限制给指定平台用户 ID；未设置时所有人都不能执行特权命令。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / nobody`
- 生效方式：`reload`

### `projects.agent.answer_profiles.fast.model`

选择 projects.agent.answer_profiles.fast 使用的模型。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.agent.answer_profiles.fast.reasoning_effort`

为 projects.agent.answer_profiles.fast 覆盖推理强度。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.agent.answer_profiles.fast.service_tier`

为一次性 /fast 回答覆盖模型目录声明的服务档位。

- 作用域：`agent`
- 类型：`string`
- 默认值：`inherit`
- 生效方式：`restart`
- 允许值: `model-catalog-driven (for example: default, fast)`

### `projects.agent.answer_profiles.quality.model`

选择 projects.agent.answer_profiles.quality 使用的模型。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.agent.answer_profiles.quality.reasoning_effort`

为 projects.agent.answer_profiles.quality 覆盖推理强度。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.agent.answer_profiles.quality.service_tier`

为一次性 /quality 回答覆盖模型目录声明的服务档位。

- 作用域：`agent`
- 类型：`string`
- 默认值：`inherit`
- 生效方式：`restart`
- 允许值: `model-catalog-driven (for example: default, fast)`

### `projects.agent.options.agent` — `opencode`

选择 CLI 暴露的子 Agent 或配置档。

- 作用域：`agent` (`opencode`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.allowed_tools` — `claudecode`

在需要审批的模式中预先允许指定 Claude Code 工具。

- 作用域：`agent` (`claudecode`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.app_server_url` — `codex`

选择 Codex app-server 的传输端点。

- 作用域：`agent` (`codex`)
- 类型：`string`
- 默认值：`stdio`
- 生效方式：`restart`

### `projects.agent.options.append_system_prompt` — `claudecode, codex`

保留 Agent 默认系统提示并追加项目指令。

- 作用域：`agent` (`claudecode, codex`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.args` — `acp, devin`

向配置的 Agent 命令传递额外参数。

- 作用域：`agent` (`acp, devin`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.auth_method` — `acp`

选择 ACP Agent 使用的认证方式。

- 作用域：`agent` (`acp`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.auto_create` — `tmux`

配置的 tmux 会话不存在时自动创建。

- 作用域：`agent` (`tmux`)
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.agent.options.backend` — `codex`

选择 Codex 执行后端；app_server 支持原生 steer 与审批。

- 作用域：`agent` (`codex`)
- 类型：`string`
- 默认值：`app_server`
- 生效方式：`restart`
- 允许值: `app_server`, `exec`

### `projects.agent.options.cli_args_flag` — `claudecode`

指定包装器接收 Agent CLI 参数的标志名。

- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.cli_path` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

覆盖 Agent CLI 可执行文件路径。

- 作用域：`agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`string`
- 默认值：`adapter default`
- 生效方式：`restart`

### `projects.agent.options.cmd` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

覆盖 Agent 命令，可同时包含全局参数。

- 作用域：`agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`string`
- 默认值：`adapter default`
- 生效方式：`restart`

### `projects.agent.options.cmd_args_flag` — `claudecode`

指定包装器转发命令参数所使用的标志名。

- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.codex_home` — `codex`

为当前项目覆盖 CODEX_HOME，不修改用户全局 Codex 目录。

- 作用域：`agent` (`codex`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.command` — `acp, antigravity, claudecode, codex, copilot, cursor, devin, gemini, iflow, kimi, opencode, pi, qoder`

设置 Agent 可执行命令；多个适配器使用的别名。

- 作用域：`agent` (`acp, antigravity, claudecode, codex, copilot, cursor, devin, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`string`
- 默认值：`adapter default`
- 生效方式：`restart`

### `projects.agent.options.disallowed_tools` — `claudecode`

即使当前模式允许，也禁止指定 Claude Code 工具。

- 作用域：`agent` (`claudecode`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.display_name` — `acp, devin`

设置通用或 ACP Agent 的用户可见名称。

- 作用域：`agent` (`acp, devin`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.env` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

向 Agent 进程注入项目级环境变量。

- 作用域：`agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`table`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.agent.options.init_command` — `tmux`

发送 tmux 提示前运行初始化命令。

- 作用域：`agent` (`tmux`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.max_context_tokens` — `claudecode`

覆盖 Claude Code 可使用的最大上下文 token 数。

- 作用域：`agent` (`claudecode`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.mode` — `acp`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`acp`)
- 类型：`string`
- 默认值：`adapter default`
- 生效方式：`restart`

### `projects.agent.options.mode` — `antigravity`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`antigravity`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`
- 允许值: `default`, `yolo`, `plan`

### `projects.agent.options.mode` — `claudecode`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`
- 允许值: `default`, `acceptEdits`, `plan`, `auto`, `bypassPermissions`, `dontAsk`

### `projects.agent.options.mode` — `codex`

选择 Codex 审批与沙箱模式。省略该键时保留 suggest 兼容回落；全新生成的配置会显式写入 yolo。

- 作用域：`agent` (`codex`)
- 类型：`string`
- 默认值：`suggest when omitted; generated starter config writes yolo`
- 生效方式：`restart`
- 允许值: `suggest`, `auto-edit`, `full-auto`, `yolo`

### `projects.agent.options.mode` — `copilot`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`copilot`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`
- 允许值: `default`, `bypassPermissions`

### `projects.agent.options.mode` — `cursor`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`cursor`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`
- 允许值: `default`, `force`, `plan`, `ask`

### `projects.agent.options.mode` — `gemini`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`gemini`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`
- 允许值: `default`, `auto_edit`, `yolo`, `plan`

### `projects.agent.options.mode` — `iflow`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`iflow`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`
- 允许值: `default`, `auto-edit`, `plan`, `yolo`

### `projects.agent.options.mode` — `kimi`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`kimi`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`
- 允许值: `default`, `yolo`, `plan`, `quiet`

### `projects.agent.options.mode` — `opencode, pi, qoder`

选择 Agent 的审批、沙箱或规划模式。

- 作用域：`agent` (`opencode, pi, qoder`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`
- 允许值: `default`, `yolo`

### `projects.agent.options.model` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

选择新 Agent 会话的默认模型。

- 作用域：`agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.model_context_window` — `codex`

声明 Codex 模型上下文窗口，用于用量展示和压缩决策。

- 作用域：`agent` (`codex`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.pane` — `tmux`

选择用于 Agent 输入输出的 tmux pane。

- 作用域：`agent` (`tmux`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.plugin_dir` — `claudecode`

从指定目录加载 Claude Code 插件。

- 作用域：`agent` (`claudecode`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.poll_interval_ms` — `tmux`

设置 tmux 输出轮询间隔（毫秒）。

- 作用域：`agent` (`tmux`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.prompt_pattern` — `tmux`

用于识别 tmux Agent 提示符的正则表达式。

- 作用域：`agent` (`tmux`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.provider` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode`

选择当前项目使用的已配置 Provider。

- 作用域：`agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`reload`

### `projects.agent.options.reasoning_effort` — `claudecode`

设置新回合默认推理强度。

- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`
- 允许值: `low`, `medium`, `high`, `max`

### `projects.agent.options.reasoning_effort` — `codex`

设置新回合默认推理强度。

- 作用域：`agent` (`codex`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`
- 允许值: `low`, `medium`, `high`, `xhigh`, `max`

### `projects.agent.options.router_api_key` — `claudecode`

用于认证已配置的 Claude Code Router。

- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.agent.options.router_url` — `claudecode`

将 Claude Code 请求路由到指定 Router 地址。

- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.run_as_env` — `claudecode`

扩展跨 OS 用户隔离传递的环境变量白名单。

- 作用域：`agent` (`claudecode`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.run_as_user` — `claudecode`

以另一个非 root 操作系统用户运行 Claude Code。

- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.service_tier` — `codex`

选择模型目录声明的服务档位；Codex 常见取值包括 default 和 fast。

- 作用域：`agent` (`codex`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`
- 允许值: `model-catalog-driven (for example: default, fast)`

### `projects.agent.options.session` — `tmux`

指定承载 Agent 的 tmux 会话名。

- 作用域：`agent` (`tmux`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.session_title_model` — `codex`

可选使用隔离的本地 Codex 模型生成简洁的 Codex App 标题。

- 作用域：`agent` (`codex`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.session_title_prefix` — `codex`

为 Codex App 会话标题添加可配置的来源前缀。

- 作用域：`agent` (`codex`)
- 类型：`string`
- 默认值：`[飞书]`
- 生效方式：`restart`

### `projects.agent.options.shell` — `tmux`

选择 tmux 适配器使用的 shell。

- 作用域：`agent` (`tmux`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.startup_wait_ms` — `tmux`

创建 tmux 会话后等待指定毫秒数。

- 作用域：`agent` (`tmux`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.strip_input_block` — `tmux`

从捕获的 tmux 输出中移除回显输入块。

- 作用域：`agent` (`tmux`)
- 类型：`boolean`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.strip_patterns` — `tmux`

移除匹配指定模式的输出行。

- 作用域：`agent` (`tmux`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.system_prompt` — `claudecode, codex`

替换当前项目中 Agent 的默认系统提示。

- 作用域：`agent` (`claudecode, codex`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.thinking` — `pi`

配置 pi Agent 的思考模式或级别。

- 作用域：`agent` (`pi`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.timeout_mins` — `antigravity, gemini, kimi`

设置适配器进程超时分钟数；0 使用默认值。

- 作用域：`agent` (`antigravity, gemini, kimi`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.tool_timeout_secs` — `iflow`

设置 iFlow 工具调用最大等待秒数。

- 作用域：`agent` (`iflow`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.agent.options.window_per_session` — `tmux`

为每个 cc-connect-next 会话使用独立 tmux window。

- 作用域：`agent` (`tmux`)
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.agent.options.work_dir` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder, tmux`

设置 Agent 使用的项目工作目录。

- 作用域：`agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder, tmux`)
- 类型：`string`
- 默认值：`.`
- 生效方式：`restart`

### `projects.agent.provider_refs`

从 projects.agent 引用共享 Provider 名称。

- 作用域：`agent`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.agent.providers.agent_model_lists.<name>.alias`

为 projects.agent.providers.agent_model_lists.<name> 设置简短的用户可见别名。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.agent_model_lists.<name>.model`

选择 projects.agent.providers.agent_model_lists.<name> 使用的模型。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.agent_models.<name>`

设置 projects.agent.providers.agent_models 中的一个命名条目。

- 作用域：`agent`
- 类型：`table`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.agent_types`

将 projects.agent.providers 限制给指定 Agent 适配器类型。

- 作用域：`agent`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.api_key`

认证 projects.agent.providers 发出的请求。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.agent.providers.base_url`

覆盖 projects.agent.providers 的服务基础地址。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.codex.env_key`

指定 projects.agent.providers.codex 读取凭据的环境变量名。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.codex.http_headers.<name>`

设置 projects.agent.providers.codex.http_headers 中的一个命名条目。

- 作用域：`agent`
- 类型：`table`
- 默认值：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.agent.providers.codex.wire_api`

选择 projects.agent.providers.codex 使用的 Wire API 协议。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.endpoints.<name>`

设置 projects.agent.providers.endpoints 中的一个命名条目。

- 作用域：`agent`
- 类型：`table`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.env.<name>`

设置 projects.agent.providers.env 中的一个命名条目。

- 作用域：`agent`
- 类型：`table`
- 默认值：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.agent.providers.model`

选择 projects.agent.providers 使用的模型。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.models.alias`

为 projects.agent.providers.models 设置简短的用户可见别名。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.models.model`

选择 projects.agent.providers.models 使用的模型。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.name`

设置 projects.agent.providers 使用的名称。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.providers.thinking`

选择 projects.agent.providers 使用的 Provider 思考模式。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.agent.type`

选择当前项目使用的 Agent 适配器。

- 作用域：`agent`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.auto_compress.enabled`

接近配置的 Token 阈值时自动执行上下文压缩。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`

### `projects.auto_compress.max_tokens`

设置触发自动压缩的估算 Token 阈值。

- 作用域：`project`
- 类型：`integer`
- 默认值：`12000`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.auto_compress.min_gap_mins`

设置两次自动压缩之间的最小间隔分钟数。

- 作用域：`project`
- 类型：`integer`
- 默认值：`30`
- 生效方式：`reload`

### `projects.base_dir`

设置动态创建多工作区的父目录。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.busy_message_mode`

为单个项目覆盖进程级忙时消息策略。

- 作用域：`project`
- 类型：`string`
- 默认值：`inherit`
- 生效方式：`restart`
- 允许值: `steer`, `queue`

### `projects.disabled_commands`

为当前项目禁用指定内置命令。

- 作用域：`project`
- 类型：`string[]`
- 默认值：`[]`
- 生效方式：`reload`

### `projects.display.card_mode`

在支持的平台选择 Rich Card 2.0 或旧卡片渲染。

- 作用域：`project`
- 类型：`string`
- 默认值：`inherit`
- 生效方式：`reload`
- 允许值: `rich`, `legacy`

### `projects.display.hide_agent_footer`

移除 Agent 自己输出的等价模型、Token 和上下文状态行。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`inherit`
- 生效方式：`reload`

### `projects.display.history_max_len`

限制每条 /history 记录长度；0 表示不截断。

- 作用域：`project`
- 类型：`integer`
- 默认值：`inherit`
- 生效方式：`reload`

### `projects.display.mode`

选择 full、compact 或 quiet 回复展示默认模式。

- 作用域：`project`
- 类型：`string`
- 默认值：`inherit`
- 生效方式：`reload`
- 允许值: `full`, `compact`, `quiet`

### `projects.display.reply_footer`

在完成回复底部显示模型、推理强度和处理耗时。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`inherit`
- 生效方式：`reload`

### `projects.display.show_context_indicator`

已废弃的无效果配置，仅保留旧配置兼容。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`inherit`
- 生效方式：`reload`
- 状态：已废弃，仅保留兼容。

### `projects.display.thinking_max_len`

限制思考进度文本长度；0 表示不截断。

- 作用域：`project`
- 类型：`integer`
- 默认值：`inherit`
- 生效方式：`reload`

### `projects.display.thinking_messages`

显示或隐藏 Agent 思考进度消息。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`inherit`
- 生效方式：`reload`

### `projects.display.tool_max_len`

限制工具进度文本长度；0 表示不截断。

- 作用域：`project`
- 类型：`integer`
- 默认值：`inherit`
- 生效方式：`reload`

### `projects.display.tool_messages`

显示或隐藏 Agent 工具进度消息。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`inherit`
- 生效方式：`reload`

### `projects.filter_external_sessions`

隐藏不是由 cc-connect-next 创建的 Agent 会话。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`

### `projects.heartbeat.enabled`

定期唤醒主会话进行状态巡检或继续未完成工作。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.heartbeat.interval_mins`

设置心跳回合间隔。

- 作用域：`project`
- 类型：`integer`
- 默认值：`30`
- 生效方式：`restart`

### `projects.heartbeat.only_when_idle`

仅在目标会话空闲时运行心跳。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `projects.heartbeat.prompt`

设置心跳 Prompt；留空时从 Agent 工作目录读取 HEARTBEAT.md。

- 作用域：`project`
- 类型：`string`
- 默认值：`HEARTBEAT.md`
- 生效方式：`restart`

### `projects.heartbeat.session_key`

选择接收心跳任务的会话。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.heartbeat.silent`

隐藏心跳开始提示。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `projects.heartbeat.timeout_mins`

限制单次心跳回合的分钟数。

- 作用域：`project`
- 类型：`integer`
- 默认值：`30`
- 生效方式：`restart`

### `projects.inject_sender`

在发送给 Agent 的提示前添加平台发送者身份。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`

### `projects.mode`

选择固定工作区或多工作区项目路由。

- 作用域：`project`
- 类型：`string`
- 默认值：`fixed`
- 生效方式：`restart`
- 允许值: ``, `multi-workspace`

### `projects.name`

设置供命令、存储和 Relay 路由使用的唯一项目名。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.observe.channel`

选择 projects.observe 使用的目标频道。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.observe.enabled`

启用或关闭 projects.observe。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.platforms.options.access_token` — `matrix`

认证 Matrix 机器人账号。

- 作用域：`platform` (`matrix`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.account_id` — `weixin`

为多个微信账号隔离持久化状态。

- 作用域：`platform` (`weixin`)
- 类型：`string`
- 默认值：`default`
- 生效方式：`restart`

### `projects.platforms.options.agent_id` — `dingtalk, wecom`

指定平台租户中的机器人应用 Agent ID。

- 作用域：`platform` (`dingtalk, wecom`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.allow_chat` — `feishu, lark`

将飞书访问限制在指定会话 ID。

- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.allow_from` — `dingtalk, discord, feishu, lark, line, matrix, max, qq, qqbot, slack, telegram, webex, wecom, weibo, weixin, wps-xiezuo`

将机器人访问限制在指定平台用户 ID；留空或 '*' 表示允许所有平台用户。

- 作用域：`platform` (`dingtalk, discord, feishu, lark, line, matrix, max, qq, qqbot, slack, telegram, webex, wecom, weibo, weixin, wps-xiezuo`)
- 类型：`string`
- 默认值：`empty / allow all platform users`
- 生效方式：`restart`

### `projects.platforms.options.api_base` — `max`

覆盖平台 REST API 基础地址。

- 作用域：`platform` (`max`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.api_base_url` — `wecom`

覆盖平台 API 基础地址。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.app_id` — `feishu, lark, qqbot, weibo, wps-xiezuo`

标识机器人应用。

- 作用域：`platform` (`feishu, lark, qqbot, weibo, wps-xiezuo`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.app_secret` — `feishu, lark, qqbot, weibo, wps-xiezuo`

认证机器人应用。

- 作用域：`platform` (`feishu, lark, qqbot, weibo, wps-xiezuo`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.app_token` — `slack`

认证 Slack Socket Mode。

- 作用域：`platform` (`slack`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.auto_join` — `matrix`

自动加入受邀的 Matrix 房间。

- 作用域：`platform` (`matrix`)
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `projects.platforms.options.auto_verify` — `matrix`

自动接受 Matrix SAS 设备验证。

- 作用域：`platform` (`matrix`)
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `projects.platforms.options.base_url` — `weixin, wps-xiezuo`

覆盖平台服务基础地址。

- 作用域：`platform` (`weixin, wps-xiezuo`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.bot_id` — `wecom`

标识企业微信 WebSocket 机器人。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.bot_secret` — `wecom`

认证企业微信 WebSocket 机器人。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.bot_token` — `slack`

认证 Slack 机器人用户。

- 作用域：`platform` (`slack`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.burst_limit` — `weixin`

限制一个窗口内独立发送的微信消息数量。

- 作用域：`platform` (`weixin`)
- 类型：`integer`
- 默认值：`4`
- 生效方式：`restart`

### `projects.platforms.options.burst_window_secs` — `weixin`

设置微信出站突发窗口长度（秒）。

- 作用域：`platform` (`weixin`)
- 类型：`integer`
- 默认值：`86400`
- 生效方式：`restart`

### `projects.platforms.options.callback_aes_key` — `wecom`

解密企业微信回调负载。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.callback_path` — `feishu, lark, line, wecom`

设置入站 Webhook 回调路径。

- 作用域：`platform` (`feishu, lark, line, wecom`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.callback_token` — `wecom`

验证企业微信回调请求。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.card_template_id` — `dingtalk`

选择钉钉互动卡片模板 ID。

- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.card_template_key` — `dingtalk`

选择钉钉卡片模板 Key。

- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.card_throttle_ms` — `dingtalk`

限制钉钉卡片更新频率（毫秒）。

- 作用域：`platform` (`dingtalk`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.cdn_base_url` — `weixin`

覆盖微信 CDN 下载/上传基础地址。

- 作用域：`platform` (`weixin`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.channel_secret` — `line`

验证 LINE Webhook 签名。

- 作用域：`platform` (`line`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.channel_token` — `line`

认证 LINE Messaging API 请求。

- 作用域：`platform` (`line`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.clean_reply` — `wps-xiezuo`

从 WPS 回复中移除思考和工具进度行。

- 作用域：`platform` (`wps-xiezuo`)
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.platforms.options.client_id` — `dingtalk`

标识钉钉应用客户端。

- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.client_secret` — `dingtalk`

认证钉钉应用客户端。

- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.corp_id` — `wecom`

标识企业微信企业。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.corp_secret` — `wecom`

认证企业微信应用。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.cross_signing_password` — `matrix`

服务器需要账号密码时初始化 Matrix 跨签名。

- 作用域：`platform` (`matrix`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.domain` — `feishu, lark`

覆盖飞书/Lark API 与 WebSocket 域名。

- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.done_emoji` — `dingtalk, feishu, lark`

选择完成表情；'none' 表示关闭。

- 作用域：`platform` (`dingtalk, feishu, lark`)
- 类型：`string`
- 默认值：`Done`
- 生效方式：`restart`

### `projects.platforms.options.enable_feishu_card` — `feishu, lark`

启用飞书互动卡片回复。

- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `projects.platforms.options.enable_markdown` — `wecom`

启用企业微信回复的 Markdown 格式。

- 作用域：`platform` (`wecom`)
- 类型：`boolean`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.enable_reactions` — `telegram`

为收到的消息添加处理中表情。

- 作用域：`platform` (`telegram`)
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.platforms.options.encrypt_key` — `feishu, lark`

解密飞书 Webhook 事件。

- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.group_only` — `feishu, lark`

仅接受飞书群聊消息。

- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.platforms.options.group_reply_all` — `discord, feishu, lark, matrix, telegram`

无需 @ 即回复所有群消息。

- 作用域：`platform` (`discord, feishu, lark, matrix, telegram`)
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.platforms.options.group_reply_all_chats` — `feishu, lark`

仅在指定飞书会话中启用无需 @ 的回复。

- 作用域：`platform` (`feishu, lark`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.group_reply_all_guilds` — `discord`

仅在指定 Discord 服务器中启用无需 @ 的回复。

- 作用域：`platform` (`discord`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.guild_id` — `discord`

将 Discord 命令注册限制到单个服务器以加快生效。

- 作用域：`platform` (`discord`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.homeserver` — `matrix`

设置 Matrix Homeserver 地址。

- 作用域：`platform` (`matrix`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.http_url` — `qq`

设置 NapCat/QQ HTTP API 地址。

- 作用域：`platform` (`qq`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.image_batch_window_ms` — `feishu, lark`

将在短时间内连续到达的飞书图片合并为一个回合。

- 作用域：`platform` (`feishu, lark`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.intents` — `qqbot`

设置 QQ Bot Gateway Intent 位掩码。

- 作用域：`platform` (`qqbot`)
- 类型：`integer`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.long_poll_timeout_ms` — `weixin`

设置微信长轮询超时（毫秒）。

- 作用域：`platform` (`weixin`)
- 类型：`integer`
- 默认值：`35000`
- 生效方式：`restart`

### `projects.platforms.options.markdown_support` — `qqbot`

启用 QQ Bot Markdown 消息。

- 作用域：`platform` (`qqbot`)
- 类型：`boolean`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.mention_map` — `feishu, lark`

将飞书 @ 身份映射为替换文本或 Agent 标识。

- 作用域：`platform` (`feishu, lark`)
- 类型：`table`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.mode` — `wecom`

选择平台连接模式，例如 WebSocket 或回调。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.name` — `weibo`

设置平台适配器使用的账号显示名称。

- 作用域：`platform` (`weibo`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.peer_bots` — `feishu, lark`

将指定飞书机器人身份识别为 Relay 对端。

- 作用域：`platform` (`feishu, lark`)
- 类型：`string[]`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.port` — `feishu, lark, line`

以带引号的字符串设置入站 Webhook 监听端口。

- 作用域：`platform` (`feishu, lark, line`)
- 类型：`string`
- 默认值：`8080`
- 生效方式：`restart`

### `projects.platforms.options.port` — `wecom`

以带引号的字符串设置入站 Webhook 监听端口。

- 作用域：`platform` (`wecom`)
- 类型：`string`
- 默认值：`8081`
- 生效方式：`restart`

### `projects.platforms.options.progress_style` — `discord, feishu, lark, telegram`

选择消息平台上的进度展示样式。

- 作用域：`platform` (`discord, feishu, lark, telegram`)
- 类型：`string`
- 默认值：`compact`
- 生效方式：`restart`
- 允许值: `legacy`, `compact`, `card`

### `projects.platforms.options.proxy` — `discord, matrix, telegram, wecom, weixin`

通过 HTTP 或 SOCKS5 代理转发平台 HTTP/WebSocket 流量。

- 作用域：`platform` (`discord, matrix, telegram, wecom, weixin`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.proxy_password` — `discord, telegram, wecom, weixin`

认证已配置的平台代理。

- 作用域：`platform` (`discord, telegram, wecom, weixin`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.proxy_username` — `discord, telegram, wecom, weixin`

设置平台代理认证用户名。

- 作用域：`platform` (`discord, telegram, wecom, weixin`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.reaction_emoji` — `dingtalk, feishu, lark`

选择处理中表情。

- 作用域：`platform` (`dingtalk, feishu, lark`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.reply_to_trigger` — `feishu, lark`

在飞书中回复到触发消息。

- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.require_mention` — `feishu, lark`

群聊中必须明确 @ 机器人才回复。

- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.resolve_mentions` — `feishu, lark`

发送给 Agent 前将飞书 @ 解析为可读名称。

- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.respond_to_at_everyone_and_here` — `discord, feishu, lark`

将 @everyone/@here 视为有效的机器人提及。

- 作用域：`platform` (`discord, feishu, lark`)
- 类型：`boolean`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.robot_code` — `dingtalk`

标识用于出站消息的钉钉机器人。

- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.route_tag` — `weixin`

设置可选的微信 SKRouteTag 请求头。

- 作用域：`platform` (`weixin`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.sandbox` — `qqbot`

使用 QQ Bot 沙箱环境。

- 作用域：`platform` (`qqbot`)
- 类型：`boolean`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.session_scope` — `slack`

选择 Slack 会话按用户、频道还是线程隔离。

- 作用域：`platform` (`slack`)
- 类型：`string`
- 默认值：`user (or channel when share_session_in_channel=true)`
- 生效方式：`restart`
- 允许值: `user`, `channel`, `thread`

### `projects.platforms.options.share_session_in_channel` — `dingtalk, discord, feishu, lark, matrix, qq, qqbot, slack, telegram`

让频道或房间内所有用户共享同一个 Agent 会话。

- 作用域：`platform` (`dingtalk, discord, feishu, lark, matrix, qq, qqbot, slack, telegram`)
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.platforms.options.state_dir` — `weixin`

覆盖平台持久化状态目录。

- 作用域：`platform` (`weixin`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.thread_isolation` — `discord`

为每个话题或线程使用独立 Agent 会话。

- 作用域：`platform` (`discord`)
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.platforms.options.thread_isolation` — `feishu, lark`

为每个飞书/Lark 话题使用独立 Agent 会话和工作区绑定。省略该键时保留 false 兼容回落；新 Starter 配置和用户接受的推荐 Profile 会显式写入 true。

- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 默认值：`false when omitted; new Starter/recommended profile writes true`
- 生效方式：`restart`

### `projects.platforms.options.token` — `discord, max, qq, telegram, webex, weixin`

认证平台机器人或网关。

- 作用域：`platform` (`discord, max, qq, telegram, webex, weixin`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.token_endpoint` — `weibo`

覆盖获取微博访问令牌的端点。

- 作用域：`platform` (`weibo`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.user_id` — `matrix`

设置或覆盖 Matrix 机器人用户 ID。

- 作用域：`platform` (`matrix`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.webhook_listen` — `max`

设置 MAX Webhook 的本地监听地址。

- 作用域：`platform` (`max`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.webhook_path` — `max`

设置 MAX Webhook URL 路径。

- 作用域：`platform` (`max`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.webhook_resubscribe_interval` — `max`

使用 Go duration 字符串定期刷新 MAX Webhook 订阅。

- 作用域：`platform` (`max`)
- 类型：`string`
- 默认值：`5m`
- 生效方式：`restart`
- 允许值: `Go duration string (for example: 30s, 5m, 1h)`
- 示例: `webhook_resubscribe_interval = "5m"`

### `projects.platforms.options.webhook_secret` — `max`

验证 MAX Webhook 请求。

- 作用域：`platform` (`max`)
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `projects.platforms.options.webhook_url` — `max`

设置外部可访问的 MAX Webhook 地址。

- 作用域：`platform` (`max`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.ws_endpoint` — `weibo`

覆盖平台 WebSocket 地址。

- 作用域：`platform` (`weibo`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.options.ws_url` — `qq`

设置 NapCat/QQ 正向 WebSocket 地址。

- 作用域：`platform` (`qq`)
- 类型：`string`
- 默认值：`unset / adapter default`
- 生效方式：`restart`

### `projects.platforms.type`

选择连接当前项目的消息平台适配器。

- 作用域：`platform`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.quiet`

旧版项目级静默开关；未设置 Display 覆盖时隐藏思考和工具消息。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`inherit`
- 生效方式：`reload`
- 状态：已废弃，仅保留兼容。

### `projects.references.display_path`

选择 projects.references 渲染给用户看的路径。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.references.enclosure_style`

选择 projects.references 包裹标准化引用的样式。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.references.marker_style`

选择 projects.references 输出的标记语法。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.references.normalize_agents`

仅对列出的 Agent 适配器应用 projects.references 标准化。

- 作用域：`project`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.references.render_platforms`

仅在列出的消息平台渲染 projects.references。

- 作用域：`project`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.reply_footer`

为单个项目覆盖回复底部状态栏。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`inherit`
- 生效方式：`reload`

### `projects.reset_on_idle_mins`

用户空闲指定时间后回来时切换到新会话；0 表示禁用。

- 作用域：`project`
- 类型：`integer`
- 默认值：`0`
- 生效方式：`reload`

### `projects.run_as_env`

允许列出的环境变量名通过 projects 用户隔离边界。

- 作用域：`project`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.run_as_user`

以另一个非 root 操作系统用户运行当前项目 Agent。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.shell`

选择 projects 使用的 Shell。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.shell_profile`

为 projects 添加 Shell 初始化命令。

- 作用域：`project`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `projects.show_context_indicator`

已废弃的无效果项目配置，仅保留旧配置兼容。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`
- 状态：已废弃，仅保留兼容。

### `projects.show_workdir_indicator`

已废弃的无效果项目配置，仅保留旧配置兼容。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`
- 状态：已废弃，仅保留兼容。

### `projects.skip_git`

允许多工作区目录不是 Git 仓库。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `projects.users.default_role`

选择未显式列出用户的默认角色。

- 作用域：`project`
- 类型：`string`
- 默认值：`member`
- 生效方式：`reload`

### `projects.users.roles.<name>.disabled_commands`

为 projects.users.roles.<name> 禁用列出的命令。

- 作用域：`project`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.users.roles.<name>.rate_limit.max_messages`

限制 projects.users.roles.<name>.rate_limit 在一个窗口内接受的消息数。

- 作用域：`project`
- 类型：`integer`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.users.roles.<name>.rate_limit.window_secs`

设置 projects.users.roles.<name>.rate_limit 的限流窗口秒数。

- 作用域：`project`
- 类型：`integer`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.users.roles.<name>.user_ids`

将列出的平台用户 ID 分配给 projects.users.roles.<name>。

- 作用域：`project`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`reload`

### `projects.workspace_idle_timeout_mins`

已废弃的项目级工作区回收超时；请改用顶层配置。

- 作用域：`project`
- 类型：`integer`
- 默认值：`inherit`
- 生效方式：`restart`
- 状态：已废弃，仅保留兼容。

### `projects.workspace_init_allow_local_paths`

允许 /workspace init 除 Git URL 外绑定本地目录。

- 作用域：`project`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `provider_presets_url`

覆盖推荐 Provider 预设使用的远程 JSON 地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.agent_model_lists.<name>.alias`

为 providers.agent_model_lists.<name> 设置简短的用户可见别名。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.agent_model_lists.<name>.model`

选择 providers.agent_model_lists.<name> 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.agent_models.<name>`

设置 providers.agent_models 中的一个命名条目。

- 作用域：`global`
- 类型：`table`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.agent_types`

将共享 Provider 限制给指定 Agent 适配器类型。

- 作用域：`global`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.api_key`

认证共享模型 Provider。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `providers.base_url`

覆盖共享 Provider API 基础地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.codex.env_key`

指定 providers.codex 读取凭据的环境变量名。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.codex.http_headers.<name>`

设置 providers.codex.http_headers 中的一个命名条目。

- 作用域：`global`
- 类型：`table`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `providers.codex.wire_api`

选择 providers.codex 使用的 Wire API 协议。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.endpoints.<name>`

设置 providers.endpoints 中的一个命名条目。

- 作用域：`global`
- 类型：`table`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.env.<name>`

设置 providers.env 中的一个命名条目。

- 作用域：`global`
- 类型：`table`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `providers.model`

选择 providers 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.models.alias`

为 providers.models 设置简短的用户可见别名。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.models.model`

选择 providers.models 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.name`

设置 providers 使用的名称。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `providers.thinking`

选择 providers 使用的 Provider 思考模式。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `queue.busy_message_mode`

将符合条件的新输入 steer 到当前回合，或始终保持 FIFO 排队。

- 作用域：`global`
- 类型：`string`
- 默认值：`steer`
- 生效方式：`restart`
- 允许值: `steer`, `queue`

### `queue.max_depth`

限制一个忙碌会话后等待的用户消息数量。

- 作用域：`global`
- 类型：`integer`
- 默认值：`5`
- 生效方式：`restart`

### `quiet`

旧版静默开关；未设置新版 Display 字段时隐藏思考和工具消息。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`reload`
- 状态：已废弃，仅保留兼容。

### `rate_limit.max_messages`

限制每个用户/会话窗口内的入站消息数；0 表示禁用。

- 作用域：`global`
- 类型：`integer`
- 默认值：`20`
- 生效方式：`restart`

### `rate_limit.window_secs`

设置入站限流窗口秒数。

- 作用域：`global`
- 类型：`integer`
- 默认值：`60`
- 生效方式：`restart`

### `relay.timeout_secs`

限制跨项目 Relay 等待回复的时长；0 表示禁用等待。

- 作用域：`global`
- 类型：`integer`
- 默认值：`120`
- 生效方式：`restart`

### `relay.visibility`

选择群内展示多少 Relay 活动。

- 作用域：`global`
- 类型：`string`
- 默认值：`full`
- 生效方式：`restart`
- 允许值: `full`, `summary`, `none`

### `shell`

选择 /shell、exec Cron、Hooks 和 Webhook exec 使用的 Shell。

- 作用域：`global`
- 类型：`string`
- 默认值：`sh on Unix; powershell.exe on Windows`
- 生效方式：`restart`

### `shell_profile`

在每条配置的 Shell 命令前执行初始化命令。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `speech.enabled`

将收到的语音消息转写后再发送给 Agent。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `speech.gemini.api_key`

认证 speech.gemini 发出的请求。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `speech.gemini.model`

选择 speech.gemini 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `speech.groq.api_key`

认证 speech.groq 发出的请求。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `speech.groq.model`

选择 speech.groq 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `speech.language`

设置 speech 使用的语言或 Locale 提示。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `speech.openai.api_key`

认证 speech.openai 发出的请求。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `speech.openai.base_url`

覆盖 speech.openai 的服务基础地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `speech.openai.model`

选择 speech.openai 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `speech.provider`

选择语音转文字 Provider。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`
- 允许值: `openai`, `groq`, `qwen`, `gemini`

### `speech.qwen.api_key`

认证 speech.qwen 发出的请求。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `speech.qwen.base_url`

覆盖 speech.qwen 的服务基础地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `speech.qwen.model`

选择 speech.qwen 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `stream_preview.disabled_platforms`

在列出的消息平台上关闭 stream_preview。

- 作用域：`global`
- 类型：`string[]`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `stream_preview.enabled`

Agent 流式输出期间持续更新一条预览消息。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `stream_preview.interval_ms`

设置流式预览更新的最小间隔。

- 作用域：`global`
- 类型：`integer`
- 默认值：`1500`
- 生效方式：`restart`

### `stream_preview.max_chars`

限制累计流式预览长度。

- 作用域：`global`
- 类型：`integer`
- 默认值：`2000`
- 生效方式：`restart`

### `stream_preview.min_delta_chars`

至少新增指定字符数后才刷新预览。

- 作用域：`global`
- 类型：`integer`
- 默认值：`30`
- 生效方式：`restart`

### `tts.agents.<name>.language_type`

设置 tts.agents.<name> 使用的 Provider 专属语言提示。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.agents.<name>.max_text_len`

文本超过该长度时跳过或截断 tts.agents.<name>；0 表示不限制。

- 作用域：`global`
- 类型：`integer`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.agents.<name>.provider`

选择 tts.agents.<name> 使用的 Provider。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.agents.<name>.speed`

设置 tts.agents.<name> 使用的语速倍率。

- 作用域：`global`
- 类型：`number`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.agents.<name>.voice`

选择 tts.agents.<name> 使用的音色。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.agents.<name>.voice_id`

设置 tts.agents.<name> 使用的 Provider 专属音色 ID。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.enabled`

启用文字转语音回复。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `tts.language_type`

设置 tts 使用的 Provider 专属语言提示。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.max_text_len`

文本超过该长度时跳过或截断 tts；0 表示不限制。

- 作用域：`global`
- 类型：`integer`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.mimo.api_key`

认证 tts.mimo 发出的请求。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `tts.mimo.base_url`

覆盖 tts.mimo 的服务基础地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.mimo.model`

选择 tts.mimo 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.minimax.api_key`

认证 tts.minimax 发出的请求。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `tts.minimax.base_url`

覆盖 tts.minimax 的服务基础地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.minimax.config_file`

覆盖 tts.minimax 使用的辅助配置文件路径。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.minimax.model`

选择 tts.minimax 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.openai.api_key`

认证 tts.openai 发出的请求。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `tts.openai.base_url`

覆盖 tts.openai 的服务基础地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.openai.model`

选择 tts.openai 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.provider`

选择文字转语音 Provider。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`
- 允许值: `qwen`, `openai`, `minimax`, `mimo`, `espeak`, `pico`, `edge`

### `tts.qwen.api_key`

认证 tts.qwen 发出的请求。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `tts.qwen.base_url`

覆盖 tts.qwen 的服务基础地址。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.qwen.model`

选择 tts.qwen 使用的模型。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.speed`

设置 tts 使用的语速倍率。

- 作用域：`global`
- 类型：`number`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.tts_mode`

选择仅语音触发时回复，或为每条符合条件的回复合成语音。

- 作用域：`global`
- 类型：`string`
- 默认值：`voice_only`
- 生效方式：`restart`
- 允许值: `voice_only`, `always`

### `tts.voice`

选择 tts 使用的音色。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `tts.voice_id`

设置 tts 使用的 Provider 专属音色 ID。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset / runtime default`
- 生效方式：`restart`

### `update_notice`

有新稳定版时向最近活跃会话发送一次升级提醒。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`true`
- 生效方式：`restart`

### `webhook.enabled`

开放可触发 Agent 提示或 Shell 命令的外部 HTTP 端点。

- 作用域：`global`
- 类型：`boolean`
- 默认值：`false`
- 生效方式：`restart`

### `webhook.path`

设置外部 Webhook URL 路径前缀。

- 作用域：`global`
- 类型：`string`
- 默认值：`/hook`
- 生效方式：`restart`

### `webhook.port`

设置外部 Webhook 监听端口。

- 作用域：`global`
- 类型：`integer`
- 默认值：`9111`
- 生效方式：`restart`

### `webhook.token`

使用共享 Token 认证 webhook。

- 作用域：`global`
- 类型：`string`
- 默认值：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。

### `workspace_idle_timeout_mins`

多工作区引擎空闲指定分钟后回收；0 表示禁用。

- 作用域：`global`
- 类型：`integer`
- 默认值：`15`
- 生效方式：`restart`
