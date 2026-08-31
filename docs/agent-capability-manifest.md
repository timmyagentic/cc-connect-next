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
| `tools` | Agent-facing local CLI tools such as send, Cron, Timer, Relay, and deferred daemon restart |
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

Built-in command permissions follow the real Engine dispatch path. Static privileged commands require `projects.admin_from`; dynamic Shell registration through `/commands addexec`, `/cron addexec`, and `/timer addexec` is admin-only. The Agent-facing `cc-connect-next daemon restart` tool is also admin-only and conditionally available only with the exact HMAC turn credential derived from a private per-Agent session secret; its client supplies no project, session, user, or platform routing. Every registered custom command with an `Exec` body is also admin-only at invocation, while Prompt-backed custom commands remain member operations. Project-level `disabled_commands` is reflected immediately; user-role policy and the concrete caller identity remain invocation-time checks, so the Manifest keeps them conditional instead of guessing identity from a platform session-key format.

Agent command files may publish only an explicit frontmatter `description`. Their Markdown Prompt body—including its first line—is never reused as Manifest or menu metadata.

Existence is not presented as executable availability: model/provider switching, TTS, multi-workspace, schedulers, Relay, Web setup, and in-flight-turn operations probe their corresponding runtime interfaces or components.

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
