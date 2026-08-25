# Migrating from official CC Connect

This document is the complete, authoritative migration and coexistence guide.
It preserves every guarantee the migration command makes; the README carries
only a summary. See also the [migration compatibility matrix](migration-compatibility.md).

## One-command production migration

The normal user journey requires no second Feishu app. Preview everything read-only, then authorize one explicit production cutover:

```bash
cc-connect-next migrate --dry-run
cc-connect-next migrate --switch
cc-connect-next daemon status
```

`--switch` repeats the full preflight, stops any old Next daemon that could write the target, stops and disables official CC Connect, final-syncs against the quiet source, then installs and starts cc-connect-next with the migrated config and the official runtime's original working directory. If final sync or successor activation fails, it disarms the successor service and makes a best-effort restoration of the official service's prior running/autostart state. Official binaries and data remain intact.

Plain `cc-connect-next migrate` remains a copy-only advanced operation for backups or an intentionally configured parallel trial; it is not the default user journey.

The one-command migration covers three sources before writing anything: exactly the official `config.toml`, the effective `data_dir` (including a custom path), and every project-local `.cc-connect` directory discoverable from configured work directories, multi-workspace roots, project state, or workspace bindings. When the config file lives outside the effective data directory, no sibling file or directory beside it is inventoried. The command therefore preserves configuration, sessions, project overrides, cron/timer and heartbeat state, bindings, local provider configuration, and staged images/attachments without accidentally copying a repository, `.env`, backup tree, or service home. External Agent stores such as Codex or Claude sessions stay in place and their existing IDs remain valid.

Every source file is hashed during preflight and the complete result is built and verified in sibling staging directories. Immediately before activation, migration rebuilds the full source inventory; any added or deleted file, changed content, changed project discovery, or changed access metadata fails closed without activating an incomplete target. Existing destinations are also snapshotted before staging, revalidated after copying, checked again immediately before each promotion, and compared once more at the backup path after the atomic rename. If another cc-connect-next process creates or changes target state during the migration—even through an already-open writer at the rename boundary, and especially during a `--force` merge—the command restores and leaves that newer target untouched instead of activating a stale staged copy. Stable destinations are then activated with atomic renames. If a later destination fails, every earlier promoted tree is preserved in a unique `.failed-migration-*/preserved` recovery directory before its pre-migration backup is restored; rollback never deletes a tree that may contain post-promotion writes, and the error prints every recovery path. Every destination is canonicalized and refused if it overlaps any official source tree; the comparison uses filesystem identity, so symlinked ancestors and case-only aliases on case-insensitive volumes are rejected too. The effective global `data_dir` and project-local `.cc-connect-next` trees preserve source directory/file modes and ownership for `run_as_user` traversal; any missing target ancestors created by migration inherit the corresponding source root's traversal access and ownership, while pre-existing ancestors are never modified. The rewritten `config.toml` remains a generated `0600` file. Runtime-only logs, sockets, locks, restart notifications, and daemon metadata are excluded; source symlinks are skipped. Existing non-empty targets are refused by default. With an explicit `--force`, merging is deliberate; every previous target that existed, including an empty one, remains available as a timestamped `*.pre-migration-*` backup and is recorded in the report and manifest. The result includes `migration-manifest.json` with every source, destination, size, and SHA-256. Use `--skip-project-data` only when project-local images and attachments are intentionally not wanted.

During copy-only migration, the official instance may remain installed and running. If it keeps writing persistent data during the migration window, the command asks you to rerun during a quieter moment instead of silently omitting new files. Production `--switch` stops it before the final scan and sync.

Custom locations are supported:

```bash
cc-connect-next migrate \
  --source /path/to/official-data \
  --target /path/to/next-data \
	  --source-version v1.5.0 \
  --dry-run
```

The current matrix covers the known persistent layout of official v1.4.1 and v1.5.0-beta.1 through stable v1.5.0. Default `--source-version auto` does not execute the binary recorded in daemon metadata; it validates the actual TOML schema, normal startup semantics, registered Agent/platform set, and persistent inventory. An exact known release can be recorded explicitly. A missing Agent/platform, invalid display mode, unsupported setting, or plugin whose behavior cannot be preserved fails before target writes; see the [migration compatibility matrix](migration-compatibility.md).

Relative `data_dir`, `work_dir`, and `base_dir` values are resolved from the official daemon's recorded working directory when available. Migration reads that metadata from the official `$HOME/.cc-connect/daemon.json` even when `--source` points to a separate config directory; a same-named file beside an arbitrary config is not trusted. A malformed metadata file or a recorded working directory that is missing or inaccessible fails preflight instead of silently resolving relative paths against a different directory. If `data_dir` is omitted, migration likewise uses `$HOME/.cc-connect` even when `--source` is a custom config root such as `/etc/cc-connect`; only that root's `config.toml` is copied. If daemon metadata is stale or the official instance was only run manually, pass `--runtime-work-dir /absolute/original/cwd` explicitly; this override has highest priority.

For safety, migration refuses an effective `data_dir` that contains the official configuration root (for example, `data_dir = "~"`). A separate custom `data_dir` is also inventoried only through the persistent paths owned by the supported official releases: sessions, project state/model caches, cron/timer state, bindings, heartbeat/history state, MiniMax local config, Weixin state, Agent prompt files, and Matrix encryption state. Any unexpected regular file or directory makes preflight fail, even when the configuration root lives elsewhere, instead of recursively copying a service home, SSH keys, browser profiles, or unrelated datasets. Point the official installation at a dedicated data directory and verify its state before rerunning migration; the command will never silently create a partial target for this case.

Configuration paths use the same `${NAME}` placeholder syntax as official CC Connect. Every referenced variable must also be present in the migration process; an unset variable fails closed instead of being replaced with an empty string that could select the wrong directory. A configured `data_dir` that has not been created yet is treated as empty, so the valid configuration file still migrates. If optional project data cannot be read, or project state/binding metadata is malformed, the global migration continues and still copies that metadata verbatim; every skipped discovery source is printed and recorded in `migration-manifest.json`. Grant access or repair the metadata, then rerun before treating project-local migration as complete.

## Coexistence and switching

Official CC Connect and cc-connect-next can be installed side by side:

| Boundary | Official | cc-connect-next |
|---|---|---|
| Command | `cc-connect` | `cc-connect-next` |
| Data | `~/.cc-connect` | `~/.cc-connect-next` |
| macOS service | `com.cc-connect.service` | `com.cc-connect-next.service` |
| Linux service | `cc-connect.service` | `cc-connect-next.service` |
| API socket | `~/.cc-connect/run/api.sock` | `~/.cc-connect-next/run/api.sock` |

Do not run both against the same Feishu app credentials at the same time: two WebSocket consumers can race or duplicate handling. Default `--switch` stops and disables the official daemon before starting Next with the migrated credentials, so no test app is needed. Only an advanced parallel trial needs separate credentials.

This rule is enforced by the product, not just documented:

- **Startup guard.** `cc-connect-next` refuses to start while an official daemon is running with a platform credential the loaded config also uses, and names the exact stop/disarm commands. A merely *armed* autostart (service registered with `RunAtLoad`/`enabled`, daemon not currently running) logs a prominent warning instead, because the conflict is one reboot away. `CC_NEXT_ALLOW_OFFICIAL_CONFLICT=1` overrides the refusal (not recommended).
- **Migration guidance.** After a copy-only migration, the report inspects the official binary, autostart registration, live daemon, and credential overlap, and makes direct `migrate --switch` the default next step; parallel trial is advanced.
- **`doctor`.** A dedicated *official CC Connect coexistence* section reports binary, autostart, daemon, and shared-credential state; a running daemon with shared credentials is a failure, an armed autostart is a warning. Credential values are redacted.
- **`migrate --switch`.** One command performs a first-time cutover: it rejects an installed Next service before touching official, then stops/disables official, final-syncs, installs/starts Next, and directly sends one private completion message to a unique or explicit operator. `--switch` cannot be combined with `--dry-run`.

No test instance or manual config/work-dir assembly is required. After the dry run succeeds:

```bash
cc-connect-next migrate --switch
cc-connect-next daemon status
```

Run it from a terminal outside the official daemon lifecycle. When `CC_SESSION_KEY` is present, the command refuses before any service mutation.

Repeat every custom path option from the dry run. With one uniquely configured Feishu/Lark operator, the CLI privately sends the completion message; otherwise pass `--notify-project` and `--notify-user`. It never searches recent chats or broadcasts to a group.

For rollback, unregister the Next service, re-enable official autostart, then start official CC Connect:

```bash
cc-connect-next daemon uninstall
# macOS: launchctl enable gui/$(id -u)/com.cc-connect.service
# Linux user: systemctl --user enable cc-connect.service
# Linux system: sudo systemctl enable cc-connect.service
# Windows: Enable-ScheduledTask -TaskName cc-connect
cc-connect daemon start
```

The official data directory remains untouched.

## Agent-readable install task

Paste this into a coding agent:

```text
	Install the published stable cc-connect-next from https://github.com/timmyagentic/cc-connect-next.
	Verify OS/architecture and whether cc-connect is running; never uninstall, overwrite,
	or delete official CC Connect. Run `cc-connect-next migrate --dry-run` and report the
	full plan, skips, old-path references, and target. When it passes, run
	`cc-connect-next migrate --switch`. Do not create a second Feishu app or manually run
	two same-credential services in parallel. Verify the command reports official stopped
	and disabled, a readable final manifest/backups, and cc-connect-next installed and
	Running with exact config/work-dir and whether the private completion message was sent or skipped. If `CC_SESSION_KEY` is present, hand the command to an external terminal instead of running the cutover in that Agent session.
```
