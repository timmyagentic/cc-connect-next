# CC Connect Feishu Plus

[![CI](https://github.com/timmyagentic/cc-connect-feishu-plus/actions/workflows/ci.yml/badge.svg)](https://github.com/timmyagentic/cc-connect-feishu-plus/actions/workflows/ci.yml)
![Status](https://img.shields.io/badge/status-foundation-orange)
![npm](https://img.shields.io/badge/npm-publication%20gated-lightgrey)

[中文](./README.zh-CN.md)

An independently maintained, compatibility-first CC Connect distribution for
better Feishu/Lark support. It keeps the native adapter, configuration, and
session data, then adds opt-in `plus_*` capabilities without depending on
upstream pull requests.

## Why this distribution exists

The native Feishu integration is useful and should remain available. Replacing
it with a second bot or proxy would create competing connections, duplicate
events, and a harder migration path. Feishu Plus therefore enhances the
existing adapter in place:

- Plus features are compiled into the compatible CC Connect binary.
- Existing `~/.cc-connect` configuration and session data remain in use.
- Every deep behavior change is behind an explicit `plus_*` option.
- With Plus disabled, the native Feishu path keeps its original defaults.
- Update checks and release downloads stay on this repository so an update
  cannot silently remove Plus features.

## Foundation feature

### Recovering fail-closed bot identity

The adapter needs its bot `open_id` before it can tell whether a group message
actually mentions the bot. The native path continues after a lookup failure
with mention filtering unavailable, which can admit unrelated group traffic
until the process is restarted.

Enable the first Plus feature in the existing Feishu platform options:

```toml
[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "your-feishu-app-id"
app_secret = "your-feishu-app-secret"
plus_enabled = true
plus_identity_mode = "retry"
```

Available modes:

| Mode | Behavior while bot identity is unavailable |
| --- | --- |
| `retry` | Default. Block protected group traffic, keep direct messages available, retry with capped exponential backoff, and recover without a restart. |
| `fail_closed` | Block protected group traffic until restart; direct messages remain available. |
| `legacy` | Preserve the native fail-open behavior for compatibility. |

This protection currently applies to WebSocket mode. Existing webhook behavior
is unchanged. See the [Feishu Plus guide](./docs/feishu-plus.md) for the exact
compatibility boundary.

## npm bootstrap

The planned public entrypoint is:

```bash
npx cc-connect-feishu-plus@latest install
```

The foundation package is intentionally private and non-mutating while signed
release assets and automatic rollback are still being built. From a checkout,
you can already inspect a native installation and preview the installation
plan:

```bash
node npm/cli.js doctor
node npm/cli.js doctor --json
node npm/cli.js install --dry-run
node npm/cli.js install --dry-run --json
```

These commands do not download, replace, stop, or restart the current service.
The doctor reports paths, binary version, file permissions, and Plus state
without returning configuration contents or credentials.

## Status and release gate

The `0.1.0` foundation includes:

- the first independently testable Feishu Plus behavior;
- separate binary identity and a repository-pinned update source;
- a read-only npm doctor and installation planner;
- regression tests for native compatibility, fail-closed behavior, retry
  recovery, update-source isolation, and non-mutating bootstrap behavior.

Public npm and modified-binary publication remain gated on:

- confirmed upstream license and attribution requirements;
- signed manifests and SHA-256 verification;
- immutable version directories with atomic activation;
- service/config backup, post-activation health checks, and automatic rollback.

See [the pinned upstream baseline](./docs/upstream-baseline.md).

## Development

Requirements: Go 1.25, Node.js 18 or newer, and pnpm 10 for the web UI.

```bash
corepack pnpm@10.28.2 --dir web install --frozen-lockfile
corepack pnpm@10.28.2 --dir web build
go test ./...
npm run check --prefix npm
```

The Go module path remains `github.com/chenhg5/cc-connect` for source
compatibility with the pinned baseline. The distribution identity, update
source, release process, and maintenance live in this repository.

## Upstream

Feishu Plus currently tracks the explicit source baseline
[`chenhg5/cc-connect@3fc360e`](https://github.com/chenhg5/cc-connect/tree/3fc360ee6acc9bab13ab1b48ddde3af44062903b).
Its original documentation and full Git history remain available there and in
this fork's history. Future baseline imports are reviewed rather than followed
automatically.
