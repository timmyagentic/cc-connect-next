<!-- Code generated from the compiled configuration catalog. DO NOT EDIT. -->

# cc-connect-next Configuration Capabilities

Catalog version: `source`. This reference describes capability and never reads or prints local configuration values.

Apply modes: `live` takes effect in the running process; `reload` can be applied with `/config reload` after saving; `new-session` affects newly started Agent sessions; `restart` requires restarting cc-connect-next.

Configuration sources: `toml` persists in config.toml; `environment` is read from the process environment; `cli` is a startup or installation flag. TOML strings support `${VAR_NAME}` environment placeholders. Dotted paths describe semantic placement. Agent and Platform options go under `[projects.agent.options]` / `[projects.platforms.options]` with the adjacent adapter `type` selected.

Precedence: an explicit CLI flag overrides its corresponding environment variable, an environment override wins over its corresponding TOML option, project fields override global fields per value, and runtime defaults apply last. Preset values describe what a generator/Profile writes explicitly; they never override an existing explicit user value.

## Capability overview

### Access control and roles (`access-control`)

Limit who can talk to the bot, who can run privileged commands, and how roles inherit limits.

Related configuration: `projects.platforms.options.allow_from`, `projects.admin_from`, `projects.users.default_role`, `projects.users.roles.<name>.user_ids`, `projects.users.roles.<name>.disabled_commands`

### Agent execution (`agent-execution`)

Choose the Agent, command, working directory, model, approval mode, prompts, and adapter-specific behavior.

Related configuration: `projects.agent.type`, `projects.agent.options.work_dir`, `projects.agent.options.mode`, `projects.agent.options.model`, `projects.agent.options.reasoning_effort`, `projects.agent.options.service_tier`

### Attachments and media (`attachments-media`)

Control file/image send-back, attachment limits, references, speech recognition, and voice replies.

Related configuration: `attachment_send`, `max_attachment_size_mb`, `speech.enabled`, `speech.provider`, `tts.enabled`, `tts.provider`

### Cron, timers, and heartbeat (`automation`)

Configure recurring defaults, one-shot task behavior, and periodic main-session awareness.

Related configuration: `cron.silent`, `cron.session_mode`, `projects.heartbeat.enabled`, `projects.heartbeat.interval_mins`, `projects.heartbeat.session_key`

### Commands, aliases, and content policy (`customization`)

Add prompt/exec commands, natural-language aliases, banned words, and per-project command restrictions.

Related configuration: `commands.name`, `commands.prompt`, `commands.exec`, `aliases.name`, `aliases.command`, `banned_words`, `projects.disabled_commands`

### Message presentation (`display`)

Control cards, reasoning/tool progress, streaming previews, immediate acknowledgements, history truncation, and reply footers.

Related configuration: `display.mode`, `display.card_mode`, `display.thinking_messages`, `display.tool_messages`, `display.reply_footer`, `stream_preview.enabled`, `instant_reply.enabled`

### Environment and operational overrides (`environment-overrides`)

Override log rotation, attachment limits, daemon secret capture, command context, and adapter state without changing TOML.

Related configuration: `CC_LOG_FILE`, `CC_LOG_MAX_SIZE`, `CC_LOG_MAX_BACKUPS`, `CC_MAX_ATTACHMENT_SIZE_MB`, `CC_DAEMON_NO_CAPTURE_SECRETS`, `CC_PROJECT`, `CC_SESSION_KEY`, `--config`, `--log-max-size`, `--log-max-backups`, `daemon install --config`, `daemon install --work-dir`, `daemon install --log-max-size`, `daemon install --log-file`, `daemon install --no-capture-secrets`

### Webhook, Bridge, and management API (`external-interfaces`)

Expose authenticated endpoints for automation, external adapters, and the Web management console.

Related configuration: `webhook.enabled`, `bridge.enabled`, `management.enabled`, `hooks.event`

### Feedback and updates (`feedback-updates`)

Control anonymous feedback availability and one-time stable-version notifications.

Related configuration: `feedback.enabled`, `feedback.endpoint`, `update_notice`

### Projects and workspaces (`multi-project`)

Run multiple named projects, dynamically bind workspaces, isolate OS users, and reap idle workspaces.

Related configuration: `projects.name`, `projects.mode`, `projects.base_dir`, `projects.workspace_init_allow_local_paths`, `projects.run_as_user`, `workspace_idle_timeout_mins`

### Messaging platforms (`platform-connections`)

Connect and tune Feishu, Telegram, Discord, Slack, DingTalk, WeCom, Weixin, QQ, Matrix, and other compiled adapters.

Related configuration: `projects.platforms.type`, `projects.platforms.options.allow_from`, `projects.platforms.options.proxy`, `projects.platforms.options.group_reply_all`, `projects.platforms.options.share_session_in_channel`

### Group replies and session boundaries (`platform-session-routing`)

Control mention-free group replies and choose whether users, channels, or platform topics share or isolate Agent sessions.

Related configuration: `projects.platforms.options.group_reply_all`, `projects.platforms.options.share_session_in_channel`, `projects.platforms.options.thread_isolation`

### Providers, models, and answer profiles (`providers-models`)

Share provider credentials, route each Agent to endpoints/models, switch providers, and configure one-shot fast/quality answers.

Related configuration: `providers.name`, `providers.base_url`, `providers.model`, `projects.agent.provider_refs`, `projects.agent.options.provider`, `projects.agent.answer_profiles.fast.model`, `projects.agent.answer_profiles.quality.reasoning_effort`

### Inbound and outbound rate limits (`rate-limits`)

Protect sessions and platform APIs with global, role-based, and per-platform limits.

Related configuration: `rate_limit.max_messages`, `rate_limit.window_secs`, `outgoing_rate_limit.max_per_second`, `projects.users.roles.<name>.rate_limit.max_messages`

### Cross-project relay (`relay`)

Bind bots/projects together and control relay timeouts and group visibility.

Related configuration: `relay.timeout_secs`, `relay.visibility`, `projects.platforms.options.peer_bots`

### Runtime, storage, and logs (`runtime-storage`)

Choose language, state directory, shell environment, log level, timeouts, and attachment limits.

Related configuration: `language`, `data_dir`, `log.level`, `shell`, `shell_profile`, `idle_timeout_mins`, `max_turn_time_mins`

### Sessions, queueing, and context (`session-lifecycle`)

Control busy-message steering, queue depth, idle rotation, external-session visibility, and automatic compression.

Related configuration: `queue.max_depth`, `queue.busy_message_mode`, `projects.busy_message_mode`, `projects.reset_on_idle_mins`, `projects.filter_external_sessions`, `projects.auto_compress.enabled`

### Shell, hooks, and event automation (`shell-hooks`)

Select a shell and run command or HTTP hooks on message, session, cron, permission, and error events.

Related configuration: `shell`, `shell_profile`, `hooks.event`, `hooks.type`, `hooks.command`, `hooks.url`

## Option reference

### `--config`

Select the config.toml file for this command or runtime.

- Source: `cli`
- Placement: `command line`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `./config.toml when present, otherwise ~/.cc-connect-next/config.toml`
- Default source: `runtime`
- Takes effect: `live`
- Example: `cc-connect-next --config /path/to/config.toml`

### `--log-max-backups`

Set rotated log backup count and override CC_LOG_MAX_BACKUPS.

- Source: `cli`
- Placement: `command line`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `3`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞`
- Example: `cc-connect-next --log-max-backups 5`

### `--log-max-size`

Set rotating log size and override CC_LOG_MAX_SIZE.

- Source: `cli`
- Placement: `command line`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `10MB`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `cc-connect-next --log-max-size 50MB`

### `CC_DAEMON_NO_CAPTURE_SECRETS`

Prevent daemon installation from capturing supported credential environment variables.

- Source: `environment`
- Placement: `process environment`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `export CC_DAEMON_NO_CAPTURE_SECRETS=true`

### `CC_DATA_DIR`

Override the data directory used by standalone send operations.

- Source: `environment`
- Placement: `process environment`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `inherit data_dir`
- Default source: `inherit`
- Takes effect: `live`
- Example: `export CC_DATA_DIR=/path/to/data`

### `CC_LOG_FILE`

Override the runtime log-file path.

- Source: `environment`
- Placement: `process environment`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `platform daemon log path`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `export CC_LOG_FILE=/path/to/cc-connect-next.log`

### `CC_LOG_MAX_BACKUPS`

Override the number of rotated log backups; an explicit --log-max-backups flag takes precedence.

- Source: `environment`
- Placement: `process environment`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `3`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞`
- Example: `export CC_LOG_MAX_BACKUPS=3`

### `CC_LOG_MAX_SIZE`

Override the rotating log-file size; an explicit --log-max-size flag takes precedence.

- Source: `environment`
- Placement: `process environment`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `10MB`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `export CC_LOG_MAX_SIZE=10MB`

### `CC_MAX_ATTACHMENT_SIZE_MB`

Override max_attachment_size_mb for the /send API.

- Source: `environment`
- Placement: `process environment`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `inherit max_attachment_size_mb`
- Default source: `inherit`
- Takes effect: `restart`
- Range: `1` to `+∞` `MiB`
- Example: `export CC_MAX_ATTACHMENT_SIZE_MB=100`

### `CC_NEXT_ALLOW_OFFICIAL_CONFLICT`

Explicitly allow startup beside a detected official CC Connect runtime conflict.

- Source: `environment`
- Placement: `process environment`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `export CC_NEXT_ALLOW_OFFICIAL_CONFLICT=true`

### `CC_PROJECT`

Provide the default project context for send, relay, cron, timer, and session helper commands.

- Source: `environment`
- Placement: `process environment`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `live`
- Example: `export CC_PROJECT=my-project`

### `CC_SESSION_KEY`

Provide the default session context for send, relay, cron, timer, and session helper commands.

- Source: `environment`
- Placement: `process environment`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `live`
- Example: `export CC_SESSION_KEY=feishu:oc_chat:ou_user`

### `CLAUDE_CONFIG_DIR` — `claudecode`

Override the Claude Code configuration directory.

- Source: `environment`
- Placement: `process environment`
- Scope: `agent` (`claudecode`)
- Type: `string`
- Requirement: `optional`
- Default: `~/.claude`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `export CLAUDE_CONFIG_DIR=/path/to/claude-config`

### `CODEX_HOME` — `codex`

Choose the Codex home used when projects.agent.options.codex_home is unset.

- Source: `environment`
- Placement: `process environment`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `~/.codex`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `export CODEX_HOME=/path/to/codex-home`

### `MATRIX_CROSS_SIGNING_PASSWORD` — `matrix`

Provide the Matrix cross-signing password without storing it in TOML.

- Source: `environment`
- Placement: `process environment`
- Scope: `platform` (`matrix`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `export MATRIX_CROSS_SIGNING_PASSWORD='${MATRIX_PASSWORD}'`

### `PI_CODING_AGENT_DIR` — `pi`

Override the pi coding-agent state directory.

- Source: `environment`
- Placement: `process environment`
- Scope: `agent` (`pi`)
- Type: `string`
- Requirement: `optional`
- Default: `upstream pi default`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `export PI_CODING_AGENT_DIR=/path/to/pi-agent`

### `aliases.command`

Choose the slash command expanded by this alias.

- Source: `toml`
- Placement: `[[aliases]]`
- Scope: `global`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `command = "value"`

### `aliases.name`

Set the natural-language trigger for a command alias.

- Source: `toml`
- Placement: `[[aliases]]`
- Scope: `global`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `name = "value"`

### `attachment_send`

Allow or block Agent-initiated image and file send-back without disabling text replies.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `on`
- Default source: `builtin`
- Takes effect: `reload`
- Allowed values: `on`, `off`
- Example: `attachment_send = "on"`

### `banned_words`

Block messages containing any configured banned word.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `string[]`
- Requirement: `optional`
- Default: `[]`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `banned_words = ["value"]`

### `bridge.cors_origins`

Allow browser requests to bridge from the listed CORS origins.

- Source: `toml`
- Placement: `[bridge]`
- Scope: `global`
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `cors_origins = ["value"]`

### `bridge.enabled`

Enable the WebSocket/REST bridge for external platform adapters.

- Source: `toml`
- Placement: `[bridge]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enabled = false`

### `bridge.insecure`

Allow a tokenless bridge for local development only.

- Source: `toml`
- Placement: `[bridge]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `insecure = false`

### `bridge.path`

Set the external adapter bridge WebSocket path.

- Source: `toml`
- Placement: `[bridge]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `/bridge/ws`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `path = "/bridge/ws"`

### `bridge.port`

Set the external adapter bridge port.

- Source: `toml`
- Placement: `[bridge]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `9810`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `port = 9810`

### `bridge.token`

Authenticate bridge clients with a shared token.

- Source: `toml`
- Placement: `[bridge]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `bridge.enabled = true and bridge.insecure != true`
- Requires: `bridge.enabled`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `token = "${TOKEN}"`

### `commands.description`

Describe the custom command in menus and help.

- Source: `toml`
- Placement: `[[commands]]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `description = "value"`

### `commands.exec`

Execute a shell command instead of prompting the Agent.

- Source: `toml`
- Placement: `[[commands]]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Required when: `commands.prompt is unset`
- Conflicts with: `commands.prompt`
- Example: `exec = "value"`

### `commands.name`

Set the custom slash-command name.

- Source: `toml`
- Placement: `[[commands]]`
- Scope: `global`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `name = "value"`

### `commands.prompt`

Expand the custom command into an Agent prompt.

- Source: `toml`
- Placement: `[[commands]]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Required when: `commands.exec is unset`
- Conflicts with: `commands.exec`
- Example: `prompt = "value"`

### `commands.work_dir`

Override the working directory for a custom exec command.

- Source: `toml`
- Placement: `[[commands]]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `work_dir = "value"`

### `cron.session_mode`

Choose whether scheduled runs reuse a session or create a fresh session per run.

- Source: `toml`
- Placement: `[cron]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `reuse`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `reuse`, `new_per_run`
- Example: `session_mode = "reuse"`

### `cron.silent`

Suppress the notification sent when a scheduled run starts.

- Source: `toml`
- Placement: `[cron]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `silent = false`

### `daemon install --config`

Choose the config.toml embedded in the daemon installation.

- Source: `cli`
- Placement: `command line`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `<work-dir>/config.toml`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `cc-connect-next daemon install --config /path/to/config.toml`

### `daemon install --log-file`

Choose the daemon log-file path at installation time.

- Source: `cli`
- Placement: `command line`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `~/.cc-connect-next/logs/cc-connect-next.log`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `cc-connect-next daemon install --log-file /path/to/cc-connect-next.log`

### `daemon install --log-max-size`

Set the installed daemon log rotation size in MiB.

- Source: `cli`
- Placement: `command line`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `10`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞` `MiB`
- Example: `cc-connect-next daemon install --log-max-size 50`

### `daemon install --no-capture-secrets`

Install the daemon without capturing supported credential environment variables.

- Source: `cli`
- Placement: `command line`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `cc-connect-next daemon install --no-capture-secrets`

### `daemon install --work-dir`

Choose the daemon runtime working directory used for relative paths.

- Source: `cli`
- Placement: `command line`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `config parent or current directory`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `cc-connect-next daemon install --work-dir /path/to/runtime`

### `data_dir`

Choose where cc-connect-next stores sessions, state, media, and runtime metadata.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `~/.cc-connect-next`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `data_dir = "~/.cc-connect-next"`

### `display.card_mode`

Choose rich Card 2.0 or legacy card rendering where supported.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `rich`
- Default source: `builtin`
- Takes effect: `reload`
- Allowed values: `rich`, `legacy`
- Example: `card_mode = "rich"`

### `display.hide_agent_footer`

Strip equivalent model/token/context footer lines emitted by the Agent itself.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `hide_agent_footer = false`

### `display.history_max_len`

Limit each /history entry; zero disables truncation.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `1000`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `0` to `+∞` `characters`
- Example: `history_max_len = 1000`

### `display.mode`

Choose full, compact, or quiet reply presentation. Omission resolves to full layout without enabling thinking/tool messages; explicitly writing full enables their mode defaults.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `full`
- Default source: `builtin`
- Takes effect: `reload`
- Allowed values: `full`, `compact`, `quiet`
- Example: `mode = "full"`

### `display.reply_footer`

Show the model, reasoning effort, and elapsed-time footer on completed replies.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `reply_footer = true`

### `display.show_context_indicator`

Deprecated no-op retained only for old config compatibility.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Status: deprecated compatibility option.
- Example: `show_context_indicator = false`

### `display.thinking_max_len`

Limit reasoning-progress text length; zero disables truncation.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `300`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `0` to `+∞` `characters`
- Example: `thinking_max_len = 300`

### `display.thinking_messages`

Show or hide Agent reasoning progress messages.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `thinking_messages = false`

### `display.tool_max_len`

Limit tool-progress text length; zero disables truncation.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `500`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `0` to `+∞` `characters`
- Example: `tool_max_len = 500`

### `display.tool_messages`

Show or hide Agent tool-progress messages.

- Source: `toml`
- Placement: `[display]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `tool_messages = false`

### `feedback.enabled`

Enable /feedback and capability-gap prompts; every submission still requires confirmation.

- Source: `toml`
- Placement: `[feedback]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enabled = true`

### `feedback.endpoint`

Override the author-operated anonymous Feedback v1 relay; requires exact /v1/feedback over HTTPS (loopback HTTP is development-only).

- Source: `toml`
- Placement: `[feedback]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `built-in author relay`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `endpoint = "value"`

### `hooks.async`

Run the hook asynchronously instead of blocking message handling.

- Source: `toml`
- Placement: `[[hooks]]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `async = true`

### `hooks.command`

Set the shell command executed by a command hook.

- Source: `toml`
- Placement: `[[hooks]]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Required when: `hooks.type = command`
- Conflicts with: `hooks.url`
- Example: `command = "value"`

### `hooks.event`

Choose the event that triggers this hook.

- Source: `toml`
- Placement: `[[hooks]]`
- Scope: `global`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Example: `event = "value"`

### `hooks.timeout`

Set the execution timeout in seconds for hooks.

- Source: `toml`
- Placement: `[[hooks]]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `timeout = 1`

### `hooks.type`

Choose command or HTTP hook execution.

- Source: `toml`
- Placement: `[[hooks]]`
- Scope: `global`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Allowed values: `command`, `http`
- Example: `type = "command"`

### `hooks.url`

Set the URL called by an HTTP hook.

- Source: `toml`
- Placement: `[[hooks]]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Required when: `hooks.type = http`
- Conflicts with: `hooks.command`
- Example: `url = "value"`

### `idle_timeout_mins`

Stop a turn when the Agent produces no events for this many minutes; zero disables it.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `120`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `minutes`
- Example: `idle_timeout_mins = 120`

### `instant_reply.content`

Override the localized immediate acknowledgement text.

- Source: `toml`
- Placement: `[instant_reply]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `content = "value"`

### `instant_reply.enabled`

Immediately acknowledge an incoming message before Agent work begins.

- Source: `toml`
- Placement: `[instant_reply]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `enabled = false`

### `language`

Choose the canonical bot-message language, or detect it from the user's first message; common regional aliases are normalized.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `zh`
- Default source: `builtin`
- Takes effect: `restart`
- Canonical values (documented aliases also accepted): `zh`, `en`, `zh-TW`, `ja`, `es`, `auto`
- Example: `language = "zh"`

### `log.level`

Set the minimum runtime log severity.

- Source: `toml`
- Placement: `[log]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `info`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `debug`, `info`, `warn`, `error`
- Example: `level = "debug"`

### `management.cors_origins`

Allow browser requests to management from the listed CORS origins.

- Source: `toml`
- Placement: `[management]`
- Scope: `global`
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `cors_origins = ["value"]`

### `management.enabled`

Enable the local management API and Web console backend.

- Source: `toml`
- Placement: `[management]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enabled = false`

### `management.port`

Set the management API listening port.

- Source: `toml`
- Placement: `[management]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `9820`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `port = 9820`

### `management.token`

Authenticate management API and Web console requests with a shared token.

- Source: `toml`
- Placement: `[management]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `management.enabled = true`
- Requires: `management.enabled`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `token = "${TOKEN}"`

### `max_attachment_size_mb`

Set the maximum size of one outbound attachment in MiB.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `50`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `0` to `+∞` `MiB`
- Example: `max_attachment_size_mb = 50`

### `max_turn_time_mins`

Cap the absolute wall-clock duration of one Agent turn; zero disables it.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `0`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `minutes`
- Example: `max_turn_time_mins = 0`

### `outgoing_rate_limit.burst`

Set the maximum immediate outbound burst.

- Source: `toml`
- Placement: `[outgoing_rate_limit]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `ceil(max_per_second)`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞`
- Example: `burst = 1`

### `outgoing_rate_limit.max_per_second`

Limit outgoing messages per second; zero means unlimited.

- Source: `toml`
- Placement: `[outgoing_rate_limit]`
- Scope: `global`
- Type: `number`
- Requirement: `optional`
- Default: `0`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `messages/second`
- Example: `max_per_second = 1.0`

### `outgoing_rate_limit.platforms.<name>.burst`

Override the outgoing burst for one platform; unset inherits the global value.

- Source: `toml`
- Placement: `[outgoing_rate_limit.platforms.<name>]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Range: `0` to `+∞`
- Example: `burst = 1`

### `outgoing_rate_limit.platforms.<name>.max_per_second`

Override outgoing messages per second for one platform; unset inherits the global value.

- Source: `toml`
- Placement: `[outgoing_rate_limit.platforms.<name>]`
- Scope: `global`
- Type: `number`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Range: `0` to `+∞` `messages/second`
- Example: `max_per_second = 1.0`

### `projects.admin_from`

Restrict privileged commands to selected platform user IDs; unset blocks privileged commands for everyone.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `reload`
- Example: `admin_from = "value"`

### `projects.agent.answer_profiles.fast.model`

Override the model for one-shot /fast answers.

- Source: `toml`
- Placement: `[projects.agent.answer_profiles.fast] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Example: `model = "value"`

### `projects.agent.answer_profiles.fast.reasoning_effort`

Override reasoning effort for one-shot /fast answers.

- Source: `toml`
- Placement: `[projects.agent.answer_profiles.fast] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Allowed values: `low`, `medium`, `high`, `xhigh`, `max`
- Example: `reasoning_effort = "low"`

### `projects.agent.answer_profiles.fast.service_tier`

Override the model-catalog service tier for one-shot /fast answers.

- Source: `toml`
- Placement: `[projects.agent.answer_profiles.fast] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Allowed values: `model-catalog-driven (for example: default, fast)`
- Example: `service_tier = "value"`

### `projects.agent.answer_profiles.quality.model`

Override the model for one-shot /quality answers.

- Source: `toml`
- Placement: `[projects.agent.answer_profiles.quality] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Example: `model = "value"`

### `projects.agent.answer_profiles.quality.reasoning_effort`

Override reasoning effort for one-shot /quality answers.

- Source: `toml`
- Placement: `[projects.agent.answer_profiles.quality] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Allowed values: `low`, `medium`, `high`, `xhigh`, `max`
- Example: `reasoning_effort = "low"`

### `projects.agent.answer_profiles.quality.service_tier`

Override the model-catalog service tier for one-shot /quality answers.

- Source: `toml`
- Placement: `[projects.agent.answer_profiles.quality] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Allowed values: `model-catalog-driven (for example: default, fast)`
- Example: `service_tier = "value"`

### `projects.agent.options.agent` — `opencode`

Select the named sub-agent or profile exposed by the CLI.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`opencode`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `agent = "value"`

### `projects.agent.options.allowed_tools` — `claudecode`

Pre-approve selected Claude Code tools in approval-based modes.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `allowed_tools = ["value"]`

### `projects.agent.options.app_server_url` — `codex`

Choose the Codex app-server transport endpoint.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `stdio`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `app_server_url = "stdio"`
- Preset `starter`: `stdio` — Launch a local app-server subprocess.

### `projects.agent.options.append_system_prompt` — `claudecode, codex`

Append project instructions while preserving the Agent's default system prompt.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode, codex`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `append_system_prompt = "value"`

### `projects.agent.options.args` — `acp`

Pass additional arguments to the configured Agent command.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp`)
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `args = ["value"]`

### `projects.agent.options.args` — `devin`

Pass additional arguments to the configured Agent command.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`devin`)
- Type: `string[]`
- Requirement: `optional`
- Default: `["acp"]`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `args = ["acp"]`

### `projects.agent.options.auth_method` — `acp`

Select the authentication method used by an ACP Agent.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `auth_method = "value"`

### `projects.agent.options.auto_create` — `tmux`

Create the configured tmux session when it does not exist.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `auto_create = true`

### `projects.agent.options.backend` — `codex`

Select the Codex execution backend; app_server supports native steering and approvals.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `app_server`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `app_server`, `exec`
- Example: `backend = "app_server"`
- Preset `starter`: `app_server` — Native steering and approval protocol.

### `projects.agent.options.cli_args_flag` — `claudecode`

Name the wrapper flag that accepts Agent CLI arguments.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `cli_args_flag = "value"`

### `projects.agent.options.cli_path` — `acp`

Override the Agent CLI executable path.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Required when: `one of cmd, cli_path, or command must be set`
- Example: `cli_path = "value"`

### `projects.agent.options.cli_path` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Override the Agent CLI executable path.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `cli_path = "value"`

### `projects.agent.options.cmd` — `acp`

Override the Agent command, optionally including global arguments.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Required when: `one of cmd, cli_path, or command must be set`
- Example: `cmd = "value"`

### `projects.agent.options.cmd` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Override the Agent command, optionally including global arguments.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `cmd = "value"`

### `projects.agent.options.cmd_args_flag` — `claudecode`

Name the wrapper flag used to forward command arguments.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `cmd_args_flag = "value"`

### `projects.agent.options.codex_home` — `codex`

Override CODEX_HOME for this project without changing the user's global Codex home.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `codex_home = "value"`

### `projects.agent.options.command` — `acp`

Set the Agent executable; an alias used by several adapters.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Required when: `one of cmd, cli_path, or command must be set`
- Example: `command = "value"`

### `projects.agent.options.command` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Set the Agent executable; an alias used by several adapters.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `command = "value"`

### `projects.agent.options.command` — `devin`

Set the Agent executable; an alias used by several adapters.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`devin`)
- Type: `string`
- Requirement: `optional`
- Default: `devin`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `command = "devin"`

### `projects.agent.options.disallowed_tools` — `claudecode`

Deny selected Claude Code tools even when the mode would otherwise allow them.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `disallowed_tools = ["value"]`

### `projects.agent.options.display_name` — `acp`

Set the user-facing name of a generic or ACP Agent.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp`)
- Type: `string`
- Requirement: `optional`
- Default: `ACP`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `display_name = "ACP"`

### `projects.agent.options.display_name` — `devin`

Set the user-facing name of a generic or ACP Agent.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`devin`)
- Type: `string`
- Requirement: `optional`
- Default: `Devin`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `display_name = "Devin"`

### `projects.agent.options.env` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Inject project-scoped environment variables into Agent processes.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `env = { example = "value" }`

### `projects.agent.options.init_command` — `tmux`

Run a shell initialization command before tmux prompts are sent.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `init_command = "value"`

### `projects.agent.options.max_context_tokens` — `claudecode`

Override the maximum context-token budget accepted by Claude Code.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `integer`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Range: `1` to `+∞`
- Example: `max_context_tokens = 1`

### `projects.agent.options.mode` — `acp`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `mode = "value"`

### `projects.agent.options.mode` — `antigravity`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`antigravity`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `default`, `yolo`, `plan`
- Example: `mode = "default"`

### `projects.agent.options.mode` — `claudecode`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `default`, `acceptEdits`, `plan`, `auto`, `bypassPermissions`, `dontAsk`
- Example: `mode = "default"`

### `projects.agent.options.mode` — `codex`

Choose the Codex approval and sandbox mode. Omitting the key keeps the suggest compatibility fallback; fresh generated configs explicitly set yolo.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `suggest`
- Default source: `adapter`
- Takes effect: `restart`
- Allowed values: `suggest`, `auto-edit`, `full-auto`, `yolo`
- Example: `mode = "suggest"`
- Preset `starter`: `yolo` — Fresh generated configuration.

### `projects.agent.options.mode` — `copilot`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`copilot`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `default`, `bypassPermissions`
- Example: `mode = "default"`

### `projects.agent.options.mode` — `cursor`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`cursor`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `default`, `force`, `plan`, `ask`
- Example: `mode = "default"`

### `projects.agent.options.mode` — `gemini`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`gemini`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `default`, `auto_edit`, `yolo`, `plan`
- Example: `mode = "default"`

### `projects.agent.options.mode` — `iflow`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`iflow`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `default`, `auto-edit`, `plan`, `yolo`
- Example: `mode = "default"`

### `projects.agent.options.mode` — `kimi`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`kimi`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `default`, `yolo`, `plan`, `quiet`
- Example: `mode = "default"`

### `projects.agent.options.mode` — `opencode, pi, qoder`

Choose the Agent approval, sandbox, or planning mode.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`opencode, pi, qoder`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `default`, `yolo`
- Example: `mode = "default"`

### `projects.agent.options.model` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Select the default model for new Agent sessions.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `model = "provider/model-name"`

### `projects.agent.options.model_context_window` — `codex`

Declare the Codex model context window used for usage reporting and compaction decisions.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `integer`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Range: `1` to `+∞`
- Example: `model_context_window = 1`

### `projects.agent.options.pane` — `tmux`

Select the tmux pane used for Agent input and output.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `string`
- Requirement: `optional`
- Default: `0`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `pane = "0"`

### `projects.agent.options.plugin_dir` — `claudecode`

Load one or more Claude Code plugin directories.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string | string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `plugin_dir = "value"`

### `projects.agent.options.poll_interval_ms` — `tmux`

Set the tmux output polling interval in milliseconds.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `integer`
- Requirement: `optional`
- Default: `200`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞` `milliseconds`
- Example: `poll_interval_ms = 200`

### `projects.agent.options.prompt_pattern` — `tmux`

Regular expression used to recognize the tmux Agent prompt.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `string`
- Requirement: `optional`
- Default: `[❯\$#>%]\s*$`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `prompt_pattern = "[❯\\$#>%]\\s*$"`

### `projects.agent.options.provider` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode`

Select the active configured provider for this project.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `reload`
- Example: `provider = "provider-name"`

### `projects.agent.options.reasoning_effort` — `claudecode`

Set the default reasoning-effort level for new turns.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Allowed values: `low`, `medium`, `high`, `max`
- Example: `reasoning_effort = "low"`

### `projects.agent.options.reasoning_effort` — `codex`

Set the default reasoning-effort level for new turns.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Allowed values: `low`, `medium`, `high`, `xhigh`, `max`
- Example: `reasoning_effort = "low"`

### `projects.agent.options.router_api_key` — `claudecode`

Authenticate to the configured Claude Code Router.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `router_api_key = "${ROUTER_API_KEY}"`

### `projects.agent.options.router_url` — `claudecode`

Route Claude Code requests through the specified router URL.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `router_url = "value"`

### `projects.agent.options.run_as_env` — `claudecode`

Extend the environment allowlist passed across OS-user isolation.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `run_as_env = ["value"]`

### `projects.agent.options.run_as_user` — `claudecode`

Run Claude Code under another non-root OS user.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `run_as_user = "value"`

### `projects.agent.options.service_tier` — `codex`

Select the model-catalog service tier; common Codex values include default and fast.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Allowed values: `model-catalog-driven (for example: default, fast)`
- Example: `service_tier = "value"`

### `projects.agent.options.session` — `tmux`

Name the tmux session that hosts the Agent.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `string`
- Requirement: `required`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `session = "agent-session"`

### `projects.agent.options.session_title_model` — `codex`

Optionally use an isolated local Codex model to generate concise Codex App titles.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `session_title_model = "value"`

### `projects.agent.options.session_title_prefix` — `codex`

Prefix Codex App session titles with a configurable source label.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`codex`)
- Type: `string`
- Requirement: `optional`
- Default: `[飞书]`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `session_title_prefix = "[飞书]"`

### `projects.agent.options.shell` — `tmux`

Select the shell used by the tmux adapter.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `shell = "value"`

### `projects.agent.options.startup_wait_ms` — `tmux`

Wait this many milliseconds after creating a tmux session.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `integer`
- Requirement: `optional`
- Default: `0 (or 2000 when init_command is set)`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `milliseconds`
- Example: `startup_wait_ms = 1`

### `projects.agent.options.strip_input_block` — `tmux`

Remove the echoed input block from captured tmux output.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `strip_input_block = true`

### `projects.agent.options.strip_patterns` — `tmux`

Remove output lines matching the listed patterns.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `string[]`
- Requirement: `optional`
- Default: `built-in Claude mode-status pattern`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `strip_patterns = ["^status:"]`

### `projects.agent.options.system_prompt` — `claudecode, codex`

Replace the Agent's default system prompt for this project.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`claudecode, codex`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `system_prompt = "value"`

### `projects.agent.options.thinking` — `pi`

Configure the pi Agent's thinking mode or level.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`pi`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `thinking = "value"`

### `projects.agent.options.timeout_mins` — `antigravity, gemini, kimi`

Set the adapter process timeout in minutes; zero uses its default.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`antigravity, gemini, kimi`)
- Type: `integer`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Range: `0` to `+∞` `minutes`
- Example: `timeout_mins = 1`

### `projects.agent.options.tool_timeout_secs` — `iflow`

Set the maximum wait for an iFlow tool call in seconds.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`iflow`)
- Type: `integer`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Range: `0` to `+∞` `seconds`
- Example: `tool_timeout_secs = 1`

### `projects.agent.options.window_per_session` — `tmux`

Use a separate tmux window for every cc-connect-next session.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`tmux`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `window_per_session = false`

### `projects.agent.options.work_dir` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder, tmux`

Set the project working directory used by the Agent.

- Source: `toml`
- Placement: `[projects.agent.options] (inside one [[projects]])`
- Scope: `agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder, tmux`)
- Type: `string`
- Requirement: `optional`
- Default: `.`
- Default source: `builtin`
- Takes effect: `restart`
- Conflicts with: `projects.mode = multi-workspace`
- Example: `work_dir = "/absolute/path/to/project"`

### `projects.agent.provider_refs`

Reference shared provider names from projects agent.

- Source: `toml`
- Placement: `[projects.agent] (inside one [[projects]])`
- Scope: `agent`
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `provider_refs = ["value"]`

### `projects.agent.providers.agent_model_lists.<name>.alias`

Set a short user-facing alias for projects agent providers agent_model_lists <name>.

- Source: `toml`
- Placement: `[[projects.agent.providers.agent_model_lists.<name>]] (inside one [[projects.agent.providers]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `alias = "value"`

### `projects.agent.providers.agent_model_lists.<name>.model`

Name a model exposed by this project-local provider for one Agent type.

- Source: `toml`
- Placement: `[[projects.agent.providers.agent_model_lists.<name>]] (inside one [[projects.agent.providers]])`
- Scope: `agent`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `model = "value"`

### `projects.agent.providers.agent_models.<name>`

Set one named entry in projects agent providers agent_models.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `example = { example = "value" }`

### `projects.agent.providers.agent_types`

Restrict projects agent providers to the listed Agent adapter types.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `agent_types = ["value"]`

### `projects.agent.providers.api_key`

Authenticate requests made by projects agent providers.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `projects.agent.providers.base_url`

Override the service base URL for projects agent providers.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `base_url = "value"`

### `projects.agent.providers.codex.env_key`

Name the environment variable from which projects agent providers codex reads its credential.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `env_key = "value"`

### `projects.agent.providers.codex.http_headers.<name>`

Set one named entry in projects agent providers codex http_headers.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `example = { example = "value" }`

### `projects.agent.providers.codex.wire_api`

Select the wire protocol used by projects agent providers codex.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `wire_api = "value"`

### `projects.agent.providers.endpoints.<name>`

Set one named entry in projects agent providers endpoints.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `example = { example = "value" }`

### `projects.agent.providers.env.<name>`

Set one named entry in projects agent providers env.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `example = { example = "value" }`

### `projects.agent.providers.model`

Select the model used by projects agent providers.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `model = "value"`

### `projects.agent.providers.models.alias`

Set a short user-facing alias for projects agent providers models.

- Source: `toml`
- Placement: `[[projects.agent.providers.models]] (inside one [[projects.agent.providers]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `alias = "value"`

### `projects.agent.providers.models.model`

Name a model exposed by this project-local provider.

- Source: `toml`
- Placement: `[[projects.agent.providers.models]] (inside one [[projects.agent.providers]])`
- Scope: `agent`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `model = "value"`

### `projects.agent.providers.name`

Name a project-local model provider for switching.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `name = "value"`

### `projects.agent.providers.thinking`

Choose the provider thinking mode used by projects agent providers.

- Source: `toml`
- Placement: `[[projects.agent.providers]] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `thinking = "value"`

### `projects.agent.type`

Select the Agent adapter used by this project.

- Source: `toml`
- Placement: `[projects.agent] (inside one [[projects]])`
- Scope: `agent`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Example: `type = "value"`

### `projects.auto_compress.enabled`

Automatically run context compression near the configured token threshold.

- Source: `toml`
- Placement: `[projects.auto_compress] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `enabled = false`

### `projects.auto_compress.max_tokens`

Set the estimated token threshold that triggers auto-compression.

- Source: `toml`
- Placement: `[projects.auto_compress] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `12000`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `0` to `+∞` `tokens`
- Example: `max_tokens = 12000`

### `projects.auto_compress.min_gap_mins`

Set the minimum gap between automatic compression runs.

- Source: `toml`
- Placement: `[projects.auto_compress] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `30`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `0` to `+∞` `minutes`
- Example: `min_gap_mins = 30`

### `projects.base_dir`

Set the parent directory for dynamically created multi-workspaces.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Required when: `projects.mode = multi-workspace`
- Conflicts with: `projects.agent.options.work_dir when projects.mode = multi-workspace`
- Example: `base_dir = "value"`

### `projects.busy_message_mode`

Override the process-wide busy-message policy for one project.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Allowed values: `steer`, `queue`
- Example: `busy_message_mode = "steer"`

### `projects.disabled_commands`

Disable selected built-in commands for this project.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string[]`
- Requirement: `optional`
- Default: `[]`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `disabled_commands = ["value"]`

### `projects.display.card_mode`

Override rich or legacy card rendering for one project.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Allowed values: `rich`, `legacy`
- Example: `card_mode = "rich"`
- Preset `starter/recommended-feishu`: `rich` — Use Card 2.0 answers.

### `projects.display.hide_agent_footer`

Override Agent-emitted footer filtering for one project.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Example: `hide_agent_footer = true`
- Preset `starter/recommended-feishu`: `true` — Remove duplicate Agent footer lines.

### `projects.display.history_max_len`

Limit each /history entry; zero disables truncation.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Range: `0` to `+∞` `characters`
- Example: `history_max_len = 1`

### `projects.display.mode`

Choose full, compact, or quiet reply presentation. Omission resolves to full layout without enabling thinking/tool messages; explicitly writing full enables their mode defaults.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Allowed values: `full`, `compact`, `quiet`
- Example: `mode = "full"`

### `projects.display.reply_footer`

Override the reply footer for one project.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Example: `reply_footer = true`
- Preset `starter/recommended-feishu`: `true` — Show model, effort, and elapsed time.

### `projects.display.show_context_indicator`

Deprecated no-op retained only for old config compatibility.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Status: deprecated compatibility option.
- Example: `show_context_indicator = true`

### `projects.display.thinking_max_len`

Limit reasoning-progress text length; zero disables truncation.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Range: `0` to `+∞` `characters`
- Example: `thinking_max_len = 1`

### `projects.display.thinking_messages`

Override reasoning-progress visibility for one project.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Example: `thinking_messages = true`
- Preset `starter/recommended-feishu`: `false` — Keep reasoning out of chat.

### `projects.display.tool_max_len`

Limit tool-progress text length; zero disables truncation.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Range: `0` to `+∞` `characters`
- Example: `tool_max_len = 1`

### `projects.display.tool_messages`

Override tool-progress visibility for one project.

- Source: `toml`
- Placement: `[projects.display] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Example: `tool_messages = true`
- Preset `starter/recommended-feishu`: `false` — Keep tool details out of chat.

### `projects.filter_external_sessions`

Hide Agent sessions that were not created by cc-connect-next.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `filter_external_sessions = false`

### `projects.heartbeat.enabled`

Wake the main session periodically for awareness or unfinished work.

- Source: `toml`
- Placement: `[projects.heartbeat] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enabled = false`

### `projects.heartbeat.interval_mins`

Set the interval between heartbeat turns.

- Source: `toml`
- Placement: `[projects.heartbeat] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `30`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞` `minutes`
- Example: `interval_mins = 30`

### `projects.heartbeat.only_when_idle`

Run heartbeat only while the target session is idle.

- Source: `toml`
- Placement: `[projects.heartbeat] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `only_when_idle = true`

### `projects.heartbeat.prompt`

Set the heartbeat prompt; empty reads HEARTBEAT.md from the Agent work directory.

- Source: `toml`
- Placement: `[projects.heartbeat] (inside one [[projects]])`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `HEARTBEAT.md`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `prompt = "HEARTBEAT.md"`

### `projects.heartbeat.session_key`

Choose the chat/session that receives heartbeat work.

- Source: `toml`
- Placement: `[projects.heartbeat] (inside one [[projects]])`
- Scope: `project`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Required when: `projects.heartbeat.enabled = true`
- Requires: `projects.heartbeat.enabled`
- Example: `session_key = "value"`

### `projects.heartbeat.silent`

Suppress the heartbeat start notification.

- Source: `toml`
- Placement: `[projects.heartbeat] (inside one [[projects]])`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `silent = true`

### `projects.heartbeat.timeout_mins`

Limit one heartbeat turn in minutes.

- Source: `toml`
- Placement: `[projects.heartbeat] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `30`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞` `minutes`
- Example: `timeout_mins = 30`

### `projects.inject_sender`

Prepend platform sender identity to prompts delivered to the Agent.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `inject_sender = false`

### `projects.mode`

Enable fixed-workspace or multi-workspace project routing.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `fixed`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `fixed`, `multi-workspace`
- Example: `mode = "fixed"`

### `projects.name`

Give the project a unique name used by commands, storage, and relay routing.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Example: `name = "value"`

### `projects.observe.channel`

Choose the destination channel used by projects observe.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `channel = "value"`

### `projects.observe.enabled`

Enable or disable projects observe.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `enabled = true`

### `projects.platforms.options.access_token` — `matrix`

Authenticate the Matrix bot account.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`matrix`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `access_token = "${ACCESS_TOKEN}"`

### `projects.platforms.options.account_id` — `weixin`

Separate persistent Weixin state for multiple accounts.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weixin`)
- Type: `string`
- Requirement: `optional`
- Default: `default`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `account_id = "default"`

### `projects.platforms.options.agent_id` — `dingtalk`

Identify the bot application Agent in the platform tenant.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `integer`
- Requirement: `conditional`
- Default: `0`
- Default source: `builtin`
- Takes effect: `restart`
- Required when: `proactive work notifications are used`
- Example: `agent_id = 123456`

### `projects.platforms.options.agent_id` — `wecom`

Identify the bot application Agent in the platform tenant.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Required when: `mode is unset or mode = callback`
- Example: `agent_id = "value"`

### `projects.platforms.options.allow_chat` — `feishu, lark`

Restrict access to a comma-separated list of Feishu/Lark chat IDs; empty or '*' allows every chat.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `optional`
- Default: `empty`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `allow_chat = "oc_chat_id"`

### `projects.platforms.options.allow_from` — `dingtalk, discord, feishu, lark, line, matrix, max, qq, qqbot, slack, telegram, webex, wecom, weibo, weixin, wps-xiezuo`

Restrict bot access to selected platform user IDs; empty or '*' allows every platform user.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk, discord, feishu, lark, line, matrix, max, qq, qqbot, slack, telegram, webex, wecom, weibo, weixin, wps-xiezuo`)
- Type: `string`
- Requirement: `optional`
- Default: `empty`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `allow_from = "user-id-1,user-id-2"`

### `projects.platforms.options.api_base` — `max`

Override the platform REST API base URL.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`max`)
- Type: `string`
- Requirement: `optional`
- Default: `https://platform-api.max.ru`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `api_base = "https://platform-api.max.ru"`

### `projects.platforms.options.api_base_url` — `wecom`

Override the platform API base URL.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `optional`
- Default: `https://qyapi.weixin.qq.com`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `api_base_url = "https://qyapi.weixin.qq.com"`

### `projects.platforms.options.app_id` — `feishu, lark`

Identify the Feishu/Lark bot application; this option is required.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Example: `app_id = "value"`

### `projects.platforms.options.app_id` — `qqbot, weibo, wps-xiezuo`

Identify the bot application.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`qqbot, weibo, wps-xiezuo`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Example: `app_id = "value"`

### `projects.platforms.options.app_secret` — `feishu, lark`

Authenticate the Feishu/Lark bot application; this sensitive option is required.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `app_secret = "${APP_SECRET}"`

### `projects.platforms.options.app_secret` — `qqbot, weibo, wps-xiezuo`

Authenticate the bot application.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`qqbot, weibo, wps-xiezuo`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `app_secret = "${APP_SECRET}"`

### `projects.platforms.options.app_token` — `slack`

Authenticate Slack Socket Mode.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`slack`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `app_token = "${APP_TOKEN}"`

### `projects.platforms.options.auto_join` — `matrix`

Automatically join invited Matrix rooms.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`matrix`)
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `auto_join = true`

### `projects.platforms.options.auto_verify` — `matrix`

Automatically accept Matrix SAS device verification.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`matrix`)
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `auto_verify = true`

### `projects.platforms.options.base_url` — `weixin`

Override the platform service base URL.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weixin`)
- Type: `string`
- Requirement: `optional`
- Default: `https://ilinkai.weixin.qq.com`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `base_url = "https://ilinkai.weixin.qq.com"`

### `projects.platforms.options.base_url` — `wps-xiezuo`

Override the platform service base URL.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wps-xiezuo`)
- Type: `string`
- Requirement: `optional`
- Default: `https://openapi.wps.cn`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `base_url = "https://openapi.wps.cn"`

### `projects.platforms.options.bot_id` — `wecom`

Identify a WeCom WebSocket bot.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Required when: `mode = websocket`
- Example: `bot_id = "value"`

### `projects.platforms.options.bot_secret` — `wecom`

Authenticate a WeCom WebSocket bot.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `mode = websocket`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `bot_secret = "${BOT_SECRET}"`

### `projects.platforms.options.bot_token` — `slack`

Authenticate the Slack bot user.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`slack`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `bot_token = "${BOT_TOKEN}"`

### `projects.platforms.options.burst_limit` — `weixin`

Limit separate Weixin outbound messages within one burst window.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weixin`)
- Type: `integer`
- Requirement: `optional`
- Default: `4`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `burst_limit = 4`

### `projects.platforms.options.burst_window_secs` — `weixin`

Set the Weixin outbound burst window length in seconds.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weixin`)
- Type: `integer`
- Requirement: `optional`
- Default: `86400`
- Default source: `builtin`
- Takes effect: `restart`
- Unit: `seconds`
- Example: `burst_window_secs = 86400`

### `projects.platforms.options.callback_aes_key` — `wecom`

Decrypt encrypted WeCom callback payloads.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `mode is unset or mode = callback`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `callback_aes_key = "${CALLBACK_AES_KEY}"`

### `projects.platforms.options.callback_path` — `feishu, lark`

Set the inbound webhook callback path used when encrypt_key enables webhook mode.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `optional`
- Default: `/feishu/webhook`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `callback_path = "/feishu/webhook"`

### `projects.platforms.options.callback_path` — `line`

Set the inbound webhook callback path.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`line`)
- Type: `string`
- Requirement: `optional`
- Default: `/callback`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `callback_path = "/callback"`

### `projects.platforms.options.callback_path` — `wecom`

Set the inbound webhook callback path.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `optional`
- Default: `/wecom/callback`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `callback_path = "/wecom/callback"`

### `projects.platforms.options.callback_token` — `wecom`

Verify WeCom callback requests.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `mode is unset or mode = callback`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `callback_token = "${CALLBACK_TOKEN}"`

### `projects.platforms.options.card_template_id` — `dingtalk`

Select the DingTalk interactive-card template ID.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `card_template_id = "value"`

### `projects.platforms.options.card_template_key` — `dingtalk`

Select the DingTalk card-template key.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `string`
- Requirement: `optional`
- Default: `content`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `card_template_key = "content"`

### `projects.platforms.options.card_throttle_ms` — `dingtalk`

Throttle DingTalk card updates in milliseconds.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `integer`
- Requirement: `optional`
- Default: `300`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞` `milliseconds`
- Example: `card_throttle_ms = 300`

### `projects.platforms.options.cdn_base_url` — `weixin`

Override the Weixin CDN download/upload base URL.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weixin`)
- Type: `string`
- Requirement: `optional`
- Default: `https://novac2c.cdn.weixin.qq.com/c2c`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `cdn_base_url = "https://novac2c.cdn.weixin.qq.com/c2c"`

### `projects.platforms.options.channel_secret` — `line`

Verify LINE webhook signatures.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`line`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `channel_secret = "${CHANNEL_SECRET}"`

### `projects.platforms.options.channel_token` — `line`

Authenticate LINE Messaging API requests.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`line`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `channel_token = "${CHANNEL_TOKEN}"`

### `projects.platforms.options.clean_reply` — `wps-xiezuo`

Strip thinking and tool-progress lines from WPS replies.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wps-xiezuo`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `clean_reply = false`

### `projects.platforms.options.client_id` — `dingtalk`

Identify the DingTalk application client.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Example: `client_id = "value"`

### `projects.platforms.options.client_secret` — `dingtalk`

Authenticate the DingTalk application client.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `client_secret = "${CLIENT_SECRET}"`

### `projects.platforms.options.corp_id` — `wecom`

Identify the WeCom enterprise.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Required when: `mode is unset or mode = callback`
- Example: `corp_id = "value"`

### `projects.platforms.options.corp_secret` — `wecom`

Authenticate the WeCom application.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `mode is unset or mode = callback`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `corp_secret = "${CORP_SECRET}"`

### `projects.platforms.options.cross_signing_password` — `matrix`

Initialize Matrix cross-signing when the server requires the account password.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`matrix`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Requires: `MATRIX_CROSS_SIGNING_PASSWORD may be used instead`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `cross_signing_password = "${CROSS_SIGNING_PASSWORD}"`

### `projects.platforms.options.domain` — `feishu`

Override the Feishu/Lark OpenAPI and WebSocket base URL; Feishu and Lark use different SDK defaults.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu`)
- Type: `string`
- Requirement: `optional`
- Default: `https://open.feishu.cn`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `domain = "https://open.feishu.cn"`

### `projects.platforms.options.domain` — `lark`

Override the Feishu/Lark OpenAPI and WebSocket base URL; Feishu and Lark use different SDK defaults.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`lark`)
- Type: `string`
- Requirement: `optional`
- Default: `https://open.larksuite.com`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `domain = "https://open.larksuite.com"`

### `projects.platforms.options.done_emoji` — `dingtalk`

Choose the completion reaction; 'none' disables it.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `string`
- Requirement: `optional`
- Default: `Done`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `done_emoji = "Done"`

### `projects.platforms.options.done_emoji` — `feishu, lark`

Choose the completion reaction. 'none' disables it; reaction_emoji = 'none' also disables the implicit completion reaction unless done_emoji is set explicitly.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `optional`
- Default: `Done`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `done_emoji = "Done"`
- Preset `starter/recommended-feishu`: `Done` — Pinned for completion notification.

### `projects.platforms.options.enable_feishu_card` — `feishu, lark`

Use Feishu/Lark interactive cards for replies; false falls back to non-card replies.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enable_feishu_card = true`
- Preset `starter/recommended-feishu`: `true` — Use interactive answer cards.

### `projects.platforms.options.enable_markdown` — `wecom`

Enable Markdown formatting for WeCom replies.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enable_markdown = false`

### `projects.platforms.options.enable_reactions` — `telegram`

Add a processing reaction to incoming messages.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`telegram`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enable_reactions = false`

### `projects.platforms.options.encrypt_key` — `feishu, lark`

Leave unset to consume events over WebSocket; set the event encrypt key to select webhook mode and decrypt webhook events.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `encrypt_key = "${ENCRYPT_KEY}"`

### `projects.platforms.options.group_only` — `feishu, lark`

Accept Feishu messages only from group chats.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `group_only = false`

### `projects.platforms.options.group_reply_all` — `discord, matrix, telegram`

Reply to every group message without requiring a mention.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord, matrix, telegram`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `group_reply_all = false`

### `projects.platforms.options.group_reply_all` — `feishu, lark`

Reply to every group message without an explicit bot mention, unless a non-empty group_reply_all_chats allowlist takes precedence.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `group_reply_all = false`
- Preset `starter/recommended-feishu`: `true` — Answer group messages without an @mention; scope allow_from/allow_chat first.

### `projects.platforms.options.group_reply_all_chats` — `feishu, lark`

Allow mention-free replies only in selected chat IDs. Accepts a comma-separated string or string array; a non-empty list takes precedence over group_reply_all.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string | string[]`
- Requirement: `optional`
- Default: `empty`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `group_reply_all_chats = "oc_chat_a,oc_chat_b"`

### `projects.platforms.options.group_reply_all_guilds` — `discord`

Enable mention-free replies for a comma-separated list of Discord guild IDs; a non-empty list takes precedence over group_reply_all.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord`)
- Type: `string`
- Requirement: `optional`
- Default: `empty`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `group_reply_all_guilds = "guild-a,guild-b"`

### `projects.platforms.options.guild_id` — `discord`

Limit Discord command registration to one guild for faster propagation.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `guild_id = "value"`

### `projects.platforms.options.homeserver` — `matrix`

Set the Matrix homeserver URL.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`matrix`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Example: `homeserver = "value"`

### `projects.platforms.options.http_url` — `qq`

Set the NapCat/QQ HTTP API endpoint.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`qq`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `http_url = "value"`

### `projects.platforms.options.image_batch_window_ms` — `feishu, lark`

Batch consecutive images from one session after this quiet window in milliseconds. Zero uses the 500 ms fallback; negative values are rejected.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `integer`
- Requirement: `optional`
- Default: `500`
- Default source: `builtin`
- Takes effect: `restart`
- Unit: `milliseconds`
- Example: `image_batch_window_ms = 500`

### `projects.platforms.options.intents` — `qqbot`

Set the QQ Bot gateway intent bitmask.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`qqbot`)
- Type: `integer`
- Requirement: `optional`
- Default: `100663296`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞`
- Example: `intents = 100663296`

### `projects.platforms.options.long_poll_timeout_ms` — `weixin`

Set the Weixin long-poll timeout in milliseconds.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weixin`)
- Type: `integer`
- Requirement: `optional`
- Default: `35000`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `milliseconds`
- Example: `long_poll_timeout_ms = 35000`

### `projects.platforms.options.markdown_support` — `qqbot`

Enable QQ Bot Markdown message support.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`qqbot`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `markdown_support = false`

### `projects.platforms.options.mention_map` — `feishu, lark`

Map a friendly bot name to its open_id for outbound native @ mentions; requires resolve_mentions = true.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `table`
- Requirement: `optional`
- Default: `empty`
- Default source: `builtin`
- Takes effect: `restart`
- Requires: `resolve_mentions = true`
- Example: `mention_map = { Reviewer-Bot = "ou_bot_open_id" }`

### `projects.platforms.options.mode` — `wecom`

Select the platform connection mode, such as WebSocket or callback.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `optional`
- Default: `callback`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `callback`, `websocket`
- Example: `mode = "callback"`

### `projects.platforms.options.name` — `weibo`

Set the account display name used by the platform adapter.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weibo`)
- Type: `string`
- Requirement: `optional`
- Default: `weibo`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `name = "weibo"`

### `projects.platforms.options.peer_bots` — `feishu, lark`

Map each peer bot app_id to a friendly alias for quoted-reply attribution.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `table`
- Requirement: `optional`
- Default: `empty`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `peer_bots = { cli_peer_app_id = "Reviewer-Bot" }`

### `projects.platforms.options.port` — `feishu, lark`

Set the webhook-mode listening port as a quoted string.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `optional`
- Default: `8080`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `port = "8080"`

### `projects.platforms.options.port` — `line`

Set the inbound webhook listening port as a quoted string.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`line`)
- Type: `string`
- Requirement: `optional`
- Default: `8080`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `port = "8080"`

### `projects.platforms.options.port` — `wecom`

Set the inbound webhook listening port as a quoted string.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`wecom`)
- Type: `string`
- Requirement: `optional`
- Default: `8081`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `port = "8081"`

### `projects.platforms.options.progress_style` — `discord, telegram`

Choose how progress is rendered on the messaging platform.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord, telegram`)
- Type: `string`
- Requirement: `optional`
- Default: `compact`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `legacy`, `compact`, `card`
- Example: `progress_style = "legacy"`

### `projects.platforms.options.progress_style` — `feishu, lark`

Choose legacy, compact, or card progress rendering for Feishu/Lark replies.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `optional`
- Default: `legacy`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `legacy`, `compact`, `card`
- Example: `progress_style = "legacy"`

### `projects.platforms.options.proxy` — `discord, matrix, telegram, wecom, weixin`

Route platform HTTP/WebSocket traffic through an HTTP or SOCKS5 proxy.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord, matrix, telegram, wecom, weixin`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `proxy = "value"`

### `projects.platforms.options.proxy_password` — `discord, telegram, wecom, weixin`

Authenticate to the configured platform proxy.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord, telegram, wecom, weixin`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Requires: `proxy`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `proxy_password = "${PROXY_PASSWORD}"`

### `projects.platforms.options.proxy_username` — `discord, telegram, wecom, weixin`

Set the username for platform proxy authentication.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord, telegram, wecom, weixin`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Requires: `proxy`
- Example: `proxy_username = "value"`

### `projects.platforms.options.reaction_emoji` — `dingtalk`

Choose the processing reaction emoji.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `string`
- Requirement: `optional`
- Default: `🤔Thinking`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `reaction_emoji = "🤔Thinking"`

### `projects.platforms.options.reaction_emoji` — `feishu, lark`

Choose the processing reaction; 'none' disables it and also suppresses the implicit done reaction.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Requirement: `optional`
- Default: `OnIt`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `reaction_emoji = "OnIt"`

### `projects.platforms.options.reply_to_trigger` — `feishu, lark`

Reply using the triggering message as the target. When false, ordinary replies are created without quoting it; a real topic isolated by thread_isolation still targets that topic's thread_id to preserve topic locality.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `reply_to_trigger = true`
- Preset `starter/recommended-feishu`: `true` — Quote the triggering message in ordinary chats.

### `projects.platforms.options.require_mention` — `feishu, lark`

Require an explicit bot mention in group chats. Setting false is a compatibility alias for group_reply_all = true; true does not override an explicit group_reply_all setting.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `require_mention = true`

### `projects.platforms.options.resolve_mentions` — `feishu, lark`

Resolve inbound Feishu/Lark mentions to readable names and enable mention_map for outbound native bot mentions.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `resolve_mentions = false`

### `projects.platforms.options.respond_to_at_everyone_and_here` — `discord, feishu, lark`

Treat @everyone/@here as a valid bot mention.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord, feishu, lark`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `respond_to_at_everyone_and_here = false`

### `projects.platforms.options.robot_code` — `dingtalk`

Identify the DingTalk robot used for outbound messages.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk`)
- Type: `string`
- Requirement: `optional`
- Default: `client_id`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `robot_code = "client_id"`

### `projects.platforms.options.route_tag` — `weixin`

Set the optional Weixin SKRouteTag request header.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weixin`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `route_tag = "value"`

### `projects.platforms.options.sandbox` — `qqbot`

Use the QQ Bot sandbox environment.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`qqbot`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `sandbox = false`

### `projects.platforms.options.session_scope` — `slack`

Choose whether Slack sessions are isolated by user, channel, or thread. When omitted, legacy share_session_in_channel=true changes the effective default to channel.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`slack`)
- Type: `string`
- Requirement: `optional`
- Default: `user`
- Default source: `runtime`
- Takes effect: `restart`
- Allowed values: `user`, `channel`, `thread`
- Example: `session_scope = "user"`

### `projects.platforms.options.share_session_in_channel` — `dingtalk, discord, matrix, qq, qqbot, slack, telegram`

Share one Agent session among all users in a channel or room.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`dingtalk, discord, matrix, qq, qqbot, slack, telegram`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `share_session_in_channel = false`

### `projects.platforms.options.share_session_in_channel` — `feishu, lark`

Share one Agent session among users in the same non-isolated channel; thread_isolation can still give real topics separate sessions.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `share_session_in_channel = false`

### `projects.platforms.options.state_dir` — `weixin`

Override the directory used for persistent platform state.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weixin`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `state_dir = "value"`

### `projects.platforms.options.thread_isolation` — `discord`

Use a separate Agent session for each platform thread or topic.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord`)
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `thread_isolation = false`

### `projects.platforms.options.thread_isolation` — `feishu, lark`

Choose Feishu/Lark topic isolation scope. off keeps legacy per-user/channel sessions. topics_only isolates only real topics whose events carry thread_id; ordinary group messages stay in the main chat. topic_per_message gives every top-level group message its own topic/session. Real topics get an independent Agent session and workspace binding in both enabled modes. Omitting the key maps to off; legacy true maps to topic_per_message and false maps to off. New Starter and recommended profiles write topics_only.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`feishu, lark`)
- Type: `string | boolean (legacy)`
- Requirement: `optional`
- Default: `off`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `off`, `topics_only`, `topic_per_message`
- Example: `thread_isolation = "off"`
- Preset `starter/recommended-feishu`: `topics_only` — Isolate real topics without promoting ordinary group messages.

### `projects.platforms.options.token` — `discord, max, telegram, webex, weixin`

Authenticate the platform bot or gateway.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`discord, max, telegram, webex, weixin`)
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `token = "${TOKEN}"`

### `projects.platforms.options.token` — `qq`

Authenticate the platform bot or gateway.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`qq`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `token = "${TOKEN}"`

### `projects.platforms.options.token_endpoint` — `weibo`

Override the endpoint used to obtain Weibo access tokens.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weibo`)
- Type: `string`
- Requirement: `optional`
- Default: `https://open-im.api.weibo.com/open/auth/ws_token`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `token_endpoint = "https://open-im.api.weibo.com/open/auth/ws_token"`

### `projects.platforms.options.user_id` — `matrix`

Set or override the Matrix bot user ID.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`matrix`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `user_id = "value"`

### `projects.platforms.options.webhook_listen` — `max`

Set the local webhook listen address; when webhook_url is set and this option is omitted, runtime uses :8080.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`max`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `webhook_listen = "value"`

### `projects.platforms.options.webhook_path` — `max`

Set the MAX webhook URL path.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`max`)
- Type: `string`
- Requirement: `optional`
- Default: `/webhook`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `webhook_path = "/webhook"`

### `projects.platforms.options.webhook_resubscribe_interval` — `max`

Periodically refresh the MAX webhook subscription using a Go duration string.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`max`)
- Type: `string`
- Requirement: `optional`
- Default: `5m`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `Go duration string (for example: 30s, 5m, 1h)`
- Example: `webhook_resubscribe_interval = "5m"`

### `projects.platforms.options.webhook_secret` — `max`

Verify MAX webhook requests.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`max`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `webhook_secret = "${WEBHOOK_SECRET}"`

### `projects.platforms.options.webhook_url` — `max`

Publish the externally reachable MAX webhook URL.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`max`)
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `adapter`
- Takes effect: `restart`
- Example: `webhook_url = "value"`

### `projects.platforms.options.ws_endpoint` — `weibo`

Override the platform WebSocket endpoint.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`weibo`)
- Type: `string`
- Requirement: `optional`
- Default: `ws://open-im.api.weibo.com/ws/stream`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `ws_endpoint = "ws://open-im.api.weibo.com/ws/stream"`

### `projects.platforms.options.ws_url` — `qq`

Set the NapCat/QQ forward WebSocket endpoint.

- Source: `toml`
- Placement: `[projects.platforms.options] (inside one [[projects.platforms]])`
- Scope: `platform` (`qq`)
- Type: `string`
- Requirement: `optional`
- Default: `ws://127.0.0.1:3001`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `ws_url = "ws://127.0.0.1:3001"`

### `projects.platforms.type`

Select a messaging-platform adapter for this entry; a normal runtime project needs at least one platform entry.

- Source: `toml`
- Placement: `[[projects.platforms]] (inside one [[projects]])`
- Scope: `platform`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `restart`
- Example: `type = "value"`

### `projects.quiet`

Legacy per-project switch that hides thinking and tool messages when display overrides are unset.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Status: deprecated compatibility option.
- Example: `quiet = true`

### `projects.references.display_path`

Choose the user-facing path rendered by project references.

- Source: `toml`
- Placement: `[projects.references] (inside one [[projects]])`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `absolute`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `absolute`, `relative`, `basename`, `dirname_basename`, `smart`
- Example: `display_path = "absolute"`
- Preset `starter/recommended-feishu`: `smart` — Short but unambiguous paths.

### `projects.references.enclosure_style`

Choose how normalized project references are enclosed.

- Source: `toml`
- Placement: `[projects.references] (inside one [[projects]])`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `none`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `none`, `bracket`, `angle`, `fullwidth`, `code`
- Example: `enclosure_style = "none"`
- Preset `starter/recommended-feishu`: `code` — Make references easy to copy.

### `projects.references.marker_style`

Choose the marker emitted for normalized project references.

- Source: `toml`
- Placement: `[projects.references] (inside one [[projects]])`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `none`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `none`, `ascii`, `emoji`
- Example: `marker_style = "none"`
- Preset `starter/recommended-feishu`: `emoji` — Visually mark references.

### `projects.references.normalize_agents`

Apply reference normalization only to the listed Agent adapters.

- Source: `toml`
- Placement: `[projects.references] (inside one [[projects]])`
- Scope: `project`
- Type: `string[]`
- Requirement: `optional`
- Default: `[]`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `all`, `codex`, `claudecode`
- Example: `normalize_agents = ["all"]`
- Preset `starter/recommended-feishu`: `["<active-agent>"]` — Normalize the active Agent's references.

### `projects.references.render_platforms`

Render normalized references only on the listed messaging platforms.

- Source: `toml`
- Placement: `[projects.references] (inside one [[projects]])`
- Scope: `project`
- Type: `string[]`
- Requirement: `optional`
- Default: `[]`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `all`, `feishu`, `weixin`
- Example: `render_platforms = ["all"]`
- Preset `starter/recommended-feishu`: `["feishu"]` — Render references for Feishu.

### `projects.reply_footer`

Override the reply footer for one project.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `reload`
- Example: `reply_footer = true`

### `projects.reset_on_idle_mins`

Rotate to a fresh session when the user returns after this idle period; zero disables it.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `0`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `0` to `+∞` `minutes`
- Example: `reset_on_idle_mins = 0`

### `projects.run_as_env`

Allow the listed environment-variable names across projects user isolation.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `run_as_env = ["value"]`

### `projects.run_as_user`

Run this project's Agent as another non-root OS user.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `run_as_user = "value"`

### `projects.shell`

Choose the shell used by projects.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `shell = "value"`

### `projects.shell_profile`

Prepend a shell initialization command for projects.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `shell_profile = "value"`

### `projects.show_context_indicator`

Deprecated no-op retained only for old project config compatibility.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Status: deprecated compatibility option.
- Example: `show_context_indicator = false`

### `projects.show_workdir_indicator`

Deprecated no-op retained only for old project config compatibility.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Status: deprecated compatibility option.
- Example: `show_workdir_indicator = false`

### `projects.skip_git`

Allow multi-workspace directories that are not Git repositories.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `skip_git = false`

### `projects.users.default_role`

Choose the role assigned to users not explicitly listed.

- Source: `toml`
- Placement: `[projects.users] (inside one [[projects]])`
- Scope: `project`
- Type: `string`
- Requirement: `optional`
- Default: `member`
- Default source: `builtin`
- Takes effect: `reload`
- Example: `default_role = "member"`

### `projects.users.roles.<name>.disabled_commands`

Disable the listed commands for projects users roles <name>.

- Source: `toml`
- Placement: `[projects.users.roles.<name>] (inside one [[projects]])`
- Scope: `project`
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `disabled_commands = ["value"]`

### `projects.users.roles.<name>.rate_limit.max_messages`

Override the inbound message count for this role; zero disables the limit.

- Source: `toml`
- Placement: `[projects.users.roles.<name>.rate_limit] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `20`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `0` to `+∞`
- Example: `max_messages = 20`

### `projects.users.roles.<name>.rate_limit.window_secs`

Override the inbound rate-limit window for this role.

- Source: `toml`
- Placement: `[projects.users.roles.<name>.rate_limit] (inside one [[projects]])`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `60`
- Default source: `builtin`
- Takes effect: `reload`
- Range: `1` to `+∞` `seconds`
- Example: `window_secs = 60`

### `projects.users.roles.<name>.user_ids`

List the platform user IDs assigned to this role; use '*' for one wildcard role.

- Source: `toml`
- Placement: `[projects.users.roles.<name>] (inside one [[projects]])`
- Scope: `project`
- Type: `string[]`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `user_ids = ["user-id"]`

### `projects.workspace_idle_timeout_mins`

Deprecated project-level workspace reaper timeout; use the top-level option instead.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `integer`
- Requirement: `optional`
- Default: `inherit`
- Default source: `inherit`
- Takes effect: `restart`
- Unit: `minutes`
- Status: deprecated compatibility option.
- Example: `workspace_idle_timeout_mins = 1`

### `projects.workspace_init_allow_local_paths`

Allow /workspace init to bind local directories in addition to Git URLs.

- Source: `toml`
- Placement: `[[projects]]`
- Scope: `project`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `workspace_init_allow_local_paths = false`

### `provider_presets_url`

Override the remote JSON source used for recommended provider presets.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `provider_presets_url = "value"`

### `providers.agent_model_lists.<name>.alias`

Set a short user-facing alias for providers agent_model_lists <name>.

- Source: `toml`
- Placement: `[[providers.agent_model_lists.<name>]] (inside one [[providers]])`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `alias = "value"`

### `providers.agent_model_lists.<name>.model`

Name a model exposed by this provider for one Agent type.

- Source: `toml`
- Placement: `[[providers.agent_model_lists.<name>]] (inside one [[providers]])`
- Scope: `global`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `model = "value"`

### `providers.agent_models.<name>`

Set one named entry in providers agent_models.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `example = { example = "value" }`

### `providers.agent_types`

Restrict a shared provider to selected Agent adapter types.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `agent_types = ["value"]`

### `providers.api_key`

Authenticate to a shared model provider.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `providers.base_url`

Override the shared provider API base URL.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `base_url = "value"`

### `providers.codex.env_key`

Name the environment variable from which providers codex reads its credential.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `env_key = "value"`

### `providers.codex.http_headers.<name>`

Set one named entry in providers codex http_headers.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `example = { example = "value" }`

### `providers.codex.wire_api`

Select the wire protocol used by providers codex.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `wire_api = "value"`

### `providers.endpoints.<name>`

Set one named entry in providers endpoints.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `example = { example = "value" }`

### `providers.env.<name>`

Set one named entry in providers env.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `table`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `example = { example = "value" }`

### `providers.model`

Select the model used by providers.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `model = "value"`

### `providers.models.alias`

Set a short user-facing alias for providers models.

- Source: `toml`
- Placement: `[[providers.models]] (inside one [[providers]])`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `alias = "value"`

### `providers.models.model`

Name a model exposed by this shared provider.

- Source: `toml`
- Placement: `[[providers.models]] (inside one [[providers]])`
- Scope: `global`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `model = "value"`

### `providers.name`

Name a shared model provider for references and switching.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `string`
- Requirement: `required`
- Default: `none`
- Default source: `none`
- Takes effect: `reload`
- Example: `name = "value"`

### `providers.thinking`

Choose the provider thinking mode used by providers.

- Source: `toml`
- Placement: `[[providers]]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `reload`
- Example: `thinking = "value"`

### `queue.busy_message_mode`

Steer eligible input into the active turn or always preserve FIFO queueing.

- Source: `toml`
- Placement: `[queue]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `steer`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `steer`, `queue`
- Example: `busy_message_mode = "steer"`

### `queue.max_depth`

Limit pending user messages queued behind one busy session.

- Source: `toml`
- Placement: `[queue]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `5`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞`
- Example: `max_depth = 5`

### `quiet`

Legacy switch that hides thinking and tool messages when newer display fields are unset.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `reload`
- Status: deprecated compatibility option.
- Example: `quiet = false`

### `rate_limit.max_messages`

Limit inbound messages per user/session window; zero disables the limit.

- Source: `toml`
- Placement: `[rate_limit]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `20`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞`
- Example: `max_messages = 20`

### `rate_limit.window_secs`

Set the inbound rate-limit window in seconds.

- Source: `toml`
- Placement: `[rate_limit]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `60`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `1` to `+∞` `seconds`
- Example: `window_secs = 60`

### `relay.timeout_secs`

Limit how long cross-project relay waits for a response; zero disables waiting.

- Source: `toml`
- Placement: `[relay]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `120`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `seconds`
- Example: `timeout_secs = 120`

### `relay.visibility`

Choose how much relay activity is echoed into the group.

- Source: `toml`
- Placement: `[relay]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `full`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `full`, `summary`, `none`
- Example: `visibility = "full"`

### `shell`

Choose the shell used by /shell, exec cron jobs, hooks, and webhook exec.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `sh on Unix; powershell.exe on Windows`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `shell = "value"`

### `shell_profile`

Prepend an initialization command to every configured shell command.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `shell_profile = "value"`

### `speech.enabled`

Transcribe incoming voice messages before sending them to the Agent.

- Source: `toml`
- Placement: `[speech]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enabled = false`

### `speech.gemini.api_key`

Authenticate the selected speech-to-text provider.

- Source: `toml`
- Placement: `[speech.gemini]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `speech.enabled = true and speech.provider = gemini`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `speech.gemini.model`

Select the model used by speech gemini.

- Source: `toml`
- Placement: `[speech.gemini]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `model = "value"`

### `speech.groq.api_key`

Authenticate the selected speech-to-text provider.

- Source: `toml`
- Placement: `[speech.groq]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `speech.enabled = true and speech.provider = groq`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `speech.groq.model`

Select the model used by speech groq.

- Source: `toml`
- Placement: `[speech.groq]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `model = "value"`

### `speech.language`

Set the language or locale hint used by speech.

- Source: `toml`
- Placement: `[speech]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `language = "value"`

### `speech.openai.api_key`

Authenticate the selected speech-to-text provider.

- Source: `toml`
- Placement: `[speech.openai]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `speech.enabled = true and speech.provider is unset or openai`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `speech.openai.base_url`

Override the service base URL for speech openai.

- Source: `toml`
- Placement: `[speech.openai]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `base_url = "value"`

### `speech.openai.model`

Select the model used by speech openai.

- Source: `toml`
- Placement: `[speech.openai]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `model = "value"`

### `speech.provider`

Choose the speech-to-text provider.

- Source: `toml`
- Placement: `[speech]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `openai`
- Default source: `builtin`
- Takes effect: `restart`
- Required when: `speech.enabled = true`
- Allowed values: `openai`, `groq`, `qwen`, `gemini`
- Example: `provider = "openai"`

### `speech.qwen.api_key`

Authenticate the selected speech-to-text provider.

- Source: `toml`
- Placement: `[speech.qwen]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `speech.enabled = true and speech.provider = qwen`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `speech.qwen.base_url`

Override the service base URL for speech qwen.

- Source: `toml`
- Placement: `[speech.qwen]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `base_url = "value"`

### `speech.qwen.model`

Select the model used by speech qwen.

- Source: `toml`
- Placement: `[speech.qwen]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `model = "value"`

### `stream_preview.disabled_platforms`

Disable stream_preview on the listed messaging platforms.

- Source: `toml`
- Placement: `[stream_preview]`
- Scope: `global`
- Type: `string[]`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `disabled_platforms = ["value"]`

### `stream_preview.enabled`

Update one preview message while the Agent is still streaming.

- Source: `toml`
- Placement: `[stream_preview]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enabled = true`

### `stream_preview.interval_ms`

Set the minimum interval between preview updates.

- Source: `toml`
- Placement: `[stream_preview]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `1500`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `milliseconds`
- Example: `interval_ms = 1500`

### `stream_preview.max_chars`

Limit the accumulated streaming-preview length.

- Source: `toml`
- Placement: `[stream_preview]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `2000`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `characters`
- Example: `max_chars = 2000`

### `stream_preview.min_delta_chars`

Require this many new characters before refreshing the preview.

- Source: `toml`
- Placement: `[stream_preview]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `30`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `characters`
- Example: `min_delta_chars = 30`

### `tts.agents.<name>.language_type`

Set the provider-specific language hint used by tts agents <name>.

- Source: `toml`
- Placement: `[tts.agents.<name>]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `language_type = "value"`

### `tts.agents.<name>.max_text_len`

Skip or truncate tts agents <name> beyond this text length; zero removes the limit.

- Source: `toml`
- Placement: `[tts.agents.<name>]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `max_text_len = 1`

### `tts.agents.<name>.provider`

Select the provider used by tts agents <name>.

- Source: `toml`
- Placement: `[tts.agents.<name>]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `provider = "value"`

### `tts.agents.<name>.speed`

Set the speech-speed multiplier used by tts agents <name>.

- Source: `toml`
- Placement: `[tts.agents.<name>]`
- Scope: `global`
- Type: `number`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `speed = 1.0`

### `tts.agents.<name>.voice`

Choose the voice used by tts agents <name>.

- Source: `toml`
- Placement: `[tts.agents.<name>]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `voice = "value"`

### `tts.agents.<name>.voice_id`

Set the provider-specific voice ID used by tts agents <name>.

- Source: `toml`
- Placement: `[tts.agents.<name>]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `voice_id = "value"`

### `tts.enabled`

Enable text-to-speech replies.

- Source: `toml`
- Placement: `[tts]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enabled = false`

### `tts.language_type`

Set the provider-specific language hint used by tts.

- Source: `toml`
- Placement: `[tts]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `language_type = "value"`

### `tts.max_text_len`

Skip or truncate tts beyond this text length; zero removes the limit.

- Source: `toml`
- Placement: `[tts]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `max_text_len = 1`

### `tts.mimo.api_key`

Authenticate the selected text-to-speech provider.

- Source: `toml`
- Placement: `[tts.mimo]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `tts.enabled = true and tts.provider = mimo`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `tts.mimo.base_url`

Override the service base URL for tts mimo.

- Source: `toml`
- Placement: `[tts.mimo]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `base_url = "value"`

### `tts.mimo.model`

Select the model used by tts mimo.

- Source: `toml`
- Placement: `[tts.mimo]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `model = "value"`

### `tts.minimax.api_key`

Authenticate the selected text-to-speech provider.

- Source: `toml`
- Placement: `[tts.minimax]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `tts.enabled = true and tts.provider = minimax and no MiniMax local config is available`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `tts.minimax.base_url`

Override the service base URL for tts minimax.

- Source: `toml`
- Placement: `[tts.minimax]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `base_url = "value"`

### `tts.minimax.config_file`

Override the auxiliary configuration-file path used by tts minimax.

- Source: `toml`
- Placement: `[tts.minimax]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `config_file = "value"`

### `tts.minimax.model`

Select the model used by tts minimax.

- Source: `toml`
- Placement: `[tts.minimax]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `model = "value"`

### `tts.openai.api_key`

Authenticate the selected text-to-speech provider.

- Source: `toml`
- Placement: `[tts.openai]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `tts.enabled = true and tts.provider is unset or openai`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `tts.openai.base_url`

Override the service base URL for tts openai.

- Source: `toml`
- Placement: `[tts.openai]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `base_url = "value"`

### `tts.openai.model`

Select the model used by tts openai.

- Source: `toml`
- Placement: `[tts.openai]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `model = "value"`

### `tts.provider`

Choose the text-to-speech provider.

- Source: `toml`
- Placement: `[tts]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `openai`
- Default source: `builtin`
- Takes effect: `restart`
- Required when: `tts.enabled = true`
- Allowed values: `qwen`, `openai`, `minimax`, `mimo`, `espeak`, `pico`, `edge`
- Example: `provider = "qwen"`

### `tts.qwen.api_key`

Authenticate the selected text-to-speech provider.

- Source: `toml`
- Placement: `[tts.qwen]`
- Scope: `global`
- Type: `string`
- Requirement: `conditional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Required when: `tts.enabled = true and tts.provider = qwen`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `api_key = "${API_KEY}"`

### `tts.qwen.base_url`

Override the service base URL for tts qwen.

- Source: `toml`
- Placement: `[tts.qwen]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `base_url = "value"`

### `tts.qwen.model`

Select the model used by tts qwen.

- Source: `toml`
- Placement: `[tts.qwen]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `model = "value"`

### `tts.speed`

Set the speech-speed multiplier used by tts.

- Source: `toml`
- Placement: `[tts]`
- Scope: `global`
- Type: `number`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `speed = 1.0`

### `tts.tts_mode`

Choose voice-only replies or synthesize every eligible response.

- Source: `toml`
- Placement: `[tts]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `voice_only`
- Default source: `builtin`
- Takes effect: `restart`
- Allowed values: `voice_only`, `always`
- Example: `tts_mode = "voice_only"`

### `tts.voice`

Choose the voice used by tts.

- Source: `toml`
- Placement: `[tts]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `voice = "value"`

### `tts.voice_id`

Set the provider-specific voice ID used by tts.

- Source: `toml`
- Placement: `[tts]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `runtime`
- Takes effect: `restart`
- Example: `voice_id = "value"`

### `update_notice`

Notify the most recently active chat once per stable release; the user reviews an exact immutable Plan before confirmation.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `true`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `update_notice = true`

### `webhook.enabled`

Expose the external HTTP endpoint that triggers Agent prompts or shell commands.

- Source: `toml`
- Placement: `[webhook]`
- Scope: `global`
- Type: `boolean`
- Requirement: `optional`
- Default: `false`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `enabled = false`

### `webhook.path`

Set the external webhook URL path prefix.

- Source: `toml`
- Placement: `[webhook]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `/hook`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `path = "/hook"`

### `webhook.port`

Set the external webhook listening port.

- Source: `toml`
- Placement: `[webhook]`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `9111`
- Default source: `builtin`
- Takes effect: `restart`
- Example: `port = 9111`

### `webhook.token`

Authenticate webhook with a shared token.

- Source: `toml`
- Placement: `[webhook]`
- Scope: `global`
- Type: `string`
- Requirement: `optional`
- Default: `unset`
- Default source: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.
- Example: `token = "${TOKEN}"`

### `workspace_idle_timeout_mins`

Reap inactive multi-workspace engines after this many minutes; zero disables it.

- Source: `toml`
- Placement: `config.toml root`
- Scope: `global`
- Type: `integer`
- Requirement: `optional`
- Default: `15`
- Default source: `builtin`
- Takes effect: `restart`
- Range: `0` to `+∞` `minutes`
- Example: `workspace_idle_timeout_mins = 15`
