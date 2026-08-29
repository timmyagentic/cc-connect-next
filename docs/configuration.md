<!-- Code generated from the compiled configuration catalog. DO NOT EDIT. -->

# cc-connect-next Configuration Capabilities

Catalog version: `source`. This reference describes capability and never reads or prints local configuration values.

Apply modes: `live` takes effect in the running process; `reload` can be applied with `/config reload` after saving; `new-session` affects newly started Agent sessions; `restart` requires restarting cc-connect-next.

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

### `aliases.command`

Choose the slash command expanded by this alias.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `aliases.name`

Set the natural-language trigger for a command alias.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `attachment_send`

Allow or block Agent-initiated image and file send-back without disabling text replies.

- Scope: `global`
- Type: `string`
- Default: `on`
- Takes effect: `reload`
- Allowed values: `on`, `off`

### `banned_words`

Block messages containing any configured banned word.

- Scope: `global`
- Type: `string[]`
- Default: `[]`
- Takes effect: `reload`

### `bridge.cors_origins`

Allow browser requests to bridge from the listed CORS origins.

- Scope: `global`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `bridge.enabled`

Enable the WebSocket/REST bridge for external platform adapters.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `bridge.insecure`

Allow a tokenless bridge for local development only.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `bridge.path`

Set the external adapter bridge WebSocket path.

- Scope: `global`
- Type: `string`
- Default: `/bridge/ws`
- Takes effect: `restart`

### `bridge.port`

Set the external adapter bridge port.

- Scope: `global`
- Type: `integer`
- Default: `9810`
- Takes effect: `restart`

### `bridge.token`

Authenticate bridge with a shared token.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `commands.description`

Describe the custom command in menus and help.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `commands.exec`

Execute a shell command instead of prompting the Agent.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `commands.name`

Set the custom slash-command name.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `commands.prompt`

Expand the custom command into an Agent prompt.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `commands.work_dir`

Override the working directory for a custom exec command.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `cron.session_mode`

Choose whether scheduled runs reuse a session or create a fresh session per run.

- Scope: `global`
- Type: `string`
- Default: `reuse`
- Takes effect: `restart`
- Allowed values: `reuse`, `new_per_run`

### `cron.silent`

Suppress the notification sent when a scheduled run starts.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `data_dir`

Choose where cc-connect-next stores sessions, state, media, and runtime metadata.

- Scope: `global`
- Type: `string`
- Default: `~/.cc-connect-next`
- Takes effect: `restart`

### `display.card_mode`

Choose rich Card 2.0 or legacy card rendering where supported.

- Scope: `global`
- Type: `string`
- Default: `rich`
- Takes effect: `reload`
- Allowed values: `rich`, `legacy`

### `display.hide_agent_footer`

Strip equivalent model/token/context footer lines emitted by the Agent itself.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`

### `display.history_max_len`

Limit each /history entry; zero disables truncation.

- Scope: `global`
- Type: `integer`
- Default: `1000`
- Takes effect: `reload`

### `display.mode`

Choose full, compact, or quiet reply presentation defaults.

- Scope: `global`
- Type: `string`
- Default: `full (process messages remain off unless explicitly selected)`
- Takes effect: `reload`
- Allowed values: `full`, `compact`, `quiet`

### `display.reply_footer`

Show the model, reasoning effort, and elapsed-time footer on completed replies.

- Scope: `global`
- Type: `boolean`
- Default: `true`
- Takes effect: `reload`

### `display.show_context_indicator`

Deprecated no-op retained only for old config compatibility.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`
- Status: deprecated compatibility option.

### `display.thinking_max_len`

Limit reasoning-progress text length; zero disables truncation.

- Scope: `global`
- Type: `integer`
- Default: `300`
- Takes effect: `reload`

### `display.thinking_messages`

Show or hide Agent reasoning progress messages.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`

### `display.tool_max_len`

Limit tool-progress text length; zero disables truncation.

- Scope: `global`
- Type: `integer`
- Default: `500`
- Takes effect: `reload`

### `display.tool_messages`

Show or hide Agent tool-progress messages.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`

### `feedback.enabled`

Enable /feedback and capability-gap prompts; every submission still requires confirmation.

- Scope: `global`
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `feedback.endpoint`

Override the author-operated anonymous feedback relay endpoint.

- Scope: `global`
- Type: `string`
- Default: `built-in author relay`
- Takes effect: `restart`

### `hooks.async`

Run the hook asynchronously instead of blocking message handling.

- Scope: `global`
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `hooks.command`

Set the shell command executed by hooks.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `hooks.event`

Choose the event that triggers hooks.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `hooks.timeout`

Set the execution timeout in seconds for hooks.

- Scope: `global`
- Type: `integer`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `hooks.type`

Choose the implementation type used by hooks.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `hooks.url`

Set the HTTP URL called by hooks.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `idle_timeout_mins`

Stop a turn when the Agent produces no events for this many minutes; zero disables it.

- Scope: `global`
- Type: `integer`
- Default: `120`
- Takes effect: `restart`

### `instant_reply.content`

Override the localized immediate acknowledgement text.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `instant_reply.enabled`

Immediately acknowledge an incoming message before Agent work begins.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`

### `language`

Choose the language used by bot messages, or detect it from the user's first message.

- Scope: `global`
- Type: `string`
- Default: `zh`
- Takes effect: `restart`
- Allowed values: `zh`, `en`, `zh-TW`, `ja`, `es`, `auto`
- Example: `language = "zh"`

### `log.level`

Set the minimum runtime log severity.

- Scope: `global`
- Type: `string`
- Default: `info`
- Takes effect: `restart`
- Allowed values: `debug`, `info`, `warn`, `error`

### `management.cors_origins`

Allow browser requests to management from the listed CORS origins.

- Scope: `global`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `management.enabled`

Enable the local management API and Web console backend.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `management.port`

Set the management API listening port.

- Scope: `global`
- Type: `integer`
- Default: `9820`
- Takes effect: `restart`

### `management.token`

Authenticate management with a shared token.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `max_attachment_size_mb`

Set the maximum size of one outbound attachment in MiB.

- Scope: `global`
- Type: `integer`
- Default: `50`
- Takes effect: `reload`

### `max_turn_time_mins`

Cap the absolute wall-clock duration of one Agent turn; zero disables it.

- Scope: `global`
- Type: `integer`
- Default: `0`
- Takes effect: `restart`

### `outgoing_rate_limit.burst`

Set the maximum immediate outbound burst.

- Scope: `global`
- Type: `integer`
- Default: `ceil(max_per_second)`
- Takes effect: `restart`

### `outgoing_rate_limit.max_per_second`

Limit outgoing messages per second; zero means unlimited.

- Scope: `global`
- Type: `number`
- Default: `0`
- Takes effect: `restart`

### `outgoing_rate_limit.platforms.<name>.burst`

Set the maximum burst size for outgoing_rate_limit platforms <name>.

- Scope: `global`
- Type: `integer`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `outgoing_rate_limit.platforms.<name>.max_per_second`

Limit how many operations outgoing_rate_limit platforms <name> sends per second.

- Scope: `global`
- Type: `number`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.admin_from`

Restrict privileged commands to selected platform user IDs; unset blocks privileged commands for everyone.

- Scope: `project`
- Type: `string`
- Default: `unset / nobody`
- Takes effect: `reload`

### `projects.agent.answer_profiles.fast.model`

Select the model used by projects agent answer_profiles fast.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.agent.answer_profiles.fast.reasoning_effort`

Override reasoning effort for projects agent answer_profiles fast.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.agent.answer_profiles.fast.service_tier`

Override the model-catalog service tier for one-shot /fast answers.

- Scope: `agent`
- Type: `string`
- Default: `inherit`
- Takes effect: `restart`
- Allowed values: `model-catalog-driven (for example: default, fast)`

### `projects.agent.answer_profiles.quality.model`

Select the model used by projects agent answer_profiles quality.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.agent.answer_profiles.quality.reasoning_effort`

Override reasoning effort for projects agent answer_profiles quality.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.agent.answer_profiles.quality.service_tier`

Override the model-catalog service tier for one-shot /quality answers.

- Scope: `agent`
- Type: `string`
- Default: `inherit`
- Takes effect: `restart`
- Allowed values: `model-catalog-driven (for example: default, fast)`

### `projects.agent.options.agent` — `opencode`

Select the named sub-agent or profile exposed by the CLI.

- Scope: `agent` (`opencode`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.allowed_tools` — `claudecode`

Pre-approve selected Claude Code tools in approval-based modes.

- Scope: `agent` (`claudecode`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.app_server_url` — `codex`

Choose the Codex app-server transport endpoint.

- Scope: `agent` (`codex`)
- Type: `string`
- Default: `stdio`
- Takes effect: `restart`

### `projects.agent.options.append_system_prompt` — `claudecode, codex`

Append project instructions while preserving the Agent's default system prompt.

- Scope: `agent` (`claudecode, codex`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.args` — `acp, devin`

Pass additional arguments to the configured Agent command.

- Scope: `agent` (`acp, devin`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.auth_method` — `acp`

Select the authentication method used by an ACP Agent.

- Scope: `agent` (`acp`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.auto_create` — `tmux`

Create the configured tmux session when it does not exist.

- Scope: `agent` (`tmux`)
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.agent.options.backend` — `codex`

Select the Codex execution backend; app_server supports native steering and approvals.

- Scope: `agent` (`codex`)
- Type: `string`
- Default: `app_server`
- Takes effect: `restart`
- Allowed values: `app_server`, `exec`

### `projects.agent.options.cli_args_flag` — `claudecode`

Name the wrapper flag that accepts Agent CLI arguments.

- Scope: `agent` (`claudecode`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.cli_path` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Override the Agent CLI executable path.

- Scope: `agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `string`
- Default: `adapter default`
- Takes effect: `restart`

### `projects.agent.options.cmd` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Override the Agent command, optionally including global arguments.

- Scope: `agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `string`
- Default: `adapter default`
- Takes effect: `restart`

### `projects.agent.options.cmd_args_flag` — `claudecode`

Name the wrapper flag used to forward command arguments.

- Scope: `agent` (`claudecode`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.codex_home` — `codex`

Override CODEX_HOME for this project without changing the user's global Codex home.

- Scope: `agent` (`codex`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.command` — `acp, antigravity, claudecode, codex, copilot, cursor, devin, gemini, iflow, kimi, opencode, pi, qoder`

Set the Agent executable; an alias used by several adapters.

- Scope: `agent` (`acp, antigravity, claudecode, codex, copilot, cursor, devin, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `string`
- Default: `adapter default`
- Takes effect: `restart`

### `projects.agent.options.disallowed_tools` — `claudecode`

Deny selected Claude Code tools even when the mode would otherwise allow them.

- Scope: `agent` (`claudecode`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.display_name` — `acp, devin`

Set the user-facing name of a generic or ACP Agent.

- Scope: `agent` (`acp, devin`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.env` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Inject project-scoped environment variables into Agent processes.

- Scope: `agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `table`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.agent.options.init_command` — `tmux`

Run a shell initialization command before tmux prompts are sent.

- Scope: `agent` (`tmux`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.max_context_tokens` — `claudecode`

Override the maximum context-token budget accepted by Claude Code.

- Scope: `agent` (`claudecode`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.mode` — `acp`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`acp`)
- Type: `string`
- Default: `adapter default`
- Takes effect: `restart`

### `projects.agent.options.mode` — `antigravity`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`antigravity`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`
- Allowed values: `default`, `yolo`, `plan`

### `projects.agent.options.mode` — `claudecode`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`claudecode`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`
- Allowed values: `default`, `acceptEdits`, `plan`, `auto`, `bypassPermissions`, `dontAsk`

### `projects.agent.options.mode` — `codex`

Choose the Codex approval and sandbox mode. Omitting the key keeps the suggest compatibility fallback; fresh generated configs explicitly set yolo.

- Scope: `agent` (`codex`)
- Type: `string`
- Default: `suggest when omitted; generated starter config writes yolo`
- Takes effect: `restart`
- Allowed values: `suggest`, `auto-edit`, `full-auto`, `yolo`

### `projects.agent.options.mode` — `copilot`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`copilot`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`
- Allowed values: `default`, `bypassPermissions`

### `projects.agent.options.mode` — `cursor`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`cursor`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`
- Allowed values: `default`, `force`, `plan`, `ask`

### `projects.agent.options.mode` — `gemini`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`gemini`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`
- Allowed values: `default`, `auto_edit`, `yolo`, `plan`

### `projects.agent.options.mode` — `iflow`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`iflow`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`
- Allowed values: `default`, `auto-edit`, `plan`, `yolo`

### `projects.agent.options.mode` — `kimi`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`kimi`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`
- Allowed values: `default`, `yolo`, `plan`, `quiet`

### `projects.agent.options.mode` — `opencode, pi, qoder`

Choose the Agent approval, sandbox, or planning mode.

- Scope: `agent` (`opencode, pi, qoder`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`
- Allowed values: `default`, `yolo`

### `projects.agent.options.model` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`

Select the default model for new Agent sessions.

- Scope: `agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.model_context_window` — `codex`

Declare the Codex model context window used for usage reporting and compaction decisions.

- Scope: `agent` (`codex`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.pane` — `tmux`

Select the tmux pane used for Agent input and output.

- Scope: `agent` (`tmux`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.plugin_dir` — `claudecode`

Load Claude Code plugins from the listed directories.

- Scope: `agent` (`claudecode`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.poll_interval_ms` — `tmux`

Set the tmux output polling interval in milliseconds.

- Scope: `agent` (`tmux`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.prompt_pattern` — `tmux`

Regular expression used to recognize the tmux Agent prompt.

- Scope: `agent` (`tmux`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.provider` — `antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode`

Select the active configured provider for this project.

- Scope: `agent` (`antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `reload`

### `projects.agent.options.reasoning_effort` — `claudecode`

Set the default reasoning-effort level for new turns.

- Scope: `agent` (`claudecode`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`
- Allowed values: `low`, `medium`, `high`, `max`

### `projects.agent.options.reasoning_effort` — `codex`

Set the default reasoning-effort level for new turns.

- Scope: `agent` (`codex`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`
- Allowed values: `low`, `medium`, `high`, `xhigh`, `max`

### `projects.agent.options.router_api_key` — `claudecode`

Authenticate to the configured Claude Code Router.

- Scope: `agent` (`claudecode`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.agent.options.router_url` — `claudecode`

Route Claude Code requests through the specified router URL.

- Scope: `agent` (`claudecode`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.run_as_env` — `claudecode`

Extend the environment allowlist passed across OS-user isolation.

- Scope: `agent` (`claudecode`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.run_as_user` — `claudecode`

Run Claude Code under another non-root OS user.

- Scope: `agent` (`claudecode`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.service_tier` — `codex`

Select the model-catalog service tier; common Codex values include default and fast.

- Scope: `agent` (`codex`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`
- Allowed values: `model-catalog-driven (for example: default, fast)`

### `projects.agent.options.session` — `tmux`

Name the tmux session that hosts the Agent.

- Scope: `agent` (`tmux`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.session_title_model` — `codex`

Optionally use an isolated local Codex model to generate concise Codex App titles.

- Scope: `agent` (`codex`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.session_title_prefix` — `codex`

Prefix Codex App session titles with a configurable source label.

- Scope: `agent` (`codex`)
- Type: `string`
- Default: `[飞书]`
- Takes effect: `restart`

### `projects.agent.options.shell` — `tmux`

Select the shell used by the tmux adapter.

- Scope: `agent` (`tmux`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.startup_wait_ms` — `tmux`

Wait this many milliseconds after creating a tmux session.

- Scope: `agent` (`tmux`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.strip_input_block` — `tmux`

Remove the echoed input block from captured tmux output.

- Scope: `agent` (`tmux`)
- Type: `boolean`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.strip_patterns` — `tmux`

Remove output lines matching the listed patterns.

- Scope: `agent` (`tmux`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.system_prompt` — `claudecode, codex`

Replace the Agent's default system prompt for this project.

- Scope: `agent` (`claudecode, codex`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.thinking` — `pi`

Configure the pi Agent's thinking mode or level.

- Scope: `agent` (`pi`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.timeout_mins` — `antigravity, gemini, kimi`

Set the adapter process timeout in minutes; zero uses its default.

- Scope: `agent` (`antigravity, gemini, kimi`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.tool_timeout_secs` — `iflow`

Set the maximum wait for an iFlow tool call in seconds.

- Scope: `agent` (`iflow`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.agent.options.window_per_session` — `tmux`

Use a separate tmux window for every cc-connect-next session.

- Scope: `agent` (`tmux`)
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.agent.options.work_dir` — `acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder, tmux`

Set the project working directory used by the Agent.

- Scope: `agent` (`acp, antigravity, claudecode, codex, copilot, cursor, gemini, iflow, kimi, opencode, pi, qoder, tmux`)
- Type: `string`
- Default: `.`
- Takes effect: `restart`

### `projects.agent.provider_refs`

Reference shared provider names from projects agent.

- Scope: `agent`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.agent.providers.agent_model_lists.<name>.alias`

Set a short user-facing alias for projects agent providers agent_model_lists <name>.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.agent_model_lists.<name>.model`

Select the model used by projects agent providers agent_model_lists <name>.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.agent_models.<name>`

Set one named entry in projects agent providers agent_models.

- Scope: `agent`
- Type: `table`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.agent_types`

Restrict projects agent providers to the listed Agent adapter types.

- Scope: `agent`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.api_key`

Authenticate requests made by projects agent providers.

- Scope: `agent`
- Type: `string`
- Default: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.agent.providers.base_url`

Override the service base URL for projects agent providers.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.codex.env_key`

Name the environment variable from which projects agent providers codex reads its credential.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.codex.http_headers.<name>`

Set one named entry in projects agent providers codex http_headers.

- Scope: `agent`
- Type: `table`
- Default: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.agent.providers.codex.wire_api`

Select the wire protocol used by projects agent providers codex.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.endpoints.<name>`

Set one named entry in projects agent providers endpoints.

- Scope: `agent`
- Type: `table`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.env.<name>`

Set one named entry in projects agent providers env.

- Scope: `agent`
- Type: `table`
- Default: `unset`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.agent.providers.model`

Select the model used by projects agent providers.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.models.alias`

Set a short user-facing alias for projects agent providers models.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.models.model`

Select the model used by projects agent providers models.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.name`

Set the name used by projects agent providers.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.providers.thinking`

Choose the provider thinking mode used by projects agent providers.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.agent.type`

Select the Agent adapter used by this project.

- Scope: `agent`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.auto_compress.enabled`

Automatically run context compression near the configured token threshold.

- Scope: `project`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`

### `projects.auto_compress.max_tokens`

Set the estimated token threshold that triggers auto-compression.

- Scope: `project`
- Type: `integer`
- Default: `12000`
- Takes effect: `reload`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.auto_compress.min_gap_mins`

Set the minimum gap between automatic compression runs.

- Scope: `project`
- Type: `integer`
- Default: `30`
- Takes effect: `reload`

### `projects.base_dir`

Set the parent directory for dynamically created multi-workspaces.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.busy_message_mode`

Override the process-wide busy-message policy for one project.

- Scope: `project`
- Type: `string`
- Default: `inherit`
- Takes effect: `restart`
- Allowed values: `steer`, `queue`

### `projects.disabled_commands`

Disable selected built-in commands for this project.

- Scope: `project`
- Type: `string[]`
- Default: `[]`
- Takes effect: `reload`

### `projects.display.card_mode`

Choose rich Card 2.0 or legacy card rendering where supported.

- Scope: `project`
- Type: `string`
- Default: `inherit`
- Takes effect: `reload`
- Allowed values: `rich`, `legacy`

### `projects.display.hide_agent_footer`

Strip equivalent model/token/context footer lines emitted by the Agent itself.

- Scope: `project`
- Type: `boolean`
- Default: `inherit`
- Takes effect: `reload`

### `projects.display.history_max_len`

Limit each /history entry; zero disables truncation.

- Scope: `project`
- Type: `integer`
- Default: `inherit`
- Takes effect: `reload`

### `projects.display.mode`

Choose full, compact, or quiet reply presentation defaults.

- Scope: `project`
- Type: `string`
- Default: `inherit`
- Takes effect: `reload`
- Allowed values: `full`, `compact`, `quiet`

### `projects.display.reply_footer`

Show the model, reasoning effort, and elapsed-time footer on completed replies.

- Scope: `project`
- Type: `boolean`
- Default: `inherit`
- Takes effect: `reload`

### `projects.display.show_context_indicator`

Deprecated no-op retained only for old config compatibility.

- Scope: `project`
- Type: `boolean`
- Default: `inherit`
- Takes effect: `reload`
- Status: deprecated compatibility option.

### `projects.display.thinking_max_len`

Limit reasoning-progress text length; zero disables truncation.

- Scope: `project`
- Type: `integer`
- Default: `inherit`
- Takes effect: `reload`

### `projects.display.thinking_messages`

Show or hide Agent reasoning progress messages.

- Scope: `project`
- Type: `boolean`
- Default: `inherit`
- Takes effect: `reload`

### `projects.display.tool_max_len`

Limit tool-progress text length; zero disables truncation.

- Scope: `project`
- Type: `integer`
- Default: `inherit`
- Takes effect: `reload`

### `projects.display.tool_messages`

Show or hide Agent tool-progress messages.

- Scope: `project`
- Type: `boolean`
- Default: `inherit`
- Takes effect: `reload`

### `projects.filter_external_sessions`

Hide Agent sessions that were not created by cc-connect-next.

- Scope: `project`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`

### `projects.heartbeat.enabled`

Wake the main session periodically for awareness or unfinished work.

- Scope: `project`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.heartbeat.interval_mins`

Set the interval between heartbeat turns.

- Scope: `project`
- Type: `integer`
- Default: `30`
- Takes effect: `restart`

### `projects.heartbeat.only_when_idle`

Run heartbeat only while the target session is idle.

- Scope: `project`
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `projects.heartbeat.prompt`

Set the heartbeat prompt; empty reads HEARTBEAT.md from the Agent work directory.

- Scope: `project`
- Type: `string`
- Default: `HEARTBEAT.md`
- Takes effect: `restart`

### `projects.heartbeat.session_key`

Choose the chat/session that receives heartbeat work.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.heartbeat.silent`

Suppress the heartbeat start notification.

- Scope: `project`
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `projects.heartbeat.timeout_mins`

Limit one heartbeat turn in minutes.

- Scope: `project`
- Type: `integer`
- Default: `30`
- Takes effect: `restart`

### `projects.inject_sender`

Prepend platform sender identity to prompts delivered to the Agent.

- Scope: `project`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`

### `projects.mode`

Enable fixed-workspace or multi-workspace project routing.

- Scope: `project`
- Type: `string`
- Default: `fixed`
- Takes effect: `restart`
- Allowed values: ``, `multi-workspace`

### `projects.name`

Give the project a unique name used by commands, storage, and relay routing.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.observe.channel`

Choose the destination channel used by projects observe.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.observe.enabled`

Enable or disable projects observe.

- Scope: `project`
- Type: `boolean`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.platforms.options.access_token` — `matrix`

Authenticate the Matrix bot account.

- Scope: `platform` (`matrix`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.account_id` — `weixin`

Separate persistent Weixin state for multiple accounts.

- Scope: `platform` (`weixin`)
- Type: `string`
- Default: `default`
- Takes effect: `restart`

### `projects.platforms.options.agent_id` — `dingtalk, wecom`

Identify the bot application Agent in the platform tenant.

- Scope: `platform` (`dingtalk, wecom`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.allow_chat` — `feishu, lark`

Restrict Feishu access to selected chat IDs.

- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.allow_from` — `dingtalk, discord, feishu, lark, line, matrix, max, qq, qqbot, slack, telegram, webex, wecom, weibo, weixin, wps-xiezuo`

Restrict bot access to selected platform user IDs; empty or '*' allows every platform user.

- Scope: `platform` (`dingtalk, discord, feishu, lark, line, matrix, max, qq, qqbot, slack, telegram, webex, wecom, weibo, weixin, wps-xiezuo`)
- Type: `string`
- Default: `empty / allow all platform users`
- Takes effect: `restart`

### `projects.platforms.options.api_base` — `max`

Override the platform REST API base URL.

- Scope: `platform` (`max`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.api_base_url` — `wecom`

Override the platform API base URL.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.app_id` — `feishu, lark, qqbot, weibo, wps-xiezuo`

Identify the bot application.

- Scope: `platform` (`feishu, lark, qqbot, weibo, wps-xiezuo`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.app_secret` — `feishu, lark, qqbot, weibo, wps-xiezuo`

Authenticate the bot application.

- Scope: `platform` (`feishu, lark, qqbot, weibo, wps-xiezuo`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.app_token` — `slack`

Authenticate Slack Socket Mode.

- Scope: `platform` (`slack`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.auto_join` — `matrix`

Automatically join invited Matrix rooms.

- Scope: `platform` (`matrix`)
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `projects.platforms.options.auto_verify` — `matrix`

Automatically accept Matrix SAS device verification.

- Scope: `platform` (`matrix`)
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `projects.platforms.options.base_url` — `weixin, wps-xiezuo`

Override the platform service base URL.

- Scope: `platform` (`weixin, wps-xiezuo`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.bot_id` — `wecom`

Identify a WeCom WebSocket bot.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.bot_secret` — `wecom`

Authenticate a WeCom WebSocket bot.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.bot_token` — `slack`

Authenticate the Slack bot user.

- Scope: `platform` (`slack`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.burst_limit` — `weixin`

Limit separate Weixin outbound messages within one burst window.

- Scope: `platform` (`weixin`)
- Type: `integer`
- Default: `4`
- Takes effect: `restart`

### `projects.platforms.options.burst_window_secs` — `weixin`

Set the Weixin outbound burst window length in seconds.

- Scope: `platform` (`weixin`)
- Type: `integer`
- Default: `86400`
- Takes effect: `restart`

### `projects.platforms.options.callback_aes_key` — `wecom`

Decrypt encrypted WeCom callback payloads.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.callback_path` — `feishu, lark, line, wecom`

Set the inbound webhook callback path.

- Scope: `platform` (`feishu, lark, line, wecom`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.callback_token` — `wecom`

Verify WeCom callback requests.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.card_template_id` — `dingtalk`

Select the DingTalk interactive-card template ID.

- Scope: `platform` (`dingtalk`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.card_template_key` — `dingtalk`

Select the DingTalk card-template key.

- Scope: `platform` (`dingtalk`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.card_throttle_ms` — `dingtalk`

Throttle DingTalk card updates in milliseconds.

- Scope: `platform` (`dingtalk`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.cdn_base_url` — `weixin`

Override the Weixin CDN download/upload base URL.

- Scope: `platform` (`weixin`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.channel_secret` — `line`

Verify LINE webhook signatures.

- Scope: `platform` (`line`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.channel_token` — `line`

Authenticate LINE Messaging API requests.

- Scope: `platform` (`line`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.clean_reply` — `wps-xiezuo`

Strip thinking and tool-progress lines from WPS replies.

- Scope: `platform` (`wps-xiezuo`)
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.platforms.options.client_id` — `dingtalk`

Identify the DingTalk application client.

- Scope: `platform` (`dingtalk`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.client_secret` — `dingtalk`

Authenticate the DingTalk application client.

- Scope: `platform` (`dingtalk`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.corp_id` — `wecom`

Identify the WeCom enterprise.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.corp_secret` — `wecom`

Authenticate the WeCom application.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.cross_signing_password` — `matrix`

Initialize Matrix cross-signing when the server requires the account password.

- Scope: `platform` (`matrix`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.domain` — `feishu, lark`

Override the Feishu/Lark API and WebSocket domain.

- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.done_emoji` — `dingtalk, feishu, lark`

Choose the completion reaction; 'none' disables it.

- Scope: `platform` (`dingtalk, feishu, lark`)
- Type: `string`
- Default: `Done`
- Takes effect: `restart`

### `projects.platforms.options.enable_feishu_card` — `feishu, lark`

Enable Feishu interactive-card replies.

- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `projects.platforms.options.enable_markdown` — `wecom`

Enable Markdown formatting for WeCom replies.

- Scope: `platform` (`wecom`)
- Type: `boolean`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.enable_reactions` — `telegram`

Add a processing reaction to incoming messages.

- Scope: `platform` (`telegram`)
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.platforms.options.encrypt_key` — `feishu, lark`

Decrypt encrypted Feishu webhook events.

- Scope: `platform` (`feishu, lark`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.group_only` — `feishu, lark`

Accept Feishu messages only from group chats.

- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.platforms.options.group_reply_all` — `discord, feishu, lark, matrix, telegram`

Reply to every group message without requiring a mention.

- Scope: `platform` (`discord, feishu, lark, matrix, telegram`)
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.platforms.options.group_reply_all_chats` — `feishu, lark`

Enable mention-free replies only in selected Feishu chats.

- Scope: `platform` (`feishu, lark`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.group_reply_all_guilds` — `discord`

Enable mention-free replies only in selected Discord guilds.

- Scope: `platform` (`discord`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.guild_id` — `discord`

Limit Discord command registration to one guild for faster propagation.

- Scope: `platform` (`discord`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.homeserver` — `matrix`

Set the Matrix homeserver URL.

- Scope: `platform` (`matrix`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.http_url` — `qq`

Set the NapCat/QQ HTTP API endpoint.

- Scope: `platform` (`qq`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.image_batch_window_ms` — `feishu, lark`

Batch Feishu images arriving close together into one turn.

- Scope: `platform` (`feishu, lark`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.intents` — `qqbot`

Set the QQ Bot gateway intent bitmask.

- Scope: `platform` (`qqbot`)
- Type: `integer`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.long_poll_timeout_ms` — `weixin`

Set the Weixin long-poll timeout in milliseconds.

- Scope: `platform` (`weixin`)
- Type: `integer`
- Default: `35000`
- Takes effect: `restart`

### `projects.platforms.options.markdown_support` — `qqbot`

Enable QQ Bot Markdown message support.

- Scope: `platform` (`qqbot`)
- Type: `boolean`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.mention_map` — `feishu, lark`

Map Feishu mention identities to replacement text or Agent handles.

- Scope: `platform` (`feishu, lark`)
- Type: `table`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.mode` — `wecom`

Select the platform connection mode, such as WebSocket or callback.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.name` — `weibo`

Set the account display name used by the platform adapter.

- Scope: `platform` (`weibo`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.peer_bots` — `feishu, lark`

Recognize selected Feishu bot identities as relay peers.

- Scope: `platform` (`feishu, lark`)
- Type: `string[]`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.port` — `feishu, lark, line`

Set the inbound webhook listening port as a quoted string.

- Scope: `platform` (`feishu, lark, line`)
- Type: `string`
- Default: `8080`
- Takes effect: `restart`

### `projects.platforms.options.port` — `wecom`

Set the inbound webhook listening port as a quoted string.

- Scope: `platform` (`wecom`)
- Type: `string`
- Default: `8081`
- Takes effect: `restart`

### `projects.platforms.options.progress_style` — `discord, feishu, lark, telegram`

Choose how progress is rendered on the messaging platform.

- Scope: `platform` (`discord, feishu, lark, telegram`)
- Type: `string`
- Default: `compact`
- Takes effect: `restart`
- Allowed values: `legacy`, `compact`, `card`

### `projects.platforms.options.proxy` — `discord, matrix, telegram, wecom, weixin`

Route platform HTTP/WebSocket traffic through an HTTP or SOCKS5 proxy.

- Scope: `platform` (`discord, matrix, telegram, wecom, weixin`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.proxy_password` — `discord, telegram, wecom, weixin`

Authenticate to the configured platform proxy.

- Scope: `platform` (`discord, telegram, wecom, weixin`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.proxy_username` — `discord, telegram, wecom, weixin`

Set the username for platform proxy authentication.

- Scope: `platform` (`discord, telegram, wecom, weixin`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.reaction_emoji` — `dingtalk, feishu, lark`

Choose the processing reaction emoji.

- Scope: `platform` (`dingtalk, feishu, lark`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.reply_to_trigger` — `feishu, lark`

Reply in Feishu using the triggering message as the reply target.

- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.require_mention` — `feishu, lark`

Require an explicit mention before replying in group chats.

- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.resolve_mentions` — `feishu, lark`

Resolve Feishu mentions to readable names before sending text to the Agent.

- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.respond_to_at_everyone_and_here` — `discord, feishu, lark`

Treat @everyone/@here as a valid bot mention.

- Scope: `platform` (`discord, feishu, lark`)
- Type: `boolean`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.robot_code` — `dingtalk`

Identify the DingTalk robot used for outbound messages.

- Scope: `platform` (`dingtalk`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.route_tag` — `weixin`

Set the optional Weixin SKRouteTag request header.

- Scope: `platform` (`weixin`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.sandbox` — `qqbot`

Use the QQ Bot sandbox environment.

- Scope: `platform` (`qqbot`)
- Type: `boolean`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.session_scope` — `slack`

Choose whether Slack sessions are isolated by user, channel, or thread.

- Scope: `platform` (`slack`)
- Type: `string`
- Default: `user (or channel when share_session_in_channel=true)`
- Takes effect: `restart`
- Allowed values: `user`, `channel`, `thread`

### `projects.platforms.options.share_session_in_channel` — `dingtalk, discord, feishu, lark, matrix, qq, qqbot, slack, telegram`

Share one Agent session among all users in a channel or room.

- Scope: `platform` (`dingtalk, discord, feishu, lark, matrix, qq, qqbot, slack, telegram`)
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.platforms.options.state_dir` — `weixin`

Override the directory used for persistent platform state.

- Scope: `platform` (`weixin`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.thread_isolation` — `discord`

Use a separate Agent session for each platform thread or topic.

- Scope: `platform` (`discord`)
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.platforms.options.thread_isolation` — `feishu, lark`

Use a separate Agent session and workspace binding only for real Feishu/Lark topics whose events carry thread_id. Ordinary group messages and non-topic replies keep legacy per-user/channel sessions and are never promoted to topics. Omitting the key keeps the false compatibility fallback; new Starter configs and accepted recommended profiles explicitly set true.

- Scope: `platform` (`feishu, lark`)
- Type: `boolean`
- Default: `false when omitted; new Starter/recommended profile writes true`
- Takes effect: `restart`

### `projects.platforms.options.token` — `discord, max, qq, telegram, webex, weixin`

Authenticate the platform bot or gateway.

- Scope: `platform` (`discord, max, qq, telegram, webex, weixin`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.token_endpoint` — `weibo`

Override the endpoint used to obtain Weibo access tokens.

- Scope: `platform` (`weibo`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.user_id` — `matrix`

Set or override the Matrix bot user ID.

- Scope: `platform` (`matrix`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.webhook_listen` — `max`

Set the local listen address for MAX webhook delivery.

- Scope: `platform` (`max`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.webhook_path` — `max`

Set the MAX webhook URL path.

- Scope: `platform` (`max`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.webhook_resubscribe_interval` — `max`

Periodically refresh the MAX webhook subscription using a Go duration string.

- Scope: `platform` (`max`)
- Type: `string`
- Default: `5m`
- Takes effect: `restart`
- Allowed values: `Go duration string (for example: 30s, 5m, 1h)`
- Example: `webhook_resubscribe_interval = "5m"`

### `projects.platforms.options.webhook_secret` — `max`

Verify MAX webhook requests.

- Scope: `platform` (`max`)
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `projects.platforms.options.webhook_url` — `max`

Publish the externally reachable MAX webhook URL.

- Scope: `platform` (`max`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.ws_endpoint` — `weibo`

Override the platform WebSocket endpoint.

- Scope: `platform` (`weibo`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.options.ws_url` — `qq`

Set the NapCat/QQ forward WebSocket endpoint.

- Scope: `platform` (`qq`)
- Type: `string`
- Default: `unset / adapter default`
- Takes effect: `restart`

### `projects.platforms.type`

Select a messaging-platform adapter connected to this project.

- Scope: `platform`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.quiet`

Legacy per-project switch that hides thinking and tool messages when display overrides are unset.

- Scope: `project`
- Type: `boolean`
- Default: `inherit`
- Takes effect: `reload`
- Status: deprecated compatibility option.

### `projects.references.display_path`

Choose the user-facing path rendered by projects references.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.references.enclosure_style`

Choose how projects references encloses normalized references.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.references.marker_style`

Choose the marker syntax emitted by projects references.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.references.normalize_agents`

Apply projects references normalization only to the listed Agent adapters.

- Scope: `project`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.references.render_platforms`

Render projects references only on the listed messaging platforms.

- Scope: `project`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.reply_footer`

Override the reply footer for one project.

- Scope: `project`
- Type: `boolean`
- Default: `inherit`
- Takes effect: `reload`

### `projects.reset_on_idle_mins`

Rotate to a fresh session when the user returns after this idle period; zero disables it.

- Scope: `project`
- Type: `integer`
- Default: `0`
- Takes effect: `reload`

### `projects.run_as_env`

Allow the listed environment-variable names across projects user isolation.

- Scope: `project`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.run_as_user`

Run this project's Agent as another non-root OS user.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.shell`

Choose the shell used by projects.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.shell_profile`

Prepend a shell initialization command for projects.

- Scope: `project`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `projects.show_context_indicator`

Deprecated no-op retained only for old project config compatibility.

- Scope: `project`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`
- Status: deprecated compatibility option.

### `projects.show_workdir_indicator`

Deprecated no-op retained only for old project config compatibility.

- Scope: `project`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`
- Status: deprecated compatibility option.

### `projects.skip_git`

Allow multi-workspace directories that are not Git repositories.

- Scope: `project`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `projects.users.default_role`

Choose the role assigned to users not explicitly listed.

- Scope: `project`
- Type: `string`
- Default: `member`
- Takes effect: `reload`

### `projects.users.roles.<name>.disabled_commands`

Disable the listed commands for projects users roles <name>.

- Scope: `project`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.users.roles.<name>.rate_limit.max_messages`

Limit how many messages projects users roles <name> rate_limit accepts in one window.

- Scope: `project`
- Type: `integer`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.users.roles.<name>.rate_limit.window_secs`

Set the rate-limit window length in seconds for projects users roles <name> rate_limit.

- Scope: `project`
- Type: `integer`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.users.roles.<name>.user_ids`

Assign the listed platform user IDs to projects users roles <name>.

- Scope: `project`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `reload`

### `projects.workspace_idle_timeout_mins`

Deprecated project-level workspace reaper timeout; use the top-level option instead.

- Scope: `project`
- Type: `integer`
- Default: `inherit`
- Takes effect: `restart`
- Status: deprecated compatibility option.

### `projects.workspace_init_allow_local_paths`

Allow /workspace init to bind local directories in addition to Git URLs.

- Scope: `project`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `provider_presets_url`

Override the remote JSON source used for recommended provider presets.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.agent_model_lists.<name>.alias`

Set a short user-facing alias for providers agent_model_lists <name>.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.agent_model_lists.<name>.model`

Select the model used by providers agent_model_lists <name>.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.agent_models.<name>`

Set one named entry in providers agent_models.

- Scope: `global`
- Type: `table`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.agent_types`

Restrict a shared provider to selected Agent adapter types.

- Scope: `global`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.api_key`

Authenticate to a shared model provider.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `providers.base_url`

Override the shared provider API base URL.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.codex.env_key`

Name the environment variable from which providers codex reads its credential.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.codex.http_headers.<name>`

Set one named entry in providers codex http_headers.

- Scope: `global`
- Type: `table`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `providers.codex.wire_api`

Select the wire protocol used by providers codex.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.endpoints.<name>`

Set one named entry in providers endpoints.

- Scope: `global`
- Type: `table`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.env.<name>`

Set one named entry in providers env.

- Scope: `global`
- Type: `table`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `providers.model`

Select the model used by providers.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.models.alias`

Set a short user-facing alias for providers models.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.models.model`

Select the model used by providers models.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.name`

Set the name used by providers.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `providers.thinking`

Choose the provider thinking mode used by providers.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `queue.busy_message_mode`

Steer eligible input into the active turn or always preserve FIFO queueing.

- Scope: `global`
- Type: `string`
- Default: `steer`
- Takes effect: `restart`
- Allowed values: `steer`, `queue`

### `queue.max_depth`

Limit pending user messages queued behind one busy session.

- Scope: `global`
- Type: `integer`
- Default: `5`
- Takes effect: `restart`

### `quiet`

Legacy switch that hides thinking and tool messages when newer display fields are unset.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `reload`
- Status: deprecated compatibility option.

### `rate_limit.max_messages`

Limit inbound messages per user/session window; zero disables the limit.

- Scope: `global`
- Type: `integer`
- Default: `20`
- Takes effect: `restart`

### `rate_limit.window_secs`

Set the inbound rate-limit window in seconds.

- Scope: `global`
- Type: `integer`
- Default: `60`
- Takes effect: `restart`

### `relay.timeout_secs`

Limit how long cross-project relay waits for a response; zero disables waiting.

- Scope: `global`
- Type: `integer`
- Default: `120`
- Takes effect: `restart`

### `relay.visibility`

Choose how much relay activity is echoed into the group.

- Scope: `global`
- Type: `string`
- Default: `full`
- Takes effect: `restart`
- Allowed values: `full`, `summary`, `none`

### `shell`

Choose the shell used by /shell, exec cron jobs, hooks, and webhook exec.

- Scope: `global`
- Type: `string`
- Default: `sh on Unix; powershell.exe on Windows`
- Takes effect: `restart`

### `shell_profile`

Prepend an initialization command to every configured shell command.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `speech.enabled`

Transcribe incoming voice messages before sending them to the Agent.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `speech.gemini.api_key`

Authenticate requests made by speech gemini.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `speech.gemini.model`

Select the model used by speech gemini.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `speech.groq.api_key`

Authenticate requests made by speech groq.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `speech.groq.model`

Select the model used by speech groq.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `speech.language`

Set the language or locale hint used by speech.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `speech.openai.api_key`

Authenticate requests made by speech openai.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `speech.openai.base_url`

Override the service base URL for speech openai.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `speech.openai.model`

Select the model used by speech openai.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `speech.provider`

Choose the speech-to-text provider.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`
- Allowed values: `openai`, `groq`, `qwen`, `gemini`

### `speech.qwen.api_key`

Authenticate requests made by speech qwen.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `speech.qwen.base_url`

Override the service base URL for speech qwen.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `speech.qwen.model`

Select the model used by speech qwen.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `stream_preview.disabled_platforms`

Disable stream_preview on the listed messaging platforms.

- Scope: `global`
- Type: `string[]`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `stream_preview.enabled`

Update one preview message while the Agent is still streaming.

- Scope: `global`
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `stream_preview.interval_ms`

Set the minimum interval between preview updates.

- Scope: `global`
- Type: `integer`
- Default: `1500`
- Takes effect: `restart`

### `stream_preview.max_chars`

Limit the accumulated streaming-preview length.

- Scope: `global`
- Type: `integer`
- Default: `2000`
- Takes effect: `restart`

### `stream_preview.min_delta_chars`

Require this many new characters before refreshing the preview.

- Scope: `global`
- Type: `integer`
- Default: `30`
- Takes effect: `restart`

### `tts.agents.<name>.language_type`

Set the provider-specific language hint used by tts agents <name>.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.agents.<name>.max_text_len`

Skip or truncate tts agents <name> beyond this text length; zero removes the limit.

- Scope: `global`
- Type: `integer`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.agents.<name>.provider`

Select the provider used by tts agents <name>.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.agents.<name>.speed`

Set the speech-speed multiplier used by tts agents <name>.

- Scope: `global`
- Type: `number`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.agents.<name>.voice`

Choose the voice used by tts agents <name>.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.agents.<name>.voice_id`

Set the provider-specific voice ID used by tts agents <name>.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.enabled`

Enable text-to-speech replies.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `tts.language_type`

Set the provider-specific language hint used by tts.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.max_text_len`

Skip or truncate tts beyond this text length; zero removes the limit.

- Scope: `global`
- Type: `integer`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.mimo.api_key`

Authenticate requests made by tts mimo.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `tts.mimo.base_url`

Override the service base URL for tts mimo.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.mimo.model`

Select the model used by tts mimo.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.minimax.api_key`

Authenticate requests made by tts minimax.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `tts.minimax.base_url`

Override the service base URL for tts minimax.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.minimax.config_file`

Override the auxiliary configuration-file path used by tts minimax.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.minimax.model`

Select the model used by tts minimax.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.openai.api_key`

Authenticate requests made by tts openai.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `tts.openai.base_url`

Override the service base URL for tts openai.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.openai.model`

Select the model used by tts openai.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.provider`

Choose the text-to-speech provider.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`
- Allowed values: `qwen`, `openai`, `minimax`, `mimo`, `espeak`, `pico`, `edge`

### `tts.qwen.api_key`

Authenticate requests made by tts qwen.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `tts.qwen.base_url`

Override the service base URL for tts qwen.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.qwen.model`

Select the model used by tts qwen.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.speed`

Set the speech-speed multiplier used by tts.

- Scope: `global`
- Type: `number`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.tts_mode`

Choose voice-only replies or synthesize every eligible response.

- Scope: `global`
- Type: `string`
- Default: `voice_only`
- Takes effect: `restart`
- Allowed values: `voice_only`, `always`

### `tts.voice`

Choose the voice used by tts.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `tts.voice_id`

Set the provider-specific voice ID used by tts.

- Scope: `global`
- Type: `string`
- Default: `unset / runtime default`
- Takes effect: `restart`

### `update_notice`

Notify the most recently active chat once when a newer stable release is available.

- Scope: `global`
- Type: `boolean`
- Default: `true`
- Takes effect: `restart`

### `webhook.enabled`

Expose the external HTTP endpoint that triggers Agent prompts or shell commands.

- Scope: `global`
- Type: `boolean`
- Default: `false`
- Takes effect: `restart`

### `webhook.path`

Set the external webhook URL path prefix.

- Scope: `global`
- Type: `string`
- Default: `/hook`
- Takes effect: `restart`

### `webhook.port`

Set the external webhook listening port.

- Scope: `global`
- Type: `integer`
- Default: `9111`
- Takes effect: `restart`

### `webhook.token`

Authenticate webhook with a shared token.

- Scope: `global`
- Type: `string`
- Default: `unset`
- Takes effect: `restart`
- Sensitive: yes; prefer an environment-variable placeholder.

### `workspace_idle_timeout_mins`

Reap inactive multi-workspace engines after this many minutes; zero disables it.

- Scope: `global`
- Type: `integer`
- Default: `15`
- Takes effect: `restart`
