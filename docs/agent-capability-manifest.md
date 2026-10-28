# Agent Capability Manifest

The Agent Capability Manifest is cc-connect-next's unified, read-only contract for the current build, project, and session. It lets a connected Agent answer what is actually available without reading source or guessing.

It describes:

- supported configuration and exact placement;
- built-in and project-defined chat commands;
- Skills discovered for the active Agent;
- active Agent, Agent-session, and Platform optional interfaces;
- parameters and caller permission;
- read-only status and write/external side effects;
- fallback/degradation behavior;
- truthful availability state and reason.

## Agent query

The bounded first-turn capability brief directs the Agent to query before answering a specific capability question or invoking a cc-connect-next operation:

```bash
cc-connect-next capabilities --search "2-4 keywords"
```

Examples:

```bash
cc-connect-next capabilities --search "existing Feishu topics"
cc-connect-next capabilities --search "switch model"
cc-connect-next capabilities --search "native audio" --format json
cc-connect-next capabilities --all --search "Slack"
```

Default queries focus on adapters active in the current project. Use `--all` for a compiled but inactive Agent/Platform; the Manifest then includes every adapter's configuration contract and an explicit `compiled but not active` / `configure-and-restart` activation entry.

`--project` and `--session-key` default to `CC_PROJECT` and `CC_SESSION_KEY`; a runtime with one project or one active session can also infer them. The CLI queries the running daemon through its `0600` local Unix socket and does not connect a platform or mutate state.

If the daemon is not running, use the build-time configuration contract instead:

```bash
cc-connect-next config capabilities --search "keywords"
```

## Schema

The JSON schema identifier is:

```text
cc-connect-next.agent-capabilities/v1
```

Top-level sections:

| Field | Contents |
|-------|----------|
| `configuration` | Active-project configuration contract; never current values |
| `tools` | Agent-facing local CLI tools such as send, Feedback, Cron, Timer, Relay, and deferred daemon restart |
| `commands` | Built-in and project custom chat commands |
| `skills` | Skills actually discovered from the active Agent's Skill directories |
| `runtime` | Active Agent, Agent-session, and Platform interface capabilities |

Executable entries share these fields:

- `parameters`: name, type, requirement, and allowed values;
- `permission`: `member`, `admin`, `conditional`, or `local-agent`;
- `read_only`;
- `side_effects`;
- `fallback`;
- `availability`.

Availability states:

| State | Meaning |
|-------|---------|
| `available` | Known runtime/context prerequisites are satisfied |
| `conditional` | An active turn, caller identity, or invocation arguments decide final availability |
| `unavailable` | An interface, runtime component, permission, or configuration prerequisite is known to be missing |

Unsupported runtime features remain visible with a reason and fallback. For example, native video can report `unavailable` while documenting file-delivery fallback when `FileSender` exists.

## Permission and side-effect contract

Built-in command permissions follow the real Engine dispatch path. Static privileged commands require `projects.admin_from`; dynamic Shell registration through `/commands addexec`, `/cron addexec`, and `/timer addexec` is admin-only. The Agent-facing `cc-connect-next daemon restart` tool is also admin-only and conditionally available only with the exact HMAC turn credential derived from a private per-Agent session secret; its client supplies no project, session, user, or platform routing. The `local-agent` Feedback tool uses the same credential to resolve trusted identity, returns a complete Draft with zero network I/O, and requires a separate one-time token bound to the initiating user/session before the shared Relay path is called. Every registered custom command with an `Exec` body is also admin-only at invocation, while Prompt-backed custom commands remain member operations. Project-level `disabled_commands` is reflected immediately; user-role policy and the concrete caller identity remain invocation-time checks, so the Manifest keeps them conditional instead of guessing identity from a platform session-key format.

Agent command files may publish only an explicit frontmatter `description`. Their Markdown Prompt body—including its first line—is never reused as Manifest or menu metadata.

Existence is not presented as executable availability: model/provider switching, TTS, multi-workspace, schedulers, Relay, Web setup, and in-flight-turn operations probe their corresponding runtime interfaces or components.

## Current-chat models and the global default

The `model` command publishes
`/model [<name|alias|number>|reset|default [<name|alias|number>]]`.
Its only subcommand is `default`; both the action's allowed values and
`privileged_when` contain only `default`. Permission is `conditional`:
`/model` and `/model default` are read-only member queries, while setting or
resetting a current-chat model is also a member operation. Only
`/model default <name|alias|number>` requires admin. Existing role and
disabled-command policies still apply; dispatch checks the concrete invocation,
so the default query does not require admin.

`/model default` queries or changes the global default of the current
bot/project's Agent. It never targets a workspace-specific Agent or another
bot. `ModelSwitcher` alone makes these operations available, including on
backends without per-turn model support. There is no separate workspace model
default: a chat without a model binding follows this global default.

Chat bindings are stored in `config.toml` under `[[projects.chat_models]]`.
Chat commands write those same entries; `/config reload` applies manual edits
and removals to later turns. Old project JSON bindings are imported once at startup.

`/model` queries, pins, or resets a verified private or group chat's selection.
Pinning additionally requires the configured backend's `PerTurnModelSupporter`
to return true (currently Codex `exec` and `app_server`), a current selectable
model, and available persistent storage. It never switches Provider.
Invocation verifies chat identity; a session-key shape alone is not proof.
An unrecognized chat can first send `/model` there. Feishu/Lark card callbacks
bound to a verified direct user can establish private-chat routing without a
fresh text event. Reset can clear an existing binding even if the backend no
longer supports pinning, provided verified identity and storage exist. The
availability reason distinguishes global-default operations from these chat
prerequisites.

The parameter contract describes isolation within each bot/project:

- Groups use `platform:groupID` (platform + stable group ID). All members and
  existing or new topics share the binding.
- Verified DMs use `platform:dm:userID`. Each direct user's selection is
  isolated from other users, groups, platforms, and bots/projects, including
  bots that share a workspace directory.

Neither binding is scoped to a topic, conversation, or workspace directory.
Both survive `/reset`, `/new`, and restarts. `ChatModelBinding` is stored under
`chat_model_bindings` in project state; session `ChatKey` uses the JSON key
`chat_key`. Chat writes declare `persistent_state`; `configuration` and
`agent_process` describe global-default changes. The overall command remains
mutating because it exposes setters as well as read-only queries. Model and
reasoning effort persist independently; `/model reset` clears only the model
binding. No new `config.toml` fields are needed.

There is no migration for older model-state fields. Previous chat model
bindings and workspace-specific model defaults are not carried over; select
the desired model again with the commands above.

All three send paths resolve each field at execution, including queued work.
Changes affect the next new turn and leave a running reply alone. An explicit
one-shot `/fast`/answer-profile or task model takes precedence for its own field;
without one, ordinary messages use the fixed chat model, then the current
bot/project's global default when no binding exists. A profile without an
explicit model still inherits the chat model. The `reject` fallback contract
covers unsupported per-turn settings, backend/Provider mismatch, unknown
selections, storage and model/application errors: a pinned request must not
silently use another model. Reselect with `/model <name>` or explicitly clear
the model binding with `/model reset` to recover. Invalid arguments and stale
card actions return the current usage without changing settings.

The read-only session capability `model_provider_identity` checks
`ModelProviderSession` without publishing its captured provider name. A
provider-switching Agent must expose this identity for pinned chat turns;
the engine validates the concrete session as well as the current Agent. A
stale conversation connection must be reconnected with `/provider switch <name>`
or replaced with `/new` before retrying.

The current-chat card displays the actual model as fixed or following the
default, with one selector using `cmd:/model MODEL` and `cmd:/model reset`.
Its secondary `cmd:/model default` button opens a separate global-default card
with one selector using `cmd:/model default MODEL`. Model values are stable
identifiers, not list positions. Commands and card selections return the
relevant card with a result message; platforms without cards receive its text
rendering. `nav:/model` opens the current-chat menu. Current-chat model status
and error messages use the `MsgModelChat*` i18n keys.

Reasoning card selections remain `cmd:/reasoning chat ...`, including `reset`;
`nav:/reasoning` still opens its menu. `/reasoning` and `/effort` semantics are
unchanged.

## Chat reasoning effort

The `reasoning`/`effort` command publishes
`/reasoning [<level>|chat [<level>|reset]]`. `/effort` aliases `/reasoning`, and
`group` remains a subcommand alias for `chat` in both verified DMs and groups.
Legacy default switching stays available via `ReasoningEffortSwitcher`; a bare
level changes the project/workspace Agent default and resets this conversation,
while chats with fixed effort keep their setting. Chat pinning additionally
requires `PerTurnReasoningEffortSupporter`, a non-empty effort catalog, verified
chat identity, durable storage and compatible per-turn session/provider support.
The `member` permission is unchanged and `persistent_state` is declared.
Unsupported effort, missing identity/storage, or backend/Provider and session
mismatches fail explicitly without silent fallback. Reselect with
`/reasoning chat <level>` or clear the effort with `/reasoning chat reset`;
reset remains available with verified identity and storage even when pinning
is unsupported.

Chat effort is stored independently of the model, using the same project-local
group or DM key. Groups share it across all members/topics; private chats are
isolated by verified user and bot/project. It persists across `/new`, `/reset`,
and restarts. Each queued turn resolves model and effort separately: an
explicit task/profile value wins only for its own field. A model-only profile
still inherits chat effort, and an effort-only profile still inherits the chat
model. Resetting either binding preserves the other; a running reply is not
interrupted. The command metadata distinguishes default switching's
conversation reset from a chat-scoped operation that preserves history.

## Cron task models

The `cron` Agent CLI tool contract includes optional `model`: prompt jobs only, omitted means inherit, and `cron edit <job-id> model ""` restores inheritance. Creating/editing writes persistent job state; execution starts an Agent turn and network requests without switching providers or changing project/Agent model defaults. Shell jobs and unsupported Agents reject explicit models rather than silently reverting to a default.

The same tool contract includes optional string `reasoning_effort`: use `cron add --reasoning-effort <value>` or `cron edit <job-id> reasoning_effort <value>` for prompt jobs, with or without a model override. Omit it to inherit; `cron edit <job-id> reasoning_effort ""` restores inheritance. The server validates supported efforts and Agent support, so the generic CLI metadata does not enumerate Agent-specific values. Shell jobs, unsupported Agents, and invalid efforts reject explicit overrides. Creating/editing changes only the job's persistent state; the effort applies to its Agent turn without changing project/Agent defaults. CLI lists and single-field info queries show the effort or inheritance; full info preserves the optional JSON field.

The runtime feature `per_turn_reasoning_effort` is available only when the Agent implements `PerTurnReasoningEffortSupporter`, returns true, and supplies a non-empty catalog through `ReasoningEffortSwitcher.AvailableReasoningEfforts()`. Missing support or a missing/empty catalog remains visible as `unavailable`; concrete session support is checked at execution. The runtime `cron` tool parameter publishes that Agent's catalog as `allowed_values`, without embedding a provider-specific list in generic metadata. Following the capability brief, query `cc-connect-next capabilities --search "cron reasoning effort"` for the parameter and runtime contract. The Web cron editor requests both `runtime` and `tools` sections to offer inheritance and advertised effort choices, falling back to a server-validated text input when no list is available.

The Agent runtime entry `per_turn_model` probes `PerTurnModelSupporter` instead of matching Agent names. Missing or false support remains visible as `unavailable` with its rejection reason. Declared support is followed by a concrete `TurnOptionsSession` check during execution; the existing session `turn_options` entry exposes that interface. Without an active session, the session entry stays `conditional`, while the Agent declaration permits configuration-time checks. Both Codex `exec` and `app_server` implement this contract. Actual model ID availability remains an execution-time Agent/provider decision; the Manifest does not certify model catalogs or credentials.

## Recent conversation background

The optional Platform capability `recent_conversation_context` reads bounded,
untrusted discussion using existing platform access before a verified user
turn. It does not execute quoted commands or grant tool permissions. Its
availability reports disabled configuration, an ineligible target, or the
conditional need for existing history access; unavailable adapters retain the
entry with a current-message fallback. Failed/truncated reads are disclosed.
Selected history can include attributed forwarded text, images and files. Media is
downloaded only after deduplication and text-budget selection, using bounded
resource requests. Unavailable content is marked; failed media remains retryable.

The associated `group_context` configuration defaults off. Reads occur only
for requests accepted by the configured group reply rules and never by polling; ID receipts persist only after a
successful turn. Busy requests queue to preserve that ordering.

## Conversation speed

`/speed` queries the next-turn speed, the active model's supported tiers, and
the running turn's speed when busy. `/speed fast` saves a speed for the current
conversation; `/speed default` explicitly selects standard speed. It does not
change model or reasoning effort, restart the bridge, or interrupt a turn.
Messages crossing a speed boundary queue for a new turn, including when busy
message steering is enabled. A queued message uses the choice in force when
its turn starts. One-shot `/fast` and `/quality` settings take precedence for
their own turn; ordinary replies then return to the saved conversation speed.

The choice persists in session storage across bridge restarts. `/new` inherits
project defaults; switching back restores that conversation's choice. The
reply explicitly identifies nonpersistent operation when storage is disabled.
The command requires member access and obeys user-role/disabled-command policy;
its Manifest declares session and persistent-state effects. Omitting the tier
is a read-only query.

A connected Codex app-server validates choices through its live `model/list`
catalog. Before a session starts, local Codex uses the catalog in its effective
`CODEX_HOME` (`model_catalog_json`, otherwise `models_cache.json`). Catalog `priority` is
exposed as `fast`; older `additional_speed_tiers` catalogs are also supported.
Unknown models, missing catalogs, unsupported tiers, unverifiable remote or
custom-argument configurations before a session is connected, and save failures return explicit errors.
Refresh missing/stale metadata with Codex CLI. A saved choice is revalidated
before future turns if the model/provider changes; there is no silent downgrade.

## Security boundary

The Manifest never includes:

- current config values or credentials;
- Skill instruction bodies or source paths;
- custom Prompt/Exec bodies or shell command bodies;
- management/bridge tokens or provider keys.

Dynamic descriptions and runtime errors receive aggressive credential, long-blob, Lark-ID, and home-path redaction. The query itself is read-only; listed side effects describe a future invocation, not an action performed by the query.

## Other consumers

- Local API: `GET /capabilities?project=...&session_key=...&search=...`
- Management API: `GET /api/v1/projects/{name}/capabilities`
- Web Chat: its command palette consumes Manifest commands and Skills and disables unavailable actions with the reason
- Bridge `capabilities_snapshot_v1`: its legacy command projection is derived from Manifest command contracts; protocol v1 does not embed the full Skill/configuration object

The exhaustive generated configuration reference remains available at [configuration.md](configuration.md).

Automatic platform actions can also carry `permission`, `read_only`, and `side_effects`. For example, `completion_notification` reports disabled or conditional availability without exposing configured recipient IDs; its [configuration and delivery contract](completion-notifications.md) includes durable deduplication receipts and immediate private messages from the bot, with no read-status polling or scheduled reminders.

## Image galleries

Pass all selected local images in **one** `cc-connect-next send` invocation by
repeating `--image`. The `image_batches` runtime capability reports whether
that platform groups them. Feishu/Lark sends two or more images as Card 2.0
`img_combination` galleries: two columns, at most five originals per message,
in input order. Ten images produce two messages. A single image still uses a
native image message; the final gallery may contain fewer than five images.
No compositing or cropping is performed on the uploaded originals.

This is enabled by default when `enable_feishu_card` is enabled. Setting that
existing option to false, or using a platform without `image_batches`, retains
individual image messages. `attachment_send`, size limits, outgoing rate limits,
workspace routing and topic/reply boundaries still apply. This does not change
the remote Markdown image resolver: local Markdown paths are not uploads.

Each group uploads all its images before publishing one card. A failed upload
publishes no partial card. Transient message retries reuse one Feishu UUID for
that group without replaying earlier successful groups. On terminal failure,
the send command returns an error with the confirmed image count; subsequent
groups and files are not sent. A fresh CLI invocation is a new delivery: inspect
the failed group before retrying only the unconfirmed remainder, since a lost
response can leave remote acceptance uncertain. There is no persistent,
cross-invocation resend queue.

## macOS doctor identity and privacy scope

`cc-connect-next doctor` and in-chat `/doctor` keep the resolved Agent CLI path
alongside its version. On macOS they also list the diagnostic process first,
then up to seven ancestors, with PID, PPID, and executable name/path. This uses
`ps` executable metadata only, never process arguments or environment variables.
Inspection is bounded to two seconds; missing processes, timeouts, cycles and
the depth limit are reported as incomplete rather than healthy ancestry.

The CLI describes its own process chain; in-chat doctor runs in the service.
Neither inventories every Agent/tool process nor identifies the application
macOS treats as responsible for TCC. Screen Recording, Accessibility and Apple
Events are explicitly **unknown (not probed)**; Apple Events also depends on the
target application. The warning is not evidence of denied access. No TCC database
is read, no authorization prompt or application is opened, and no permission is
reset. Do not repeat authorization or restart solely on this report. No new
configuration or native build dependency is required; other operating systems
keep their existing checks. Remove private executable paths before sharing a
report publicly.
