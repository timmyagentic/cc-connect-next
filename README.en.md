<div align="center">

<img src="docs/images/banner-next.svg" alt="cc-connect-next" width="720">

**The Feishu-native remote for the AI coding agents on your machine — plus Telegram, Slack, Discord and 11 more platforms.**

A privacy-first successor to [CC Connect](https://github.com/chenhg5/cc-connect) with a native Feishu Card 2.0 answer lifecycle, mid-turn steering, and an auditable one-command migration.

[![Release](https://img.shields.io/github/v/release/timmyagentic/cc-connect-next?color=0284c7&label=release)](https://github.com/timmyagentic/cc-connect-next/releases/latest)
[![npm](https://img.shields.io/npm/v/cc-connect-next?color=cb3837&logo=npm&logoColor=white)](https://www.npmjs.com/package/cc-connect-next)
[![License: MIT](https://img.shields.io/github/license/timmyagentic/cc-connect-next?color=blue)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](go.mod)
[![Agents](https://img.shields.io/badge/agents-14-7c3aed)](#-supported-agents--platforms)
[![Platforms](https://img.shields.io/badge/platforms-15-0ea5e9)](#-supported-agents--platforms)

[中文文档](README.md) · [Install](INSTALL.md) · [Usage guide](docs/usage.md) · [Feishu setup](docs/feishu.md) · [Migration](docs/migration.md)

<img src="docs/images/turn-demo.svg" alt="One agent turn in Feishu: message, streaming card lifecycle, mid-turn steer handoff, exactly one Done card" width="820">

</div>

---

Run Claude Code, Codex, Cursor, or any of 14 coding agents on your own machine — then drive them from the chat app already in your pocket. cc-connect-next is a single Go binary: no MCP server, no proxy, no companion plugin, and no modification to official CC Connect. It owns its command, data directory, daemon, and npm package, and coexists with an official install side by side.

## ✨ Highlights

- 🔒 **Privacy-first Feishu Card 2.0 lifecycle** — one agent turn stays in one quoted native card: `⏳ thinking` → `⏳ calling tools` → `✍️ answering` (CardKit typewriter streaming) → `✅ done`. Only anonymous progress counts are ever rendered; reasoning text, tool names, inputs, results, model, token, and workdir details are dropped at **two** layers and the card has no expandable panel.
- 🎛️ **Steer the running turn** — messages that arrive while the agent is busy join the task **already running** (the default) via Codex's native `turn/steer`, with the live card handing over to the newest message; agents without the capability fall back to the FIFO queue transparently, and `busy_message_mode = "queue"` restores queue-always. `/ps` steers explicitly on any mode.
- 🚚 **Auditable one-command migration** — `cc-connect-next migrate` inventories the official install, hashes every source file, stages, verifies, and activates atomically with timestamped backups and a full SHA-256 manifest. It fails closed rather than ever activating an incomplete target.
- 🔔 **Self-maintaining installs** — `cc-connect-next update` follows the stable channel with checksum verification for both npm and standalone binaries, and a running daemon reminds each project's most recent chat **once per new release** (`update_notice = false` to opt out).
- 🤖 **14 agents × 15 platforms** — one process hosts multiple projects, each binding a code directory to its own agent and platforms, with per-project permissions, providers, models, and display settings.
- 🌍 **Production niceties** — `doctor` diagnostics, launchd/systemd/Windows daemon, web admin (beta), cron & webhooks, bot-to-bot relay, voice in/out (STT/TTS), multi-workspace routing, and full i18n in five languages (en, zh, zh-TW, ja, es).

- **One-command feedback.** Hit a bug or a missing capability? `/feedback` reports it straight to the author as a GitHub issue — redacted automatically, no GitHub account needed, and the daemon points you at it whenever a turn fails.

## 🎬 What it looks like

<div align="center">
<img src="docs/images/card-lifecycle.svg" alt="Card lifecycle and steer handoff" width="820">
</div>

When a supplement arrives mid-turn in steer mode (or via `/ps`), the input joins the **same** agent turn — no new turn, no concurrent process. The previous card freezes in a neutral grey *"Continued in a newer message"* state that keeps everything already visible, and all further progress plus the final answer render only in the card replying to the newest message. Exactly one card reaches Done per turn.

## 🥇 Feishu as a first-class platform

Most bridges port a Telegram bot to Feishu and stop at "it sends messages". cc-connect-next is built the other way around: Feishu is the reference platform, the integration is specified by an executable [answer-card contract](docs/feishu-card-contract.md), and it exercises the platform's advanced surface end to end. If your team lives in Feishu, this is the difference you feel daily:

- **Answer cards, engineered** — CardKit typewriter streaming with monotonic sequencing (a delayed frame can never overwrite newer content) and a ≥900 ms answering dwell so even one-shot answers visibly animate. CardKit unavailable? Automatic fallback to in-place card updates. Tables beyond Feishu's component budget render as fenced text in the same card instead of overflowing into extra messages; a failed terminal update falls back to one tracked replacement card — never untracked multi-part replies. `NO_REPLY` recalls the optimistic card; recalling your question deletes the card and keeps partial output out of assistant history; `done_emoji` reacts to your message only after a visible successful answer.
- **Mentions that actually notify** — `@DisplayName` in answers resolves to native Feishu at tags (lazy member fetch, 1-hour cache, longest-name-first matching; `mention_map` targets other bots with validated `ou_` IDs). Because at tags inside cards display but never notify, a final answer containing a resolved mention is delivered as a tracked quoted text message that genuinely notifies — then the lifecycle card is removed.
- **Topics and groups done right** — `thread_isolation` gives every topic its own workspace binding, and the first @ in an existing topic backfills the thread history as context, in order. Per-chat no-@ allowlists (`group_reply_all_chats`) ship with the exact `im:message.group_msg` permission boundary documented. Quoted-file downloads are double-gated: the bot must be explicitly @-ed *and* the quoted file's uploader must be the requester. Agent-relay prompts stay inside the topic they started in.
- **Operations that stay out of your way** — WebSocket long connection (no public IP, domain, or certificate) with automatic reconnection; permission confirmations and provider/model switching as interactive cards; remote markdown images uploaded once and reused by URL with a one-minute failure backoff, plus multi-image batching; `cc-connect-next feishu setup` interactive onboarding with a recommended app profile; per-turn locale snapshots across five languages.

Every item above is a documented config key in the [Feishu guide](docs/feishu.md) or a tested guarantee in the [card contract](docs/feishu-card-contract.md) — not marketing copy.

## 🚀 Quick start

```bash
# 1. Install (macOS / Linux / Windows, amd64 & arm64)
npm install -g cc-connect-next

# 2. Create the starter config and fill in the REPLACE values
cc-connect-next            # writes ~/.cc-connect-next/config.toml, then guides you
cc-connect-next feishu setup   # interactive Feishu app setup with the recommended profile

# 3. Verify and run
cc-connect-next doctor
cc-connect-next
```

Startup refuses to run while any `REPLACE` placeholder remains, naming the key and the step that resolves it — instead of reporting healthy and then failing to connect. `doctor` checks config, agent CLI login state, platforms, dependencies, and network without opening a platform connection.

Install as a service once things work:

```bash
cc-connect-next daemon install --config ~/.cc-connect-next/config.toml
```

See [INSTALL.md](INSTALL.md) for standalone binaries, building from source, and update details.

## 🎛️ Queue vs steer

Two intents exist for a message that arrives while the agent is busy — and they are different features:

| | `steer` (default) | `queue` |
|---|---|---|
| Meaning | "incorporate this correction into the task **already running**" | "finish this task, then handle my message as a new request" |
| Mechanism | native `turn/steer` pinned to the active turn (`expectedTurnId`) | per-session FIFO, new turn after the current one |
| Card behavior | live card hands over to the newest message | new card when its turn starts |
| Requirements | Codex (the default app-server/stdio backend; others fall back to queue) | any agent |

```toml
[queue]
busy_message_mode = "steer"     # "steer" (default) or "queue"

[projects.agent.options]
backend = "app_server"
app_server_url = "stdio"
```

`/ps <message>` (alias `/btw`) always steers explicitly regardless of the configured mode. Definitive failures fall back safely; an unknown outcome is never re-queued, so your input can never be delivered twice. Details in the [usage guide](docs/usage.md#busy-messages-queue-vs-steer).

## 🧩 Supported agents & platforms

| | |
|---|---|
| **Agents** | Claude Code · Codex · Cursor · Gemini CLI · GitHub Copilot CLI · Kimi Code · OpenCode · iFlow · Qoder · Pi · Devin · Antigravity · generic ACP · tmux |
| **Platforms** | Feishu/Lark · Telegram · Slack · Discord · DingTalk · WeCom · Weixin (personal) · QQ · QQ Bot · LINE · Weibo · Matrix · Webex · MAX · WPS 协作 — plus a WebSocket [bridge](docs/bridge-protocol.md) for external adapters |

Every agent and platform compiles in by default and can be excluded with build tags (`make build EXCLUDE=discord,qq`).

## 🏗️ Architecture

```
┌───────────────────────────────────────────────┐
│                cmd/cc-connect                 │  CLI · daemon · migrate · update · doctor
├───────────────────────────────────────────────┤
│                    core/                      │  engine · sessions · cards · steer ·
│      (imports stdlib only — never plugins)    │  queue · i18n · cron · webhook · relay
├──────────────────────┬────────────────────────┤
│       agent/*        │      platform/*        │  self-contained adapters, registered
│  claudecode codex …  │  feishu telegram …     │  via capability interfaces
└──────────────────────┴────────────────────────┘
```

Core never hardcodes an agent or platform name; optional capabilities (rich cards, steering, model switching, …) are interface assertions that adapters opt into. One process can host any number of `[[projects]]`.

## 🔄 Migrating from official CC Connect

```bash
cc-connect-next migrate --dry-run   # inspect the full plan first
cc-connect-next migrate             # copy config, sessions, cron/heartbeat state, bindings…
cc-connect-next --config ~/.cc-connect-next/config.toml
```

The migration never stops, uninstalls, or modifies the official install, and both can stay installed side by side (separate commands, data directories, services, and sockets). Every source file is hashed, staged, and re-verified before an atomic activation; any concurrent change fails the run closed with timestamped backups and a `migration-manifest.json` recording every path and SHA-256.

📖 **[Full migration & coexistence guide](docs/migration.md)** — including custom paths, the compatibility matrix, service switchover, rollback, and a paste-into-your-agent task block.

## ⚙️ Recommended Feishu configuration

New configs already use these defaults (also applied by `cc-connect-next feishu setup`):

```toml
[display]
mode = "compact"
card_mode = "rich"          # "legacy" opts back into inherited CC Connect rendering
thinking_messages = false
tool_messages = false
show_context_indicator = false
reply_footer = false
hide_agent_footer = true

[[projects]]
name = "my-project"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/absolute/path/to/project"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "${FEISHU_APP_ID}"
app_secret = "${FEISHU_APP_SECRET}"
reply_to_trigger = true
done_emoji = "Done"
```

The exact lifecycle, privacy boundary, fallback behavior, and verification commands are defined in the [Feishu answer-card contract](docs/feishu-card-contract.md).

## 🆚 Versus official CC Connect

cc-connect-next forked from CC Connect v1.4.1 and tracks upstream through per-change audits instead of merges ([policy & history](docs/upstream-v1.5.0-beta.3-audit.md)). Coming from the official version? The long-form walkthrough — what's actually different, the zero-risk trial, and rollback — is **[Coming from official CC Connect](docs/coming-from-cc-connect.md)**.

| | CC Connect | cc-connect-next |
|---|---|---|
| Feishu answers | message stream / legacy cards | single Card 2.0 lifecycle with typewriter streaming |
| Reasoning & tool details in chat | rendered | anonymous counts only, enforced at two layers |
| Busy-session messages | FIFO queue; `/ps` raw send | native steer with card handoff by default, queue configurable |
| Updates | manual | stable-channel updater + once-per-release daemon notice |
| Migration path | — | audited one-command migration with manifest and rollback |
| Runtime identity | `cc-connect` · `~/.cc-connect` | independent everything; both coexist |

## 📚 Documentation

| Guide | What's inside |
|---|---|
| [INSTALL.md](INSTALL.md) | npm / standalone / source install, updates, daemon setup |
| [Usage guide](docs/usage.md) | sessions, queue vs steer, permissions, providers, models, cron, relay, TTS/STT |
| [Feishu setup](docs/feishu.md) | app creation, permissions, event subscription |
| [Answer-card contract](docs/feishu-card-contract.md) | the exact card lifecycle and privacy guarantees |
| [Coming from CC Connect](docs/coming-from-cc-connect.md) | honest comparison, zero-risk trial, switchover and rollback |
| [Migration guide](docs/migration.md) | full migration, coexistence, and switchover reference |
| [Migration matrix](docs/migration-compatibility.md) | which official versions and settings migrate |
| [Bridge protocol](docs/bridge-protocol.md) | build your own platform adapter over WebSocket |
| [Management API](docs/management-api.md) | local HTTP control surface |
| Platform guides | [Telegram](docs/telegram.md) · [Slack](docs/slack.md) · [Discord](docs/discord.md) · [DingTalk](docs/dingtalk.md) · [WeCom](docs/wecom.md) · [QQ](docs/qq.md) · [Matrix](docs/matrix.md) · [more](docs/) |

## 🛠️ Development

```bash
make web            # build web admin assets once
go test ./...       # full suite
make build-noweb    # fast binary without the web dashboard
```

Focused suites: `go test ./platform/feishu -run TestBuildRichCard`, `go test ./core -run TestCUJ_Steer`, `go test -tags no_web ./cmd/cc-connect -run TestMigrateLegacyData`. Contributions follow the layering rules in [CLAUDE.md](CLAUDE.md): core imports stdlib only, adapters register capabilities, every user-facing string ships in five languages, and new features come with regression tests.

## 🙏 Attribution & license

cc-connect-next starts from CC Connect v1.4.1 and preserves its full Git history — thanks to [@chenhg5](https://github.com/chenhg5) and every upstream contributor. Upstream changes are adopted through reviewed, per-change audits with credit in the audit log. See [NOTICE](NOTICE) for attribution and [LICENSE](LICENSE) for MIT terms.
