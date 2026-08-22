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
