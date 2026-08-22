# Coming from official CC Connect: what's different, and how to try it with zero risk

[中文版](coming-from-cc-connect.zh-CN.md)

If you use [official CC Connect](https://github.com/chenhg5/cc-connect) and you're happy with it, you don't need to switch — it remains a fine project, and cc-connect-next is a fork of it (v1.4.1, MIT). This article is for a different reader: someone who wants a native Feishu card experience, a stricter privacy boundary, or the ability to add "one more thing" while a task is already running. Every difference below is verifiable, and the trial and rollback paths at the end are genuinely zero-risk.

## What this is, exactly

cc-connect-next is an **independent successor**, not a patch, proxy, or companion plugin:

- a single Go binary with its own command (`cc-connect-next`), data directory (`~/.cc-connect-next`), daemon service name, and npm package;
- installing, migrating, and trialing **never stops, uninstalls, or modifies** official CC Connect — the two coexist indefinitely;
- upstream is treated with respect: instead of blind merges, every upstream change is audited individually and either adopted or deliberately diverged from ([audit policy & history](upstream-v1.5.0-beta.3-audit.md)).

## Five things that are actually different

### 1. Feishu answers are one native card, not a message stream

An agent turn lives in **one** Card 2.0 card quoting your question, from start to finish: `⏳ thinking` → `⏳ calling tools` → `✍️ answering` (CardKit typewriter streaming) → `✅ done`. No message flooding, no stitched-together fragments — progress and the final answer evolve in a single place. The exact state machine and verification commands are specified in the [Feishu answer-card contract](feishu-card-contract.md).

### 2. The privacy boundary: only anonymous counts reach the chat

The card renders anonymous progress like "reasoning ×N · tools ×M" — nothing else. Reasoning text, tool names, inputs, results, model names, token counts, and working directories are dropped at **two** layers (event and render), and the card has no expandable detail panel. Your code and the agent's intermediate work never pass through Feishu's message storage.

### 3. You can steer a turn that's already running

Official CC Connect queues busy-time messages FIFO: they wait for the next turn. cc-connect-next defaults to **steer**: a new message joins the *currently running* turn via Codex's native `turn/steer`. The old card freezes grey ("continued in a newer message", keeping everything already visible) and progress continues only on the card replying to your newest message — exactly one card per turn reaches Done. Agents without steer support fall back to the queue transparently, `busy_message_mode = "queue"` restores the official behavior, and `/ps` steers explicitly in any mode.

### 4. Installs take care of themselves

`cc-connect-next update` follows the stable channel with checksum verification for both npm and standalone binaries; after a release, the daemon reminds each project's most recent chat **once** (`update_notice = false` to opt out). `doctor` checks config, agent CLI login state, platforms, dependencies, and network in one command.

### 5. When something breaks, feedback is one command

`/feedback` in the chat files the problem as a GitHub issue to the author: automatically redacted, anonymously relayed, no GitHub account needed — and the daemon points you at it whenever a turn fails.

Everything else (14 agents × 15 platforms, cron & webhooks, voice in/out, web admin, five-language i18n) is in the [main README](../README.md).

## Trying it requires no commitment

The two systems are fully isolated:

| Boundary | Official | cc-connect-next |
|---|---|---|
| Command | `cc-connect` | `cc-connect-next` |
| Data | `~/.cc-connect` | `~/.cc-connect-next` |
| macOS service | `com.cc-connect.service` | `com.cc-connect-next.service` |
| Linux service | `cc-connect.service` | `cc-connect-next.service` |
| API socket | `~/.cc-connect/run/api.sock` | `~/.cc-connect-next/run/api.sock` |

The one rule: **don't let both processes consume the same Feishu app credentials simultaneously** (two WebSocket consumers race for messages). Create a test Feishu app for the trial; the official daemon keeps running untouched.

## Why the one-command migration can be trusted

`cc-connect-next migrate` is not a copy-the-directory script. It inventories every persistent source of the official install (`config.toml`, the effective `data_dir`, each project-local `.cc-connect` directory), computes a **SHA-256 for every source file**, builds and verifies the complete result in sibling staging directories, and re-checks the source inventory immediately before activation — any added, deleted, changed, or re-permissioned file **fails the run closed** instead of activating an incomplete target. Activation is an atomic rename; pre-existing targets are preserved in timestamped backups, and the result ships with a `migration-manifest.json` recording every path and hash. If in doubt, start with:

```bash
cc-connect-next migrate --dry-run
```

It prints the full plan without writing anything. Supported official versions and settings are listed in the [migration compatibility matrix](migration-compatibility.md) (currently v1.4.1 and v1.5.0-beta.1 – beta.3); complete semantics are in the [migration guide](migration.md).

## Switching for real, and rolling back

To move production traffic, stop the official daemon, run a final synchronizing migration (bringing over sessions, bindings, and timers the official install wrote during your trial), then install the service:

```bash
cc-connect daemon stop
cc-connect-next migrate --dry-run --force
cc-connect-next migrate --force
cc-connect-next daemon install --config ~/.cc-connect-next/config.toml
```

Rollback is always two commands — the official data directory is never touched:

```bash
cc-connect-next daemon stop
cc-connect daemon start
```

## Three minutes to first run

```bash
npm install -g cc-connect-next
cc-connect-next                # writes ~/.cc-connect-next/config.toml and guides you
cc-connect-next feishu setup   # interactive Feishu app setup with the recommended profile
cc-connect-next doctor
cc-connect-next
```

## Common concerns

- **Will it touch my official install?** No. Migration never stops, uninstalls, or modifies it; the coexistence boundaries are in the table above.
- **What about future upstream releases?** Each upstream change is audited individually — adopted when valuable, recorded as a deliberate divergence otherwise. No wholesale merges.
- **I don't use Feishu — still relevant?** Steer, self-updates, `/feedback`, and `doctor` are platform-independent; the Feishu card contract is just the largest single difference.
- **License?** MIT, same as upstream, with upstream attribution preserved. Thanks to [chenhg5](https://github.com/chenhg5) and all CC Connect contributors — this project exists because of that work.
