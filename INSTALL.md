# cc-connect-next installation and maintenance

**First time here? [Copy one install prompt](docs/getting-started.md#install-with-one-prompt) / [复制一段安装 Prompt](docs/getting-started.zh-CN.md#交给-agent-一键安装)** and let your coding agent perform setup. You do not need official CC Connect, an existing bot, or a hand-edited Next configuration.

This page is the reference for installation methods, configuration, services, and updates. Official users can follow the [migration guide](docs/migration.md). cc-connect-next can be installed beside official CC Connect, but the two runtimes must not connect to the same Feishu app at the same time.

| Starting point | Path |
|---|---|
| New installation | [One-prompt installation](#install-with-a-coding-agent) · [Agent execution guide](docs/getting-started.md#execution-guide-for-the-installing-agent) |
| Existing official CC Connect | [Migration and rollback](docs/migration.md) |
| Existing Next | [Update](#update-a-published-installation), [service control](#background-service-and-uninstall) |
| Alternative installer | [Standalone binary](#github-release-asset) or [source build](#current-source) |

Repository: <https://github.com/timmyagentic/cc-connect-next>

## Install with a coding agent

Paste this into an agent that can run commands on the target machine:

```text
Install the published stable cc-connect-next from
https://github.com/timmyagentic/cc-connect-next for my first use with Feishu/Lark. Prefer my
current working coding agent and project directory. Execute dependency checks and installation,
generate and fill in the configuration including the real work_dir, guide me through QR bot
registration, verify the first reply, and install and start the background service. Ask me only
when the agent or directory is unclear, or when I must sign in, scan a QR code, confirm app
publication, or send a verification message. Identify and preserve existing configurations and
services; do not migrate or overwrite them. Finish by reporting the version, config path,
agent/project directory, messaging verification, and background status, explicitly marking
anything unverified.
```

The [agent execution guide](docs/getting-started.md#execution-guide-for-the-installing-agent) covers resuming setup, filling the configuration, presenting QR codes while keeping setup alive, verifying replies, and starting the service. The sections below are the command reference; the agent should execute them rather than hand them back as a to-do list.

## 1. Requirements and existing installations

For npm installation, install [current Node.js LTS](https://nodejs.org/en/download) and check `node --version` and `npm --version`. No Go toolchain or repository clone is required. The npm launcher continues to use Node.js after installation; standalone binaries do not.

Install and authenticate an agent separately in the same OS account that will run Next. The starter uses Codex, not Claude Code; see [agent preparation](docs/getting-started.md#1-prepare-the-host-and-agent). A working chat bot requires an awake host, model access, and platform app permissions.

If you already have an installation, inspect it first. The following block is for macOS/Linux:

```bash
uname -s
uname -m
command -v cc-connect || true
command -v cc-connect-next || true
cc-connect --version 2>/dev/null || true
cc-connect-next --version 2>/dev/null || true
```

Do not modify official CC Connect during inspection, installation, or copy-only migration. The explicit `migrate --switch` cutover is the sole exception and preserves official binaries/data.

## 2. Install

### Published release

```bash
npm install -g cc-connect-next
cc-connect-next --version
```

For a deliberate Beta installation, use `npm install -g cc-connect-next@beta`.

If npm reports `EACCES`, use a user-writable Node.js installation or global prefix. On macOS/Linux, npm puts command links in `<prefix>/bin`; on Windows, it puts command launchers directly in `<prefix>`. Inspect `npm prefix -g`, add that command directory to PATH using your shell's normal configuration, and reopen the terminal. Do not switch to a different OS account just to install Next: that account will not inherit your agent login.

Windows PowerShell can use `npm.cmd` and `cc-connect-next.cmd` if execution policy blocks the `.ps1` launchers.

The npm installer downloads the same-version native asset from the GitHub release. Supported targets are macOS, Linux, and Windows on amd64 or arm64.

### Update a published installation

```bash
cc-connect-next update
```

`update` defaults to the Stable channel and accepts an explicit Beta opt-in: `cc-connect-next update [stable|beta]` or `--channel`. It detects whether the running binary belongs to a global npm package or is a standalone executable. npm installs are updated in their existing global prefix to the exact selected version; standalone installs verify the same-release archive against `checksums.txt`, stage and probe the binary, lock the target, and retain rollback protection before replacement. Restart a running daemon after the command completes:

```bash
cc-connect-next daemon restart
```

Prerelease updates remain explicit through `npm install -g cc-connect-next@beta`; `cc-connect-next update --pre` and `--beta` are intentionally rejected.

**How you learn about new releases.** A running daemon checks `update_channel` (`stable` by default, or explicit `beta`) shortly after startup and then daily. When exactly one configured platform supports direct-user messaging, it privately notifies all explicitly listed `admin_from` users. Recent groups, topics, and ordinary sessions are never reminder targets; empty, wildcard, or ambiguous targets stay silent because they cannot identify a fixed private recipient safely. A pass records the project/channel/version only after every administrator succeeds; after a partial failure, the next pass sends to the full explicit list again. From chat, `/upgrade [stable|beta]` shows the exact channel, release type, and artifact, and a later `/upgrade confirm` applies that immutable Plan. Set `update_notice = false` to disable reminders. Pull-based checks remain available with `/upgrade [stable|beta]` or `cc-connect-next check-update [stable|beta]`.

### Current source

When testing an unreleased commit, use the Go version required by [go.mod](go.mod) or newer, current Node.js LTS, Git, and Make. These are developer prerequisites:

```bash
git clone https://github.com/timmyagentic/cc-connect-next.git
cd cc-connect-next
make build
./cc-connect-next --version
```

`make build` builds the embedded web UI before compiling the native binary. For a backend-only development build:

```bash
make build-noweb
```

### GitHub release asset

Release archives use these names:

```text
cc-connect-next-v<VERSION>-darwin-amd64.tar.gz
cc-connect-next-v<VERSION>-darwin-arm64.tar.gz
cc-connect-next-v<VERSION>-linux-amd64.tar.gz
cc-connect-next-v<VERSION>-linux-arm64.tar.gz
cc-connect-next-v<VERSION>-windows-amd64.zip
cc-connect-next-v<VERSION>-windows-arm64.zip
```

Download the exact asset shown on the [release page](https://github.com/timmyagentic/cc-connect-next/releases), verify it against `checksums.txt`, extract it, and place the binary on `PATH`.

Homebrew is not currently a supported cc-connect-next installation method.

### macOS permissions after an update

Release builds use a stable Developer ID Application identity for both Mac
architectures. Moving from an older ad hoc build to the first Developer ID build
may require you to grant the needed permissions once again. Later signed builds
must satisfy the previous build's designated requirement; the release gate checks
the extracted binaries as well as archive checksums. npm and the built-in updater
install the same signed release bytes.

macOS may attribute an Agent child process's access to the background
cc-connect-next service. The Agent receives the effective state directory and
instructions to keep searches within relevant locations. Grant only the access
your task needs. Do not reset the TCC database or grant unrelated directory access
to work around an identity mismatch. See the [macOS signing runbook](docs/macos-signing.md)
for release setup and the separate real-device upgrade acceptance check.

从旧的 ad hoc 版本首次切换到 Developer ID 签名版本时，可能需要重新授权一次。
后续版本保持相同产品签名身份；Agent 搜索使用实际数据目录，找不到文件时不应自动
扩大到整个用户目录。不要通过清空 TCC 或授予无关目录权限处理签名失配。

## 3. Create or migrate configuration

### New installation

Follow the [first-time guide](docs/getting-started.md) for the full sequence. Configuration resolution is `--config` → `./config.toml` → `~/.cc-connect-next/config.toml`, so use an explicit path for a predictable first installation.

On macOS/Linux:

```bash
cc-connect-next --config "$HOME/.cc-connect-next/config.toml"
```

On Windows PowerShell:

```powershell
cc-connect-next --config "$env:USERPROFILE\.cc-connect-next\config.toml"
```

When that file does not exist, the command creates it and exits without connecting a bot. The file is created with mode `0600` on Unix; Windows access is governed by the user's profile ACLs. Current source also creates a missing configuration directory with mode `0700`, including when the instance lock creates it first. Existing directory permissions are preserved; older releases may create it with mode `0755`.

If you prefer automatic path discovery, run:

```bash
cc-connect-next
```

The generated file already carries the [recommended Feishu configuration](#the-recommended-feishu-configuration) — it is rendered from the same definition `feishu setup` applies, so the two cannot drift apart. What it leaves to you is marked `REPLACE`: the working directory and the Feishu credentials. Do not commit the configuration.

Startup refuses to run while any `REPLACE` value is still in place, and names both the key and the step that resolves it. That refusal is deliberate: with placeholder credentials the process would otherwise print `platform ready`, `engine started` and `cc-connect-next is running`, and only then fail to connect.

Before configuring Feishu, edit the existing `[projects.agent.options]` section: set `work_dir` to an existing absolute project directory and choose the agent and permission mode. The starter uses Codex app-server with `mode = "yolo"`; first-time users can choose `suggest` for a read-only starting sandbox and interactive approvals. For Claude Code, use `type = "claudecode"`, `mode = "default"`, and remove `backend` / `app_server_url`.

For Feishu, `cc-connect-next feishu setup` fills the credentials for you, offers
the recommended configuration, and then offers the official `lark-cli`
companion — see [section 4](#4-configure-feishu). In unattended setup, add
`--lark-cli` explicitly; otherwise no external CLI is installed or configured.

### Migrate official CC Connect

The normal user journey is a direct production cutover. It reuses the existing
platform credentials; you do not need to create a second Feishu app. Start with
a read-only dry run, then authorize the switch explicitly:

```bash
cc-connect-next migrate --dry-run
cc-connect-next migrate --switch
```

After a real migration, an interactive terminal also offers to install and bind
the official `lark-cli` with the migrated bot. For unattended migration, make
that write explicit:

```bash
cc-connect-next migrate --switch --lark-cli
# When different bots exist:
cc-connect-next migrate --switch --lark-cli --lark-cli-project my-project
```

`--dry-run --lark-cli` only reports what a real migration would do; it never
installs `lark-cli` or changes a profile.

`--switch` must run outside a connected CC Agent session and rejects an installed
Next service before touching official. It stops and disables official CC Connect, performs the
final migration against a quiet source, then installs and starts cc-connect-next
with the migrated config and the detected official runtime work directory. The
cutover waits for the local API and every configured platform to report Ready;
only then does it privately notify one unique or explicit Feishu/Lark operator.
If final sync or activation/readiness fails, official is restored only after
Next is unregistered, its runtime socket no longer answers, and the migrated
config lock is free. Otherwise official stays disabled to prevent duplicate
consumers. Official binaries and data are never deleted.

Plain `cc-connect-next migrate` remains a copy-only advanced operation. It does
not switch traffic or start a daemon and is useful for backups or a deliberately
configured parallel trial.

The default `--source-version auto` validates the actual configuration and
persistent layout without executing any binary recorded by the official daemon.
The current compatibility matrix covers official v1.4.1 and v1.5.0-beta.1
through stable v1.5.0. When provenance is known, record it explicitly on both the dry
run and real migration, for example:

```bash
cc-connect-next migrate --source-version v1.5.0 --dry-run
cc-connect-next migrate --source-version v1.5.0 --switch
```

Unknown configuration fields and unavailable Agent/platform implementations
fail before target writes instead of being silently discarded. See the
[migration compatibility matrix](docs/migration-compatibility.md) for the exact
version-specific boundary.

The defaults are:

```text
source: ~/.cc-connect
target: ~/.cc-connect-next
```

The command reads exactly the official `config.toml`, inventories the effective `data_dir` (including a custom location), and inventories project-local `.cc-connect` directories referenced by configured work directories, multi-workspace roots, project state, or workspace bindings. If the config file is separate from the effective data directory, its sibling files and directories are never inventoried. It copies persistent configuration, sessions, project overrides, cron/timer/heartbeat state, bindings, local provider configuration, and project-local images/attachments without copying a surrounding repository, `.env`, backup tree, or service home. Agent-native stores such as Codex and Claude sessions remain in their original locations, so their existing IDs stay valid.

Before activation it hashes every source, builds the complete result in sibling staging directories, verifies the staged files, and checks the sources again. It activates each destination with an atomic rename and rolls back earlier activations if a later one fails, then writes `migration-manifest.json` with source, target, size, and SHA-256 records. Source/target overlap checks use filesystem identity, rejecting symlinked ancestors and case-only aliases on case-insensitive volumes. Global `data_dir` and project-local access modes and ownership are preserved so `run_as_user` agents retain traversal access. Missing custom-target ancestors created by migration inherit the corresponding source root's access and owner; existing ancestors are not modified. The rewritten config remains `0600`. Logs, sockets, locks, restart notifications, daemon metadata, and source symlinks are excluded. A non-empty target is rejected unless `--force` is explicit; `--switch` supplies that explicit final merge and preserves the previous target as a timestamped `*.pre-migration-*` backup. Use `--skip-project-data` only to deliberately omit project-local images and attachments. Copy-only migration never stops or modifies official CC Connect; `--switch` changes only its service running/autostart state and keeps its binary and data intact.

For custom locations:

```bash
cc-connect-next migrate \
  --source /absolute/path/to/official-data \
  --target /absolute/path/to/next-data \
  --dry-run
```

Relative `data_dir`, `work_dir`, and `base_dir` values are resolved from the official daemon's recorded working directory when available. That metadata is read from the official `$HOME/.cc-connect/daemon.json` even when `--source` is a separate config directory; a same-named config sibling is not trusted. Malformed metadata, or a recorded working directory that is missing or inaccessible, fails preflight instead of silently changing the base for relative paths. An omitted `data_dir` still means `$HOME/.cc-connect`, and only the custom config root's `config.toml` is copied. A separate custom `data_dir` is accepted only when every regular path matches known CC Connect persistent state; unexpected files or directories fail preflight instead of being copied from a broad service home. If daemon metadata is stale or the official process was launched manually from another directory, add `--runtime-work-dir /absolute/original/cwd`; the explicit override wins.

Configuration paths follow official CC Connect's `${NAME}` placeholder semantics. A configured `data_dir` that does not exist yet is treated as empty, so the valid configuration file still migrates. Unreadable optional project data or malformed project state/binding metadata does not discard the global migration, and the metadata file itself is still copied verbatim; each skipped discovery source is printed and recorded in `migration-manifest.json`. Grant access or repair the metadata, then rerun before treating project-local migration as complete.

## 4. Configure Feishu

Create the configuration first, and use its existing project name. To create a new bot by QR code:

```bash
cc-connect-next feishu setup --project my-project
```

Pass the same `--config` used at startup when using a custom or explicit path. Setup does not install/authenticate the agent or replace an existing project's `work_dir`. A new `--project` name creates an additional project; it does not rename the starter.

For an existing app, enter its credentials in the existing configuration or follow the [Feishu binding guide](docs/feishu.md). `lark-cli` is optional; it is not required to chat with the bot.

After the credentials are saved, setup prints the recommended Feishu
configuration and asks whether to apply it. It then offers the official
`lark-cli` companion. Answer `--recommended` / `--no-recommended` and
`--lark-cli` / `--no-lark-cli` on the command line to decide in advance; with
neither pair and a non-interactive stdin, neither optional write is performed.

### Reuse the same bot in the official lark-cli

The companion uses the bot already selected by cc-connect-next. It creates an
isolated profile such as `cc-connect-next-my-project`, makes it the default, and
sets its default identity to `bot`. A profile for the same App is reused. A
same-name profile for another App is preserved and a collision-free name is
created. The previous default profile and every existing user OAuth login remain
available; `lark-cli profile use -` switches back.

The companion passes the App Secret to
`lark-cli profile add --app-secret-stdin`, never through a child argv, inherited
environment value, cc-connect-next output, or temporary handoff file. lark-cli
then persists it in the profile it owns.
The companion does not run `auth login`, grant scopes, or send a test message.
Bot operations use the permissions already published for that Feishu App; user
calendar, Drive, mail, and other personal resources still require a separate,
explicit user OAuth flow.

Run or repair the companion independently at any time:

```bash
cc-connect-next lark-cli setup --project my-project
```

If several different Feishu/Lark bots are configured, `--project` is required;
use `--platform-index` when that project itself has more than one. Do not run
`lark-cli event consume` with this same profile while cc-connect-next is
running: normal HTTP API commands can share the App, but a second event
connection can compete for the bot's events.

### The recommended Feishu configuration

This is the shape the project is operated with day to day, not a theoretical ideal: a quoted answer card that carries the final answer plus a compact status footer, file references rendered so they can be clicked, and a bot that participates in its group without being @mentioned every time.

```toml
[projects.display]
card_mode = "rich"               # Feishu Card 2.0 answer card: counts only, never reasoning text
thinking_messages = false        # keep reasoning out of the chat
tool_messages = false            # keep tool calls and their arguments out of the chat
show_context_indicator = false   # deprecated no-op, kept for config compatibility
reply_footer = true              # show the compact "model · effort · elapsed" footer
hide_agent_footer = true         # strip the equivalent lines the agent emits itself

[projects.references]
normalize_agents = ["codex"]     # the project's own agent; "all" when its syntax is not known
render_platforms = ["feishu"]
display_path = "smart"
marker_style = "emoji"
enclosure_style = "code"

[projects.platforms.options]
enable_feishu_card = true        # interactive card client instead of plain messages
reply_to_trigger = true          # reply in a quote of the triggering message
thread_isolation = "topics_only" # isolate real Feishu topics; use topic_per_message to auto-topic every group message
done_emoji = "Done"              # react when a turn finishes, so the chat pushes a notification
group_reply_all = true           # answer every group message without an @mention
```

Most of these already match the built-in defaults. The profile still spells them out, so a configuration you accepted keeps producing the same chat surface when a future release changes what an unset key means.

Two things it deliberately does not touch, because they are specific to your installation rather than to Feishu: credentials, and the `allow_from` / `allow_chat` scope. **Set that scope before relying on `group_reply_all`** — without it the bot answers every message in every group it belongs to:

```toml
[projects.platforms.options]
allow_from = "ou_your_open_id"   # or "*"
allow_chat = "oc_your_chat_id"   # or "*"
```

The card appears immediately, shows only anonymous reasoning/tool counts, streams the answer in the same quoted card, and ends with a localized completion label (`✅ Done` in English, `✅ 已完成` in Chinese) or a localized generic failure label. Reasoning, tool details, token/context metadata, and working directories are omitted; the default footer adds only model, effort, and elapsed time and can be disabled with `reply_footer = false`.

See the [Feishu answer-card contract](docs/feishu-card-contract.md) for the exact lifecycle, privacy boundary, fallback behavior, locale coverage, and executable verification commands.

Set `card_mode = "legacy"` only when intentionally opting back into inherited CC Connect rendering.

## 5. Validate installation

### First installation

Use the same config path for both commands; this block uses the default discovery path:

```bash
cc-connect-next doctor
cc-connect-next
```

Keep the foreground process running. Send `/version` to the bot in a private chat, then an ordinary prompt such as “Describe this project without changing files.” The command reply checks messaging; the ordinary reply checks agent execution. `doctor` never opens a platform connection and cannot replace this acceptance step. Configure user access and administrators as described in the [first-time guide](docs/getting-started.md#5-verify-the-first-reply-in-the-foreground).

### Migration only

Validate the source and environment before switching production:

```bash
cc-connect-next --version
cc-connect-next migrate --dry-run
```

The dry run uses the same config, plugin, path, permissions, persistent-state,
source-identity, and target checks as the real switch without changing either
service. Review every skipped discovery and source-reference warning before
continuing.

After a successful `--switch`, run:

```bash
cc-connect-next daemon status
cc-connect-next doctor
```

`--switch` already waits for the service manager to report cc-connect-next as
installed and running. `doctor` checks the migrated configuration, Agent CLI and
login state, every configured platform, local dependencies, network reachability,
and official-service coexistence. It never opens a platform connection, so one
real message remains the final client-visible acceptance.

If a platform cannot connect after startup, the process stays up and retries, and says so 30 seconds in:

```text
level=ERROR msg="platform startup incomplete" detail="my-project/feishu (1000040346: app_id is invalid) cannot deliver messages ..."
```

No separate Feishu app is needed for the normal migration. The official daemon
is stopped and disabled before Next starts with the migrated credentials, so
there is never an intentional two-consumer window. A separate app is required
only for an advanced parallel trial using copy-only migration.

Expected isolated identities:

| Boundary | Official CC Connect | cc-connect-next |
|---|---|---|
| Command | `cc-connect` | `cc-connect-next` |
| Data | `~/.cc-connect` | `~/.cc-connect-next` |
| macOS launchd | `com.cc-connect.service` | `com.cc-connect-next.service` |
| Linux systemd | `cc-connect.service` | `cc-connect-next.service` |
| API socket | `~/.cc-connect/run/api.sock` | `~/.cc-connect-next/run/api.sock` |

## 6. Direct production switch

Run the one explicit cutover command after the dry run succeeds. Repeat any
custom `--source`, `--target`, `--source-version`, or `--runtime-work-dir`
options from the dry run:

```bash
cc-connect-next migrate --switch
cc-connect-next daemon status
```

The command requires no installed Next service, then stops/disables official,
final-syncs, installs/starts Next, waits for the local API plus every configured
platform to report Ready, and directly sends the private completion message.
`daemon status` shows Service separately from Runtime/Platforms. Use
`--notify-project` and `--notify-user` when the operator is not unique.
Notification failure does not roll back a successful cutover.

Rollback leaves official data intact:

```bash
cc-connect-next daemon uninstall
# Re-enable official autostart for your service manager:
# macOS: launchctl enable gui/$(id -u)/com.cc-connect.service
# Linux user: systemctl --user enable cc-connect.service
# Linux system: sudo systemctl enable cc-connect.service
# Windows: Enable-ScheduledTask -TaskName cc-connect
cc-connect daemon start
```

## Background service and uninstall

After a real foreground reply succeeds, stop the foreground process with `Ctrl+C`. Install under the OS account where the agent is authenticated. `daemon install` also starts the service.

macOS/Linux:

```bash
cc-connect-next daemon install --config "$HOME/.cc-connect-next/config.toml"
```

Windows PowerShell:

```powershell
cc-connect-next daemon install --config "$env:USERPROFILE\.cc-connect-next\config.toml"
```

Use the actual path for custom configurations. If any configuration paths are relative, pass `--work-dir` with the original runtime working directory as well. Check both service state and Runtime/Platforms readiness:

```bash
cc-connect-next daemon status
cc-connect-next daemon logs -f
```

Send another ordinary prompt after installing the service. If foreground worked but the service does not, compare Binary, Config, WorkDir, PATH, and OS account with the working terminal. Linux user services may require linger to survive the last logout; follow the install command's message. Windows uses Task Scheduler and macOS uses launchd. Keep the host awake and connected.

Stop or remove the service with:

```bash
cc-connect-next daemon stop
cc-connect-next daemon uninstall
```

For an npm installation, remove the package only after removing its service:

```bash
npm uninstall -g cc-connect-next
```

For a standalone installation, remove the binary you placed on PATH. Configuration, credentials, and history remain in the data directory; back them up and delete them separately only if you intend to discard that data. Uninstalling Next does not uninstall the agent or revoke the chat app.

## Agent completion checklist

For a **new installation**, report:

- OS/architecture, exact version, and installation source;
- the actual config path, chosen agent, and working directory;
- successful independent agent login/use, or **UNVERIFIED**;
- doctor failures/warnings, bot publication and access scope;
- a real `/version` reply and ordinary agent reply, separately;
- foreground shutdown, service/runtime readiness, and a reply after service installation if requested;
- any remaining steps as **UNVERIFIED**, including real platform checks that were not performed.

For a **migration**, additionally report:

- exact cc-connect-next version and installation source;
- target OS and architecture;
- `~/.cc-connect-next` and config permission checks;
- dry-run migration result and whether `--switch` was authorized;
- confirmation that `--switch` ran outside a connected CC Agent session and no Next service was already installed;
- migration manifest path and every timestamped pre-migration backup;
- confirmation that the official daemon was stopped and disabled before the final sync, and that Next was installed and reported running afterward;
- the reported official runtime work directory when the config contains relative paths;
- confirmation that official binaries/data were preserved and the old service was disabled;
- independent command, data directory, service, and API socket names;
- confirmation that the normal cutover reused the migrated platform credentials without a parallel consumer;
- whether the private completion message was sent or why the target/send was skipped;
- any validation that remains unverified.
