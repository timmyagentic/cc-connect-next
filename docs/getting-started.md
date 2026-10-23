# First-time setup

[中文](getting-started.zh-CN.md) · [README](../README.en.md) · [Installation and maintenance reference](../INSTALL.md)

This guide starts with **no official CC Connect installation, no old configuration, and no Feishu bot**. It uses Codex + Feishu/Lark to reach a real agent reply, then a background service. Existing official users should follow the [migration guide](migration.md).

The sequence is **prepare the agent → install Next → select a directory → create a bot → verify a reply → run a service**. Each step has a separate success check.

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
