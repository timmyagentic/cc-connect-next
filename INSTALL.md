# CC Connect Feishu Plus installation

> [!IMPORTANT]
> The `0.1.0` foundation is not yet distributed as an npm package or modified
> binary. The commands below inspect and build the project; they do not replace
> a running native CC Connect service. Public installation remains gated on
> signed assets, automatic rollback, and license confirmation.

中文说明见 [README.zh-CN.md](./README.zh-CN.md)。

## Inspect an existing native installation

From a repository checkout:

```bash
node npm/cli.js doctor
node npm/cli.js doctor --json
```

The doctor looks for the existing `~/.cc-connect/config.toml`, the current
binary, and known service metadata. It reports only paths, version, permissions,
and whether Plus is enabled; it does not print configuration contents or
credentials.

Preview the future migration without downloading, writing, stopping, or
restarting:

```bash
node npm/cli.js install --dry-run
node npm/cli.js install --dry-run --json
```

Calling `install` without `--dry-run` fails closed and makes no changes in
the foundation release.

## Build from source for development

Requirements:

- Go 1.25;
- Node.js 18 or newer;
- pnpm 10 for the embedded web UI.

```bash
git clone https://github.com/timmyagentic/cc-connect-feishu-plus.git
cd cc-connect-feishu-plus

corepack pnpm@10.28.2 --dir web install --frozen-lockfile
corepack pnpm@10.28.2 --dir web build
go build -o cc-connect ./cmd/cc-connect
./cc-connect --version
```

The locally built executable identifies itself as `cc-connect-feishu-plus`.
Do not replace a production service until you have an explicit backup and
rollback procedure.

## Enable the current Plus capability

Add these options to the existing Feishu/Lark platform entry:

```toml
[projects.platforms.options]
plus_enabled = true
plus_identity_mode = "retry"
```

This preserves the native configuration and session directory. The default
`retry` mode blocks protected group messages while bot identity is unknown,
keeps direct messages available, and restores group mention filtering
automatically after identity lookup recovers.

See [docs/feishu-plus.md](./docs/feishu-plus.md) for all modes and compatibility
details.

## Native CC Connect setup reference

Agent and platform setup outside the Plus-specific behavior remains compatible
with the pinned upstream baseline. Until this repository has a complete
independent setup guide, consult the
[upstream installation guide at the pinned commit](https://github.com/chenhg5/cc-connect/blob/3fc360ee6acc9bab13ab1b48ddde3af44062903b/INSTALL.md).
Those upstream npm packages and release assets install the native distribution,
not Feishu Plus.
