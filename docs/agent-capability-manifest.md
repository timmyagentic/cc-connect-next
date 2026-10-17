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

## Recent conversation background

The optional Platform capability `recent_conversation_context` reads bounded,
untrusted discussion using existing platform access before a verified user
turn. It does not execute quoted commands or grant tool permissions. Its
availability reports disabled configuration, an ineligible target, or the
conditional need for existing history access; unavailable adapters retain the
entry with a current-message fallback. Failed/truncated reads are disclosed.
The associated `group_context` configuration defaults off. Reads occur only
for accepted mentions and never by polling; ID receipts persist only after a
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
