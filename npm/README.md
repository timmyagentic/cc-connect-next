# cc-connect-next

Use local coding agents such as Codex and Claude Code from Feishu/Lark, Telegram, Slack, Discord, and other chat platforms.

**New user? Start with the [first-time setup guide](https://github.com/timmyagentic/cc-connect-next/blob/main/docs/getting-started.md) / [从零安装教程](https://github.com/timmyagentic/cc-connect-next/blob/main/docs/getting-started.zh-CN.md).** No original CC Connect installation or existing bot is needed.

## Install with one prompt (recommended)

Copy this into a coding agent with local terminal access, such as Codex, Claude Code, or Cursor. It handles dependency installation, configuration, bot setup, and the background service; you complete the required login, QR scan, and verification messages.

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

Follow the [agent execution guide](https://github.com/timmyagentic/cc-connect-next/blob/main/docs/getting-started.md#execution-guide-for-the-installing-agent) / [中文执行流程](https://github.com/timmyagentic/cc-connect-next/blob/main/docs/getting-started.zh-CN.md#给安装-agent-的执行流程) for details.

## Manual installation

If you prefer to run the steps yourself, install Node.js LTS and then:

```bash
npm install -g cc-connect-next
cc-connect-next --version
```

Supported targets are macOS, Linux, and Windows on x64 or ARM64. The installer downloads the same-version native GitHub Release archive and verifies `checksums.txt` before extraction. Both npm and GitHub Release downloads must be reachable. The npm launcher needs Node.js after installation too.

Continue with the [manual first-time guide](https://github.com/timmyagentic/cc-connect-next/blob/main/docs/getting-started.md#manual-installation): prepare/authenticate an agent, generate the configuration and set the actual working directory, create a bot, verify `/version` and a normal reply, then install the background service. Installation alone does not configure a working bot.

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
