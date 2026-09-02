# Coming from official CC Connect: what's different, and how to migrate directly

[中文版](coming-from-cc-connect.zh-CN.md)

If you use [official CC Connect](https://github.com/chenhg5/cc-connect) and you're happy with it, you don't need to switch — it remains a fine project, and cc-connect-next is a fork of it (v1.4.1, MIT). This article is for a different reader: someone who wants a native Feishu card experience, a stricter privacy boundary, or the ability to add "one more thing" while a task is already running. Every difference below is verifiable, and the direct migration path needs no second Feishu app while preserving a safe rollback source.

## What this is, exactly

cc-connect-next is an **independent successor**, not a patch, proxy, or companion plugin:

- a single Go binary with its own command (`cc-connect-next`), data directory (`~/.cc-connect-next`), daemon service name, and npm package;
- ordinary installation and copy-only migration do not modify official CC Connect; explicit production cutover stops and disables only its service while preserving binaries and data;
- upstream is treated with respect: instead of blind merges, every upstream change is audited individually and either adopted or deliberately diverged from ([audit policy & history](upstream-v1.5.0-beta.3-audit.md)).

## Five things that are actually different

### 1. Feishu answers are one native card, not a message stream

An agent turn lives in **one** Card 2.0 card quoting your question, from start to finish: `⏳ thinking` → `⏳ calling tools` → `✍️ answering` (CardKit typewriter streaming) → `✅ done`. No message flooding, no stitched-together fragments — progress and the final answer evolve in a single place. The exact state machine and verification commands are specified in the [Feishu answer-card contract](feishu-card-contract.md).

### 2. The privacy boundary: progress metadata stays minimal

During progress, the card renders only anonymous counts like "reasoning ×N · tools ×M". Reasoning text, tool names, inputs, results, model names, token counts, and working directories are dropped at **two** layers (event and render), and the card has no expandable detail panel. The final answer is still sent to and stored by Feishu as the visible answer body; if that answer quotes code or a patch, that content passes through Feishu. The privacy guarantee covers hidden intermediate progress, not the answer you asked the agent to deliver.

### 3. You can steer a turn that's already running

Official CC Connect queues busy-time messages FIFO: they wait for the next turn. cc-connect-next defaults to **steer**: a new message joins the *currently running* turn via Codex's native `turn/steer`. The old card freezes grey ("continued in a newer message", keeping everything already visible) and progress continues only on the card replying to your newest message — exactly one card per turn reaches Done. Agents without steer support fall back to the queue transparently, `busy_message_mode = "queue"` restores the official behavior, and `/ps` steers explicitly in any mode.

### 4. Installs take care of themselves

`cc-connect-next update [stable|beta]` selects an explicit channel and defaults to Stable. Standalone installs use an immutable Plan with same-release checksums, staged/installed probes, locking, backup, and rollback. The daemon follows `update_channel`, reminds the configured administrators **once** per channel/version, and asks the user to review the exact plan before confirmation. `doctor` checks config, agent CLI login state, platforms, dependencies, and network in one command.

### 5. When something breaks, feedback stays one short flow

`/feedback` renders the complete redacted Foundation draft first; a separate button or `confirm` submits that exact preview through the anonymous author relay. Cancel and missing approval make no request, no GitHub account is needed, and failed turns can prepare the same reviewable preview proactively.

Everything else (14 agents × 15 platforms, cron & webhooks, voice in/out, web admin, five-language i18n) is in the [main README](../README.en.md).

## Migration needs no second Feishu app

The two systems are fully isolated:

| Boundary | Official | cc-connect-next |
|---|---|---|
| Command | `cc-connect` | `cc-connect-next` |
| Data | `~/.cc-connect` | `~/.cc-connect-next` |
| macOS service | `com.cc-connect.service` | `com.cc-connect-next.service` |
| Linux service | `cc-connect.service` | `cc-connect-next.service` |
| API socket | `~/.cc-connect/run/api.sock` | `~/.cc-connect-next/run/api.sock` |

Default migration reuses the existing Feishu credentials. `migrate --switch` stops and disables the official daemon before starting Next, so it does not intentionally create two same-credential consumers. No test app is required. The separate runtime identities and preserved official data still provide rollback; only an advanced parallel trial needs a second app.

## Why the one-command migration can be trusted

`cc-connect-next migrate` is not a copy-the-directory script. It inventories every supported and accessible persistent source of the official install (`config.toml`, the effective `data_dir`, each discovered project-local `.cc-connect` directory), computes a **SHA-256 for every inventoried source file**, builds and verifies the result in sibling staging directories, and re-checks the source inventory immediately before activation — any added, deleted, changed, or re-permissioned inventoried file **fails the run closed** instead of activating a stale target. Optional project discovery can still be skipped when state, bindings, or project directories are unreadable; the command prints every `skipped_project_discovery` entry and records it in `migration-manifest.json`. Resolve those skips and rerun before treating project-local data as complete. Activation is an atomic rename and pre-existing targets are preserved in timestamped backups. If in doubt, start with:

```bash
cc-connect-next migrate --dry-run
```

It prints the full plan without writing anything. Supported official versions and settings are listed in the [migration compatibility matrix](migration-compatibility.md) (currently v1.4.1 and v1.5.0-beta.1 through stable v1.5.0); complete semantics are in the [migration guide](migration.md).

## One-command switch and rollback

Preview the complete plan, then run the direct cutover:

```bash
cc-connect-next migrate --dry-run
cc-connect-next migrate --switch
cc-connect-next daemon status
```

Run `--switch` outside a connected CC Agent session and with no installed Next service. It stops/disables official, final-syncs, installs/starts Next, waits for the local API and every configured platform to report Ready, then privately sends the completion message to one unique or explicit Feishu/Lark operator. Failed activation restores official only after Next is proven unregistered and stopped. For manual rollback, unregister Next before restoring official autostart:

```bash
cc-connect-next daemon uninstall
# Re-enable official autostart with launchctl/systemctl/Task Scheduler first.
cc-connect daemon start
```

## Three minutes to migrate

```bash
npm install -g cc-connect-next
cc-connect-next migrate --dry-run
cc-connect-next migrate --switch
cc-connect-next doctor
cc-connect-next daemon status
```

## Common concerns

- **Will it delete my official install?** No. `--switch` stops and disables its service, but the binary, configuration, and data remain intact for automatic failure recovery or manual rollback.
- **What about future upstream releases?** Each upstream change is audited individually — adopted when valuable, recorded as a deliberate divergence otherwise. No wholesale merges.
- **I don't use Feishu — still relevant?** Steer, self-updates, `/feedback`, and `doctor` are platform-independent; the Feishu card contract is just the largest single difference.
- **License?** MIT, same as upstream, with upstream attribution preserved. Thanks to [chenhg5](https://github.com/chenhg5) and all CC Connect contributors — this project exists because of that work.
