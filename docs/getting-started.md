# First installation of cc-connect-next

[中文](getting-started.zh-CN.md) · [README](../README.en.md) · [Installation reference](../INSTALL.md)

For users without official CC Connect, an old configuration, or a bot. Existing official users should follow the [migration guide](migration.md).

## Agent-readable install task

Paste this into a coding agent with local terminal access. The agent performs installation, runs commands, and edits the configuration.

```text
Install the published stable cc-connect-next from https://github.com/timmyagentic/cc-connect-next
for a first-time Feishu/Lark setup. Verify OS/architecture, the agent CLI and its login state,
and the absolute project directory; prefer my current agent/project and ask only for missing
information. Preserve existing CC Connect/Next configurations and services; do not migrate or overwrite.
Choose an explicit config path. If it does not exist, run `cc-connect-next --config <config-path>`
to create it, then fill in the real work_dir, agent type, and its options yourself. Keep one project name.
Run `cc-connect-next feishu setup --config <config-path> --project <project-name>`, show the QR code,
and keep the process waiting while I scan it. Accept the recommended settings and confirm the official
lark-cli companion; use `--recommended --lark-cli` for non-interactive execution, preserve existing
profiles, and make the same bot the default bot profile.
Run `cc-connect-next doctor --config <config-path>` and start in the foreground. Ask me to send
/version, /whoami, and an ordinary prompt in a private chat; verify replies and configure access
using the actual user ID. Stop the foreground process you started, then run
`cc-connect-next daemon install --config <config-path>`. Check Service and Runtime/Platforms in
`cc-connect-next daemon status` and verify replies still work in the background. Report version,
config path, work_dir, and verification results. Call on me for login, QR scanning, or app publication;
mark any incomplete real-message verification UNVERIFIED.
```

## Essential notes

- The agent replaces `<config-path>` and `<project-name>` with real values and uses them consistently for initialization, setup, doctor, and daemon. Inspect existing configuration before resuming; do not regenerate it or create another unresolved starter project.
- The installing agent prepares a missing agent CLI; you complete official account sign-in. Ask for the project only when it is unclear, and do not use a temporary directory as the project.
- Keep setup running until scanning completes; `--qr-image` can expose the QR code. Keep temporary login data out of repositories. QR registration does not establish publication, permissions, or availability; see the [Feishu guide](feishu.md).
- lark-cli reuses the same bot and preserves old profiles. Do not perform user `auth login` or open a second `event consume` connection. For basic chat only, `--no-lark-cli` skips the companion.
- `/version` verifies messaging; an ordinary prompt verifies agent execution. Doctor success alone is not real-message verification. Let the user send messages requiring their identity, and report incomplete checks truthfully.
- See [INSTALL.md](../INSTALL.md) for macOS/Linux/Windows installation methods, standalone binaries, updates, and migration. See the [usage guide](usage.md) for everyday commands.
