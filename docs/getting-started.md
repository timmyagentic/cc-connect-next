# First-time setup

[中文](getting-started.zh-CN.md) · [README](../README.en.md) · [Installation and maintenance reference](../INSTALL.md)

This guide starts with **no official CC Connect installation, no old configuration, and no Feishu bot**. It uses Codex + Feishu/Lark to reach a real agent reply, then a background service. Existing official users should follow the [migration guide](migration.md).

The sequence is **prepare the agent → install Next → select a directory → create a bot → verify a reply → run a service**. Each step has a separate success check.

## Install with one prompt

Copy the whole block into a coding agent with **local terminal access**, such as Codex, Claude Code, or Cursor. You do not need to install Next or edit its configuration first. The agent handles setup and calls on you for sign-in, QR scanning, app publication, and verification messages.

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

The prompt describes the full task without requiring an existing Next installation. The installing agent should follow the [execution guide](#execution-guide-for-the-installing-agent). For individual commands, use the [manual steps](#manual-installation).

## Execution guide for the installing agent

**Execute the installation, configuration edits, and checks.** The request already includes first-time setup and a background service. Complete steps that can be determined from context and local inspection. Keep progress when user participation is needed, then continue after their response.

### A. Identify the target

Inspect OS, architecture, account, Node/npm, available agent CLIs, the project directory, and any CC Connect/Next installation or service. Prefer the user's current working agent and the project clearly associated with their task. Do not treat a temporary folder, home directory, or tooling control directory as the project. Ask for the agent and directory together only when intent is unclear.

When no agent is available, prepare Codex unless the user selected another agent. Install the chosen CLI and guide its official sign-in. Account access, paid access, and organization permissions remain user actions; keep authentication inside the official login flow.

If configuration already exists, inspect its structure and resume completed steps. Preserve existing installations. A first-time request does not authorize migrating, overwriting, or replacing an existing official/Next service: explain the current state and resolve the target before changing an existing installation.

### B. Install stable and verify the executable

Use the published stable release described in [INSTALL.md](../INSTALL.md#published-release). For npm, install current Node.js LTS and set up PATH if needed; alternatively, install a standalone binary verified against the same release's `checksums.txt`. Use the OS account that authenticated the selected agent.

Run `cc-connect-next --version` and the relevant `--help`. Record source and version. Follow the installed release's flags; newer main-branch documentation is not a reason to switch to Beta or build from source.

### C. Generate and fill in the configuration yourself

Choose an explicit absolute configuration path and use it consistently for startup, setup, doctor, and daemon commands. If it does not exist, run Next once as shown in [configuration creation](#3-create-and-edit-the-configuration); its first-run exit is expected.

Then **edit the generated TOML yourself**: set the confirmed real `work_dir` and agent type, use one consistent project name, and remove options belonging to another agent. Honor the user's confirmed permission preference; otherwise use this guide's first-trial mode (`suggest` for Codex, `default` for Claude Code). Do not ask the user to find the file, replace paths, or assemble TOML. Preserve unrelated existing settings.

Before bot binding, check parsing, directory existence, and agent CLI startup. Missing Feishu credentials mean bot setup is still pending.

### D. Create the bot and present the QR code

A new user has no App ID/App Secret. Use `feishu setup` to create a bot, with the configuration path and project name from C. The starter already has card styling; use `--no-recommended --no-lark-cli` for the first chat and add optional extensions later as needed.

Start setup in a terminal session that can remain running. Use `--qr-image` to save an image in a private temporary location when needed, show the QR code or session login link, and **keep setup waiting while the user scans it**. Keep QR images, credentials, and temporary login links out of repositories and delivery logs. Read the result and confirm credentials were saved. After a timeout, inspect completed steps before retrying so you do not create duplicate apps.

If QR registration is unavailable, use the [manual app reference](feishu.md). Continue doing the local configuration yourself, handing back only the developer-console actions the user must perform. Verify bot capability, long-connection message events, card callbacks, publication, and app availability. Use an app available only to this user for initial verification.

### E. Verify messaging and the agent, then set user access

Run doctor and fix required configuration, directory, or agent failures. Assess optional warnings such as voice dependencies against actual needs. Start Next in the foreground, inspect the real connection, and keep it running for verification. Doctor success or a live process does not establish working chat.

Ask the user to send `/version`, `/whoami`, and “Briefly describe this project without changing files” in a private chat with the new bot. These messages need the real user's identity. Inspect accessible replies or wait for the user's report; do not invent successful verification. **Write the access configuration yourself** using the app-scoped user ID returned by `/whoami`: project `admin_from`, platform `allow_from`, and `group_reply_all = false` for the initial group setup. Preserve an already explicit user-selected access scope.

If `/version` fails, investigate messaging, permissions, and connectivity. If an ordinary prompt fails, investigate agent authentication, model access, and the working directory. Perform deterministic repairs yourself.

### F. Start the service and report the result

After the first reply succeeds, stop the foreground process you started for this installation. Follow [background setup](#6-run-in-the-background) under the same OS account, explicitly passing the configuration path and preserving the original runtime directory when relative paths are used. Leave one consumer for the configuration.

Check both Service and Runtime/Platforms in `daemon status`, inspect logs, and verify another real reply after service startup. Investigate persistent readiness failures instead of repeatedly reinstalling or using `--force` to finish.

Report the version/source, configuration path, agent/project directory, messaging result, service status, and status/log/stop commands. If login, QR registration, publication, or a real reply is still pending, state the exact remaining user action and mark it **UNVERIFIED**. File existence and a passing doctor are not complete installation evidence.

## Manual installation

The following sections provide the commands and configuration reference for the agent workflow, or for users who prefer manual setup.

## 1. Prepare the host and agent

You need:

- An awake, connected macOS, Linux, or Windows host on x64 or ARM64.
- [Current Node.js LTS](https://nodejs.org/en/download), including npm. The npm launcher needs Node.js; a standalone binary removes Next's own Node.js dependency.
- A Feishu or Lark account and mobile app, with permission to create, publish, and use an app in your organization.
- An existing project directory and an installed, authenticated agent CLI. Next does not provide model access or sign in to the agent for you.

Check these in the same terminal and OS account that will run Next:

```bash
node --version
npm --version
```

The starter uses Codex. Follow the [official Codex CLI documentation](https://learn.chatgpt.com/docs/codex/cli), or install via npm:

```bash
npm install -g @openai/codex
codex --version
```

Enter the project directory you want to operate remotely. Run `codex`, complete sign-in, and ask it to describe the directory without changing files. Exit Codex after it succeeds. A working desktop app does not establish that the terminal CLI is installed or authenticated.

For Claude Code, follow its [official setup instructions](https://code.claude.com/docs/en/overview) and verify `claude` can answer in the same terminal. Step 3 explains the template changes. Follow each agent's documentation for other choices.

**Success check:** the agent independently answers in the intended directory. Resolve login, model access, and quota errors there first.

## 2. Install Next

Install the stable release:

```bash
npm install -g cc-connect-next
cc-connect-next --version
```

You do not need Go or a repository clone. The installer downloads the native binary from the matching GitHub Release and verifies `checksums.txt`, so both npm and GitHub Release downloads must be reachable.

If Windows PowerShell blocks `npm.ps1`, use `npm.cmd`; the Next launcher is also available as `cc-connect-next.cmd`. For WSL, install Node.js, the agent, Next, configuration, and service in that same WSL environment. Keep Windows and Linux paths separate.

**Success check:** `--version` prints `cc-connect-next v…`. This does not mean a bot is configured yet.

See [INSTALL.md](../INSTALL.md) for Beta, standalone binaries, and source builds. Start with stable. The main branch's documentation may include capabilities newer than stable; consult the installed version's `--help`.

## 3. Create and edit the configuration

Run once with an explicit path so a current-directory `config.toml` cannot select a different configuration.

macOS / Linux:

```bash
cc-connect-next --config "$HOME/.cc-connect-next/config.toml"
```

Windows PowerShell:

```powershell
cc-connect-next --config "$env:USERPROFILE\.cc-connect-next\config.toml"
```

The first run prints `Created default config at …` and exits. Open that file in a text editor. Edit the **existing sections**; do not append a second copy of this example.

Keep `name = "my-project"` under `[[projects]]` and configure the agent:

```toml
[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/absolute/path/to/your/project"
mode = "suggest"
backend = "app_server"
app_server_url = "stdio"
```

- Replace `work_dir` with an existing absolute project path. On macOS/Linux, run `pwd` in that directory.
- For Windows, TOML single quotes avoid backslash escaping: for example, `work_dir = 'C:\Users\you\Projects\demo'`. Use your actual `(Get-Location).Path` value.
- The generated starter uses `mode = "yolo"`, with no approval prompts or sandbox restrictions. For the first trial, the example uses `suggest`: Codex app-server starts with a read-only sandbox and supports interactive approvals. Choose other [permission modes](usage.md#permission-modes) when you want automatic edits.
- Leave the bot's `app_id` and `app_secret` placeholders for step 4.

For Claude Code, change `type` to `claudecode`, change `mode` to `default`, and remove the Codex-only `backend` and `app_server_url` lines. Keep the real `work_dir`.

**Success check:** the directory exists and the agent matches the one you prepared. The configuration will contain credentials; keep it out of version control. `feishu setup` does not update an existing project's `work_dir`.

## 4. Create a Feishu / Lark bot

macOS / Linux:

```bash
cc-connect-next feishu setup --config "$HOME/.cc-connect-next/config.toml" --project my-project --no-recommended --no-lark-cli
```

Windows PowerShell:

```powershell
cc-connect-next feishu setup --config "$env:USERPROFILE\.cc-connect-next\config.toml" --project my-project --no-recommended --no-lark-cli
```

Scan the terminal QR code with the Feishu/Lark mobile app and follow its registration steps. The command saves credentials to your configuration. If you renamed the project, use that exact name with `--project`; a different name creates another project and leaves the starter's unresolved placeholders behind.

The starter already includes the recommended card appearance. `--no-recommended` skips reapplying it; `--no-lark-cli` skips the optional companion, which is unnecessary for the first chat. You can add [lark-cli later](feishu.md#可选把同一个机器人设为-lark-cli-默认-profile) when you need agent access to Feishu APIs.

Registration may provision permissions and events. Still verify in [Feishu Open Platform](https://open.feishu.cn/) or [Lark Developer](https://open.larksuite.com/) that:

- Bot capability is enabled, the app is published, and its availability includes your account.
- `im.message.receive_v1` uses a long connection, with private-message receive and bot-message send permissions.
- Interactive buttons have a `card.action.trigger` callback through the long connection.

**No QR code, failed registration, or insufficient organization permissions:** use the [manual app guide](feishu.md#第一步创建飞书企业自建应用), then enter App ID and App Secret in the existing `[projects.platforms.options]`. For Lark, use platform `type = "lark"`. Keep the real App Secret out of issues, chats, and command examples.

**Success check:** setup reports a configured bot; the two credential placeholders are gone, and the app is available to you. Messaging has not been tested yet.

## 5. Verify the first reply in the foreground

macOS / Linux:

```bash
cc-connect-next doctor --config "$HOME/.cc-connect-next/config.toml"
cc-connect-next --config "$HOME/.cc-connect-next/config.toml"
```

Windows PowerShell:

```powershell
cc-connect-next doctor --config "$env:USERPROFILE\.cc-connect-next\config.toml"
cc-connect-next --config "$env:USERPROFILE\.cc-connect-next\config.toml"
```

Resolve failed doctor checks before starting; assess warnings against the capabilities you need. Doctor checks configuration, the agent, dependencies, and networking without opening a platform connection. It cannot prove a reply was delivered.

Keep the terminal open. Search for your bot in Feishu/Lark, open a private chat, and send:

1. `/version` — a version reply proves messages can enter and leave Next.
2. `/whoami` — obtain your user ID in this bot's app scope.
3. “Briefly describe this project without changing files.” — a completed answer checks the agent, directory, authentication, and model access.

Begin with an app available only to you and a private chat. Use the **user ID** returned by `/whoami` to configure access and administration:

Add the administrator after `name` in the existing `[[projects]]` section:

```toml
[[projects]]
name = "my-project"
admin_from = "ou_your_user_id"
```

Then edit the project's existing `[projects.platforms.options]` section. Keep the saved credentials and add or change just these two options:

```toml
[projects.platforms.options]
# Keep the existing app_id, app_secret, and other settings here
allow_from = "ou_your_user_id"
group_reply_all = false
```

Replace both `ou_your_user_id` values with the actual `/whoami` result. Do not add duplicate table headers: `admin_from` belongs to the project and `allow_from` belongs to the platform. The starter answers every group message by default; the settings above require an @mention in your first group. See the [Feishu guide](feishu.md) for more users and `allow_chat` group restrictions.

After editing, stop with `Ctrl+C` and restart using the same command. Check `/version` and a normal prompt again. Empty `admin_from` allows ordinary chat but leaves privileged commands such as `/upgrade`, `/restart`, and `/shell` unavailable.

**Success check:** both `/version` and a normal agent prompt succeed. A running process or a passing doctor alone is insufficient.

## 6. Run in the background

Use the same OS account that authenticated the agent. Stop the foreground process with `Ctrl+C`, then install the service.

macOS / Linux:

```bash
cc-connect-next daemon install --config "$HOME/.cc-connect-next/config.toml"
```

Windows PowerShell:

```powershell
cc-connect-next daemon install --config "$env:USERPROFILE\.cc-connect-next\config.toml"
```

Installation also starts the service. Inspect it:

```bash
cc-connect-next daemon status
cc-connect-next daemon logs -f
```

Check that the service is running and Runtime/Platforms are ready, then send another ordinary prompt. `Ctrl+C` exits log following without stopping the service. macOS uses launchd; Linux uses systemd, with linger needed to keep a user service alive after the final login ends; Windows uses Task Scheduler. Sleep, loss of connectivity, or shutdown interrupts the bot.

For relative configuration paths, pass `--work-dir` with the original runtime directory when installing. This guide uses an absolute project path. Run one instance per configuration; do not start a foreground instance alongside the service or hide the conflict with `--force`.

**Success check:** the bot still replies after the foreground terminal closes, consistent with daemon status.

## Troubleshooting

| Symptom | Next step |
|---|---|
| `node` or `npm` is missing | Install Node.js LTS, open a new terminal, and check versions |
| Global npm install reports `EACCES` | Use a user-writable Node.js installation/prefix; see [npm installation](../INSTALL.md#published-release) |
| `cc-connect-next` is missing after installation | Check the command directory for `npm prefix -g` is on PATH; reopen the terminal; try the `.cmd` launcher on Windows |
| Download or checksum failure | Check npm and GitHub Release connectivity; preserve checksum verification, including for manual downloads |
| Setup says the configuration does not exist | Complete step 3 and use the same `--config` for setup and startup |
| Startup reports a `REPLACE` placeholder | Fix the exact key reported; check `work_dir` and whether setup accidentally created a differently named project |
| `/version` receives no reply | Check publication, app availability, message scopes, long-connection events, `allow_from`, duplicate services, and logs |
| `/version` works but an agent prompt fails | Verify agent login, directory, model access, and quota under the same OS account; run doctor |
| Card buttons time out | Check `card.action.trigger` and app publication; see [callback configuration](feishu.md#53-选择回调配置) |
| Foreground works but service fails | Inspect Binary, Config, WorkDir, PATH, and the agent's OS account; see [INSTALL.md](../INSTALL.md) |

Continue with the [usage guide](usage.md). For updates, service control, and uninstalling, see [INSTALL.md](../INSTALL.md). Bug reports should include OS, installation source, version, and redacted errors, without uploading the entire configuration or credentials.
