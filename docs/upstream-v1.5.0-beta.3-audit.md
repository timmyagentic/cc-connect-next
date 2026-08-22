# Upstream v1.5.0-beta.3 audit

Audit date: 2026-08-14

Historical note: this table records the `v0.1.0-beta.1` release-gate decision. The previously deferred Feishu bundle was implemented in the 2026-08-15 follow-up; see [Upstream Feishu parity](upstream-feishu-parity-2026-08-15.md). The original decisions below remain unchanged so the release history is auditable.

cc-connect-next forked from official CC Connect v1.4.1 (`7fcad099`). Official v1.5.0-beta.3 is `ad196294`; the histories have diverged by 82 upstream commits, 23 successor commits, 190 changed files, and more than 26,000 added lines. The beta is therefore a patch source, not a safe merge target: importing it wholesale would replace independently reviewed migration and Feishu card-lifecycle code.

## Release-candidate decisions

| Upstream change | Decision for v0.1.0-beta.1 | Reason / implementation |
|---|---|---|
| `d484ae6` — admin-gate `/commands addexec` and `/cron addexec` | Reimplemented | Security boundary. Non-admin users can no longer register shell execution; listing and prompt-only management remain available. |
| `91c3d44` — time out blocked Codex app-server writes | Reimplemented | A request timeout now includes the stdin write and aborts the stuck transport. |
| `230ee3c` — resume preview after permission resolution | Adapted | Legacy previews restart as a fresh segment. Rich cards keep their single-card lifecycle and never freeze at the prompt boundary. |
| `5bda531` — prevent attachment filename collisions | Adapted | Next keeps the existing Agent interface, scopes files by trigger message ID, uses exclusive non-overwriting creates, and preserves the `.cc-connect-next` namespace. |
| `d7f06db` — create queue placeholder around idle reset | Already covered | Next creates the placeholder before attempting the session lock. A concurrent message also creates/adopts the placeholder, so the upstream loss window is absent. |
| `f943666` — hide Agent-emitted status footer | Reimplemented | `hide_agent_footer` is supported globally and per project; the recommended Feishu profile enables it. |
| `129753c` — topic-scoped workspace bindings | Deferred | This changes workspace-routing persistence, not the answer-card contract. It needs migration and real topic QA as one unit. |
| `c22d5e2` — on-demand quoted-file download | Deferred | Valuable, but it adds Feishu resource API reads and same-user privacy rules. It requires dedicated real-client/file-permission QA. |
| `94e2154` — bootstrap the first mention in an existing topic | Deferred with topic bundle | Depends on the topic/session semantics above and should not be imported alone. |
| `97b96ad`, `6fc59a3` — relay visibility and outbound bot mentions | Deferred | Bot-to-bot relay behavior is outside the first Feishu answer-card and migration release gate. |
| `760079b` — live Agent process idle timeout | Deferred, migration-gated | If an imported config uses this new field, migration fails before writing rather than silently ignoring its behavior. |
| `2973210` — Yuanbao platform | Deferred, migration-gated | The first beta does not advertise Yuanbao. A migrated project that uses it fails compatibility preflight before writing. |

## Policy for later upstream syncs

1. Compare from the last audited official tag, not from upstream `main`.
2. Classify each change as reimplemented, already covered, accepted verbatim, or deferred.
3. Preserve the Next identities (`cc-connect-next`, `.cc-connect-next`, independent services and sockets).
4. Treat Feishu routing, reply context, card lifecycle, and migration inventory as protected contracts.
5. Require focused regression tests before implementation, then full Go, race, release-local, archive, and real-client gates.

## v1.5.0 final delta (audited 2026-08-19)

Official v1.5.0 (`17c61062`, released 2026-08-16) stabilizes beta.1–beta.5.
Delta since the audited `v1.5.0-beta.3`: 12 commits, of which one touches
core/Feishu (`44c07b61`, #1693 P1 stability). No overlap with Next-only
features (busy-message steer, rich-card handoff, usage-limit card, unified
updater — none exist upstream; upstream `/ps` still calls plain `Send`).

| Upstream change | Decision | Reason / implementation |
|---|---|---|
| `44c07b61` P1-A — recover in `runPendingRestartNotify` | Reimplemented | Next shares the same goroutine shape with no higher-level recover; a platform panic during restart notify would kill the daemon. Ported with the panic/stack log and a crash-reproducing regression test. |
| `44c07b61` P1-B — flush pending image batch before non-image dispatch | Adapted | Next has the same image batch + user-message watermark stale-drop, so a rapid image+text pair could drop the buffered image. Next routes every branch through one `dispatchToCore` closure, so the flush lives there instead of upstream's eight per-branch call sites; the non-batched image dispatch path is covered as well. |
| `44c07b61` P1-C — idle-close timer arming order | Not applicable | Fixes `agent_session_idle_timeout_mins` (#1338), which was deferred at the beta.3 gate and never imported. |
| `1ddbac04`, `6c860798` — Kimi Code CLI native dialect + probe gate | Deferred | Feature work for the Kimi agent; import as one unit with real-CLI QA when needed. |
| `c2bfb444`, `e3ea0cb5` — Pi v0.84.0 toolcall_end + willRetry | Deferred | Pi compatibility updates; import when a Pi deployment needs v0.84.0. |
| `cc869faa` — WeCom quoted messages as agent context | Deferred | Platform enhancement outside the Feishu contract; needs WeCom QA. |
| `d5144e88` — web work_dir validation | Deferred | Next's web admin diverged; revisit with the next web batch. |

## Post-v1.5.0 sync (audited 2026-08-22)

Upstream `main` has exactly one commit after the `v1.5.0` tag. This pass
imported it plus three previously deferred agent-compat changes whose target
CLI versions are now current; each landed with the upstream regression tests
adapted to Next's code shape.

| Upstream change | Decision | Reason / implementation |
|---|---|---|
| `3727b740` — Codex `/list` reads `session_index.jsonl` thread names, keeps only the first `session_meta`, and skips subagent rollouts | Adapted | Next's `list.go` shares the pre-fix lineage. The title override is applied before Next's `patchSessionSource` (which only rewrites string-form `"source":"exec"`, so the object-form subagent source never collides). Four upstream regression tests imported. |
| `6c860798` — gate Kimi `--work-dir` on the help probe | Reimplemented | Newest Kimi Code CLI builds reject `--work-dir` outright, breaking every non-default `work_dir` project. Next's probe already parses all help flags; `kimiFlagSupport` gains `WorkDir` and `buildArgs` gates on it. `exec.Cmd.Dir` is set separately, so behavior is identical either way. |
| `c2bfb444` — Pi v0.84.0 `toolcall_end` without cumulative message | Adapted | Pi ≥ 0.84.0 removed the `message`/`partial` snapshots Next's extractor relied on; the direct `toolCall` object is now read first with the old snapshot path kept as fallback for older CLIs. |
| `e3ea0cb5` — keep the turn open while Pi auto-retries (`willRetry`) | Adapted | Next surfaced every assistant `errorMessage` immediately, so a transient 429 Pi was already retrying looked like a hard turn failure. Errors are now buffered (`pendingErr`), cleared by a healthy assistant message, flushed on terminal `agent_end`, and flushed by `readLoop` on process exit — adapted to Next's JSON-mode-only session (upstream also patches an RPC mode Next does not have). |
| `1ddbac04` — native Kimi Code CLI dialect | Deferred | Large surface (+795/−120 across the kimi adapter). Import as one unit with real-CLI QA before advertising newest-Kimi support. |
| `cc869faa` — WeCom quoted messages | Already covered | Next's WeCom adapter already collects main + quote + mixed inbound parts (`wsQuoteBlock`, `wsCollectInboundParts`). |
| `760079b` / `44c07b61` P1-C — agent idle close timer | Not applicable (re-verified) | Next has no per-process idle-close timer; its idle mechanisms (event-gap turn timeout, workspace reaper, `reset_on_idle_mins`) do not share upstream's arming-order bug shape. |

## Full-history gap audit v1.4.1 → v1.5.0 (audited 2026-08-22)

The release-gate tables above decided a curated subset. This pass enumerated
**all 91 non-merge official commits** between `v1.4.1` (`7fcad099`) and
`v1.5.0` (`17c61062`) and classified every one. Matching is by PR number and
normalized subject, because the fork launch squashed upstream content under
new hashes.

### Ported in this pass

| Upstream | Area | What / why |
|---|---|---|
| `8df29ce4`/`52dfe8b7` (#1546) | codex | `/model` dropped the stale `openaiChatModels` allowlist for pattern-based `isCodexChatModel`; gpt-5.x, o5-*, codex-* now appear. |
| `f8b82946` (#1630) | core | All ten reply-splitting call sites use `SplitMessageCodeFenceAware`; long answers no longer break Markdown code fences mid-chunk. |
| `9d9e20e9` (#1583) | core | `selectUsageWindows` no longer renders a lone 7-day window twice (primary fallback now skips the slot the secondary already owns). |
| `57bf3fd0` (#1233) | core | `tool_max_len` applies to tool input in `progress_style = "card"` (rendered text and structured payload); `event.ToolInput` untouched. |
| `523f109a` (#1425) | claudecode | Missing `work_dir` fails at `New()` with a clear message instead of a confusing first-turn CLI error. |
| `afb21393` (#1549) | claudecode | Session list prefers `custom-title` > `ai-title` > first user prompt. |
| `065a705f` (#1107) | claudecode | `sonnet[1m]` added to the fallback model list. |
| `12a589fc` (#1584) | antigravity | Conversation-ID detection moved after `cmd.Wait()` — agy flushes the chat file on exit, so resume previously lost the ID. |
| `d743b147` (#1636) | pi | `/model` falls back to pi's `models-store.json` catalog when `enabledModels` is unset. |
| `2ca32186` (#1373) | i18n | `/model` switch confirmation clarifies the model applies to the current session too (all five languages). |
| `a79cbb9a` (#353) | cli | Unknown non-flag top-level commands are rejected with the subcommand list instead of silently starting the runtime. |

### Verified already present (independently implemented or included at launch)

`e993ba7c` kimi `--print` gate (ported earlier as `147b5b77`) · `a4b46593` +
`5e2d501f` Send/cleanup race (#1436) · `e739e2eb` recall-probe throttle ·
`79e3132f` post-restart notify queue · `303fd46d` system-prompt chmod ·
`9301cfaa` Zhipu GLM presets · `fec094f9` `.worktrees/` gitignore ·
`5cf23794` DingTalk stream-loop recover · `b97eea89` DingTalk `@userid`
extraction (`extractAtUserIds`) · `a5d441e9` claudecode `EventToolResult` ·
`560635c1` send cwd workdir · `355793ea` unified `cmd` parsing (as
`d515c808`; array form verified in `ParseCmdOpts`) · `fa86932f` `/history`
truncation config · `8e3203a7` Codex `model_catalog_json` priority ·
`80903872` Slack size-limit fallback · `d6c0053a` nav.cron i18n · `54a3195f`
Feishu image batch window (500 ms default) · `7e1b53c6` absolute attachment
paths (`SaveFilesToDisk` uses `filepath.Abs`) · `7b55d4fc` NO_REPLY
suppression on streaming cards (covered by Next's platform-agnostic
`silentHold` machinery) · `72d6fb61` WPS platform (present as `wps-xiezuo`) ·
`bc30f5aa`/`ad196294` `CheckLinger` non-Linux stub (`daemon/unsupported.go`) ·
`3fa54174` beta.5 P1 content (ported earlier from `44c07b61`).

### Deferred, with owner decisions pending

| Upstream | Why deferred |
|---|---|
| `6e0c7c36` + `a2bda906` + `1c03a593` — pi RPC mode, Windows build fix, permission-mode env | One bundle: the env injection serves pi's permission-gate extension, which arrives with RPC mode. Needs real-CLI QA. |
| `2c52b53e` — antigravity tool-permission bridge | Depends on a newer antigravity surface; needs real-CLI QA. |
| `9fe3ea26` — Weixin ret=-2 per-account send budget rework | Next has the earlier retry+token-refresh variant; the rework needs real-account QA. |
| `022d661c` — DingTalk chat-list title derivation | Platform UX enhancement; needs DingTalk QA. |
| `7b93b085` Google Chat · `fc315d21` TuiTui · `118dd3b6` cloud_web · `fd6dbcc3` Reasonix agent · `58ae8f8d` Russian locale | New-surface product decisions; each also adds an i18n/QA obligation. |

### Not applicable

`f2e4ed83` + `c4ef126c` Kimi provider presets — upstream sponsorship content
carrying upstream's affiliate links; Next maintains its own
`provider-presets.json`. `a79e9816` web ProjectDetail and `18c7dae7`
cloud_web test — Next's web admin diverged and cloud_web is not compiled in.
Remaining 17 commits are sponsor banners, release stamps, README copy, and
docs with no code effect.
