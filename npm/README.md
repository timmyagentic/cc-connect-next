# cc-connect-next

Use local coding agents such as Codex and Claude Code from Feishu/Lark, Telegram, Slack, Discord, and other chat platforms.

**New user? Start with the [first-time setup guide](https://github.com/timmyagentic/cc-connect-next/blob/main/docs/getting-started.md) / [从零安装教程](https://github.com/timmyagentic/cc-connect-next/blob/main/docs/getting-started.zh-CN.md).** No original CC Connect installation or existing bot is needed.

## Install

Install Node.js LTS first, then install the stable release:

```bash
npm install -g cc-connect-next
cc-connect-next --version
```

Supported targets are macOS, Linux, and Windows on x64 or ARM64. The installer downloads the same-version native GitHub Release archive and verifies `checksums.txt` before extraction. Both npm and GitHub Release downloads must be reachable. The npm launcher needs Node.js after installation too.

## First reply

1. Install and authenticate your agent CLI separately. The starter uses Codex; first verify `codex` can answer in the intended project directory.
2. Run `cc-connect-next` once. It creates a starter configuration and exits. Open the printed path, set `work_dir` to your real absolute project directory, and choose an agent permission mode.
3. Run `cc-connect-next feishu setup --project my-project --no-recommended --no-lark-cli`. Scan the QR code with Feishu/Lark. The optional lark-cli companion is not needed for chatting.
4. Run `cc-connect-next doctor`, resolve failures, then run `cc-connect-next` in the foreground.
5. Send `/version` to the bot, followed by a normal prompt. Only the latter verifies agent execution. Once both work, stop the foreground process and follow the guide to install the background service.

Use the same `--config` path throughout if you selected one. Setup fills bot credentials, not the agent login or an existing project's working directory. The full guide includes Windows commands, user access settings, alternative agents, and troubleshooting.

## Update

Update an existing npm or standalone installation to stable:

```bash
cc-connect-next update
```

To install a public Beta explicitly:

```bash
npm install -g cc-connect-next@beta
```

See the [installation reference](https://github.com/timmyagentic/cc-connect-next/blob/main/INSTALL.md) for restarting services, standalone binaries, and uninstalling. Existing official users should use the [migration guide](https://github.com/timmyagentic/cc-connect-next/blob/main/docs/migration.md).
