# Changelog

## Unreleased

### Defaults

- Fresh starter configs now use Codex `mode = "yolo"`, so new installations
  run without approval prompts or sandbox restrictions. Existing explicit
  modes are unchanged, and hand-written configs that omit `mode` keep the
  adapter's compatibility fallback.
- `reply_footer` now defaults to `true`, showing the compact model, reasoning
  effort, and elapsed-time line on completed replies. Global or per-project
  `reply_footer = false` still disables it and takes precedence over the new
  built-in default.
- New Starter configs and accepted recommended Feishu/Lark profiles now write
  `thread_isolation = "topics_only"`, so real group topics get independent
  Agent sessions and workspace bindings while ordinary group messages remain
  in the main chat. Existing configs remain compatible: omission and legacy
  `false` map to `off`, while legacy `true` preserves the historical
  `topic_per_message` behavior.

### Agent-friendly configuration knowledge

- The catalog is now an executable configuration contract rather than a
  key/description index. Public entries declare TOML/environment/CLI source,
  exact placement, required and conditional relationships, omitted-default
  source, presets, accepted types/enums, numeric bounds/units, dependencies,
  conflicts, apply mode, sensitivity, and a validated example. Dynamic
  Agent/Platform options are checked against that contract before adapter
  construction, and the Web settings constants are generated from it too.
- Starter access-control guidance now uses the Feishu string form actually
  consumed by runtime (`allow_from = "..."`, `allow_chat = "..."`), preventing
  an attempted allowlist from silently becoming allow-all. Cross-adapter type,
  required-credential, constructor-default, provider, hook, speech/TTS, and
  reload metadata now have regression gates, and common natural-language
  paraphrases are ranked to their exact options.
- Every compiled build now carries a structured configuration capability
  catalog covering typed global/project settings and the exact dynamic option
  surfaces of all included Agent and messaging-platform adapters. Each entry
  explains purpose, scope, type, default/allowed values, apply mode,
  sensitivity, and bilingual natural-language keywords.
- `cc-connect-next config capabilities` exposes that catalog without reading
  the operator's actual config or credentials. It supports natural-language
  search, exact-key lookup, active Agent/platform filtering, and stable
  Markdown or JSON output; generated English and Chinese references are
  checked into `docs/` from the same source.
- A bounded, version-matched catalog capsule is injected once per Agent
  session. It tells the Agent to query the local catalog before answering
  configuration questions, explain exact TOML paths and apply semantics, and
  report unsupported wishes honestly through `/feedback` instead of inventing
  keys. Agent and Platform option-map typos now join the existing startup
  warning and capability-gap flow instead of remaining silent.

### Correctness and upstream compatibility

- Bridge message delivery now treats a non-empty `msg_id` as a short-window
  idempotency key scoped by adapter, project, and session. Duplicate frames are
  dropped before Engine/FIFO admission, including across adapter WebSocket
  replacement, while empty IDs retain their compatibility behavior.
- Persisted Cron/Timer targets now honor an optional platform durability gate.
  Browser-connected Web/Bridge sessions are rejected before create, update,
  enable, or manual trigger, and the Web editor disables them with an explicit
  explanation instead of accepting jobs that fail after `triggered`.
- Feishu/Lark `thread_isolation` is now an explicit scope mode: `off`,
  `topics_only`, or `topic_per_message`. `topics_only` isolates only messages
  that the platform marks as real topics (`thread_id` is present), while
  `topic_per_message` preserves the previous behavior in which every top-level
  group message becomes an isolated topic. Topic routing now also takes
  precedence over `reply_to_trigger`, so disabling quote-style replies cannot
  leak a real-topic response back into the main chat.
- Feishu/Lark WebSocket projects now fail closed for mention-gated group
  traffic when startup cannot resolve the bot `open_id`. The degraded identity
  reaches the existing runtime health/readiness surface, transient discovery
  is retried, and a background supervisor restores normal filtering after
  recovery. Webhook/private deployments and explicit `group_reply_all` remain
  unchanged.
- Claude Code sessions no longer pass `--replay-user-messages`, which made
  current Claude CLI processes exit after each turn and prevented native
  commands such as `/compact` from reaching the live session. The existing
  bidirectional `stream-json` and stdio permission protocol is preserved.
- Explicit `/switch`, management, and Bridge session selection records a
  persisted activation timestamp. The first message stays in the selected
  conversation even when its prior activity is older than
  `reset_on_idle_mins`; the normal idle window applies again afterward.

### Session UX

- `reply_footer = true` now shows the processing time of each completed turn,
  alongside the existing model and reasoning effort, on plain replies, rich
  cards, and native streaming cards (for example,
  `gpt-5.6-sol · effort:max · ⏱ 12.3s`). Duration units follow the turn's
  language. Queued turns start timing when their processing actually begins
  rather than while waiting behind an in-flight turn, and presentation-only
  rich-card dwell is excluded.
- Fresh Codex app-server sessions now receive a concise Codex App title from
  the request that creates them immediately after `thread/start` returns and
  before the first turn starts. Capability briefs,
  sender metadata, known quote scaffolding, links, emails, and token-like
  secrets are excluded or redacted; explicit `/name` updates are also synchronized
  when a compatible live Codex session is available. Titles default to the
  configurable `[飞书]` source prefix. Optional `session_title_model` generation
  runs through an isolated local Codex ephemeral process and safely falls back
  without affecting the user turn.

## v0.2.0 (2026-08-26)

Stable release focused on Codex app-server isolation, safer migration and
update flows, session/runtime correctness, one-shot answer profiles, streamlined
session creation, the official lark-cli companion, and a clean security and
quality baseline for both Go and Web dependencies.

See `changelogs/v0.2.0.md` for the bilingual release notes.

### Session UX

- `/new <prompt>` now starts a fresh session and immediately handles `<prompt>`
  as its first user message, so resetting context no longer requires a second
  send. Plain `/new` still creates an empty session, and explicit titles remain
  available through `/name`.

### Official lark-cli companion

- `feishu setup`, completed migrations, and the standalone
  `cc-connect-next lark-cli setup` command can install the official `lark-cli`
  when missing and reuse the selected Feishu/Lark bot as an isolated named
  profile. The profile becomes the default with bot as its default identity;
  existing profiles and user OAuth logins are preserved.
- App secrets reach `lark-cli` only through stdin. Same-App profiles are reused,
  same-name profiles for another App are never overwritten, multiple bots
  require an explicit project, and migration dry-runs never change `lark-cli`.
  The companion does not run user OAuth, send a test message, or open an event
  consumer; operators are warned not to run `lark-cli event consume` with the
  same App while cc-connect-next owns its event connection.

### Codex app-server correctness

- Native subagent thread, turn, item, error, token-usage, approval, and
  request-user-input traffic is isolated from the parent turn, preventing
  child work from leaking into or prematurely completing the parent reply.
- Interactive requests owned by another turn are rejected with a bounded
  write; a blocked rejection aborts the transport instead of wedging the
  app-server read loop.
- `model_context_window` is now a first-class Codex option and reaches both
  exec and app-server backends.

### Session and runtime correctness

- All declared session state, including the active provider and last user
  activity, survives persistence and restart.
- Provider selection and attachment staging now use shared implementations
  across agents, reducing divergent edge cases.
- Cron and timer jobs share one shell-execution path, including the documented
  `timeout = 0` no-timeout behavior.
- Turn handling is split into a dedicated processor without changing the
  public engine contract, and superseded Web/session surfaces were removed.

### End-to-end hardening

- The Web chat now opens its own persistent Web session instead of rendering
  another platform's latest history and then sending into a fresh context;
  external sessions remain available through explicit selection.
- `--config <path> doctor` is normalized to the doctor command, while any
  other unconsumed positional argument is rejected before locks, platforms,
  or the daemon can start.
- Bridge readiness now follows the adapter lifecycle: the first registered
  adapter marks every Bridge platform ready, and the last disconnect marks it
  unavailable with an explicit reason instead of a false `never connected`.
- Accepted queued and steered messages refresh `last_user_activity`, keeping
  opt-in idle session rotation aligned with real user activity.
- Doctor checks the effective configured `data_dir` in both CLI and in-chat
  reports instead of always inspecting `~/.cc-connect-next`.

### Security and release quality

- The build baseline is Go 1.25.13, with `golang.org/x/crypto`, `x/net`,
  `x/sys`, `x/term`, `x/text`, `gorilla/websocket`, and `slack-go` upgraded;
  Slack file events and uploads now use the current SDK contracts.
- Vite, PostCSS, React Router, and transitive Web dependencies were upgraded;
  the production audit has no known advisories and the full audit has no
  high or critical findings.
- The pnpm workspace build-script allowlist now uses valid YAML and permits
  the required esbuild install step without mutating project configuration.
- The full golangci-lint configuration passes with zero findings; no linter
  was disabled or excluded to create the baseline.
### One-shot answer profiles

- Codex projects can configure `fast` and `quality` answer profiles and apply
  them to one message with `/fast <task>`, `/quality <task>`, or conservative
  Chinese leading phrases such as “用快速模式……” and “用高质量模式……”.
- Ordinary messages always reuse the existing project defaults. Profiled turns
  pass model, reasoning effort, and service tier through both Codex backends;
  the next ordinary turn explicitly restores the defaults.
- A profiled message received while a turn is busy stays in the FIFO as its own
  turn because `turn/steer` cannot carry profile settings. There is no
  `/balanced` or default-mode command.

### Review hardening

- `migrate --switch` is now a complete first-time production cutover: it
  rejects an installed successor, stops and disables official CC Connect, performs the
  final consistent migration, installs/starts Next with the exact migrated
  config and original work directory, then waits for the local API and every
  configured platform to report Ready. It must run outside a connected CC Agent session. On success
  the CLI directly sends one private Feishu/Lark completion message; ambiguous
  targets and send failures are reported without group fallback or rollback.
- Activation recovery is fail-closed across systemd, launchd, and Windows:
  service-manager query/stop/uninstall errors propagate, and official CC Connect
  is restored only after Next is unregistered, its runtime socket is silent,
  and the migrated config lock is free. Windows treats only a successful empty
  Task Scheduler enumeration as not installed.
- Feishu/Lark now use the official SDK's connection lifecycle callbacks, and
  `daemon status` reports Service separately from Runtime/Platforms.
- Exact migration provenance now accepts official `v1.5.0-beta.4`,
  `v1.5.0-beta.5`, and stable `v1.5.0`; the same strict plugin/config gates
  still reject unsupported behavior before any target write.
- `migrate --switch` now validates the full migration before stopping the
  official daemon; credential overlap resolves `${ENV}` app IDs; Linux
  switchover selects the detected user or system service manager.
- A temporarily unavailable Claude Code work directory no longer prevents
  unrelated projects from starting, and Markdown code-fence splitting always
  stays within the platform message limit.
- QQ Bot command buttons now route through the normal command entrypoint, and
  Traditional Chinese `確認` / `確定` are accepted by the update flow.
- Migration, Feishu privacy, and topic-context documentation now state their
  real work-dir, skipped-discovery, answer-storage, and bounded-history limits.

### Natural-language updates

The update flow no longer demands command syntax. The release reminder
carries an **[update now]** button (Feishu cards / inline keyboards, text
fallback elsewhere); messages like 更新 / 升级到最新版 / “update” / a
post-prompt “确认” drive the same upgrade pipeline. Interception is
conservative — the whole message must match a short phrase list, ambiguous
bare verbs count only inside an update conversation, and everything else
reaches the agent untouched. Matched intents are dispatched through the
normal command path, so the `admin_from` and disabled-command gates still
apply. `/upgrade` keeps working for those who prefer it.

Each message carries exactly **one primary** call to action. Copy shown
with a button stops after the release information; the natural-language
route stays discoverable as a footnote (“也可以直接回复「更新」”, small grey
on Feishu cards, a trailing line on inline-keyboard platforms). Only
surfaces that cannot render a button state the typed reply outright as the
instruction — including when a card or button send fails and delivery falls
back to text.

### Dual-daemon conflict protection

The "never run both daemons against the same platform credentials" rule is
now enforced by the product instead of documentation:

- startup refuses to race a running official daemon that shares credentials
  with the loaded config (override: `CC_NEXT_ALLOW_OFFICIAL_CONFLICT=1`),
  and warns when the official autostart is merely armed for the next boot;
- `migrate` ends with a status-aware switchover/trial guide instead of the
  passive "was not modified or stopped" line;
- `migrate --switch` stops the official daemon, disarms and verifies its
  autostart, final-syncs, installs/starts Next, and recovers official service
  state on downstream failure (binaries and data stay untouched);
- `doctor` gains an "official CC Connect coexistence" section with redacted
  credential-overlap reporting.

Full-history upstream audit: every one of the 91 official commits between
v1.4.1 and v1.5.0 is now classified (ported / already present / deferred /
not applicable) in `docs/upstream-v1.5.0-beta.3-audit.md`. Fixes imported in
this pass:

- **codex**: `/model` uses pattern matching instead of a stale allowlist, so
  gpt-5.x / o5 / codex-* models from `/v1/models` appear in the chooser.
- **core**: long replies split without breaking Markdown code fences; the
  usage card no longer renders a lone 7-day window twice; `tool_max_len` now
  applies to tool input in `progress_style = "card"`.
- **claudecode**: session list prefers Claude Code's own ai/custom titles;
  a missing `work_dir` fails at startup with a clear error; `sonnet[1m]`
  joins the fallback model list.
- **pi**: `/model` falls back to pi's `models-store.json` catalog when
  `enabledModels` is unset.
- **antigravity**: resume works — the conversation ID is detected after
  process exit, when agy actually flushes its chat file.
- **cli**: unknown top-level commands are rejected with the subcommand list
  instead of silently starting the runtime.
- **i18n**: `/model` switch confirmation says the model applies to the
  current session too, in all five languages.

## v0.1.5 (2026-08-22)

Stable release: a complete in-app feedback loop (problem summarized, one tap
files an anonymous GitHub issue through the author's relay — no GitHub
account needed), a per-session capability brief so the agent answers
configuration questions from the real option set, the issue #37 fix for cmd
extra args on the Codex app-server backend, first-class `service_tier`,
`reasoning_effort = "max"`, and an opt-in `model · effort` footer on rich
cards.

See `changelogs/v0.1.5.md` for the bilingual release notes.

### Feedback loop

- Failed turns are followed by an ask card ("Report to the author?") with
  agree/ignore buttons; agreement files a redacted anonymous issue with the
  error and config context attached. `/feedback <description>` works
  everywhere; duplicates thread onto the existing issue as "+1" comments.
- A capability brief injected once per session tells the agent exactly which
  agent options exist, so unsupported wishes are answered honestly and
  routed to feedback instead of invented config keys.
- Unknown top-level config keys are announced after startup.

### Codex

- `cmd` extra args and custom binaries now reach the app-server backend
  (issue #37); structured options win on duplicate `-c` keys.
- `service_tier` is a first-class option; `reasoning_effort` accepts `max`.

### Cards

- `reply_footer = true` now renders on rich cards too, slimmed to a single
  `model · effort` line; `show_context_indicator` / `show_workdir_indicator`
  are deprecated no-ops.


## v0.1.4 (2026-08-21)

Patch release that makes the v0.1.3 steer-by-default policy usable after a
direct install. New starter configs now select Codex app-server over stdio,
and existing Codex configs that omit backend details resolve to the same
native-steer path. Explicit `backend = "exec"` remains supported and continues
to fall back to FIFO for busy messages.

See `changelogs/v0.1.4.md` for the bilingual release notes.

### Direct installs now get a steer-capable default chain

- The generated starter config explicitly sets
  `[queue] busy_message_mode = "steer"`, uses Codex, and configures
  `backend = "app_server"` with `app_server_url = "stdio"`.
- Codex configurations that omit `backend` or `app_server_url` now default to
  app-server and stdio respectively, so upgrading an existing minimal config
  fixes the same gap without requiring a config rewrite.
- `cc-connect-next doctor` reports whether the selected agent/backend can
  natively steer or whether busy messages will use the FIFO fallback.
- Installation, configuration, usage, npm, and Feishu card-contract docs now
  describe the actual default and the explicit exec compatibility opt-out.

## v0.1.3 (2026-08-20)

Stable release: steer becomes the default busy-message mode, the daemon now
proactively reminds users once per new stable release, the two applicable
upstream v1.5.0 P1 stability fixes are ported, and the README is rebuilt as a
modern bilingual landing page.

See `changelogs/v0.1.3.md` for the bilingual release notes.

### Steer is now the default busy-message mode

`busy_message_mode` now defaults to `"steer"`: a message that arrives while
the agent is busy joins the turn already running (Codex app-server backend),
with the live card handing over to the newest message. The default is safe for
every agent — sessions without the steer capability, the Codex exec backend,
and just-ended turns all fall back transparently to the FIFO queue. Set
`busy_message_mode = "queue"` globally or per project to restore the previous
queue-always behavior.

### Proactive update notice

The daemon now reminds users when a newer stable release ships. Every update
surface used to be pull-only (`/upgrade`, `check-update`, the CLI usage hint),
so a user running an old daemon never learned that a new version existed. The
daemon now checks GitHub for the newest stable release (2 minutes after
startup, then every 24 hours) and sends each project's most recently active
chat one localized notice per version — state persists across restarts, so a
version is never announced twice. Disable with `update_notice = false`.

Also ports the applicable upstream v1.5.0 P1 stability fixes (#1693):
restart-notify panic recovery and the cross-type image batch flush that
prevented a rapid image+text pair from dropping the buffered image.

## v0.1.2 (2026-08-19)

Stable release built from the latest `main`. Delivers issue #27 end to end:
configurable queue-vs-steer handling for busy sessions, native Codex
`turn/steer` on the app-server backend, the `/ps` concurrent-exec fix, and the
Feishu rich-card handoff to the newest steered message. Verified against real
Feishu and a real Codex app-server.

See `changelogs/v0.1.2.md` for the bilingual release notes.

### Configurable queue vs native steer for busy sessions (#27)

Messages that arrive while a turn is active can now be **steered** into the
running turn instead of queued, via `[queue] busy_message_mode = "steer"`
(default remains `"queue"`; per-project override: `projects.busy_message_mode`).

- New agent-neutral `SteerableSession` capability in core. Codex implements
  native steering on the app-server backend with `turn/steer` +
  `expectedTurnId`; the exec backend explicitly reports steering as
  unsupported, so `/ps` can no longer launch a concurrent `codex exec resume`
  against a running turn.
- `/ps` (and `/btw`) dispatch through the steer capability. Definitive
  failures produce clear localized errors; an unknown steer outcome (RPC
  timeout) is never silently re-queued, to avoid duplicate delivery.
- Rich-card presentation handoff: a successful steer creates a successor card
  replying to the newest message, freezes the previous card in a neutral grey
  "Continued in a newer message" state that retains its visible partial
  answer, and renders all further progress plus the final answer only in the
  newest card. Rapid consecutive steers chain; exactly one card reaches Done
  per turn.

## v0.1.1 (2026-08-18)

Stable release promoted from `v0.1.1-beta.1`, built from the latest `main`.
It delivers the unified stable-only updater, the dedicated Codex usage-limit
card, and the Feishu lifecycle-card copy fix. `npm latest` moves to this
version; the explicit `@beta` channel remains available for future prereleases.

See `changelogs/v0.1.1.md` for the bilingual release notes.

## v0.1.1-beta.1 (2026-08-18)

First beta after the v0.1.0 stable release, built from the latest `main`.
This beta carries the unified stable-only updater and the recent Feishu card
and Codex usage-limit fixes. Install it explicitly from the beta channel;
`cc-connect-next update` remains stable-only.

See `changelogs/v0.1.1-beta.1.md` for the bilingual release notes.

## v0.1.0 (2026-08-17)

First stable release. Code-identical to `v0.1.0-beta.3`: this tag promotes the
beta line, so the GitHub release becomes the latest one and npm `latest` moves
off the prerelease channel. `@beta` continues to track prereleases.

The stable line covers the native Feishu Card 2.0 answer-card lifecycle, the
fail-closed one-command migration from official CC Connect v1.4.1 /
v1.5.0-beta.1–beta.3, the upstream Feishu capabilities merged after the fork
point, quiet-by-default display settings, and the hardened first install
(placeholder refusal, `cc-connect-next doctor`, single-definition starter
template). Real Feishu tenant rendering, permissions, quoted-file access, and
bot-to-bot delivery remain external release gates and are UNVERIFIED here.

See `changelogs/v0.1.0.md` for the bilingual release notes, and the
`v0.1.0-beta.*` entries below for the change-by-change history.

## v0.1.0-beta.3 (2026-08-16)

### Fixed

- **A starter config no longer reports itself running.** Startup refuses a configuration that still carries the placeholders this binary wrote (`work_dir`, Feishu `app_id`/`app_secret`), naming each key and the step that replaces it, before any engine is created. Previously `platform ready`, `engine started` and `cc-connect-next is running` were all printed before Feishu rejected the placeholder credentials, and the process then hung there looking healthy.
- **A platform that cannot connect is reported.** The long connection is opened after `Start` returns, so a rejected credential surfaced only as a stray SDK error. Feishu now records why the connection ended, and startup logs every platform that is still unusable 30 seconds in, pointing at `doctor`.
- **A configured `work_dir` that is missing or is not a directory** is reported once at startup instead of failing per turn.

### Added

- **`cc-connect-next doctor`** runs the full health check from the command line: config file and placeholders, work directory, Agent CLI and login state, per-platform configuration validation, dependencies, and network. It never opens a platform connection, so it works while the instance is down — which is when it is needed — and its platform section does not claim `connected`. `--config` and `--project` narrow the run; `doctor` is now listed in `--help`; `doctor user-isolation` is unchanged and the full check also runs on Windows.
- `core.RunDoctorChecksWithPlatformResults` and `Engine.PlatformStatuses` expose those two capabilities to callers that have no live platform.

### Changed

- **The first-run template is generated from the recommended Feishu profile** instead of being a second copy of it. New installations therefore get project-level `[projects.display]`, no pinned display `mode`, and the `enable_feishu_card`, `thread_isolation`, and `group_reply_all` settings the profile has always included, with the `allow_from`/`allow_chat` scope documented next to `group_reply_all`.
- **The recommended Feishu profile regains `hide_agent_footer = true`**, matching `config.example.toml`, the Feishu guide, and the answer-card contract.
- The first-run message points at `cc-connect-next feishu setup` and `cc-connect-next doctor`.

## v0.1.0-beta.2 (2026-08-15)

### Added

- **Feishu upstream parity bundle:** topic-scoped workspace bindings with non-destructive inheritance from the chat default, first-mention root-context bootstrap for existing topics, privacy-gated on-demand quoted-file download, and topic-local relay visibility.
- **Native bot-to-bot mentions:** `mention_map` gives explicit bot aliases priority over group-member names. A resolved mention uses tracked `MsgTypeText` terminal delivery so Feishu emits the real notification event; the Rich Card lifecycle remains unchanged for ordinary answers.

### Fixed

- **Feishu onboarding from an empty config:** `feishu new` no longer indexes an empty project list and panics before the first project is created; empty and whitespace-only configuration files now have regression coverage.

### Configurability

- **Per-group no-mention replies:** `group_reply_all_chats` accepts a comma-separated list or TOML string array of Feishu chat IDs. Listed groups accept unmentioned messages while every other group still requires an explicit bot mention; the sensitive `im:message.group_msg` permission remains required for Feishu delivery.

### Safety

- `mention_map` now fails startup/migration validation unless `resolve_mentions = true`, every alias is valid, and every target is a bot `open_id` beginning with `ou_`.
- Quoted files are downloaded only after the user explicitly mentions this bot and only when the quoted file was uploaded by that same user.
- Native mentions resolve only in visible prose; code, hidden URL/reference metadata, image alt text, footnote identifiers, and HTML attributes/raw code cannot notify a bot. Topic follow-ups remain FIFO while first-mention context bootstrap is in flight, and a partially fetched reply chain stays retryable after a transient ancestor failure. Topic-local relay echoes remain in-thread even when ordinary trigger replies are disabled, and failed visibility delivery emits an operator warning.

## v0.1.0-beta.1 (2026-08-15)

First public prerelease of cc-connect-next: an independently installed successor with a privacy-first Feishu Card 2.0 lifecycle, fail-closed migration from official CC Connect, and checksum-verified GitHub/npm distribution. See `changelogs/v0.1.0-beta.1.md` for bilingual release notes and migration guidance.

### Highlights

- One quoted card per Agent turn, immediate non-empty feedback, anonymous reasoning/tool counts, native answer streaming, and explicit Done/error states.
- No reasoning text, tool details, model/token/context/work-directory metadata, or expandable private payload. Fragmented Agent footers are filtered only after logical-line reconstruction.
- Explicit migration preserves supported persistent state without modifying official CC Connect, validates both TOML schema and normal startup semantics before writing, and produces verified manifests and recoverable backups.
- Six checksum-verified platform archives and the `cc-connect-next` npm beta are built from the same tag.

## v1.4.1 (2026-06-28)

Patch release focused on Kimi CLI compatibility for users on the newer `kimi-code` 0.14.x (which removed the `--print` flag). v1.4.1 probes the Kimi CLI at startup and conditionally passes `--print` only when the installed binary supports it. Older `kimi-cli` 1.48.x users keep working as-is — no config change required either way.

This is the first release under the post-v1.4.0 SOP correction: every commit since v1.4.0 (in this case #1461) was put through a fresh manual QA cycle by cc-connect/qa-cursor before stable promotion, including owner-paired smoke testing on the actual binary against both Kimi CLI versions.

See `changelogs/v1.4.1.md` for the full details with credits.

### Fixed
- **Kimi CLI `--print` compatibility** (#1461 fixing #1456, @chenhg5, reported by @WeiFengJL): newer `kimi-code` 0.14.x removed the `--print` flag, causing `error: unknown option '--print' (Did you mean --prompt?)` on agent startup. cc-connect now probes the Kimi CLI `--help` output and conditionally passes `--print` only when supported. Verified against `kimi-code` 0.14.2 (Print=false) and `kimi-cli` 1.48.0 (Print=true) on real binaries.

## v1.4.0 (2026-06-28)

Stable release of the v1.4.0 series. **Two new platforms join the family** (Cisco Webex, Matrix with E2EE), broader configurability across agents and platforms, Korean i18n, and a long list of fixes — including last-minute critical fixes for a `Send` goroutine race (#1436), a Feishu recall-probe quota burn (#1321), and a `run_as_user` EACCES regression (#1433).

This stable rolls up everything from v1.4.0-beta.1 → beta.2 → beta.3 plus the 3 post-beta.3 cherry-picks (#1436, #1321 and a `drainPendingMessages` follow-up alignment).

See `changelogs/v1.4.0.md` for the full themed summary with credits.

### 🚨 Critical fixes shipped late in the cycle
- **core Send-goroutine nil-pointer panic race** (#1436, @gotang; follow-up alignment of the third call site in `drainPendingMessages`): would crash the whole cc-connect process and drop every platform connection when an agent process exited just before a `Send` goroutine was scheduled. 100% reproducible in production.
- **Feishu recall-probe quota burn** (#1321, @qvictl): `MessageRecallDetector` fallback path was polling every 2s, burning ~1.3M Feishu OpenAPI calls/month per stuck session and exhausting the 1M free quota. Probe interval now 1 minute with per-message dedup + in-flight guard.
- **claudecode `run_as_user` EACCES regression** (#1433, @chenhg5; reported by @vuyiv #1429): chmod `0o644` on per-spawn system-prompt temp file so non-root child processes can read it. Fixes a regression introduced by v1.3.4 (#1376) — `run_as_user` users on beta.1/beta.2 were 100% blocked at agent startup.

### v1.4.0 cycle highlights
- **Cisco Webex** and **Matrix (with E2EE)** as new first-class platforms (#1402, #834).
- **agent option parsing refactor**: centralize cmd/env option parsing into core with unified `cmd` field across all agent adapters (#1297).
- **Slack streaming preview + aggregated turn card** (#1333).
- **Feishu after_click card replacement for `cmd:` actions** (#1299).
- **Codex custom `system_prompt` / `append_system_prompt` config** (#1345); codex `model_catalog_json` highest-priority source (#1074).
- **Zhipu GLM provider presets** for `z.ai` and `bigmodel` CN endpoint (#1412).
- **Korean (ko) i18n** for the Web admin UI (#1343), plus `nav.cron` translations for ko/ja/es.
- **`plugin_dir` for Claude Code plugins** (#1325), `cc-connect send --cwd` workdir support (#1380), `max_attachment_size_mb` (#1392), `CC_LOG_MAX_BACKUPS` env var (#1260), configurable `/history` truncation (#1291).
- 30+ fixes across feishu, slack, dingtalk, claudecode, codex, core engine, runas, web admin and i18n. Full list in `changelogs/v1.4.0.md`.

### Upgrade notes
- `cli_path` config field is deprecated in favour of unified `cmd` (#1297). Existing configs continue to work; a deprecation warning is logged. Migrate when convenient.
- `imageBatchWindow` default for Feishu changed from 150 ms → 500 ms. Override in config if you preferred the older value.
- `MessageRecallDetector` fallback probe interval changed from 2 s → 60 s. If you relied on the old aggressive polling for custom integrations, the new behaviour is deduped and gated.

## v1.4.0-beta.3 (2026-06-28)

Rolling beta with 3 additional commits on top of beta.2: one critical regression fix + two low-risk additions. Two additional critical fixes were cherry-picked on top of beta.3 ahead of the v1.4.0 stable cut.

See `changelogs/v1.4.0-beta.3.md` for the full themed summary with credits.

### Fixed
- **🚨 claudecode `run_as_user` EACCES regression**: chmod `0o644` on per-spawn system-prompt temp file so non-root child processes can read it. Regression introduced by v1.3.4 (#1376) — `run_as_user` users on v1.4.0-beta.1/beta.2 were 100% blocked at agent startup. Reported by @vuyiv (#1429), fixed by @chenhg5 (#1433).
- **🚨 core Send goroutine nil-pointer panic race**: capture `state.agentSession` under `state.mu` before launching `Send` goroutines and add a nil-check fallback inside each goroutine. `cleanupInteractiveState` nils `agentSession` while three `Send` goroutines previously read it without holding the lock; when an agent process exited just before its `Send` was scheduled, the whole cc-connect process would panic and drop every platform connection (#1436, @gotang; follow-up alignment of the third call site in `drainPendingMessages` by @qa-cursor).
- **🚨 Feishu recall-probe quota burn**: throttle the `MessageRecallDetector` fallback path — was polling `GET /im/v1/messages/{message_id}` every 2 s for the same active message, burning ~1.3M Feishu OpenAPI calls/month per stuck session and exhausting the 1M/month free quota. Probe interval now 1 minute, per-message dedup + in-flight guard inside `interactiveState`. Reset on each new active message so recall detection still works for normal turns (#1321, @qvictl).

### Features
- **Zhipu GLM provider presets**: add `z.ai` and `bigmodel` (CN) preset entries to `provider-presets.json` (#1412, @clingnet).

### Chore
- `.gitignore`: add `.worktrees/` for local-only multi-agent scratch dirs (#1443, @chenhg5). Local-only, does not affect binary.

## v1.4.0-beta.2 (2026-06-23)

Rolling beta with 10 additional commits on top of beta.1: 3 QA-found hotfixes + 7 cherry-picked PRs from main. QA has verified all changes.

See `changelogs/v1.4.0-beta.2.md` for the full themed summary with credits.

### Features
- **codex model_catalog_json**: prefer Codex's own `model_catalog_json` as highest-priority model source (#1074, @happyTonakai).
- **`cc-connect send --cwd`**: specify the working directory for send commands (#1380, @MMMarcinho).

### Refactor
- **agent option parsing**: centralize cmd/env option parsing into core with a unified `cmd` field. Touches 39 files across all agent adapters (#1297, @happyTonakai). ⚠️ Broad refactor — monitor agent startup compatibility with existing configs.

### Fixed
- **feishu image batch window**: make `imageBatchWindow` configurable; bump default from 150 ms → 500 ms to better handle mobile multi-image bursts (QA hotfix, @qa-cursor).
- **i18n nav.cron**: translate `nav.cron` for Korean, Japanese, Spanish — was leaking raw "Cron" string (QA hotfix, @qa-cursor).
- **slack streaming card**: stop `chat.update` once payload exceeds Slack's size limit; deliver full reply via fresh `postMessage` instead of crashing with `msg_too_long` (QA hotfix, @qa-cursor).
- **core /history truncation**: make `/history` entry truncation length configurable (#1291, @AaronZ345).
- **core post-restart notification**: queue post-restart notification and dispatch on platform ready (#1388, @chenhg5).
- **claudecode progress card**: emit `EventToolResult` so tool output reaches the progress card (#1407, @coolrockin).
- **dingtalk stream panic**: recover panic in DingTalk `StreamClient` `processLoop` to prevent process crash on closed channel (#1390, fix for issue reported by @gd0094).
- **claudecode run_as_user EACCES**: fix per-spawn system-prompt temp file `EACCES` under `run_as_user` (#1429). The per-spawn temp file written by `writeTempAppendPromptFile` inherited `os.CreateTemp`'s `0600` mode owned by the cc-connect process user (often root under systemd). When the agent was spawned under a different `run_as_user`, it could not read the file and exited before any prompt was loaded. The file is now `chmod 0o644` immediately after write, matching the shared `ensureSharedSystemPromptFile` path. Prompt content is non-secret (a superset of the already-shared base prompt), so `0644` is consistent with the shared file. Does not affect the shared-file path (already `0644` since #1376) or the daemon-mode path resolution (#1419) (#1433, @chenhg5).

## v1.4.0-beta.1 (2026-06-22)

First beta of the v1.4.0 series. **Two new platforms join the family** (Cisco Webex, Matrix with E2EE) plus broader configurability for claudecode/codex, Korean i18n, and a batch of fixes carried forward from late v1.3 development.

The `v1.3.4` Windows-cmdline hotfix (released 2026-06-16 from `release/v1.3.4`) is also rolled into this branch via PR #1378.

See `changelogs/v1.4.0-beta.1.md` for the full themed summary with credits.

### New Platforms
- **Cisco Webex** — first-class Webex Teams adapter with bot identity, message routing, and reply support (#1402).
- **Matrix (with E2EE)** — first-class Matrix adapter including end-to-end encrypted room support (#834).

### Features
- **Slack streaming preview + aggregated turn card**: live streaming preview while the agent is thinking, collapses into a single aggregated turn card on completion (#1333).
- **Feishu after_click card replacement for `cmd:` actions**: rich cards can now replace their content after a button click for `cmd:` action handlers (#1299).
- **Codex custom `system_prompt` / `append_system_prompt` config**: codex agent now honours per-config `system_prompt` and `append_system_prompt` like claudecode does (#1345).
- **Korean (ko) i18n**: Web admin UI now ships Korean translations alongside zh/en/ja (#1343).
- **`plugin_dir` for Claude Code plugins**: claudecode agent supports loading plugins via `--plugin-dir`, exposed as a new `plugin_dir` config option (#1325).
- **`max_attachment_size_mb` for `cc-connect send`**: configurable upper bound for outbound attachment size; default preserved (#1392).
- **`CC_LOG_MAX_BACKUPS` env var**: control daemon log rotation backup count via env (#1260).

### Fixed
- **claudecode (Windows cmdline limit)**: file-based system prompt delivery via `--append-system-prompt-file` lands on main (port of the v1.3.4 hotfix). Windows users on claudecode are no longer affected by the cmd.exe 8192-byte cmdline limit (#1378 porting #1376).
- **feishu (batch images dropped)**: coalesce consecutive image messages from the same session into a single multi-image dispatch (150 ms quiet window) so first-image-of-N is no longer dropped by the `create_time` watermark from PR #1168 (#1408 carrying #1395).
- **feishu (markdown text_color)**: keep `text_color` on `plain_text` elements only; removing it from `markdown` elements fixes rendering glitches (#1278).
- **workspace model persistence**: workspace-level model selections now persist across restarts instead of resetting to default (#1372).
- **acp graceful `/stop`**: new `AgentSessionCanceller` interface lets `/stop` shut a session down gracefully via ACP (#1275).
- **core FIFO drain**: queued messages now drain strictly in FIFO order, preventing earlier queued messages from being dropped as stale when a later one has a higher `create_time` (#1286).
- **skill discovery depth-1 only**: skill scanning no longer recurses into subdirectories. Only `<skill_dir>/<name>/SKILL.md` is registered; nested SKILL.md files are treated as skill assets and ignored, matching Claude Code CLI conventions. Previously, nested SKILL.md leaked into platform command menus as phantom slash commands (101 leaked commands from `frontend-design` skill alone) (#1317 carrying #1304).
- **engine workspace binding under `run_as_user`**: keep workspace binding even when the supervisor cannot stat it under `run_as_user` (#1316).
- **runas workspace**: run the agent in its own workspace under `run_as_user` (#1315).
- **claudecode mid-turn compaction**: keep the turn running when a compaction event arrives mid-turn (fixes #481) (#1272).
- **cron permission lookup with composite keys**: pending permission lookup for cron sessions now resolves correctly when the key is composite (#1067).
- **core (post-restart notification race)**: queue the `/restart` success notification on the engine and dispatch it when the target platform reaches `OnPlatformReady` (with bounded retry + 10s safety timeout) instead of firing immediately after engine startup. Closes the race where notifications were silently dropped on platforms with async connect (Telegram: ~2.6s). Covers Discord / Weixin / Matrix for free (#1388 closing #1383).

### Tests
- Regression test for issue #814 — verifies queued messages use their own `replyCtx` via the outer drain (#1261).

### Docs
- Remove four lapsed sponsor entries from README sponsor section (#1404).

### Behavior Changes
None. Existing configs upgrade without changes. New config options (`plugin_dir`, `max_attachment_size_mb`, codex `system_prompt`/`append_system_prompt`, `CC_LOG_MAX_BACKUPS`) are all optional with safe defaults.

### Breaking Changes
None.

## v1.3.4 (2026-06-16) — Windows hotfix

Emergency single-fix release for a Windows-only regression introduced in v1.3.3.
**Upgrade strongly recommended for Windows users running the claudecode agent.**
Linux / macOS and non-claudecode users are unaffected by the v1.3.3 bug and may
upgrade at leisure. No config changes required. See `changelogs/v1.3.4.md` for the
full root-cause writeup and verification notes.

### Fixed
- **Windows: all messages silently never reply on v1.3.3** ([#1376](https://github.com/chenhg5/cc-connect/issues/1376)).
  Root cause: v1.3.3 expanded `core.AgentSystemPrompt()` from 2707 → 9055 bytes,
  which busted Windows `cmd.exe`'s 8192-byte command-line limit when cc-connect
  spawned `claude.exe` with `--append-system-prompt <inline 9KB>`. Fix: pass the
  prompt to `claude.exe` via `--append-system-prompt-file <path>` instead of inline
  (single shared file at `<data_dir>/agent-prompts/cc-connect-system.md` for the
  99% path; per-spawn temp file for the 1% edge cases like Slack / Weixin / MAX /
  user-configured `append_system_prompt`). Prompt content is **identical** to
  v1.3.3 — only the delivery mechanism changed.

### Scope
3 files changed, +323/-9, all in `agent/claudecode/`. No changes elsewhere.

### Credits
Reporters [@secountAiAccount](https://github.com/secountAiAccount) and
[@softxyz1](https://github.com/softxyz1) for unblocking us within hours of v1.3.3
going stable.

## v1.3.3 (2026-06-15)

First stable release of the 1.3.3 series. Stabilizes the v1.3.3-beta.1 → v1.3.3-beta.5
line (≈ 235 PRs since v1.3.2) plus 7 post-beta.5 fixes. See `changelogs/v1.3.3.md` for
the full themed summary; per-beta details remain in the beta sections below.

### Highlights
- **New agents**: Devin CLI, Google Antigravity (`agy`), GitHub Copilot — all first
  class. Hardened coverage for Cursor, OpenCode, Qoder, Kimi, Pi.
- **Platform expansion**: QQ Bot inline keyboards + file send/receive (OneBot), WeCom
  `SendFile` (WebSocket), Feishu audio + video native media, Slack Assistant API, MAX
  webhook delivery, DingTalk @mentions / richText / image / file inbound, WPS Xiezuo
  (金山协作), broader Weibo DM.
- **Long-running turn hardening**: new `max_turn_time_mins` wall-clock cap with
  soft-stop + force-kill + auto-resume — long bash / test commands can no longer lock
  a session indefinitely.
- **Core commands**: `/timer`, `/cancel`, `/ps` (replaces `/btw`), `cron add --silent`,
  agent-driven TTS.
- **Multi-user / permissions**: reply-to-unauthorized-IM-senders, @mention-tolerant
  permission keywords (`@Bot/permit` ≡ `/permit`), Bridge requires token when enabled.
- **Observability**: blackbox testing framework (P0/P1/P2 + config-switch matrix), CUJ
  test framework, agent-resume regression suite, Pi context-usage reporter.
- **Provider ecosystem**: NekoCode, VisionCoder, AIHubMix, MiniMax M3 presets; Claude
  Code 1M-context Opus + `append_system_prompt` + PermissionRequest hooks; Codex
  `request_user_input` app-server events; configurable shell + shell profile.

### Post-beta.5 fixes (delta from beta.5)
- **qoder**: emit streaming text without dropping final result (#1290)
- **weixin**: use `ilink_user_id` in `getConfigReq` for typing ticket (#1308)
- **daemon**: remove redundant `linger_other.go` that breaks non-linux builds (#1314)
- **wps-xiezuo**: preserve newlines in outbound messages — fixes unreadable `/status`
  (#1361)
- **core**: `/switch` no longer loses history; persist user msgs immediately; add CUJ
  test framework (#1348)
- **core**: queued FIFO drains no longer drop earlier queued messages as stale just
  because a later queued message has a higher `create_time` (#1286)
- **core**: make `/history` entry truncation configurable via
  `[display].history_max_len`, defaulting to 1000; `0` disables truncation (#1291)
- **tts/minimax**: drop `status=2` trailer chunk to stop audio playing twice (#1364)
- **tests**: add provider-resume regression tests for codex / opencode / kimi (#1366)

### ⚠️ Behavior Changes (carried forward from the beta cycle)
All behavior changes from beta.1 → beta.5 remain in effect for v1.3.3. **Most likely
to affect existing configs:**
- `progress_style` default for Telegram & Discord is now `compact` (was `legacy`). Set
  `progress_style = "legacy"` to revert. (#1354)
- QQ Bot default `intents` now include `INTERACTION_CREATE` (bit 26). Custom `intents`
  must include `1<<26` for inline keyboard buttons.
- DingTalk `msgtype=file` inbound now reaches the agent (#1357).
- Engine permission keyword matching is @mention-tolerant: `@Bot/permit` ≡ `/permit`
  (#1358).
- `reset_on_idle_mins` default is now 30 minutes (#494). Set to `0` to disable.
- Bridge with no `[bridge].token` configured will refuse to start (#408).

### Breaking Changes
**None.** Fully additive release.

### Upgrade
```bash
npm i -g cc-connect@1.3.3
# or
go install github.com/chenhg5/cc-connect/cmd/cc-connect@v1.3.3
```

Coming from a `v1.3.3-beta.*`: this is a small fix-only upgrade. No config change
required.

Coming from `v1.3.2`: review the Behavior Changes above before upgrading.

---

## v1.3.3-beta.5 (2026-06-15)

Large beta with 74 PRs from 28 contributors. New agents (Google Antigravity `agy`, GitHub Copilot), QQ
file send/receive via OneBot, WeCom `SendFile` (WebSocket), Feishu audio/video media, agent-driven TTS,
`/timer` and `/cancel` commands, and broad platform fixes across Telegram, Discord, DingTalk, Feishu,
WeCom, WeiXin, Cursor, OpenCode, Pi, and Codex. See `changelogs/v1.3.3-beta.5.md` for the full list.

### New Features
- **Google Antigravity (`agy`)** agent as a first-class integration (#1123)
- **GitHub Copilot** agent as a first-class integration (#865)
- **`/timer`** — one-shot delayed task system (#1012)
- **`/cancel`** — interrupt and reset the current session (#957)
- **Session prune** command to remove duplicate sessions (#603)
- **Agent-driven TTS** send (#1230)
- **Reply to unauthorized IM senders** option (#1190)
- **QQ Bot inline keyboard buttons** and INTERACTION_CREATE events. Permission requests now render as clickable buttons (#1131)
- **QQ (OneBot) file send/receive** via HTTP API (#323)
- **WeCom `SendFile`** in WebSocket mode (#1199)
- **Feishu audio + video attachments** as native media (#1202)
- **Feishu rich card rendering + panel handling** refresh (#1204)
- **DingTalk reaction emoji** support (#1213)
- **DingTalk @mention via `send --at-users` / `--at-all`** (#1188)
- **Slack + tmux** per-thread session scope with per-session tmux windows (#1179)
- **`cron add --silent`** CLI flag (#1285, closes #858)
- **Codex `request_user_input`** app-server events + relay group visibility (#1200, #1209)
- **Claude Code** custom `append_system_prompt` + PermissionRequest hooks (#1175, #850)
- **Pi** `ContextUsageReporter` for reply footer token stats (#1235)
- **Daemon** hardened service-file env capture + EnvDiscoverer plugin hook (#1034)
- **Configurable shell + shell profile** for `exec` (#870)

### Fixed
- Many fixes across the engine, agents, and platforms. Highlights: Telegram/Discord progress style,
  DingTalk file inbound, Feishu link/card URLs, WeCom long-message split, Cursor session titles,
  OpenCode tool rejection, Codex resume + sandbox_mode, Pi `/dir` and `/model`, Windows instance
  lock, and Claude Code provider preservation. See `changelogs/v1.3.3-beta.5.md`.

### ⚠️ Behavior Notes
- **`progress_style` default** for Telegram and Discord is now `compact` (was `legacy`). Set
  `progress_style = "legacy"` in the platform config to restore previous behavior (#1354).
- **DingTalk `msgtype=file`** inbound messages now reach the agent. Previously silently dropped (#1357).
- **Engine permission keyword matching** is now @mention-tolerant: `@Bot/permit` matches the same as
  `/permit` (#1358).

### ⚠️ QQ Bot Intent Configuration
The default intents for QQ Bot now include `INTERACTION_CREATE` (bit 26, value `1<<26`). If you
previously set a custom `intents` value without this bit, inline keyboard buttons will not work —
update your `intents` to include bit 26. If you use the default intents, no action is needed. See
`config.example.toml` for the new `intents` option.

## v1.3.3-beta.4 (2026-05-28)

### New Features
- **`max_turn_time_mins`**: new config option — absolute wall-clock cap per agent turn that does NOT reset on tool-call events. Prevents long-running bash commands from permanently locking the session (#1091). Uses a two-phase shutdown: soft stop (10s grace) then force-kill. Session is preserved and resumed via `--resume` on the next message.

### Fixed
- **Web console 404 regression**: `make release-all` did not depend on `make web`, so release binaries were built without frontend assets when `web/dist/` was empty (gitignored). All routes on the management port returned `404`. Fixed by adding `web` as a prerequisite of `release-all` (#1136)
- **Slack @mention without space**: `stripAppMentionText` only matched `"> "` (with trailing space), so `@Bot/command` (no space) was forwarded verbatim to Claude instead of being parsed as a command
- **DingTalk `msgtype="picture"` dropped**: image messages delivered as `"picture"` (instead of `"image"`) were silently dropped. Both types now route to the image handler (#1128)
- **Feishu `require_mention = false` ignored**: the platform read `group_reply_all` but users set `require_mention = false`; now both are treated as equivalent (#1141)
- **AskUserQuestion resolved with empty answer**: delivery receipts and read-notifications (empty messages) were accepted as valid answers to `AskUserQuestion`, resolving it within ~500ms before the user could respond. Empty/whitespace content is now rejected (#1086)

## v1.3.3-beta.3 (2026-05-24)

Beta release with blackbox testing infrastructure, cursor/opencode agent support, and bug fixes.

### New Features
- **Blackbox testing framework**: Phase 1-2 blackbox testing with P0/P1/P2 coverage, config-switch, and NewEnvWithSetup infrastructure
- **Cursor/OpenCode agents**: add cursor and opencode agent support in blackbox tests

### Fixed
- **Core italic wrapping**: restore italic wrapping on reply footer
- **Feishu footer asterisks**: strip asterisks from footer to prevent Feishu markdown italic rendering
- **Kimi session UUID**: capture session UUID from stderr instead of stdout
- **Codex stdio sentinel**: add stdio sentinel for Codex app_server backend
- **Windows cross-compile**: add missing `CheckLinger` stub to `daemon/windows.go` and `daemon/unsupported.go` so `make release-all` succeeds for all target platforms

## v1.3.3-beta.2 (2026-05-09)

Beta release with Slack Assistant API, DingTalk improvements, MAX platform webhook mode, and numerous platform fixes. No breaking changes.

### New Features
- **Slack Assistant API**: support Slack Assistant API (Agent toggle) with natural on/off switching (#844)
- **DingTalk richText**: support richText message type for DingTalk platform (#828)
- **DingTalk image handling**: add DingTalk image message support (#828)
- **MAX webhook delivery mode**: add webhook delivery mode for MAX messenger platform with deployment docs (#818)
- **Claude Code env vars**: support project-level environment variables via `env` config section (#812)
- **display_mode enum**: add `display_mode` enum to replace boolean `quiet` config, with quiet/compact/normal/full options (#655)
- **Core reset_on_idle_mins default**: default to 30 minutes to prevent context drift (#494)
- **Claude Code custom system prompt**: add support for custom system prompt configuration via `system_prompt` option (#534)

### Fixed
- **Bridge security**: require token when Bridge is enabled to prevent unauthorized access (#408)
- **Feishu recalled messages**: handle recalled messages gracefully (#841)
- **Feishu media download failure**: notify user when media download fails instead of silent drop (#815)
- **WeChat video messages**: send video files as proper video messages in WeChat (#813)
- **WeChat incomplete delivery**: notify user on incomplete message delivery and enhance retry logging (#771)
- **Telegram private topics**: preserve private topic session keys (#804)
- **Kimi session UUID**: capture session UUID from stderr instead of stdout (#766)
- **Codex app_server config**: app_server backend should honor model/effort/provider config + add stdio sentinel (#837)
- **Codex progress rendering**: render progress in rich Card 2.0 format (#838)
- **Core ellipsis events**: suppress ellipsis-only events and handle context indicator in footer
- **Core Markdown table**: render inline formatting inside GFM table cells (#675)
- **Feishu user id resolution**: guard user id resolution against edge cases
- **Feishu thread topics**: skip quote injection in thread-isolated topics (#767)
- **Config display mode**: honor project display mode setting
- **Daemon restart**: add --force flag to daemon restart command (#736)
- **AskUserQuestion**: use question text as answers key for proper answer routing (#822)

## v1.3.3-beta.1 (2026-04-25)

Beta release with new agents, new features, and broad platform fixes. No breaking changes.

### New Features
- **Devin agent**: add Devin CLI as a first-class agent with full `/list`, `/mode`, and session management (#672)
- **`/ps` command** (replaces `/btw`): send a message to a busy session mid-turn; `/btw` kept as alias for backward compatibility (#620)
- **`!` shell shortcut**: use `!ls -la` as shorthand for `/shell ls -la`, with optional `--timeout` parameter (#658)
- **NO_REPLY suppression**: agents can return `NO_REPLY` to silently skip platform delivery, useful for cron/analysis tasks (#682)
- **Feishu shared WebSocket**: multiple projects sharing the same `app_id` now share one WebSocket connection with per-project `allow_chat` / `group_only` filtering (#613)
- **Message queue depth configurable**: new `[queue] max_depth` config option (default 5) (#690)
- **Claude Code opus[1m]**: add 1M-context Opus model option with shorthand descriptions (#660)
- **QQ Bot file send/receive**: full file attachment support with robustness checks (#685)
- **Bridge ImageSender/FileSender**: `cc-connect send --image/--file` now works through bridge protocol (#712)
- **Provider presets**: add NekoCode, VisionCoder, and AIHubMix to provider presets; add Trae CLI ACP and COCO ACP config examples (#739)

### Fixed
- **OpenCode image handling**: inbound images from WeChat/WeCom are now correctly passed to OpenCode CLI via `--file` flags (#717)
- **Slack Markdown**: convert standard Markdown to Slack mrkdwn format (bold, italic, strike, links, headings) (#680)
- **QQ Bot reconnect**: cancel stale goroutines on WebSocket reconnect to prevent race conditions (#678)
- **Gemini multiline prompt**: pass prompt via stdin to preserve newlines (#695)
- **Telegram HTML fallback**: upgrade silent HTML parse failures to Warn-level logs (#674)
- **Telegram /skills**: show Telegram-safe skill command format (#571)
- **Feishu webhook mode**: skip bot open_id fetch in webhook mode for private deployments (#696)
- **Reply footer**: suppress footer when only workdir is known (#701)
- **Web UI add-platform**: fix "project not found" error when adding a new platform to an uncreated project

### Contributors
Thanks to all contributors who made this release possible:
- @YoungShook — Devin agent integration, Telegram HTML fallback
- @Cigarrr — /ps command, NO_REPLY feature
- @vinnyxiong — Feishu shared WebSocket and allow_chat
- @happyTonakai — Shell `!` prefix and `--timeout`
- @AaronZ345 — Claude Code opus[1m] model
- @ferocknew — QQ Bot file support
- @soaringk — OpenCode image fix
- @Zx55 — Telegram /skills fix
- @zhaomoran — Feishu webhook mode fix
- @LyInfi — Reply footer suppression
- @meloalright — Trae/COCO ACP config examples

## v1.3.2 (2026-04-21)

Hotfix release: session filtering is now configurable and defaults to showing all sessions.

### Fixed
- **`/list` shows all sessions by default**: the session filter introduced in v1.3.0 (which hid sessions not created by cc-connect) was accidentally merged and caused confusion. The filter is now **off by default** — `/list`, `/switch`, and `/delete` show all agent sessions regardless of origin.

### Added
- **`filter_external_sessions` config option**: users who *do* want to hide externally-created sessions can set `filter_external_sessions = true` in `[[projects]]` to restore the old filtering behavior.
- **Comprehensive integration tests**: real-agent E2E tests for both Codex and Claude Code covering the full `/list` → `/new` → conversation → `/list` lifecycle with provider-based authentication (no env-var API keys required). Plus 9 adapter-level filter tests using real Codex/Claude Code session file fixtures.

## v1.3.1 (2026-04-20)

Patch release with critical bug fixes for session management, config preservation, and Weibo media support.

### Fixed
- **Session visibility (`/list`)**: historical Codex sessions disappeared after upgrade due to `AgentSessionID` being cleared on `/new` or provider switch without preservation. Added `PastAgentSessionIDs` tracking with legacy data migration so existing sessions remain visible.
- **Session naming (`/new xxx`)**: custom session names from `/new` were not mapped to the agent session ID for agents where the ID is established asynchronously (Codex, Qoder, Kimi, etc.). Added name mapping to all `EventResult` and `EventText` handlers across interactive, relay, and drain paths.
- **Config comment preservation**: `/provider switch`, `/model`, `/lang`, display settings, and TTS changes now use surgical text-level editing instead of full TOML re-serialization, preserving all comments, unknown fields, and formatting.
- **Codex `codex_home` path**: session listing, history, and deletion now consistently use the configured `codex_home` instead of hardcoded `~/.codex`.
- **Feishu card callback hint**: log a reminder when interactive card mode is enabled but `card.action.trigger` may not be subscribed.

### Added
- **Weibo image & file support**: send and receive images and files in Weibo DMs via base64 encoding within the WebSocket `send_message` payload. Implements `ImageSender` and `FileSender` interfaces.
- **Comprehensive session tests**: 12 new `SessionManager` unit tests covering `PastAgentSessionIDs`, legacy data migration, and version-based schema detection. 9 new `Engine` integration tests covering `/list` visibility across `/new`, provider switch, and real-world legacy data scenarios, plus end-to-end session name mapping tests for all three agent ID patterns (immediate, EventText, EventResult).
- **Config preservation tests**: 8 new tests verifying comment and field preservation for `SaveActiveProvider`, `SaveAgentModel`, `SaveProviderModel`, `SaveLanguage`, `SaveDisplayConfig`, `SaveTTSMode`, multi-project config, and global provider refs.

## v1.3.0 (2026-04-19)

First stable release of the 1.3 series. 555 commits since v1.2.1 with major new features, platform improvements, and broad community contributions.

### Highlights

- **Web Admin UI** — Full management dashboard embedded in the binary via `go:embed`. Project CRUD, session monitoring, cron editor, provider management, chat interface, and i18n (en/zh/zh-TW/ja/es). Use `cc-connect web` to open directly in the browser with auto-login.
- **Lifecycle Event Hooks** — New `[[hooks]]` config to trigger shell commands or HTTP webhooks on 7 event types: `message.received`, `message.sent`, `session.started`, `session.ended`, `cron.triggered`, `permission.requested`, `error`. Async by default, fail-open, non-blocking.
- **Skill Management** — New `/skills` page in the web UI with local skill browser (per-project, per-agent) and recommended skill presets fetched from remote.
- **Global Provider Management** — Add, edit, delete providers in the web UI; import from cc-switch config; per-agent-type provider presets with featured/star badges.

### New Features
- `cc-connect web` CLI command: auto-configure web admin, open browser with token-based login
- Feishu: auto-resolve `@name` mentions to clickable at-tags (`resolve_mentions` config)
- Feishu: multi-level reply chain recognition; done-emoji reaction after streaming
- Feishu: configurable progress display styles (compact/card)
- Claude Code: support CLI wrappers via `cli_path`; `/effort` command for reasoning effort; `auto` permission mode; `disallowed_tools` config
- Codex: runtime reply footer; preserve workspace app-server options
- Kimi CLI: new agent support
- Pi: new agent support
- Discord: preserve table formatting; proxy support; `@everyone`/`@here` broadcast
- Telegram: forum topic support; markdown table monospace rendering; command menu adaptation
- WeCom: configurable `api_base_url` for private deployments; file receiving via HTTP callback
- Weixin (ilink): personal chat platform with CDN media, QR setup, image/file/audio send
- Config: support `${ENV_VAR}` placeholders in TOML values
- Core: `/workspace init` with local directory paths; `/dir` directory history; `agent-sid` command; auto-compress context on token threshold; outgoing rate limiting
- Daemon: preserve proxy env in systemd service

### Bug Fixes
- Fix Windows cross-compilation (duplicate runas stub file)
- Fix web footer double 'v' prefix in version display
- Fix web modal overlay not covering full viewport (portal rendering)
- Fix provider preset cards: action buttons pinned to card bottom
- Fix web page content overlapping footer (global layout restructure)
- Fix Gemini image handling: save to workspace, prompt-based file references
- Fix Claude Code: unblock readLoop when child subprocesses hold stdout pipe
- Fix Codex: multiline prompt on resume; force-kill process group on stop
- Fix core: race condition during session cleanup; follow symlinked skill directories; persist agent_session_id; filter `/list` to cc-connect owned sessions
- Fix Feishu: slash commands in thread/reply context; user/chat name resolution in async goroutine
- Fix Telegram: UTF-8-safe command menu descriptions
- Fix TTS: don't send empty language_type to Qwen TTS API
- Fix config: `formatTOML` no longer strips user-set zero values
- Security: mask bridge token in `/api/v1/status`; path traversal protection for static files

### Contributors

Thanks to all contributors who made this release possible:

- [@leoliang1997](https://github.com/leoliang1997) — Feishu card rendering, auto-resolve @mentions
- [@xukp20](https://github.com/xukp20) — Provider env handling, skill discovery, Codex options
- [@boyu-zhu](https://github.com/boyu-zhu) — Telegram markdown table rendering
- [@RukawaKaede](https://github.com/RukawaKaede) — Claude Code CLI wrapper support
- [@meishaoqing](https://github.com/meishaoqing) — Feishu multi-level reply chain
- [@Zx55](https://github.com/Zx55) — Telegram command menu, symlinked skill dirs
- [@leighstillard](https://github.com/leighstillard) — Claude Code `/effort` command
- [@ht290](https://github.com/ht290) — inject_sender display name
- [@Sentixxx](https://github.com/Sentixxx) — Claude Code readLoop subprocess fix
- [@bugwz](https://github.com/bugwz) — WeCom private deployment API base URL
- [@cold2600438-lgtm](https://github.com/cold2600438-lgtm) — Kimi CLI agent
- [@MeteorSkyOne](https://github.com/MeteorSkyOne) — Discord table formatting
- [@happyTonakai](https://github.com/happyTonakai) — Feishu done-emoji reaction
- [@xxb](https://github.com/xxb) — Codex reply footer, Discord session routing
- [@q107580018](https://github.com/q107580018) — Feishu delete/model card flows
- [@Cigarrr](https://github.com/Cigarrr) — Workspace binding parsing
- [@g1f9](https://github.com/g1f9) — Local directory workspace init
- [@0xsegfaulted](https://github.com/0xsegfaulted) — agent-sid command
- [@yzlu0917](https://github.com/yzlu0917) — Env var config placeholders
- [@sidney061212-ai](https://github.com/sidney061212-ai) — Agent session ID persistence
- [@zkunzhu](https://github.com/zkunzhu) — Daemon proxy env preservation
- [@Yuri0314](https://github.com/Yuri0314) — TTS language type fix

## v1.2.2-beta.5 (2026-03-31)

Beta release with embedded web admin, Discord proxy support, multimodal fixes, and major platform improvements.

### New Features
- **Embedded Web Admin**: Web frontend is now compiled into the binary via `go:embed` — no separate `npm install` needed. Use `/web setup` to configure, or build with `no_web` tag to exclude. Binary size increases ~1MB (#356)
- **Web Admin Dashboard**: Full-featured management UI with project CRUD, session management, cron job editor, global settings, chat interface with bridge WebSocket, slash commands, and i18n (en/zh/zh-TW/ja/es) (#316)
- **Discord Proxy Support**: Discord platform now supports `proxy`, `proxy_username`, `proxy_password` options for HTTP API and WebSocket Gateway connections
- **Feishu Progress Styles**: Configurable progress display styles (compact/card) to reduce message spam
- **Claude Code Auto-Permission Mode**: New `auto` permission mode for Claude Code agent (#329)
- **WeCom File Receiving**: WeCom HTTP callback now supports receiving files and forwarding them to the agent (#330)
- **Outgoing Rate Limiting**: Per-platform outgoing message rate limiting
- **Telegram Forum Topics**: Migrated to `go-telegram/bot` library with forum topic support (#321)
- **Global Settings UI**: Expose global configurations (language, quiet, display, stream preview, rate limit, log) in the web admin

### Bug Fixes
- **Gemini Image Handling**: Save attachments to workspace directory instead of `/tmp` so Gemini CLI tools can access them; use prompt-based file references instead of unsupported `--image` flag
- **Security**: Mask bridge token in `/api/v1/status` endpoint; add path traversal protection for static file serving
- **Codex**: Fix multiline prompt preservation on resume (#341); force kill session process group on stop (#340)
- **Session Recycling**: Wait for old session to close before creating new one (#352)
- **Discord**: Harden session routing and remove implicit continue bridge (#322); execute slash commands when defer fails (#300)
- **Slack**: Pass file uploads to agent (#296)
- **Telegram**: UTF-8-safe command menu descriptions (#301)
- **WeCom**: Strip @bot mentions from inbound text (#303)
- **Daemon**: macOS launchd do not respawn on clean exit (#304)
- **Core**: Route workspace model changes through session context (#339); outgoing rate limit refinements and i18n tightening
- **Config**: `formatTOML` no longer strips user-set zero values (e.g. `quiet = false`)

### Improvements
- **CI**: Add Node.js setup for web frontend build in CI pipeline; use `no_web` tag for e2e/smoke tests
- **Tests**: Expanded coverage across agents, config, and core packages
- **Selective Compilation**: Added `no_web` build tag to exclude web assets from binary

### Contributors

Special thanks to all contributors who made this release possible:

- **cg33** — Embedded web admin, Discord proxy, Gemini fix, security hardening
- **xxb** — Discord session routing fix, codex process kill, workspace reconnect (#322, #340, #315)
- **dev-null-sec** — Codex multiline prompt fix (#341)
- **xukp20** — Workspace model routing (#339)
- **zhengbuqian** — Telegram go-telegram/bot migration and forum topics (#321)
- **huangdijia** — Claude Code auto permission mode (#329)
- **buddhism5080** — Discord file sending (#307)

## v1.2.2-beta.4 (2026-03-22)

Beta release with Weixin (ilink) personal chat support, session/continue improvements, and platform fixes.

### New Features
- **Weixin Personal (ilink)**: New platform with long-poll `getUpdates` / `sendMessage`, QR `weixin setup`, CDN decrypt for inbound media and `ImageSender`/`FileSender` outbound (#257)
- **Telegram**: Voice/audio reply support (#225) and async startup recovery
- **Discord**: `@everyone` / `@here` broadcast support (#132)
- **Cron**: Optional new session per run and per-job timeout (#236)
- **Claude Code**: `disallowed_tools` configuration option (#232)
- **Auto-Compress**: Compress context when estimated tokens exceed threshold (#231)
- **Continue / Sessions**: Fork session on `--continue` to avoid context contamination (#244); replace persisted `ContinueSession` sentinel with real agent session id; reserve CLI `--continue` bridge for real user traffic
- **Core**: `/dir` directory history; `/model` switching aligned with provider flow (#246)
- **Providers**: MiniMax M2.7 high-speed model added to example configs (#217)

### Bug Fixes
- **Weixin**: Harden send path (empty body skip, response body cap, dedup keys, multi-voice segments); treat `sendMessage` JSON `ret != 0` as failure so quota/API errors surface correctly
- **Feishu**: Always reply to the original message; dispatch message handling asynchronously (#57)
- **Codex**: Mode switch and `--json` flag position fixes (#240, #239)
- **Multi-Workspace**: Workspace command prefix missing leading slash (#135)
- **Non-Claude Agents**: Ignore `ContinueSession` sentinel where inappropriate (#244 follow-up)
- **npm / Update**: Version sync after update; pre-release version comparison normalization

### Improvements
- **Tests**: Expanded coverage across `config`, `core`, agents, and platforms
- **Logging / Errors**: Additional error logging in several code paths

### Contributors

Special thanks to all contributors who made this release possible:

- **cg33** — Weixin ilink platform, setup CLI, and CDN media (#257)
- **Shawn** — Feishu async dispatch and reply-to-original fixes (#57)
- **quabug** — Discord broadcast and non-Claude ContinueSession handling (#132, #244)
- **huluma1314** — Auto-compress when token threshold exceeded (#231)
- **Leigh Stillard** — Fork session on `--continue` (#244)
- **Deeka Wong** — Telegram audio replies and core `/model` provider flow (#225, #246)
- **q107580018** — Telegram async startup recovery
- **just4zeroq** — Codex mode and JSON flag fixes (#240)
- **术士木星** — Cron session-per-run and job timeout (#236)
- **hushicai** — Claude `disallowed_tools` (#232)
- **Octopus** — MiniMax M2.7 high-speed in examples (#217)
- **alinnb** — `/dir` directory history
- **Claude** — Continue-session bridge fixes, auto-compress/cron edge cases, Weixin send hardening and API error handling, and broad test improvements

## v1.2.2-beta.3 (2026-03-19)

Beta release with major multi-user mode, improved workspace stability, and platform enhancements.

### New Features
- **Multi-User Mode**: Per-user rate limits, role-based ACL (allow_from/admin_from), and audit logging
- **ImageSender**: Unified image sending support for 6 platforms (Feishu, Telegram, Discord, Slack, DingTalk, QQ)
- **MiniMax M2.7**: Upgraded default model from M2.5 to M2.7 for improved reasoning
- **/whoami Command**: Display user ID for allow_from/admin_from configuration
- **/btw Command**: Inject messages into busy sessions without interrupting
- **/dir Command**: Dynamic runtime work directory switching
- **Cron Muting**: Mute/unmute cron jobs with platform wrapper and UI integration
- **Interrupt Support**: Send interrupt signal to agent sessions (Ctrl+C equivalent)
- **CORS Support**: Cross-origin requests enabled for Bridge API
- **Message Queuing**: Queue messages when agent is busy instead of discarding
- **QQ Bot Markdown**: Full Markdown message support for QQ Bot

### Bug Fixes
- **Workspace Session Persistence**: Sessions now persist to disk in multi-workspace mode
- **Race Conditions**: Multiple data race fixes (adminFrom, degraded field, userRolesMu)
- **Memory Leaks**: Fixed pendingAcks leak on WeCom WebSocket disconnect, goroutine leaks
- **i18n**: Complete translation coverage for error messages
- **Relay Timeout**: Return partial text after timeout instead of error
- **QQ Bot Reconnect**: Handle nil wsConn on failed reconnect

### Improvements
- **Message Queue**: Extracted message queue handling into dedicated method
- **Cron UX**: Improved human-readable cron expressions
- **Slack**: Typing indicator, file download error handling, auth diagnostics
- **Provider Config**: `models` list for per-provider model selection via alias
- **Build**: Test infrastructure with P0/P1分层测试targets

### Contributors

Special thanks to all contributors who made this release possible:

- **sean2077** - Multi-user mode, ACL, and audit logging
- **0xsegfaulted** - Multi-workspace fixes and interrupt support
- **octo-patch** - MiniMax M2.7 upgrade
- **windli2018** - Bridge CORS support
- **jenvan** - CORS fixes

## v1.2.2-beta.2 (2026-03-16)

Beta release with significant improvements to agent stability, platform onboarding, and user experience.

### New Features
- **Feishu/Lark CLI Onboarding**: New `cc-connect feishu setup` command with QR code terminal display for quick bot configuration, supporting both new bot creation and existing bot binding
- **Pi Agent**: Added support for Pi coding agent with full session management and tool handling
- **Session TUI Browser**: New `cc-connect sessions` subcommand with terminal UI for browsing session history
- **Multi-Workspace Mode**: Channel-based workspace resolution with auto-binding by convention and interactive init flow
- **Design Documentation**: Added comprehensive design plans for multi-workspace and session resilience features
- **Slack Enhancements**: Typing indicator via emoji reactions, mrkdwn formatting guidance in system prompt
- **Session Resilience**: Automatic `--continue` on first connection, resume-failure fallback, and context usage indicators
- **Management API**: HTTP REST API endpoints for external management tools with WebSocket bridge support
- **Cron Setup Command**: `/cron setup` for easy cron job configuration with memory file integration

### Bug Fixes
- **RateLimiter Goroutine Leak**: Fixed cleanup goroutine not stopped on replacement and engine shutdown
- **DrainEvents Infinite Loop**: Fixed infinite loop when channel is closed in `drainEvents`
- **InteractiveKey Consistency**: Fixed `executeCardAction` using wrong key for `interactiveStates` lookup in multi-workspace mode
- **Workspace Command Prefix**: Fixed missing leading slash in workspace command prefix check
- **Agent Session Close**: Always close events channel on session timeout to prevent goroutine leaks
- **Pi Agent Mutex**: Move thinking field read inside mutex in `StartSession` to prevent race condition
- **Session AgentID Protection**: Protect `Session.AgentSessionID` writes with mutex to prevent data races
- **Session Routing Race**: Prevent session routing race when `/new` runs during active turn
- **Discord Duplicate Messages**: Deduplicate gateway `MessageCreate` events causing duplicate responses
- **Codex JSON Lines**: Handle large stdout JSON lines without scanner buffer overflow
- **UTF-8 Safety**: Use rune-based splitting in `splitMessage` to prevent invalid UTF-8 sequences

### Improvements
- **Gemini Display**: Enhanced tool display with diff syntax highlighting and improved Telegram markdown rendering
- **Thread Safety**: Added comprehensive thread-safe accessors for Session fields
- **Test Engine**: Thread safety improvements to test engine and fixed test assertions
- **Input Validation**: Consolidated interactive state cleanup and added input validation
- **i18n**: Updated rate limit messages to mention `/btw` command for adding context during processing

### Contributors

Special thanks to all contributors who made this release possible:

- **kevinWangSheng** - Multiple critical bug fixes (RateLimiter, drainEvents, UTF-8 safety, session routing)
- **q107580018** - Feishu CLI onboarding with QR code integration
- **sean2077** - Session TUI browser and sessions management
- **quabug** - Pi agent implementation and Discord fixes
- **AtticusZeller** - Gemini tool display and Telegram markdown enhancements
- **leighstillard** - Multi-workspace design, session resilience, and Slack improvements
- **Shawn** - Thread safety fixes and test improvements
- **zhuguanqi** - Session management and data race fixes
- **Steve-Rye** - JSON lines handling improvements
- **Xihui He** - iFlow and agent enhancements
- **Mr.QiuW** - Various platform improvements

## v1.2.2-beta.1 (2026-03-12)

Beta release with major new features and security improvements.

### New Features
- **`/usage` Command**: Add a built-in quota usage command with a generic agent usage-reporting interface; Codex now supports ChatGPT OAuth usage lookup via `~/.codex/auth.json`
- **Feishu Interactive Cards**: Beautiful card-based UI for slash commands (/help, /list, /status, etc.) with tabbed navigation and in-place updates
- **Lark Platform Support**: Added support for Lark (飞书国际版) with proper domain handling
- **Codex Reasoning Effort**: New `/reasoning` command to switch reasoning effort levels (low/medium/high)
- **Codex Model Cache Fallback**: `/model` command now falls back to local `~/.codex/models_cache.json` when API is unavailable
- **Gemini Timeout Config**: New `timeout_mins` option to configure per-turn timeout for Gemini agent
- **Batch Session Deletion**: `/delete` now supports comma lists, ranges, and mixed forms for batch deletion
- **TTS Support**: Text-to-speech with Qwen and OpenAI providers
- **Admin Privilege System**: Admin-only commands for privileged operations
- **iFlow Tool Timeout**: Configurable tool timeout and reset timer on partial completion
- **Card-based Permission Prompts**: Permission requests now use interactive cards with callback support
- **Shared Session Support**: Share sessions across all platforms with `share_session_in_channel` option

### Bug Fixes
- **Security Hardening**: Socket permissions tightened (0600), token redaction in logs, warning for open `allow_from`
- **Slack @mention Support**: Fixed AppMentionEvent handling for channel @mentions
- **Update Fallback**: Self-update now falls back to .tar.gz/.zip archive when bare binary returns 404
- **Skill Symlink**: Fixed skill directory scanning to follow symbolic links
- **QQBot Error Handling**: Added error logging for json.Unmarshal and WriteJSON calls
- **Claude Code Path**: Fixed underscore handling in findProjectDir path matching

### Improvements
- **Daemon Config Flag**: Support daemon install with config file path
- **Message Tracing**: Added message tracing and threaded replies
- **Scanner Buffer**: Optimized scanner buffer sizes for large outputs

## v1.2.1 (2026-03-09)

Patch release with bug fixes and minor enhancements.

### Bug Fixes
- **Engine: Idle Timer During Permission Wait** - Stop idle timer while waiting for user permission response to prevent session termination
- **Feishu: Nil Pointer Checks** - Add nil checks for `SenderId.OpenId` and `msg.Content` to prevent panics
- **Feishu: URL Validation** - Validate URLs before creating hyperlinks to prevent rejection of non-HTTP(S) URLs
- **Cron: Error Logging** - Log `json.Unmarshal` errors instead of silently ignoring when cron file is corrupted
- **Engine: Stale Event Prevention** - Add `drainEvents` utility to clear buffered events between turns

### New Features
- **Bind Setup Command** - `/bind setup` writes relay instructions to memory file for better bot-to-bot relay configuration

## v1.2.0 (2026-03-08)

This is the first stable release of cc-connect 1.2.0, consolidating all beta changes and adding new features.

### New Features (since beta.7)
- **Official QQ Bot Platform**: Native integration with Tencent's official QQ Bot Platform via WebSocket, supporting text, image, and document messages
- **iFlow CLI Agent**: Full support for iFlow CLI agent with interactive tool-call handling and mode switching
- **Shell Command Execution**: Custom commands can execute shell commands directly with `exec` field in config
- **Telegram Bot Menu**: Auto-register bot command menu on startup for better discoverability
- **DingTalk Reply Preprocessing**: Improved markdown content preprocessing for reply messages
- **Multi-Bot Relay Persistence**: Relay bindings now persist across restarts with improved binding messages

### Improvements
- **Quiet Mode**: `/quiet` now supports both per-session and global scope modes
- **Compression Command**: Improved `/compress` command handling and code refactoring
- **i18n**: Added new message keys and improved command formatting

### All 1.2.0 Highlights (from beta releases)
- **Bot-to-Bot Relay**: Forward messages between different messaging platforms
- **Streaming Preview**: Real-time message preview on Telegram, Discord, and Feishu
- **Typing Indicators**: Visual processing feedback on supported platforms
- **Session Search**: Search sessions by name, ID prefix, or summary
- **Custom Slash Commands**: Define reusable prompt templates
- **Agent Skills Discovery**: Auto-discover and invoke user-defined skills
- **Daemon Mode**: Run as background service with systemd/launchd support
- **Rate Limiting**: Per-session sliding-window rate limiter
- **Command Aliases**: Define shortcut aliases for commands
- **Self-Update**: In-place binary updates with auto-restart
- And many more improvements and bug fixes...

## v1.2.0-beta.7 (2026-03-07)

### New Features
- **Multi-Bot Relay Binding**: `/bind` now supports binding multiple bots in a group chat; use `/bind <project>` to add, `/bind -<project>` to remove specific project
- **System-level Systemd**: Daemon mode now supports system-level systemd (`/etc/systemd/system/`) when running as root, useful for servers and containers
- **Config Example Command**: `cc-connect config-example` prints embedded config template for quick reference
- **Interactive Command Buttons**: `/lang`, `/model`, `/mode` commands now show interactive button menus for easy selection
- **Exec Commands**: Custom commands can execute shell commands directly with `exec` field in config
- **Configurable Idle Timeout**: Agent idle timeout can be configured via `idle_timeout_mins` in config

### Improvements
- **Daemon Error Messages**: Improved systemd detection and error messages for WSL2, containers, and SSH environments
- **Codex CLI Visibility**: Patched codex session source to make CLI output visible

### Bug Fixes
- **Streaming Preview**: Fixed stale preview messages when streaming degrades

## v1.2.0-beta.6 (2026-03-06)

### New Features
- **Bot-to-Bot Relay**: Forward messages between different messaging platforms via CLI (`cc-connect relay`) and internal API; enables cross-platform bot communication
- **Session Search**: Search sessions by name, ID prefix, or summary with `/search <keyword>` command
- **List Pagination**: `/list` now supports pagination with `--page` and `--page-size` flags for large session counts
- **Per-Platform Streaming Preview Control**: Configure streaming preview per platform via `streaming_preview` setting (Telegram, Discord, Feishu)
- **Silent Cron Mode**: Suppress cron job notification messages with `silent = true` in cron job config
- **Voice Qwen Mode**: Voice function now supports Qwen audio model for speech-to-text
- **Feishu Three-Tier Rendering**: Intelligent markdown rendering strategy — simple text uses plain messages, rich markdown uses Post, code blocks/tables use Card

### Improvements
- **Status Display**: Improved `/status` command output with better formatting and Feishu message rendering fixes
- **Self-Update**: Auto-restart after update; added Gitee mirror support for Chinese users
- **Windows Self-Update**: Full Windows support for in-place binary updates
- **Message Splitting**: Improved boundary checks for cleaner message chunking
- **Platform Startup**: Better error handling and logging during platform initialization
- **Session Switch i18n**: Added translation for session switch success message

### Bug Fixes
- **Idle Session Timeout**: Added timeout for unresponsive agent sessions to prevent hangs
- **Streaming Preview**: Removed `maxChars` check that caused premature preview termination
- **Message Deduplication**: Deduplicate messages by process start time to prevent duplicate processing

## v1.2.0-beta.5 (2026-03-06)

### New Features
- **Streaming Preview**: Real-time message preview that updates in-place as the agent streams output; supported on Telegram, Discord, and Feishu with configurable interval, min delta, and max length
- **Rate Limiting**: Per-session sliding-window rate limiter to prevent message flooding; configurable `max_messages` and `window_secs`
- **Typing Indicators**: Visual processing feedback — Telegram/Discord show native typing action, Feishu adds emoji reaction (auto-removed on completion)
- **Command Aliases**: Define shortcut aliases for commands (`[[aliases]]` in config.toml or `/alias add`); e.g. map "帮助" → "/help"
- **Banned Words Filter**: Block messages containing configured sensitive words (`banned_words` in config.toml)
- **Project-level Command Disabling**: Disable specific commands per project via `disabled_commands` config
- **Session Deletion**: Delete sessions with `/del` command
- **`/switch` Fuzzy Matching**: Switch sessions by name, ID prefix, or summary substring in addition to numeric index

### Improvements
- **Streaming Preview + Tool Messages UX**: In non-quiet mode, when thinking/tool messages are sent, the streaming preview freezes and the final response is delivered as a new message at the bottom of the chat (instead of silently updating an older message above the tool messages)
- **Telegram Markdown→HTML**: Full Markdown-to-HTML conversion with proper escaping, placeholder-based tag nesting, and automatic fallback to plain text on parse errors
- **Discord Code-Fence-Aware Splitting**: Message chunking now respects code block boundaries, closing and re-opening fences across splits
- **Feishu Dual Rendering**: Simple markdown uses Post messages (normal font), code blocks/tables use Card messages (native rendering); matches Claude-to-IM's approach
- **Feishu Permission Interaction**: Confirmed WebSocket mode incompatibility with card button callbacks; uses text-based `/perm` commands (consistent with Claude-to-IM)
- **Session Creation & Naming**: Improved session naming with last user message as summary
- **Graceful Shutdown**: Improved context handling and lock release during shutdown
- **Unit Tests**: Added ~50 new test cases covering markdown conversion, message splitting, session management, and engine logic

### Bug Fixes
- **Telegram HTML Crossed Tags**: Fixed `<b><i>...</b></i>` nesting issues by using placeholder-based formatting pipeline
- **Telegram HTML Attribute Escaping**: Fixed `"` in URLs breaking `<a href>` attributes (escape to `&quot;`)
- **Telegram Duplicate Messages**: Fixed duplicate sends caused by streaming preview optimization skipping final HTML update
- **Streaming Preview Cursor**: Removed trailing `▍` cursor from final messages
- **Feishu Message Recall**: Unified preview and final message types to Card, eliminating unnecessary delete-and-resend
- **Feishu Reaction Cleanup**: Register empty handler for `im.message.reaction.deleted_v1` to suppress error logs
- **`fmt.Sprintf` Warnings**: Remove non-constant format strings flagged by `go vet`

## v1.2.0-beta.2 (2026-03-01)

### New Features
- **`/upgrade` Command**: Check for available updates (including beta) and self-update the binary in-place; queries both GitHub and Gitee releases
- **`/restart` Command**: Restart cc-connect service from chat with post-restart success notification
- **`/config reload` Command**: Hot-reload configuration (display, providers, commands) without restarting
- **`/name` Command**: Set custom display names for sessions (e.g. `/name my-feature`, `/name 3 bugfix`); names persist across restarts and show in `/list`, `/switch`, `/status`
- **Default Quiet Mode**: Configure `quiet = true` globally or per-project in config.toml to suppress thinking/tool progress by default; users can still toggle with `/quiet`
- **Command Prefix Matching**: Type shortened commands like `/pro l` for `/provider list`, `/sw 2` for `/switch 2`; works for all commands and subcommands
- **Numeric Session Switching**: `/list` shows numbered sessions; `/switch 3` switches by number instead of copying long IDs
- **Group Chat Mention Filtering**: Feishu, Discord, and Telegram bots now only respond to @mentions in group chats instead of all messages
- **Claude Code Router Support**: Integration with Claude Code Router for enhanced routing capabilities
- **Third-party Provider Proxy**: Local reverse proxy rewrites incompatible `thinking` parameters for third-party LLM providers (e.g. SiliconFlow)

### Improvements
- **Session History for Claude Code**: `/history` now works after `/switch` by reading from agent JSONL files
- **List Summary**: `/list` now shows the most recent user message as summary instead of the first
- **Session Names in UI**: Custom session names display with 📌 prefix in `/list`, `/switch`, `/status`
- **API Server Shutdown**: Clean shutdown without "use of closed network connection" error
- **Agent Session Timeouts**: 8-second graceful shutdown timeout for all agent sessions with kill fallback
- **Feishu Rich Text**: Use Post (rich text) messages instead of Interactive Cards for normal font size

### Bug Fixes
- **DingTalk Startup**: Fix false startup failure when stream client returns nil error
- **Deadlock on /new and /switch**: Release lock before async agent session close to prevent hangs
- **Provider Command**: Correctly list providers when no active provider is set
- **Unknown Command Handling**: Show i18n-friendly warning and fall through to agent for native commands

### Security & Reliability
- **Race Condition Fixes**: `sync.Once` for channel close, mutex protection for concurrent fields, non-blocking event sends
- **Atomic File Writes**: Config, session, and cron files use temp+rename pattern
- **Message Deduplication**: Platform-level dedup for Feishu and DingTalk webhooks
- **HTTP Client Timeouts**: Shared 30s-timeout HTTP client for all outbound requests
- **Path Traversal Protection**: Validate command file paths
- **Sensitive Data Redaction**: Redact API keys and tokens in logs

## v1.2.0-beta.1 (2026-03-01)

### New Features
- **Custom Slash Commands**: Define reusable prompt templates as global slash commands (`[[commands]]` in config.toml or `/commands add`); supports positional parameters (`{{1}}`), rest parameters (`{{2*}}`), default values (`{{1:default}}`), and runtime add/del/list
- **Agent Skills Discovery**: Auto-discover and invoke user-defined skills from agent directories (e.g. `.claude/skills/<name>/SKILL.md`); list with `/skills`, invoke with `/<skill-name> [args]`; supports all agents (Claude Code, Cursor, Gemini, Codex, Qoder)
- **`/config` Command**: View and modify runtime configuration (e.g. `thinking_max_len`, `tool_max_len`) from chat, with persistent save to `config.toml`
- **`/doctor` Command**: Run system diagnostics covering agent authentication, platform connectivity, system resources, dependencies, and network latency; fully i18n-supported
- **Discord Slash Commands**: Register native Discord Application Commands so typing `/` shows an autocomplete menu; supports per-guild instant registration via `guild_id` config
- **Daemon Mode**: Run cc-connect as a background service (`cc-connect daemon install/start/stop/status/logs`); supports systemd (Linux) and launchd (macOS)
- **Qoder CLI Agent**: Full support for the Qoder coding agent with streaming JSON, mode switching, and model selection
- **Telegram Proxy**: Support HTTP/SOCKS5 proxy for Telegram bot API connections
- **WeChat Work Proxy Auth**: Add `proxy_username` / `proxy_password` for authenticated forward proxies
- **i18n Expansion**: Add Traditional Chinese (zh-TW), Japanese (ja), and Spanish (es) language support
- **`--stdin` Support**: Read prompt from stdin for CLI usage (`echo "hello" | cc-connect send --stdin`)

### Improvements
- **Slow Operation Monitoring**: Warn-level logs for slow platform send (>2s), agent start (>5s), agent close (>3s), agent send (>2s), and agent first event (>15s); turn completion logs now include `turn_duration`
- **`tool_max_len=0` Fix**: Remove hardcoded 200-char truncation in all agent sessions (Claude Code, Cursor, Codex, Gemini, Qoder), making the user-configurable `tool_max_len` setting authoritative
- **Cursor `/list` Improvements**: Parse binary blob structure to show accurate message counts and first user message summary

### Bug Fixes
- **Telegram proxy**: Only override `http.Transport` when proxy is actually configured
- **Discord interaction fallback**: Gracefully fallback to channel messages when interaction token expires

## v1.1.0 (2026-03-02)

### New Features
- **`/compress` Command**: Compress/compact conversation context by forwarding native commands to agents (Claude Code `/compact`, Codex `/compact`, Gemini `/compress`); keeps long sessions manageable
- **Auto-Compress**: Added optional automatic context compression when estimated token usage exceeds a configurable threshold (`[projects.auto_compress]`).
- **Telegram Inline Buttons**: Permission prompts on Telegram now use clickable inline keyboard buttons (Allow / Deny / Allow All) instead of requiring text replies
- **`/model` Command**: View and switch AI models at runtime; supports numbered quick-select and custom model names. Fetches available models from provider API in real-time (Anthropic, OpenAI, Google), with built-in fallback list
- **`/memory` Command**: View and edit agent memory files (CLAUDE.md, AGENTS.md, GEMINI.md) directly from chat; supports both project-level and global-level (`/memory global`)
- **`/status` Command**: Display system status including project, agent, platforms, uptime, language, permission mode, session info, and cron job count

### Improvements
- **Cron list display**: Multi-line card-style formatting with human-readable schedule translations and next execution time
- **Model switch resets session**: Switching model via `/model` now starts a fresh agent session instead of resuming the old one, preventing stale context from affecting the new model
- **Permission modes docs**: README now documents permission modes for all four agents (Claude Code, Codex, Cursor Agent, Gemini CLI)
- **Natural language scheduling docs**: INSTALL.md now explains how to enable cron job creation via natural language for non-Claude agents
- **README revamp**: Redesigned project header with architecture diagram, feature highlights, and multi-agent positioning

### Bug Fixes
- **Gemini `/list` summary**: Fixed session list showing raw JSON (`{"dummy": true}`) instead of actual user message summary
- **GitHub Issue Templates**: Added structured templates for bug reports, feature requests, and platform/agent support requests

## v1.1.0-beta.7 (2026-03-02)

(see v1.1.0 above — beta.7 changes are included in the stable release)

## v1.1.0-beta.6 (2026-02-28)

### New Features
- **QQ Platform** (Beta): Support QQ messaging via OneBot v11 / NapCat WebSocket
- **Cron Scheduling**: Schedule recurring tasks via `/cron` command or CLI (`cc-connect cron add`), with JSON persistence and agent-aware session injection
- **Feishu Emoji Reaction**: Auto-add emoji reaction (default: "OnIt") on incoming messages to confirm receipt; configurable via `reaction_emoji`
- **Display Truncation Config**: New `[display]` config section to control thinking/tool message truncation (`thinking_max_len`, `tool_max_len`); set to 0 to disable truncation
- **`/version` Command**: Check current cc-connect version from within chat

### Bug Fixes
- **Windows `/list` fix**: Claude Code sessions now discoverable on Windows despite drive letter colon in project key paths
- **CLAUDECODE env filter**: Prevent nested Claude Code session crash by filtering CLAUDECODE env var from subprocesses

### Docs
- Clarified global config path `~/.cc-connect/config.toml` in INSTALL.md
- Fixed markdown image syntax in Chinese README

## v1.1.0-beta.5 (2026-03-01)

### New Features
- **Gemini CLI Agent**: Full support for `gemini` CLI with streaming JSON, mode switching, and provider management
- **Cursor Agent**: Integration with Cursor Agent CLI (`agent`) with mode and provider support

## v1.1.0-beta.4 (2026-03-01)

### Bug Fixes
- Fixed npm install: check binary version on install, replace outdated binary instead of skipping
- Added auto-reinstall logic for outdated binaries in `run.js`

## v1.1.0-beta.3 (2026-03-01)

### New Features
- **Voice Messages (STT)**: Transcribe voice messages to text via OpenAI Whisper, Groq Whisper, or SiliconFlow SenseVoice; requires `ffmpeg`
- **Image Support**: Handle image messages across platforms with multimodal content forwarding to agents
- **CLI Send**: `cc-connect send` command and internal Unix socket API for programmatic message sending
- **Message Dedup**: Prevent duplicate processing of WeChat Work messages

## v1.1.0-beta.2 (2026-03-01)

### New Features
- **Provider Management**: `/provider` command for runtime API provider switching; CLI `cc-connect provider add/list`
- **Configurable Data Dir**: Session data stored in `~/.cc-connect/` by default (configurable via `data_dir`)
- **Markdown Stripping**: Plain text fallback for platforms that don't support markdown (e.g. WeChat)

## v1.1.0-beta.1 (2026-03-01)

### New Features
- **Codex Agent**: OpenAI Codex CLI integration
- **Self-Update**: `cc-connect update` and `cc-connect check-update` commands
- **I18n**: Auto-detect language, `/lang` command to switch between English and Chinese
- **Session Persistence**: Sessions saved to disk as JSON, restored on restart

## v1.0.1 (2026-02-28)

- Bug fixes and stability improvements

## v1.0.0 (2026-02-28)

- Initial release
- Claude Code agent support
- Platforms: Feishu, DingTalk, Telegram, Slack, Discord, LINE, WeChat Work
- Commands: `/new`, `/list`, `/switch`, `/history`, `/quiet`, `/mode`, `/allow`, `/stop`, `/help`
