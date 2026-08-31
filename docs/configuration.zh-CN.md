<!-- Code generated from the compiled configuration catalog. DO NOT EDIT. -->

# cc-connect-next 配置能力参考

当前目录版本：`source`。本参考描述能力，不读取或显示本机配置值。

生效方式：`live` 表示当前运行态立即生效；`reload` 表示保存后可用 `/config reload` 应用；`new-session` 表示新 Agent 会话生效；`restart` 表示需要重启 cc-connect-next。

配置来源：`toml` 表示持久化在 config.toml；`environment` 表示进程环境变量；`cli` 表示启动或安装参数。TOML 字符串支持 `${VAR_NAME}` 环境变量占位符。点路径描述语义位置；Agent/Platform 选项分别写入 `[projects.agent.options]` / `[projects.platforms.options]`，并在相邻表中设置适配器 `type`。

优先级：显式 CLI 覆盖对应环境变量，环境覆盖项覆盖对应 TOML，项目级字段逐项覆盖全局字段，之后才使用运行时默认值。预设值只表示生成器/Profile 会显式写入什么，不会覆盖用户已有显式配置。

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

### 环境变量与运维覆盖 (`environment-overrides`)

无需修改 TOML 即可覆盖日志滚动、附件限制、daemon 凭据捕获、命令上下文和适配器状态。

相关配置： `CC_LOG_FILE`, `CC_LOG_MAX_SIZE`, `CC_LOG_MAX_BACKUPS`, `CC_MAX_ATTACHMENT_SIZE_MB`, `CC_DAEMON_NO_CAPTURE_SECRETS`, `CC_PROJECT`, `CC_SESSION_KEY`, `--config`, `--log-max-size`, `--log-max-backups`, `daemon install --config`, `daemon install --work-dir`, `daemon install --log-max-size`, `daemon install --log-file`, `daemon install --no-capture-secrets`

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

### `--config`

为当前命令或运行时选择 config.toml 文件。

- 来源：`cli`
- 配置位置：`command line`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`./config.toml when present, otherwise ~/.cc-connect-next/config.toml`
- 默认值来源：`runtime`
- 生效方式：`live`
- 示例: `cc-connect-next --config /path/to/config.toml`

### `--log-max-backups`

设置滚动日志备份数量并覆盖 CC_LOG_MAX_BACKUPS。

- 来源：`cli`
- 配置位置：`command line`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`3`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞`
- 示例: `cc-connect-next --log-max-backups 5`

### `--log-max-size`

设置滚动日志大小并覆盖 CC_LOG_MAX_SIZE。

- 来源：`cli`
- 配置位置：`command line`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`10MB`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `cc-connect-next --log-max-size 50MB`

### `CC_DAEMON_NO_CAPTURE_SECRETS`

阻止 daemon 安装捕获受支持的凭据环境变量。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `export CC_DAEMON_NO_CAPTURE_SECRETS=true`

### `CC_DATA_DIR`

覆盖独立 send 操作使用的数据目录。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit data_dir`
- 默认值来源：`inherit`
- 生效方式：`live`
- 示例: `export CC_DATA_DIR=/path/to/data`

### `CC_LOG_FILE`

覆盖运行日志文件路径。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`platform daemon log path`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `export CC_LOG_FILE=/path/to/cc-connect-next.log`

### `CC_LOG_MAX_BACKUPS`

覆盖滚动日志备份数量；显式 --log-max-backups 参数优先。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`3`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞`
- 示例: `export CC_LOG_MAX_BACKUPS=3`

### `CC_LOG_MAX_SIZE`

覆盖滚动日志文件大小；显式 --log-max-size 参数优先。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`10MB`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `export CC_LOG_MAX_SIZE=10MB`

### `CC_MAX_ATTACHMENT_SIZE_MB`

为 /send API 覆盖 max_attachment_size_mb。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`inherit max_attachment_size_mb`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 范围: `1` 到 `+∞` `MiB`
- 示例: `export CC_MAX_ATTACHMENT_SIZE_MB=100`

### `CC_NEXT_ALLOW_OFFICIAL_CONFLICT`

检测到官方 CC Connect 运行冲突时显式允许继续启动。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `export CC_NEXT_ALLOW_OFFICIAL_CONFLICT=true`

### `CC_PROJECT`

为 send、relay、cron、timer 和 session 辅助命令提供默认项目上下文。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`live`
- 示例: `export CC_PROJECT=my-project`

### `CC_SESSION_KEY`

为 send、relay、cron、timer 和 session 辅助命令提供默认会话上下文。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`live`
- 示例: `export CC_SESSION_KEY=feishu:oc_chat:ou_user`

### `CLAUDE_CONFIG_DIR` — `claudecode`

覆盖 Claude Code 配置目录。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`~/.claude`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `export CLAUDE_CONFIG_DIR=/path/to/claude-config`

### `CODEX_HOME` — `codex`

projects.agent.options.codex_home 未设置时选择 Codex Home。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`~/.codex`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `export CODEX_HOME=/path/to/codex-home`

### `MATRIX_CROSS_SIGNING_PASSWORD` — `matrix`

无需写入 TOML 即可提供 Matrix 跨签名密码。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`platform` (`matrix`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `export MATRIX_CROSS_SIGNING_PASSWORD='${MATRIX_PASSWORD}'`

### `PI_CODING_AGENT_DIR` — `pi`

覆盖 pi coding-agent 状态目录。

- 来源：`environment`
- 配置位置：`process environment`
- 作用域：`agent` (`pi`)
- 类型：`string`
- 要求：`可选`
- 默认值：`upstream pi default`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `export PI_CODING_AGENT_DIR=/path/to/pi-agent`

### `aliases.command`

选择该别名展开成的 Slash Command。

- 来源：`toml`
- 配置位置：`[[aliases]]`
- 作用域：`global`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `command = "value"`

### `aliases.name`

设置命令别名的自然语言触发词。

- 来源：`toml`
- 配置位置：`[[aliases]]`
- 作用域：`global`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `name = "value"`

### `attachment_send`

允许或阻止 Agent 主动回传图片和文件，不影响文本回复。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`on`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 允许值: `on`, `off`
- 示例: `attachment_send = "on"`

### `banned_words`

阻止包含任一已配置违禁词的消息。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`[]`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `banned_words = ["value"]`

### `bridge.cors_origins`

允许列出的 CORS Origin 访问 bridge。

- 来源：`toml`
- 配置位置：`[bridge]`
- 作用域：`global`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `cors_origins = ["value"]`

### `bridge.enabled`

启用供外部平台适配器使用的 WebSocket/REST Bridge。

- 来源：`toml`
- 配置位置：`[bridge]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enabled = false`

### `bridge.insecure`

仅为本地开发允许无 Token Bridge。

- 来源：`toml`
- 配置位置：`[bridge]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `insecure = false`

### `bridge.path`

设置外部适配器 Bridge WebSocket 路径。

- 来源：`toml`
- 配置位置：`[bridge]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`/bridge/ws`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `path = "/bridge/ws"`

### `bridge.port`

设置外部适配器 Bridge 端口。

- 来源：`toml`
- 配置位置：`[bridge]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`9810`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `port = 9810`

### `bridge.token`

使用共享 Token 认证 Bridge 客户端。

- 来源：`toml`
- 配置位置：`[bridge]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `bridge.enabled = true and bridge.insecure != true`
- 依赖: `bridge.enabled`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `token = "${TOKEN}"`

### `commands.description`

在菜单和帮助中说明自定义命令。

- 来源：`toml`
- 配置位置：`[[commands]]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `description = "value"`

### `commands.exec`

执行 Shell 命令而不是向 Agent 发送 Prompt。

- 来源：`toml`
- 配置位置：`[[commands]]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 必填条件: `commands.prompt is unset`
- 冲突: `commands.prompt`
- 示例: `exec = "value"`

### `commands.name`

设置自定义 Slash Command 名称。

- 来源：`toml`
- 配置位置：`[[commands]]`
- 作用域：`global`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `name = "value"`

### `commands.prompt`

将自定义命令展开为 Agent Prompt。

- 来源：`toml`
- 配置位置：`[[commands]]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 必填条件: `commands.exec is unset`
- 冲突: `commands.exec`
- 示例: `prompt = "value"`

### `commands.work_dir`

覆盖自定义 Exec 命令的工作目录。

- 来源：`toml`
- 配置位置：`[[commands]]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `work_dir = "value"`

### `cron.session_mode`

选择定时任务复用会话还是每次创建新会话。

- 来源：`toml`
- 配置位置：`[cron]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`reuse`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `reuse`, `new_per_run`
- 示例: `session_mode = "reuse"`

### `cron.silent`

禁止定时任务开始执行时的提示消息。

- 来源：`toml`
- 配置位置：`[cron]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `silent = false`

### `daemon install --config`

选择 daemon 安装记录的 config.toml。

- 来源：`cli`
- 配置位置：`command line`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`<work-dir>/config.toml`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `cc-connect-next daemon install --config /path/to/config.toml`

### `daemon install --log-file`

安装 daemon 时选择日志文件路径。

- 来源：`cli`
- 配置位置：`command line`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`~/.cc-connect-next/logs/cc-connect-next.log`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `cc-connect-next daemon install --log-file /path/to/cc-connect-next.log`

### `daemon install --log-max-size`

设置已安装 daemon 的日志滚动大小（MiB）。

- 来源：`cli`
- 配置位置：`command line`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`10`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞` `MiB`
- 示例: `cc-connect-next daemon install --log-max-size 50`

### `daemon install --no-capture-secrets`

安装 daemon 时不捕获受支持的凭据环境变量。

- 来源：`cli`
- 配置位置：`command line`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `cc-connect-next daemon install --no-capture-secrets`

### `daemon install --work-dir`

选择 daemon 解析相对路径时使用的运行工作目录。

- 来源：`cli`
- 配置位置：`command line`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`config parent or current directory`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `cc-connect-next daemon install --work-dir /path/to/runtime`

### `data_dir`

选择 cc-connect-next 存储会话、状态、媒体和运行元数据的位置。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`~/.cc-connect-next`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `data_dir = "~/.cc-connect-next"`

### `display.card_mode`

在支持的平台选择 Rich Card 2.0 或旧卡片渲染。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`rich`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 允许值: `rich`, `legacy`
- 示例: `card_mode = "rich"`

### `display.hide_agent_footer`

移除 Agent 自己输出的等价模型、Token 和上下文状态行。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `hide_agent_footer = false`

### `display.history_max_len`

限制每条 /history 记录长度；0 表示不截断。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`1000`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `characters`
- 示例: `history_max_len = 1000`

### `display.mode`

选择 full、compact 或 quiet 回复展示。省略时使用 full 布局但不会开启思考/工具消息；显式写入 full 才启用该模式的消息默认值。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`full`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 允许值: `full`, `compact`, `quiet`
- 示例: `mode = "full"`

### `display.reply_footer`

在完成回复底部显示模型、推理强度和处理耗时。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `reply_footer = true`

### `display.show_context_indicator`

已废弃的无效果配置，仅保留旧配置兼容。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 状态：已废弃，仅保留兼容。
- 示例: `show_context_indicator = false`

### `display.thinking_max_len`

限制思考进度文本长度；0 表示不截断。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`300`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `characters`
- 示例: `thinking_max_len = 300`

### `display.thinking_messages`

显示或隐藏 Agent 思考进度消息。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `thinking_messages = false`

### `display.tool_max_len`

限制工具进度文本长度；0 表示不截断。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`500`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `characters`
- 示例: `tool_max_len = 500`

### `display.tool_messages`

显示或隐藏 Agent 工具进度消息。

- 来源：`toml`
- 配置位置：`[display]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `tool_messages = false`

### `feedback.enabled`

启用 /feedback 和能力缺口提示；每次提交仍需确认。

- 来源：`toml`
- 配置位置：`[feedback]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enabled = true`

### `feedback.endpoint`

覆盖作者维护的匿名 Feedback v1 中继；必须是 HTTPS 的精确 /v1/feedback（仅本机开发可用 HTTP）。

- 来源：`toml`
- 配置位置：`[feedback]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`built-in author relay`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `endpoint = "value"`

### `hooks.async`

异步运行 Hook，避免阻塞消息处理。

- 来源：`toml`
- 配置位置：`[[hooks]]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `async = true`

### `hooks.command`

设置 command Hook 执行的 Shell 命令。

- 来源：`toml`
- 配置位置：`[[hooks]]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 必填条件: `hooks.type = command`
- 冲突: `hooks.url`
- 示例: `command = "value"`

### `hooks.event`

选择触发该 Hook 的事件。

- 来源：`toml`
- 配置位置：`[[hooks]]`
- 作用域：`global`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 示例: `event = "value"`

### `hooks.timeout`

设置 hooks 的执行超时秒数。

- 来源：`toml`
- 配置位置：`[[hooks]]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `timeout = 1`

### `hooks.type`

选择命令或 HTTP Hook 执行方式。

- 来源：`toml`
- 配置位置：`[[hooks]]`
- 作用域：`global`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 允许值: `command`, `http`
- 示例: `type = "command"`

### `hooks.url`

设置 HTTP Hook 调用的 URL。

- 来源：`toml`
- 配置位置：`[[hooks]]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 必填条件: `hooks.type = http`
- 冲突: `hooks.command`
- 示例: `url = "value"`

### `idle_timeout_mins`

Agent 连续指定分钟无事件时终止回合；0 表示禁用。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`120`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `minutes`
- 示例: `idle_timeout_mins = 120`

### `instant_reply.content`

覆盖本地化即时确认文案。

- 来源：`toml`
- 配置位置：`[instant_reply]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `content = "value"`

### `instant_reply.enabled`

收到消息后、Agent 开始工作前立即发送确认。

- 来源：`toml`
- 配置位置：`[instant_reply]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `enabled = false`

### `language`

选择机器人消息的规范语言，或从用户首条消息自动检测；常见地区别名会被归一化。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`zh`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 规范值（也接受文档化别名）: `zh`, `en`, `zh-TW`, `ja`, `es`, `auto`
- 示例: `language = "zh"`

### `log.level`

设置运行日志的最低严重级别。

- 来源：`toml`
- 配置位置：`[log]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`info`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `debug`, `info`, `warn`, `error`
- 示例: `level = "debug"`

### `management.cors_origins`

允许列出的 CORS Origin 访问 management。

- 来源：`toml`
- 配置位置：`[management]`
- 作用域：`global`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `cors_origins = ["value"]`

### `management.enabled`

启用本地管理 API 和 Web 控制台后端。

- 来源：`toml`
- 配置位置：`[management]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enabled = false`

### `management.port`

设置管理 API 监听端口。

- 来源：`toml`
- 配置位置：`[management]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`9820`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `port = 9820`

### `management.token`

使用共享 Token 认证管理 API 与 Web 控制台请求。

- 来源：`toml`
- 配置位置：`[management]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `management.enabled = true`
- 依赖: `management.enabled`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `token = "${TOKEN}"`

### `max_attachment_size_mb`

设置单个出站附件的最大大小（MiB）。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`50`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `MiB`
- 示例: `max_attachment_size_mb = 50`

### `max_turn_time_mins`

限制单次 Agent 回合的绝对运行时长；0 表示禁用。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`0`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `minutes`
- 示例: `max_turn_time_mins = 0`

### `outgoing_rate_limit.burst`

设置出站消息的最大瞬时突发数量。

- 来源：`toml`
- 配置位置：`[outgoing_rate_limit]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`ceil(max_per_second)`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞`
- 示例: `burst = 1`

### `outgoing_rate_limit.max_per_second`

限制每秒出站消息数；0 表示不限。

- 来源：`toml`
- 配置位置：`[outgoing_rate_limit]`
- 作用域：`global`
- 类型：`number`
- 要求：`可选`
- 默认值：`0`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `messages/second`
- 示例: `max_per_second = 1.0`

### `outgoing_rate_limit.platforms.<name>.burst`

为单个平台覆盖出站突发数量；省略时继承全局值。

- 来源：`toml`
- 配置位置：`[outgoing_rate_limit.platforms.<name>]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 范围: `0` 到 `+∞`
- 示例: `burst = 1`

### `outgoing_rate_limit.platforms.<name>.max_per_second`

为单个平台覆盖每秒出站消息数；省略时继承全局值。

- 来源：`toml`
- 配置位置：`[outgoing_rate_limit.platforms.<name>]`
- 作用域：`global`
- 类型：`number`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `messages/second`
- 示例: `max_per_second = 1.0`

### `projects.admin_from`

将特权命令限制给指定平台用户 ID；未设置时所有人都不能执行特权命令。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`reload`
- 示例: `admin_from = "value"`

### `projects.agent.answer_profiles.fast.model`

为一次性 /fast 回答覆盖模型。

- 来源：`toml`
- 配置位置：`[projects.agent.answer_profiles.fast] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 示例: `model = "value"`

### `projects.agent.answer_profiles.fast.reasoning_effort`

为一次性 /fast 回答覆盖推理强度。

- 来源：`toml`
- 配置位置：`[projects.agent.answer_profiles.fast] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 允许值: `low`, `medium`, `high`, `xhigh`, `max`
- 示例: `reasoning_effort = "low"`

### `projects.agent.answer_profiles.fast.service_tier`

为一次性 /fast 回答覆盖模型目录声明的服务档位。

- 来源：`toml`
- 配置位置：`[projects.agent.answer_profiles.fast] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 允许值: `model-catalog-driven (for example: default, fast)`
- 示例: `service_tier = "value"`

### `projects.agent.answer_profiles.quality.model`

为一次性 /quality 回答覆盖模型。

- 来源：`toml`
- 配置位置：`[projects.agent.answer_profiles.quality] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 示例: `model = "value"`

### `projects.agent.answer_profiles.quality.reasoning_effort`

为一次性 /quality 回答覆盖推理强度。

- 来源：`toml`
- 配置位置：`[projects.agent.answer_profiles.quality] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 允许值: `low`, `medium`, `high`, `xhigh`, `max`
- 示例: `reasoning_effort = "low"`

### `projects.agent.answer_profiles.quality.service_tier`

为一次性 /quality 回答覆盖模型目录声明的服务档位。

- 来源：`toml`
- 配置位置：`[projects.agent.answer_profiles.quality] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 允许值: `model-catalog-driven (for example: default, fast)`
- 示例: `service_tier = "value"`

### `projects.agent.options.agent` — `opencode`

选择 CLI 暴露的子 Agent 或配置档。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`opencode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `agent = "value"`

### `projects.agent.options.allowed_tools` — `claudecode`

在需要审批的模式中预先允许指定 Claude Code 工具。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `allowed_tools = ["value"]`

### `projects.agent.options.app_server_url` — `codex`

选择 Codex app-server 的传输端点。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`stdio`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `app_server_url = "stdio"`
- 预设 `starter`: `stdio` — 启动本地 app-server 子进程。

### `projects.agent.options.append_system_prompt` — `claudecode, codex`

保留 Agent 默认系统提示并追加项目指令。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode, codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `append_system_prompt = "value"`

### `projects.agent.options.args` — `acp`

向配置的 Agent 命令传递额外参数。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp`)
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `args = ["value"]`

### `projects.agent.options.args` — `devin`

向配置的 Agent 命令传递额外参数。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`devin`)
- 类型：`string[]`
- 要求：`可选`
- 默认值：`["acp"]`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `args = ["acp"]`

### `projects.agent.options.auth_method` — `acp`

选择 ACP Agent 使用的认证方式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `auth_method = "value"`

### `projects.agent.options.auto_create` — `tmux`

配置的 tmux 会话不存在时自动创建。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `auto_create = true`

### `projects.agent.options.backend` — `codex`

选择 Codex 执行后端；app_server 支持原生 steer 与审批。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`app_server`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `app_server`, `exec`
- 示例: `backend = "app_server"`
- 预设 `starter`: `app_server` — 使用原生 steer 与审批协议。

### `projects.agent.options.cli_args_flag` — `claudecode`

指定包装器接收 Agent CLI 参数的标志名。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `cli_args_flag = "value"`

### `projects.agent.options.cli_path` — `acp`

覆盖 Agent CLI 可执行文件路径。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 必填条件: `one of cmd, cli_path, or command must be set`
- 示例: `cli_path = "value"`

### `projects.agent.options.cli_path` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

覆盖 Agent CLI 可执行文件路径。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `cli_path = "value"`

### `projects.agent.options.cmd` — `acp`

覆盖 Agent 命令，可同时包含全局参数。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 必填条件: `one of cmd, cli_path, or command must be set`
- 示例: `cmd = "value"`

### `projects.agent.options.cmd` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

覆盖 Agent 命令，可同时包含全局参数。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `cmd = "value"`

### `projects.agent.options.cmd_args_flag` — `claudecode`

指定包装器转发命令参数所使用的标志名。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `cmd_args_flag = "value"`

### `projects.agent.options.codex_home` — `codex`

为当前项目覆盖 CODEX_HOME，不修改用户全局 Codex 目录。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `codex_home = "value"`

### `projects.agent.options.command` — `acp`

设置 Agent 可执行命令；多个适配器使用的别名。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 必填条件: `one of cmd, cli_path, or command must be set`
- 示例: `command = "value"`

### `projects.agent.options.command` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

设置 Agent 可执行命令；多个适配器使用的别名。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `command = "value"`

### `projects.agent.options.command` — `devin`

设置 Agent 可执行命令；多个适配器使用的别名。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`devin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`devin`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `command = "devin"`

### `projects.agent.options.disallowed_tools` — `claudecode`

即使当前模式允许，也禁止指定 Claude Code 工具。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `disallowed_tools = ["value"]`

### `projects.agent.options.display_name` — `acp`

设置通用或 ACP Agent 的用户可见名称。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp`)
- 类型：`string`
- 要求：`可选`
- 默认值：`ACP`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `display_name = "ACP"`

### `projects.agent.options.display_name` — `devin`

设置通用或 ACP Agent 的用户可见名称。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`devin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`Devin`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `display_name = "Devin"`

### `projects.agent.options.env` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

向 Agent 进程注入项目级环境变量。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `env = { example = "value" }`

### `projects.agent.options.init_command` — `tmux`

发送 tmux 提示前运行初始化命令。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `init_command = "value"`

### `projects.agent.options.max_context_tokens` — `claudecode`

覆盖 Claude Code 可使用的最大上下文 token 数。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 范围: `1` 到 `+∞`
- 示例: `max_context_tokens = 1`

### `projects.agent.options.mode` — `acp`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `mode = "value"`

### `projects.agent.options.mode` — `antigravity`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`antigravity`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `default`, `yolo`, `plan`
- 示例: `mode = "default"`

### `projects.agent.options.mode` — `claudecode`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `default`, `acceptEdits`, `plan`, `auto`, `bypassPermissions`, `dontAsk`
- 示例: `mode = "default"`

### `projects.agent.options.mode` — `codex`

选择 Codex 审批与沙箱模式。省略该键时保留 suggest 兼容回落；全新生成的配置会显式写入 yolo。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`suggest`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 允许值: `suggest`, `auto-edit`, `full-auto`, `yolo`
- 示例: `mode = "suggest"`
- 预设 `starter`: `yolo` — 全新生成配置。

### `projects.agent.options.mode` — `copilot`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`copilot`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `default`, `bypassPermissions`
- 示例: `mode = "default"`

### `projects.agent.options.mode` — `cursor`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`cursor`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `default`, `force`, `plan`, `ask`
- 示例: `mode = "default"`

### `projects.agent.options.mode` — `gemini`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`gemini`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `default`, `auto_edit`, `yolo`, `plan`
- 示例: `mode = "default"`

### `projects.agent.options.mode` — `iflow`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`iflow`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `default`, `auto-edit`, `plan`, `yolo`
- 示例: `mode = "default"`

### `projects.agent.options.mode` — `kimi`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`kimi`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `default`, `yolo`, `plan`, `quiet`
- 示例: `mode = "default"`

### `projects.agent.options.mode` — `opencode, pi, qoder`

选择 Agent 的审批、沙箱或规划模式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`opencode, pi, qoder`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `default`, `yolo`
- 示例: `mode = "default"`

### `projects.agent.options.model` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

选择新 Agent 会话的默认模型。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `model = "provider/model-name"`

### `projects.agent.options.model_context_window` — `codex`

声明 Codex 模型上下文窗口，用于用量展示和压缩决策。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 范围: `1` 到 `+∞`
- 示例: `model_context_window = 1`

### `projects.agent.options.pane` — `tmux`

选择用于 Agent 输入输出的 tmux pane。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`string`
- 要求：`可选`
- 默认值：`0`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `pane = "0"`

### `projects.agent.options.plugin_dir` — `claudecode`

加载一个或多个 Claude Code 插件目录。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string | string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `plugin_dir = "value"`

### `projects.agent.options.poll_interval_ms` — `tmux`

设置 tmux 输出轮询间隔（毫秒）。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`200`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞` `milliseconds`
- 示例: `poll_interval_ms = 200`

### `projects.agent.options.prompt_pattern` — `tmux`

用于识别 tmux Agent 提示符的正则表达式。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`string`
- 要求：`可选`
- 默认值：`[❯\$#>%]\s*$`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `prompt_pattern = "[❯\\$#>%]\\s*$"`

### `projects.agent.options.provider` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode`

选择当前项目使用的已配置 Provider。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`reload`
- 示例: `provider = "provider-name"`

### `projects.agent.options.reasoning_effort` — `claudecode`

设置新回合默认推理强度。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 允许值: `low`, `medium`, `high`, `max`
- 示例: `reasoning_effort = "low"`

### `projects.agent.options.reasoning_effort` — `codex`

设置新回合默认推理强度。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 允许值: `low`, `medium`, `high`, `xhigh`, `max`
- 示例: `reasoning_effort = "low"`

### `projects.agent.options.router_api_key` — `claudecode`

用于认证已配置的 Claude Code Router。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `router_api_key = "${ROUTER_API_KEY}"`

### `projects.agent.options.router_url` — `claudecode`

将 Claude Code 请求路由到指定 Router 地址。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `router_url = "value"`

### `projects.agent.options.run_as_env` — `claudecode`

扩展跨 OS 用户隔离传递的环境变量白名单。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `run_as_env = ["value"]`

### `projects.agent.options.run_as_user` — `claudecode`

以另一个非 root 操作系统用户运行 Claude Code。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `run_as_user = "value"`

### `projects.agent.options.service_tier` — `codex`

选择模型目录声明的服务档位；Codex 常见取值包括 default 和 fast。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 允许值: `model-catalog-driven (for example: default, fast)`
- 示例: `service_tier = "value"`

### `projects.agent.options.session` — `tmux`

指定承载 Agent 的 tmux 会话名。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`string`
- 要求：`必填`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `session = "agent-session"`

### `projects.agent.options.session_title_model` — `codex`

可选使用隔离的本地 Codex 模型生成简洁的 Codex App 标题。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `session_title_model = "value"`

### `projects.agent.options.session_title_prefix` — `codex`

为 Codex App 会话标题添加可配置的来源前缀。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`[飞书]`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `session_title_prefix = "[飞书]"`

### `projects.agent.options.shell` — `tmux`

选择 tmux 适配器使用的 shell。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `shell = "value"`

### `projects.agent.options.startup_wait_ms` — `tmux`

创建 tmux 会话后等待指定毫秒数。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`0 (or 2000 when init_command is set)`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `milliseconds`
- 示例: `startup_wait_ms = 1`

### `projects.agent.options.strip_input_block` — `tmux`

从捕获的 tmux 输出中移除回显输入块。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `strip_input_block = true`

### `projects.agent.options.strip_patterns` — `tmux`

移除匹配指定模式的输出行。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`string[]`
- 要求：`可选`
- 默认值：`built-in Claude mode-status pattern`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `strip_patterns = ["^status:"]`

### `projects.agent.options.system_prompt` — `claudecode, codex`

替换当前项目中 Agent 的默认系统提示。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`claudecode, codex`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `system_prompt = "value"`

### `projects.agent.options.thinking` — `pi`

配置 pi Agent 的思考模式或级别。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`pi`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `thinking = "value"`

### `projects.agent.options.timeout_mins` — `antigravity, gemini, kimi`

设置适配器进程超时分钟数；0 使用默认值。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`antigravity, gemini, kimi`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `minutes`
- 示例: `timeout_mins = 1`

### `projects.agent.options.tool_timeout_secs` — `iflow`

设置 iFlow 工具调用最大等待秒数。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`iflow`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `seconds`
- 示例: `tool_timeout_secs = 1`

### `projects.agent.options.window_per_session` — `tmux`

为每个 cc-connect-next 会话使用独立 tmux window。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`tmux`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `window_per_session = false`

### `projects.agent.options.work_dir` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder, tmux`

设置 Agent 使用的项目工作目录。

- 来源：`toml`
- 配置位置：`[projects.agent.options] (inside one [[projects]])`
- 作用域：`agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder, tmux`)
- 类型：`string`
- 要求：`可选`
- 默认值：`.`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 冲突: `projects.mode = multi-workspace`
- 示例: `work_dir = "/absolute/path/to/project"`

### `projects.agent.provider_refs`

从 projects.agent 引用共享 Provider 名称。

- 来源：`toml`
- 配置位置：`[projects.agent] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `provider_refs = ["value"]`

### `projects.agent.providers.agent_model_lists.<name>.alias`

为 projects.agent.providers.agent_model_lists.<name> 设置简短的用户可见别名。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers.agent_model_lists.<name>]] (inside one [[projects.agent.providers]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `alias = "value"`

### `projects.agent.providers.agent_model_lists.<name>.model`

设置该项目内 Provider 针对某个 Agent 类型暴露的模型名。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers.agent_model_lists.<name>]] (inside one [[projects.agent.providers]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `model = "value"`

### `projects.agent.providers.agent_models.<name>`

设置 projects.agent.providers.agent_models 中的一个命名条目。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `example = { example = "value" }`

### `projects.agent.providers.agent_types`

将 projects.agent.providers 限制给指定 Agent 适配器类型。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `agent_types = ["value"]`

### `projects.agent.providers.api_key`

认证 projects.agent.providers 发出的请求。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `projects.agent.providers.base_url`

覆盖 projects.agent.providers 的服务基础地址。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `base_url = "value"`

### `projects.agent.providers.codex.env_key`

指定 projects.agent.providers.codex 读取凭据的环境变量名。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `env_key = "value"`

### `projects.agent.providers.codex.http_headers.<name>`

设置 projects.agent.providers.codex.http_headers 中的一个命名条目。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `example = { example = "value" }`

### `projects.agent.providers.codex.wire_api`

选择 projects.agent.providers.codex 使用的 Wire API 协议。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `wire_api = "value"`

### `projects.agent.providers.endpoints.<name>`

设置 projects.agent.providers.endpoints 中的一个命名条目。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `example = { example = "value" }`

### `projects.agent.providers.env.<name>`

设置 projects.agent.providers.env 中的一个命名条目。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `example = { example = "value" }`

### `projects.agent.providers.model`

选择 projects.agent.providers 使用的模型。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `model = "value"`

### `projects.agent.providers.models.alias`

为 projects.agent.providers.models 设置简短的用户可见别名。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers.models]] (inside one [[projects.agent.providers]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `alias = "value"`

### `projects.agent.providers.models.model`

设置该项目内 Provider 暴露的模型名。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers.models]] (inside one [[projects.agent.providers]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `model = "value"`

### `projects.agent.providers.name`

为项目内模型 Provider 设置供切换使用的名称。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `name = "value"`

### `projects.agent.providers.thinking`

选择 projects.agent.providers 使用的 Provider 思考模式。

- 来源：`toml`
- 配置位置：`[[projects.agent.providers]] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `thinking = "value"`

### `projects.agent.type`

选择当前项目使用的 Agent 适配器。

- 来源：`toml`
- 配置位置：`[projects.agent] (inside one [[projects]])`
- 作用域：`agent`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 示例: `type = "value"`

### `projects.auto_compress.enabled`

接近配置的 Token 阈值时自动执行上下文压缩。

- 来源：`toml`
- 配置位置：`[projects.auto_compress] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `enabled = false`

### `projects.auto_compress.max_tokens`

设置触发自动压缩的估算 Token 阈值。

- 来源：`toml`
- 配置位置：`[projects.auto_compress] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`12000`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `tokens`
- 示例: `max_tokens = 12000`

### `projects.auto_compress.min_gap_mins`

设置两次自动压缩之间的最小间隔分钟数。

- 来源：`toml`
- 配置位置：`[projects.auto_compress] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`30`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `minutes`
- 示例: `min_gap_mins = 30`

### `projects.base_dir`

设置动态创建多工作区的父目录。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 必填条件: `projects.mode = multi-workspace`
- 冲突: `projects.agent.options.work_dir when projects.mode = multi-workspace`
- 示例: `base_dir = "value"`

### `projects.busy_message_mode`

为单个项目覆盖进程级忙时消息策略。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 允许值: `steer`, `queue`
- 示例: `busy_message_mode = "steer"`

### `projects.disabled_commands`

为当前项目禁用指定内置命令。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`[]`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `disabled_commands = ["value"]`

### `projects.display.card_mode`

为单个项目覆盖 rich 或 legacy 卡片渲染。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 允许值: `rich`, `legacy`
- 示例: `card_mode = "rich"`
- 预设 `starter/recommended-feishu`: `rich` — 使用 Card 2.0 回答。

### `projects.display.hide_agent_footer`

为单个项目覆盖 Agent 自带状态尾巴过滤。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 示例: `hide_agent_footer = true`
- 预设 `starter/recommended-feishu`: `true` — 移除重复的 Agent 状态尾巴。

### `projects.display.history_max_len`

限制每条 /history 记录长度；0 表示不截断。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `characters`
- 示例: `history_max_len = 1`

### `projects.display.mode`

选择 full、compact 或 quiet 回复展示。省略时使用 full 布局但不会开启思考/工具消息；显式写入 full 才启用该模式的消息默认值。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 允许值: `full`, `compact`, `quiet`
- 示例: `mode = "full"`

### `projects.display.reply_footer`

为单个项目覆盖回复底部状态栏。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 示例: `reply_footer = true`
- 预设 `starter/recommended-feishu`: `true` — 展示模型、推理强度和耗时。

### `projects.display.show_context_indicator`

已废弃的无效果配置，仅保留旧配置兼容。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 状态：已废弃，仅保留兼容。
- 示例: `show_context_indicator = true`

### `projects.display.thinking_max_len`

限制思考进度文本长度；0 表示不截断。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `characters`
- 示例: `thinking_max_len = 1`

### `projects.display.thinking_messages`

为单个项目覆盖思考进度可见性。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 示例: `thinking_messages = true`
- 预设 `starter/recommended-feishu`: `false` — 不在聊天中展示推理。

### `projects.display.tool_max_len`

限制工具进度文本长度；0 表示不截断。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `characters`
- 示例: `tool_max_len = 1`

### `projects.display.tool_messages`

为单个项目覆盖工具进度可见性。

- 来源：`toml`
- 配置位置：`[projects.display] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 示例: `tool_messages = true`
- 预设 `starter/recommended-feishu`: `false` — 不在聊天中展示工具详情。

### `projects.filter_external_sessions`

隐藏不是由 cc-connect-next 创建的 Agent 会话。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `filter_external_sessions = false`

### `projects.heartbeat.enabled`

定期唤醒主会话进行状态巡检或继续未完成工作。

- 来源：`toml`
- 配置位置：`[projects.heartbeat] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enabled = false`

### `projects.heartbeat.interval_mins`

设置心跳回合间隔。

- 来源：`toml`
- 配置位置：`[projects.heartbeat] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`30`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞` `minutes`
- 示例: `interval_mins = 30`

### `projects.heartbeat.only_when_idle`

仅在目标会话空闲时运行心跳。

- 来源：`toml`
- 配置位置：`[projects.heartbeat] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `only_when_idle = true`

### `projects.heartbeat.prompt`

设置心跳 Prompt；留空时从 Agent 工作目录读取 HEARTBEAT.md。

- 来源：`toml`
- 配置位置：`[projects.heartbeat] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`HEARTBEAT.md`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `prompt = "HEARTBEAT.md"`

### `projects.heartbeat.session_key`

选择接收心跳任务的会话。

- 来源：`toml`
- 配置位置：`[projects.heartbeat] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 必填条件: `projects.heartbeat.enabled = true`
- 依赖: `projects.heartbeat.enabled`
- 示例: `session_key = "value"`

### `projects.heartbeat.silent`

隐藏心跳开始提示。

- 来源：`toml`
- 配置位置：`[projects.heartbeat] (inside one [[projects]])`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `silent = true`

### `projects.heartbeat.timeout_mins`

限制单次心跳回合的分钟数。

- 来源：`toml`
- 配置位置：`[projects.heartbeat] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`30`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞` `minutes`
- 示例: `timeout_mins = 30`

### `projects.inject_sender`

在发送给 Agent 的提示前添加平台发送者身份。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `inject_sender = false`

### `projects.mode`

选择固定工作区或多工作区项目路由。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`fixed`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `fixed`, `multi-workspace`
- 示例: `mode = "fixed"`

### `projects.name`

设置供命令、存储和 Relay 路由使用的唯一项目名。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 示例: `name = "value"`

### `projects.observe.channel`

选择 projects.observe 使用的目标频道。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `channel = "value"`

### `projects.observe.enabled`

启用或关闭 projects.observe。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `enabled = true`

### `projects.platforms.options.access_token` — `matrix`

认证 Matrix 机器人账号。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`matrix`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `access_token = "${ACCESS_TOKEN}"`

### `projects.platforms.options.account_id` — `weixin`

为多个微信账号隔离持久化状态。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weixin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`default`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `account_id = "default"`

### `projects.platforms.options.agent_id` — `dingtalk`

指定平台租户中的机器人应用 Agent ID。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`integer`
- 要求：`条件必填`
- 默认值：`0`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 必填条件: `proactive work notifications are used`
- 示例: `agent_id = 123456`

### `projects.platforms.options.agent_id` — `wecom`

指定平台租户中的机器人应用 Agent ID。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 必填条件: `mode is unset or mode = callback`
- 示例: `agent_id = "value"`

### `projects.platforms.options.allow_chat` — `feishu, lark`

将访问限制为逗号分隔的飞书/Lark 会话 ID；留空或 '*' 表示允许所有会话。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`可选`
- 默认值：`empty`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `allow_chat = "oc_chat_id"`

### `projects.platforms.options.allow_from` — `dingtalk, discord, feishu, lark, line, matrix, max, qq, qqbot, slack, telegram, webex, wecom, weibo, weixin, wps-xiezuo`

将机器人访问限制在指定平台用户 ID；留空或 '*' 表示允许所有平台用户。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk, discord, feishu, lark, line, matrix, max, qq, qqbot, slack, telegram, webex, wecom, weibo, weixin, wps-xiezuo`)
- 类型：`string`
- 要求：`可选`
- 默认值：`empty`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `allow_from = "user-id-1,user-id-2"`

### `projects.platforms.options.api_base` — `max`

覆盖平台 REST API 基础地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`max`)
- 类型：`string`
- 要求：`可选`
- 默认值：`https://platform-api.max.ru`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `api_base = "https://platform-api.max.ru"`

### `projects.platforms.options.api_base_url` — `wecom`

覆盖平台 API 基础地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`可选`
- 默认值：`https://qyapi.weixin.qq.com`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `api_base_url = "https://qyapi.weixin.qq.com"`

### `projects.platforms.options.app_id` — `feishu, lark`

标识飞书/Lark 机器人应用；此配置必填。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 示例: `app_id = "value"`

### `projects.platforms.options.app_id` — `qqbot, weibo, wps-xiezuo`

标识机器人应用。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`qqbot, weibo, wps-xiezuo`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 示例: `app_id = "value"`

### `projects.platforms.options.app_secret` — `feishu, lark`

认证飞书/Lark 机器人应用；此敏感配置必填。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `app_secret = "${APP_SECRET}"`

### `projects.platforms.options.app_secret` — `qqbot, weibo, wps-xiezuo`

认证机器人应用。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`qqbot, weibo, wps-xiezuo`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `app_secret = "${APP_SECRET}"`

### `projects.platforms.options.app_token` — `slack`

认证 Slack Socket Mode。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`slack`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `app_token = "${APP_TOKEN}"`

### `projects.platforms.options.auto_join` — `matrix`

自动加入受邀的 Matrix 房间。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`matrix`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `auto_join = true`

### `projects.platforms.options.auto_verify` — `matrix`

自动接受 Matrix SAS 设备验证。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`matrix`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `auto_verify = true`

### `projects.platforms.options.base_url` — `weixin`

覆盖平台服务基础地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weixin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`https://ilinkai.weixin.qq.com`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `base_url = "https://ilinkai.weixin.qq.com"`

### `projects.platforms.options.base_url` — `wps-xiezuo`

覆盖平台服务基础地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wps-xiezuo`)
- 类型：`string`
- 要求：`可选`
- 默认值：`https://openapi.wps.cn`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `base_url = "https://openapi.wps.cn"`

### `projects.platforms.options.bot_id` — `wecom`

标识企业微信 WebSocket 机器人。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 必填条件: `mode = websocket`
- 示例: `bot_id = "value"`

### `projects.platforms.options.bot_secret` — `wecom`

认证企业微信 WebSocket 机器人。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `mode = websocket`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `bot_secret = "${BOT_SECRET}"`

### `projects.platforms.options.bot_token` — `slack`

认证 Slack 机器人用户。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`slack`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `bot_token = "${BOT_TOKEN}"`

### `projects.platforms.options.burst_limit` — `weixin`

限制一个窗口内独立发送的微信消息数量。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weixin`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`4`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `burst_limit = 4`

### `projects.platforms.options.burst_window_secs` — `weixin`

设置微信出站突发窗口长度（秒）。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weixin`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`86400`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 单位: `seconds`
- 示例: `burst_window_secs = 86400`

### `projects.platforms.options.callback_aes_key` — `wecom`

解密企业微信回调负载。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `mode is unset or mode = callback`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `callback_aes_key = "${CALLBACK_AES_KEY}"`

### `projects.platforms.options.callback_path` — `feishu, lark`

设置 encrypt_key 启用 Webhook 模式后使用的入站回调路径。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`可选`
- 默认值：`/feishu/webhook`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `callback_path = "/feishu/webhook"`

### `projects.platforms.options.callback_path` — `line`

设置入站 Webhook 回调路径。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`line`)
- 类型：`string`
- 要求：`可选`
- 默认值：`/callback`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `callback_path = "/callback"`

### `projects.platforms.options.callback_path` — `wecom`

设置入站 Webhook 回调路径。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`可选`
- 默认值：`/wecom/callback`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `callback_path = "/wecom/callback"`

### `projects.platforms.options.callback_token` — `wecom`

验证企业微信回调请求。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `mode is unset or mode = callback`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `callback_token = "${CALLBACK_TOKEN}"`

### `projects.platforms.options.card_template_id` — `dingtalk`

选择钉钉互动卡片模板 ID。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `card_template_id = "value"`

### `projects.platforms.options.card_template_key` — `dingtalk`

选择钉钉卡片模板 Key。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 要求：`可选`
- 默认值：`content`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `card_template_key = "content"`

### `projects.platforms.options.card_throttle_ms` — `dingtalk`

限制钉钉卡片更新频率（毫秒）。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`300`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞` `milliseconds`
- 示例: `card_throttle_ms = 300`

### `projects.platforms.options.cdn_base_url` — `weixin`

覆盖微信 CDN 下载/上传基础地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weixin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`https://novac2c.cdn.weixin.qq.com/c2c`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `cdn_base_url = "https://novac2c.cdn.weixin.qq.com/c2c"`

### `projects.platforms.options.channel_secret` — `line`

验证 LINE Webhook 签名。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`line`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `channel_secret = "${CHANNEL_SECRET}"`

### `projects.platforms.options.channel_token` — `line`

认证 LINE Messaging API 请求。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`line`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `channel_token = "${CHANNEL_TOKEN}"`

### `projects.platforms.options.clean_reply` — `wps-xiezuo`

从 WPS 回复中移除思考和工具进度行。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wps-xiezuo`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `clean_reply = false`

### `projects.platforms.options.client_id` — `dingtalk`

标识钉钉应用客户端。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 示例: `client_id = "value"`

### `projects.platforms.options.client_secret` — `dingtalk`

认证钉钉应用客户端。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `client_secret = "${CLIENT_SECRET}"`

### `projects.platforms.options.corp_id` — `wecom`

标识企业微信企业。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 必填条件: `mode is unset or mode = callback`
- 示例: `corp_id = "value"`

### `projects.platforms.options.corp_secret` — `wecom`

认证企业微信应用。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `mode is unset or mode = callback`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `corp_secret = "${CORP_SECRET}"`

### `projects.platforms.options.cross_signing_password` — `matrix`

服务器需要账号密码时初始化 Matrix 跨签名。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`matrix`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 依赖: `MATRIX_CROSS_SIGNING_PASSWORD may be used instead`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `cross_signing_password = "${CROSS_SIGNING_PASSWORD}"`

### `projects.platforms.options.domain` — `feishu`

覆盖飞书/Lark OpenAPI 与 WebSocket 基础地址；飞书和 Lark 使用不同的 SDK 默认值。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu`)
- 类型：`string`
- 要求：`可选`
- 默认值：`https://open.feishu.cn`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `domain = "https://open.feishu.cn"`

### `projects.platforms.options.domain` — `lark`

覆盖飞书/Lark OpenAPI 与 WebSocket 基础地址；飞书和 Lark 使用不同的 SDK 默认值。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`lark`)
- 类型：`string`
- 要求：`可选`
- 默认值：`https://open.larksuite.com`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `domain = "https://open.larksuite.com"`

### `projects.platforms.options.done_emoji` — `dingtalk`

选择完成表情；'none' 表示关闭。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 要求：`可选`
- 默认值：`Done`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `done_emoji = "Done"`

### `projects.platforms.options.done_emoji` — `feishu, lark`

选择完成时的表情回应。'none' 表示关闭；reaction_emoji = 'none' 也会关闭隐式完成回应，除非显式设置 done_emoji。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`可选`
- 默认值：`Done`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `done_emoji = "Done"`
- 预设 `starter/recommended-feishu`: `Done` — 固定完成通知表情。

### `projects.platforms.options.enable_feishu_card` — `feishu, lark`

使用飞书/Lark 互动卡片回复；设为 false 时回退为非卡片回复。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enable_feishu_card = true`
- 预设 `starter/recommended-feishu`: `true` — 使用互动回答卡片。

### `projects.platforms.options.enable_markdown` — `wecom`

启用企业微信回复的 Markdown 格式。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enable_markdown = false`

### `projects.platforms.options.enable_reactions` — `telegram`

为收到的消息添加处理中表情。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`telegram`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enable_reactions = false`

### `projects.platforms.options.encrypt_key` — `feishu, lark`

留空时通过 WebSocket 消费事件；设置事件 Encrypt Key 后切换为 Webhook 模式并解密 Webhook 事件。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `encrypt_key = "${ENCRYPT_KEY}"`

### `projects.platforms.options.group_only` — `feishu, lark`

仅接受飞书群聊消息。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `group_only = false`

### `projects.platforms.options.group_reply_all` — `discord, matrix, telegram`

无需 @ 即回复所有群消息。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord, matrix, telegram`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `group_reply_all = false`

### `projects.platforms.options.group_reply_all` — `feishu, lark`

无需明确 @ 机器人即可回复所有群消息；但非空 group_reply_all_chats 白名单优先。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `group_reply_all = false`
- 预设 `starter/recommended-feishu`: `true` — 群聊无需 @ 即回复；应先配置 allow_from/allow_chat 范围。

### `projects.platforms.options.group_reply_all_chats` — `feishu, lark`

仅在指定会话 ID 中允许无需 @ 的回复。支持逗号分隔字符串或字符串数组；非空列表优先于 group_reply_all。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string | string[]`
- 要求：`可选`
- 默认值：`empty`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `group_reply_all_chats = "oc_chat_a,oc_chat_b"`

### `projects.platforms.options.group_reply_all_guilds` — `discord`

为逗号分隔的 Discord 服务器 ID 列表启用无需 @ 的回复；非空列表优先于 group_reply_all。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord`)
- 类型：`string`
- 要求：`可选`
- 默认值：`empty`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `group_reply_all_guilds = "guild-a,guild-b"`

### `projects.platforms.options.guild_id` — `discord`

将 Discord 命令注册限制到单个服务器以加快生效。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `guild_id = "value"`

### `projects.platforms.options.homeserver` — `matrix`

设置 Matrix Homeserver 地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`matrix`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 示例: `homeserver = "value"`

### `projects.platforms.options.http_url` — `qq`

设置 NapCat/QQ HTTP API 地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`qq`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `http_url = "value"`

### `projects.platforms.options.image_batch_window_ms` — `feishu, lark`

同一会话的连续图片在该静默窗口（毫秒）后合并处理。0 仍使用 500 ms 回退值；负数会被拒绝。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`500`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 单位: `milliseconds`
- 示例: `image_batch_window_ms = 500`

### `projects.platforms.options.intents` — `qqbot`

设置 QQ Bot Gateway Intent 位掩码。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`qqbot`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`100663296`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞`
- 示例: `intents = 100663296`

### `projects.platforms.options.long_poll_timeout_ms` — `weixin`

设置微信长轮询超时（毫秒）。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weixin`)
- 类型：`integer`
- 要求：`可选`
- 默认值：`35000`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `milliseconds`
- 示例: `long_poll_timeout_ms = 35000`

### `projects.platforms.options.markdown_support` — `qqbot`

启用 QQ Bot Markdown 消息。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`qqbot`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `markdown_support = false`

### `projects.platforms.options.mention_map` — `feishu, lark`

将友好机器人名称映射到 open_id，以便出站消息使用原生 @；要求 resolve_mentions = true。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`table`
- 要求：`可选`
- 默认值：`empty`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 依赖: `resolve_mentions = true`
- 示例: `mention_map = { Reviewer-Bot = "ou_bot_open_id" }`

### `projects.platforms.options.mode` — `wecom`

选择平台连接模式，例如 WebSocket 或回调。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`可选`
- 默认值：`callback`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `callback`, `websocket`
- 示例: `mode = "callback"`

### `projects.platforms.options.name` — `weibo`

设置平台适配器使用的账号显示名称。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weibo`)
- 类型：`string`
- 要求：`可选`
- 默认值：`weibo`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `name = "weibo"`

### `projects.platforms.options.peer_bots` — `feishu, lark`

将每个对端机器人 app_id 映射为友好别名，用于引用回复归因。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`table`
- 要求：`可选`
- 默认值：`empty`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `peer_bots = { cli_peer_app_id = "Reviewer-Bot" }`

### `projects.platforms.options.port` — `feishu, lark`

以带引号的字符串设置 Webhook 模式监听端口。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`可选`
- 默认值：`8080`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `port = "8080"`

### `projects.platforms.options.port` — `line`

以带引号的字符串设置入站 Webhook 监听端口。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`line`)
- 类型：`string`
- 要求：`可选`
- 默认值：`8080`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `port = "8080"`

### `projects.platforms.options.port` — `wecom`

以带引号的字符串设置入站 Webhook 监听端口。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`wecom`)
- 类型：`string`
- 要求：`可选`
- 默认值：`8081`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `port = "8081"`

### `projects.platforms.options.progress_style` — `discord, telegram`

选择消息平台上的进度展示样式。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord, telegram`)
- 类型：`string`
- 要求：`可选`
- 默认值：`compact`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `legacy`, `compact`, `card`
- 示例: `progress_style = "legacy"`

### `projects.platforms.options.progress_style` — `feishu, lark`

选择飞书/Lark 回复的 legacy、compact 或 card 进度展示样式。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`可选`
- 默认值：`legacy`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `legacy`, `compact`, `card`
- 示例: `progress_style = "legacy"`

### `projects.platforms.options.proxy` — `discord, matrix, telegram, wecom, weixin`

通过 HTTP 或 SOCKS5 代理转发平台 HTTP/WebSocket 流量。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord, matrix, telegram, wecom, weixin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `proxy = "value"`

### `projects.platforms.options.proxy_password` — `discord, telegram, wecom, weixin`

认证已配置的平台代理。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord, telegram, wecom, weixin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 依赖: `proxy`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `proxy_password = "${PROXY_PASSWORD}"`

### `projects.platforms.options.proxy_username` — `discord, telegram, wecom, weixin`

设置平台代理认证用户名。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord, telegram, wecom, weixin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 依赖: `proxy`
- 示例: `proxy_username = "value"`

### `projects.platforms.options.reaction_emoji` — `dingtalk`

选择处理中表情。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 要求：`可选`
- 默认值：`🤔Thinking`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `reaction_emoji = "🤔Thinking"`

### `projects.platforms.options.reaction_emoji` — `feishu, lark`

选择处理中表情回应；'none' 会关闭它，并同时抑制隐式完成回应。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string`
- 要求：`可选`
- 默认值：`OnIt`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `reaction_emoji = "OnIt"`

### `projects.platforms.options.reply_to_trigger` — `feishu, lark`

以触发消息作为回复目标。设为 false 时普通回复不再引用该消息；由 thread_isolation 隔离的真实话题仍会指向该话题的 thread_id，以保持话题归属。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `reply_to_trigger = true`
- 预设 `starter/recommended-feishu`: `true` — 在普通会话中引用触发消息。

### `projects.platforms.options.require_mention` — `feishu, lark`

群聊中要求明确 @ 机器人。设为 false 是 group_reply_all = true 的兼容别名；true 不会覆盖显式 group_reply_all 配置。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `require_mention = true`

### `projects.platforms.options.resolve_mentions` — `feishu, lark`

将入站飞书/Lark @ 解析为可读名称，并启用 mention_map 以发送原生机器人 @。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `resolve_mentions = false`

### `projects.platforms.options.respond_to_at_everyone_and_here` — `discord, feishu, lark`

将 @everyone/@here 视为有效的机器人提及。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord, feishu, lark`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `respond_to_at_everyone_and_here = false`

### `projects.platforms.options.robot_code` — `dingtalk`

标识用于出站消息的钉钉机器人。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk`)
- 类型：`string`
- 要求：`可选`
- 默认值：`client_id`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `robot_code = "client_id"`

### `projects.platforms.options.route_tag` — `weixin`

设置可选的微信 SKRouteTag 请求头。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weixin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `route_tag = "value"`

### `projects.platforms.options.sandbox` — `qqbot`

使用 QQ Bot 沙箱环境。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`qqbot`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `sandbox = false`

### `projects.platforms.options.session_scope` — `slack`

选择 Slack 会话按用户、频道还是线程隔离。省略时，旧 share_session_in_channel=true 会把有效默认值改为 channel。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`slack`)
- 类型：`string`
- 要求：`可选`
- 默认值：`user`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 允许值: `user`, `channel`, `thread`
- 示例: `session_scope = "user"`

### `projects.platforms.options.share_session_in_channel` — `dingtalk, discord, matrix, qq, qqbot, slack, telegram`

让频道或房间内所有用户共享同一个 Agent 会话。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`dingtalk, discord, matrix, qq, qqbot, slack, telegram`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `share_session_in_channel = false`

### `projects.platforms.options.share_session_in_channel` — `feishu, lark`

让同一非隔离频道内的用户共享一个 Agent 会话；thread_isolation 仍可为真实话题建立独立会话。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `share_session_in_channel = false`

### `projects.platforms.options.state_dir` — `weixin`

覆盖平台持久化状态目录。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weixin`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `state_dir = "value"`

### `projects.platforms.options.thread_isolation` — `discord`

为每个话题或线程使用独立 Agent 会话。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord`)
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `thread_isolation = false`

### `projects.platforms.options.thread_isolation` — `feishu, lark`

选择飞书/Lark 话题隔离范围。off 沿用旧版按用户/频道会话；topics_only 隔离事件携带 thread_id 的所有真实话题（包括 P2P 私聊话题），普通群消息留在群主会话，普通无话题私聊保持原会话；topic_per_message 还会让每条群主会话消息拥有独立话题/session。两种启用模式都会给真实话题独立 Agent 会话和工作区绑定。省略该键映射 off；旧 true 映射 topic_per_message，旧 false 映射 off；新 Starter 和推荐 Profile 写入 topics_only。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`feishu, lark`)
- 类型：`string | boolean (legacy)`
- 要求：`可选`
- 默认值：`off`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `off`, `topics_only`, `topic_per_message`
- 示例: `thread_isolation = "off"`
- 预设 `starter/recommended-feishu`: `topics_only` — 隔离真实话题，不把普通群消息提升为话题。

### `projects.platforms.options.token` — `discord, max, telegram, webex, weixin`

认证平台机器人或网关。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`discord, max, telegram, webex, weixin`)
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `token = "${TOKEN}"`

### `projects.platforms.options.token` — `qq`

认证平台机器人或网关。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`qq`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `token = "${TOKEN}"`

### `projects.platforms.options.token_endpoint` — `weibo`

覆盖获取微博访问令牌的端点。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weibo`)
- 类型：`string`
- 要求：`可选`
- 默认值：`https://open-im.api.weibo.com/open/auth/ws_token`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `token_endpoint = "https://open-im.api.weibo.com/open/auth/ws_token"`

### `projects.platforms.options.user_id` — `matrix`

设置或覆盖 Matrix 机器人用户 ID。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`matrix`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `user_id = "value"`

### `projects.platforms.options.webhook_listen` — `max`

设置本地 Webhook 监听地址；设置 webhook_url 且省略本项时，运行时使用 :8080。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`max`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `webhook_listen = "value"`

### `projects.platforms.options.webhook_path` — `max`

设置 MAX Webhook URL 路径。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`max`)
- 类型：`string`
- 要求：`可选`
- 默认值：`/webhook`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `webhook_path = "/webhook"`

### `projects.platforms.options.webhook_resubscribe_interval` — `max`

使用 Go duration 字符串定期刷新 MAX Webhook 订阅。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`max`)
- 类型：`string`
- 要求：`可选`
- 默认值：`5m`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `Go duration string (for example: 30s, 5m, 1h)`
- 示例: `webhook_resubscribe_interval = "5m"`

### `projects.platforms.options.webhook_secret` — `max`

验证 MAX Webhook 请求。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`max`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `webhook_secret = "${WEBHOOK_SECRET}"`

### `projects.platforms.options.webhook_url` — `max`

设置外部可访问的 MAX Webhook 地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`max`)
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`adapter`
- 生效方式：`restart`
- 示例: `webhook_url = "value"`

### `projects.platforms.options.ws_endpoint` — `weibo`

覆盖平台 WebSocket 地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`weibo`)
- 类型：`string`
- 要求：`可选`
- 默认值：`ws://open-im.api.weibo.com/ws/stream`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `ws_endpoint = "ws://open-im.api.weibo.com/ws/stream"`

### `projects.platforms.options.ws_url` — `qq`

设置 NapCat/QQ 正向 WebSocket 地址。

- 来源：`toml`
- 配置位置：`[projects.platforms.options] (inside one [[projects.platforms]])`
- 作用域：`platform` (`qq`)
- 类型：`string`
- 要求：`可选`
- 默认值：`ws://127.0.0.1:3001`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `ws_url = "ws://127.0.0.1:3001"`

### `projects.platforms.type`

为当前条目选择消息平台适配器；正常运行的项目至少需要一个平台条目。

- 来源：`toml`
- 配置位置：`[[projects.platforms]] (inside one [[projects]])`
- 作用域：`platform`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`restart`
- 示例: `type = "value"`

### `projects.quiet`

旧版项目级静默开关；未设置 Display 覆盖时隐藏思考和工具消息。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 状态：已废弃，仅保留兼容。
- 示例: `quiet = true`

### `projects.references.display_path`

选择项目引用展示给用户的路径形式。

- 来源：`toml`
- 配置位置：`[projects.references] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`absolute`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `absolute`, `relative`, `basename`, `dirname_basename`, `smart`
- 示例: `display_path = "absolute"`
- 预设 `starter/recommended-feishu`: `smart` — 简短但不歧义的路径。

### `projects.references.enclosure_style`

选择标准化项目引用的包裹样式。

- 来源：`toml`
- 配置位置：`[projects.references] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`none`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `none`, `bracket`, `angle`, `fullwidth`, `code`
- 示例: `enclosure_style = "none"`
- 预设 `starter/recommended-feishu`: `code` — 让引用便于复制。

### `projects.references.marker_style`

选择标准化项目引用使用的标记样式。

- 来源：`toml`
- 配置位置：`[projects.references] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`none`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `none`, `ascii`, `emoji`
- 示例: `marker_style = "none"`
- 预设 `starter/recommended-feishu`: `emoji` — 用视觉标记突出引用。

### `projects.references.normalize_agents`

仅对列出的 Agent 适配器应用引用标准化。

- 来源：`toml`
- 配置位置：`[projects.references] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`[]`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `all`, `codex`, `claudecode`
- 示例: `normalize_agents = ["all"]`
- 预设 `starter/recommended-feishu`: `["<active-agent>"]` — 标准化当前 Agent 的引用。

### `projects.references.render_platforms`

仅在列出的消息平台渲染标准化引用。

- 来源：`toml`
- 配置位置：`[projects.references] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`[]`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `all`, `feishu`, `weixin`
- 示例: `render_platforms = ["all"]`
- 预设 `starter/recommended-feishu`: `["feishu"]` — 为飞书渲染引用。

### `projects.reply_footer`

为单个项目覆盖回复底部状态栏。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`reload`
- 示例: `reply_footer = true`

### `projects.reset_on_idle_mins`

用户空闲指定时间后回来时切换到新会话；0 表示禁用。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`0`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `0` 到 `+∞` `minutes`
- 示例: `reset_on_idle_mins = 0`

### `projects.run_as_env`

允许列出的环境变量名通过 projects 用户隔离边界。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `run_as_env = ["value"]`

### `projects.run_as_user`

以另一个非 root 操作系统用户运行当前项目 Agent。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `run_as_user = "value"`

### `projects.shell`

选择 projects 使用的 Shell。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `shell = "value"`

### `projects.shell_profile`

为 projects 添加 Shell 初始化命令。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `shell_profile = "value"`

### `projects.show_context_indicator`

已废弃的无效果项目配置，仅保留旧配置兼容。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 状态：已废弃，仅保留兼容。
- 示例: `show_context_indicator = false`

### `projects.show_workdir_indicator`

已废弃的无效果项目配置，仅保留旧配置兼容。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 状态：已废弃，仅保留兼容。
- 示例: `show_workdir_indicator = false`

### `projects.skip_git`

允许多工作区目录不是 Git 仓库。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `skip_git = false`

### `projects.users.default_role`

选择未显式列出用户的默认角色。

- 来源：`toml`
- 配置位置：`[projects.users] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string`
- 要求：`可选`
- 默认值：`member`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 示例: `default_role = "member"`

### `projects.users.roles.<name>.disabled_commands`

为 projects.users.roles.<name> 禁用列出的命令。

- 来源：`toml`
- 配置位置：`[projects.users.roles.<name>] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `disabled_commands = ["value"]`

### `projects.users.roles.<name>.rate_limit.max_messages`

为该角色覆盖入站消息数量；0 表示禁用。

- 来源：`toml`
- 配置位置：`[projects.users.roles.<name>.rate_limit] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`20`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `0` 到 `+∞`
- 示例: `max_messages = 20`

### `projects.users.roles.<name>.rate_limit.window_secs`

为该角色覆盖入站限流窗口。

- 来源：`toml`
- 配置位置：`[projects.users.roles.<name>.rate_limit] (inside one [[projects]])`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`60`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 范围: `1` 到 `+∞` `seconds`
- 示例: `window_secs = 60`

### `projects.users.roles.<name>.user_ids`

列出分配给该角色的平台用户 ID；一个角色可使用 '*' 通配。

- 来源：`toml`
- 配置位置：`[projects.users.roles.<name>] (inside one [[projects]])`
- 作用域：`project`
- 类型：`string[]`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `user_ids = ["user-id"]`

### `projects.workspace_idle_timeout_mins`

已废弃的项目级工作区回收超时；请改用顶层配置。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`integer`
- 要求：`可选`
- 默认值：`inherit`
- 默认值来源：`inherit`
- 生效方式：`restart`
- 单位: `minutes`
- 状态：已废弃，仅保留兼容。
- 示例: `workspace_idle_timeout_mins = 1`

### `projects.workspace_init_allow_local_paths`

允许 /workspace init 除 Git URL 外绑定本地目录。

- 来源：`toml`
- 配置位置：`[[projects]]`
- 作用域：`project`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `workspace_init_allow_local_paths = false`

### `provider_presets_url`

覆盖推荐 Provider 预设使用的远程 JSON 地址。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `provider_presets_url = "value"`

### `providers.agent_model_lists.<name>.alias`

为 providers.agent_model_lists.<name> 设置简短的用户可见别名。

- 来源：`toml`
- 配置位置：`[[providers.agent_model_lists.<name>]] (inside one [[providers]])`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `alias = "value"`

### `providers.agent_model_lists.<name>.model`

设置该 Provider 针对某个 Agent 类型暴露的模型名。

- 来源：`toml`
- 配置位置：`[[providers.agent_model_lists.<name>]] (inside one [[providers]])`
- 作用域：`global`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `model = "value"`

### `providers.agent_models.<name>`

设置 providers.agent_models 中的一个命名条目。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `example = { example = "value" }`

### `providers.agent_types`

将共享 Provider 限制给指定 Agent 适配器类型。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `agent_types = ["value"]`

### `providers.api_key`

认证共享模型 Provider。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `providers.base_url`

覆盖共享 Provider API 基础地址。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `base_url = "value"`

### `providers.codex.env_key`

指定 providers.codex 读取凭据的环境变量名。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `env_key = "value"`

### `providers.codex.http_headers.<name>`

设置 providers.codex.http_headers 中的一个命名条目。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `example = { example = "value" }`

### `providers.codex.wire_api`

选择 providers.codex 使用的 Wire API 协议。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `wire_api = "value"`

### `providers.endpoints.<name>`

设置 providers.endpoints 中的一个命名条目。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `example = { example = "value" }`

### `providers.env.<name>`

设置 providers.env 中的一个命名条目。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`table`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`reload`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `example = { example = "value" }`

### `providers.model`

选择 providers 使用的模型。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `model = "value"`

### `providers.models.alias`

为 providers.models 设置简短的用户可见别名。

- 来源：`toml`
- 配置位置：`[[providers.models]] (inside one [[providers]])`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `alias = "value"`

### `providers.models.model`

设置该共享 Provider 暴露的模型名。

- 来源：`toml`
- 配置位置：`[[providers.models]] (inside one [[providers]])`
- 作用域：`global`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `model = "value"`

### `providers.name`

为共享模型 Provider 设置供引用和切换使用的名称。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`string`
- 要求：`必填`
- 默认值：`none`
- 默认值来源：`none`
- 生效方式：`reload`
- 示例: `name = "value"`

### `providers.thinking`

选择 providers 使用的 Provider 思考模式。

- 来源：`toml`
- 配置位置：`[[providers]]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`reload`
- 示例: `thinking = "value"`

### `queue.busy_message_mode`

将符合条件的新输入 steer 到当前回合，或始终保持 FIFO 排队。

- 来源：`toml`
- 配置位置：`[queue]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`steer`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `steer`, `queue`
- 示例: `busy_message_mode = "steer"`

### `queue.max_depth`

限制一个忙碌会话后等待的用户消息数量。

- 来源：`toml`
- 配置位置：`[queue]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`5`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞`
- 示例: `max_depth = 5`

### `quiet`

旧版静默开关；未设置新版 Display 字段时隐藏思考和工具消息。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`reload`
- 状态：已废弃，仅保留兼容。
- 示例: `quiet = false`

### `rate_limit.max_messages`

限制每个用户/会话窗口内的入站消息数；0 表示禁用。

- 来源：`toml`
- 配置位置：`[rate_limit]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`20`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞`
- 示例: `max_messages = 20`

### `rate_limit.window_secs`

设置入站限流窗口秒数。

- 来源：`toml`
- 配置位置：`[rate_limit]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`60`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `1` 到 `+∞` `seconds`
- 示例: `window_secs = 60`

### `relay.timeout_secs`

限制跨项目 Relay 等待回复的时长；0 表示禁用等待。

- 来源：`toml`
- 配置位置：`[relay]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`120`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `seconds`
- 示例: `timeout_secs = 120`

### `relay.visibility`

选择群内展示多少 Relay 活动。

- 来源：`toml`
- 配置位置：`[relay]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`full`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `full`, `summary`, `none`
- 示例: `visibility = "full"`

### `shell`

选择 /shell、exec Cron、Hooks 和 Webhook exec 使用的 Shell。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`sh on Unix; powershell.exe on Windows`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `shell = "value"`

### `shell_profile`

在每条配置的 Shell 命令前执行初始化命令。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `shell_profile = "value"`

### `speech.enabled`

将收到的语音消息转写后再发送给 Agent。

- 来源：`toml`
- 配置位置：`[speech]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enabled = false`

### `speech.gemini.api_key`

认证所选语音转文字 Provider。

- 来源：`toml`
- 配置位置：`[speech.gemini]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `speech.enabled = true and speech.provider = gemini`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `speech.gemini.model`

选择 speech.gemini 使用的模型。

- 来源：`toml`
- 配置位置：`[speech.gemini]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `model = "value"`

### `speech.groq.api_key`

认证所选语音转文字 Provider。

- 来源：`toml`
- 配置位置：`[speech.groq]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `speech.enabled = true and speech.provider = groq`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `speech.groq.model`

选择 speech.groq 使用的模型。

- 来源：`toml`
- 配置位置：`[speech.groq]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `model = "value"`

### `speech.language`

设置 speech 使用的语言或 Locale 提示。

- 来源：`toml`
- 配置位置：`[speech]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `language = "value"`

### `speech.openai.api_key`

认证所选语音转文字 Provider。

- 来源：`toml`
- 配置位置：`[speech.openai]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `speech.enabled = true and speech.provider is unset or openai`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `speech.openai.base_url`

覆盖 speech.openai 的服务基础地址。

- 来源：`toml`
- 配置位置：`[speech.openai]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `base_url = "value"`

### `speech.openai.model`

选择 speech.openai 使用的模型。

- 来源：`toml`
- 配置位置：`[speech.openai]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `model = "value"`

### `speech.provider`

选择语音转文字 Provider。

- 来源：`toml`
- 配置位置：`[speech]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`openai`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 必填条件: `speech.enabled = true`
- 允许值: `openai`, `groq`, `qwen`, `gemini`
- 示例: `provider = "openai"`

### `speech.qwen.api_key`

认证所选语音转文字 Provider。

- 来源：`toml`
- 配置位置：`[speech.qwen]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `speech.enabled = true and speech.provider = qwen`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `speech.qwen.base_url`

覆盖 speech.qwen 的服务基础地址。

- 来源：`toml`
- 配置位置：`[speech.qwen]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `base_url = "value"`

### `speech.qwen.model`

选择 speech.qwen 使用的模型。

- 来源：`toml`
- 配置位置：`[speech.qwen]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `model = "value"`

### `stream_preview.disabled_platforms`

在列出的消息平台上关闭 stream_preview。

- 来源：`toml`
- 配置位置：`[stream_preview]`
- 作用域：`global`
- 类型：`string[]`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `disabled_platforms = ["value"]`

### `stream_preview.enabled`

Agent 流式输出期间持续更新一条预览消息。

- 来源：`toml`
- 配置位置：`[stream_preview]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enabled = true`

### `stream_preview.interval_ms`

设置流式预览更新的最小间隔。

- 来源：`toml`
- 配置位置：`[stream_preview]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`1500`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `milliseconds`
- 示例: `interval_ms = 1500`

### `stream_preview.max_chars`

限制累计流式预览长度。

- 来源：`toml`
- 配置位置：`[stream_preview]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`2000`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `characters`
- 示例: `max_chars = 2000`

### `stream_preview.min_delta_chars`

至少新增指定字符数后才刷新预览。

- 来源：`toml`
- 配置位置：`[stream_preview]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`30`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `characters`
- 示例: `min_delta_chars = 30`

### `tts.agents.<name>.language_type`

设置 tts.agents.<name> 使用的 Provider 专属语言提示。

- 来源：`toml`
- 配置位置：`[tts.agents.<name>]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `language_type = "value"`

### `tts.agents.<name>.max_text_len`

文本超过该长度时跳过或截断 tts.agents.<name>；0 表示不限制。

- 来源：`toml`
- 配置位置：`[tts.agents.<name>]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `max_text_len = 1`

### `tts.agents.<name>.provider`

选择 tts.agents.<name> 使用的 Provider。

- 来源：`toml`
- 配置位置：`[tts.agents.<name>]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `provider = "value"`

### `tts.agents.<name>.speed`

设置 tts.agents.<name> 使用的语速倍率。

- 来源：`toml`
- 配置位置：`[tts.agents.<name>]`
- 作用域：`global`
- 类型：`number`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `speed = 1.0`

### `tts.agents.<name>.voice`

选择 tts.agents.<name> 使用的音色。

- 来源：`toml`
- 配置位置：`[tts.agents.<name>]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `voice = "value"`

### `tts.agents.<name>.voice_id`

设置 tts.agents.<name> 使用的 Provider 专属音色 ID。

- 来源：`toml`
- 配置位置：`[tts.agents.<name>]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `voice_id = "value"`

### `tts.enabled`

启用文字转语音回复。

- 来源：`toml`
- 配置位置：`[tts]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enabled = false`

### `tts.language_type`

设置 tts 使用的 Provider 专属语言提示。

- 来源：`toml`
- 配置位置：`[tts]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `language_type = "value"`

### `tts.max_text_len`

文本超过该长度时跳过或截断 tts；0 表示不限制。

- 来源：`toml`
- 配置位置：`[tts]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `max_text_len = 1`

### `tts.mimo.api_key`

认证所选文字转语音 Provider。

- 来源：`toml`
- 配置位置：`[tts.mimo]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `tts.enabled = true and tts.provider = mimo`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `tts.mimo.base_url`

覆盖 tts.mimo 的服务基础地址。

- 来源：`toml`
- 配置位置：`[tts.mimo]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `base_url = "value"`

### `tts.mimo.model`

选择 tts.mimo 使用的模型。

- 来源：`toml`
- 配置位置：`[tts.mimo]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `model = "value"`

### `tts.minimax.api_key`

认证所选文字转语音 Provider。

- 来源：`toml`
- 配置位置：`[tts.minimax]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `tts.enabled = true and tts.provider = minimax and no MiniMax local config is available`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `tts.minimax.base_url`

覆盖 tts.minimax 的服务基础地址。

- 来源：`toml`
- 配置位置：`[tts.minimax]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `base_url = "value"`

### `tts.minimax.config_file`

覆盖 tts.minimax 使用的辅助配置文件路径。

- 来源：`toml`
- 配置位置：`[tts.minimax]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `config_file = "value"`

### `tts.minimax.model`

选择 tts.minimax 使用的模型。

- 来源：`toml`
- 配置位置：`[tts.minimax]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `model = "value"`

### `tts.openai.api_key`

认证所选文字转语音 Provider。

- 来源：`toml`
- 配置位置：`[tts.openai]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `tts.enabled = true and tts.provider is unset or openai`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `tts.openai.base_url`

覆盖 tts.openai 的服务基础地址。

- 来源：`toml`
- 配置位置：`[tts.openai]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `base_url = "value"`

### `tts.openai.model`

选择 tts.openai 使用的模型。

- 来源：`toml`
- 配置位置：`[tts.openai]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `model = "value"`

### `tts.provider`

选择文字转语音 Provider。

- 来源：`toml`
- 配置位置：`[tts]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`openai`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 必填条件: `tts.enabled = true`
- 允许值: `qwen`, `openai`, `minimax`, `mimo`, `espeak`, `pico`, `edge`
- 示例: `provider = "qwen"`

### `tts.qwen.api_key`

认证所选文字转语音 Provider。

- 来源：`toml`
- 配置位置：`[tts.qwen]`
- 作用域：`global`
- 类型：`string`
- 要求：`条件必填`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 必填条件: `tts.enabled = true and tts.provider = qwen`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `api_key = "${API_KEY}"`

### `tts.qwen.base_url`

覆盖 tts.qwen 的服务基础地址。

- 来源：`toml`
- 配置位置：`[tts.qwen]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `base_url = "value"`

### `tts.qwen.model`

选择 tts.qwen 使用的模型。

- 来源：`toml`
- 配置位置：`[tts.qwen]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `model = "value"`

### `tts.speed`

设置 tts 使用的语速倍率。

- 来源：`toml`
- 配置位置：`[tts]`
- 作用域：`global`
- 类型：`number`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `speed = 1.0`

### `tts.tts_mode`

选择仅语音触发时回复，或为每条符合条件的回复合成语音。

- 来源：`toml`
- 配置位置：`[tts]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`voice_only`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 允许值: `voice_only`, `always`
- 示例: `tts_mode = "voice_only"`

### `tts.voice`

选择 tts 使用的音色。

- 来源：`toml`
- 配置位置：`[tts]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `voice = "value"`

### `tts.voice_id`

设置 tts 使用的 Provider 专属音色 ID。

- 来源：`toml`
- 配置位置：`[tts]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`runtime`
- 生效方式：`restart`
- 示例: `voice_id = "value"`

### `update_notice`

仅当存在唯一明确的私聊平台时，每个稳定版私聊提醒明确列出的 admin_from 用户一次；空值/通配符/歧义目标保持静默，绝不投递最近群聊/话题，用户确认前先查看精确 immutable Plan。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`true`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `update_notice = true`

### `webhook.enabled`

开放可触发 Agent 提示或 Shell 命令的外部 HTTP 端点。

- 来源：`toml`
- 配置位置：`[webhook]`
- 作用域：`global`
- 类型：`boolean`
- 要求：`可选`
- 默认值：`false`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `enabled = false`

### `webhook.path`

设置外部 Webhook URL 路径前缀。

- 来源：`toml`
- 配置位置：`[webhook]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`/hook`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `path = "/hook"`

### `webhook.port`

设置外部 Webhook 监听端口。

- 来源：`toml`
- 配置位置：`[webhook]`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`9111`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 示例: `port = 9111`

### `webhook.token`

使用共享 Token 认证 webhook。

- 来源：`toml`
- 配置位置：`[webhook]`
- 作用域：`global`
- 类型：`string`
- 要求：`可选`
- 默认值：`unset`
- 默认值来源：`unset`
- 生效方式：`restart`
- 敏感信息：是；优先使用环境变量占位符。
- 示例: `token = "${TOKEN}"`

### `workspace_idle_timeout_mins`

多工作区引擎空闲指定分钟后回收；0 表示禁用。

- 来源：`toml`
- 配置位置：`config.toml root`
- 作用域：`global`
- 类型：`integer`
- 要求：`可选`
- 默认值：`15`
- 默认值来源：`builtin`
- 生效方式：`restart`
- 范围: `0` 到 `+∞` `minutes`
- 示例: `workspace_idle_timeout_mins = 15`
