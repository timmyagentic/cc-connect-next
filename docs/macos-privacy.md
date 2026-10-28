# macOS privacy prompts

On macOS, cc-connect-next supplies native filesystem permission profiles to
supported local Codex exec and app-server sessions by default. These deny
unrelated personal-directory access, including in YOLO mode. They apply to new,
resumed, and subsequent turns without relying on the model remembering a prompt.
Ordinary development outside personal folders does not require editing local
AGENTS.md files or granting Full Disk Access.

## Why native enforcement

A reminder to avoid `find ~` is not a boundary. In the reported incident the
Agent received that reminder and still scanned the home directory to locate
cc-connect-next. The runtime now supplies its exact executable path as well as
the effective data directory. Cron management should use the declared CLI/API.

The privacy rules are enforced by Codex's own sandbox. There is no external
`sandbox-exec` around the Agent: macOS rejects initializing a second Seatbelt
sandbox inside an existing one, even when the outer sandbox allows everything.
Blanket wrapping breaks native Agent sandboxes and is not a compatible fix.
Approval behavior and filesystem access are separate: YOLO still automatically
approves ordinary work, but does not remove these personal-directory rules.

## Protected paths

Rules deny access under these paths in the service user's home:

- Desktop, Documents, Downloads, Pictures, Music, Movies
- Library/Containers and Library/Group Containers
- Library/Mail, Messages, Safari, Calendars, Reminders, HomeKit,
  IdentityServices, PersonalizationPortrait, Metadata/CoreSpotlight
- Library/Mobile Documents and Library/CloudStorage

The runtime does not enumerate these directories. Ordinary code directories,
tool installations, caches, and Library/Keychains remain subject to the selected
Codex mode's existing permissions. This is a finite policy, not a complete list
of every location that macOS may protect.

Protection is at **directory-category granularity**. If a selected `work_dir`
is inside Documents, this session retains the mode's existing access to
Documents, while Desktop, Downloads, app containers, and the other categories
remain denied. This is broader than a single-project exception: Git requires
ancestor metadata that Codex 0.163 cannot separately allow under a denied
parent. Existing mode restrictions still apply; workspace-write does not gain
write access to sibling projects.

Selecting `~` or `/` does not exempt all personal directories beneath it.
For intentional access to an additional category, configure its name:

```toml
[macos_privacy]
enabled = true
allow_directories = ["Documents"]
```

Explicit exceptions apply to supported Codex sessions in this service and
retain the selected execution mode's access level for the **whole category**.
Names are exactly those in the list above, such as `Library/Containers`.
Arbitrary paths and globs are rejected. Home and workspace paths containing
glob/control characters are unsupported because native profile keys interpret
patterns; no directory scanning is performed to expand them.
An exception does not grant TCC access or authorize a new task. A workspace
located in a protected category can still require the relevant macOS consent;
the goal is to prevent unrelated prompt bursts, not suppress legitimate consent.

The setting takes effect after restarting cc-connect-next. Existing Agent
processes retain their previous configuration. `enabled = false` explicitly
restores legacy Codex permission selection without changing OS permissions.
Linux and Windows keep their existing behavior.

On access denial the Agent is instructed to stop that access and report the
missing scope once, rather than retry with another tool or broaden its search.
The deterministic guarantee is the native filesystem policy, not that the
model will always follow the explanation.

## Coverage and compatibility

This change covers filesystem operations enforced by supported local Codex
exec/app-server sessions, including Agent shell descendants and prompt-based
scheduled jobs routed through those sessions. It does not claim to guard:

- Claude Code or other Agent adapters.
- Standalone exec cron/timers, custom shell commands, hooks, or webhook exec.
- Remote app-server, desktop bridges, tmux servers, or external MCP/services.
- Host-native file operations such as attachment and configuration APIs.
- Other users' home directories, pre-opened file descriptors, hard links,
  relocated protected directories, or newly protected macOS locations.
- Apple Events, Accessibility, screen capture, and non-filesystem permissions.

Stable Codex CLI 0.163.0 or later is required; older, prerelease, and unrecognized
versions fail before a local session starts. Runtime version checks do not
authenticate or request a model response. Conflicting custom permission,
sandbox, approval, or profile flags are rejected rather than silently overriding
the generated policy. Use the normal cc-connect-next mode setting instead.

For `auto-edit` / `full-auto`, custom legacy `sandbox_workspace_write` settings
(additional writable roots, network access, or temporary-directory exclusions)
are currently rejected after reading Codex's effective configuration. Inheriting
`:workspace` would otherwise drop those settings and could widen write access.
They are **not migrated or silently replaced** by this change.

YOLO retains ordinary repository and linked-worktree Git writes through narrow
metadata grants. Resolving `.git`, `gitdir`, and `commondir` does not invoke Git
or enumerate personal folders. A pointer into a denied directory receives no
implicit exception. Read-only/workspace modes retain native metadata protection.

Unsupported native configuration must fail explicitly; do not retry using
unrestricted mode. This policy does not isolate hostile workloads from
IPC/network services or replace existing
managed security requirements. The app-server `suggest` mode retains its
interactive approval behavior; an explicitly approved escalation is not a
guarantee of sandboxed execution. Automatic modes continue to use `never`.

“Access data from other apps” is an App Data filesystem prompt, not an Apple
Events prompt. Its consent can expire with the process even with stable signing.
Stable Developer ID signing remains necessary for persistent folder consent
across upgrades, but cannot prevent an unintended access.

## Release acceptance

Tests must use synthetic personal directories, an isolated Codex home, and no
model requests. Verify that the actual native sandbox denies protected reads,
writes, traversal and symlinks while allowing normal work and declared category
exceptions.
Verify native profile selection for exec, resume, app-server thread start/resume,
and every new turn. Test unsupported runtimes without an unrestricted fallback.

Before declaring a released binary free of the reported prompts, use a fresh
macOS VM/account with a signed launchd installation:

1. Ordinary chat, cron management, a long/resumed/compacted session, and a
   deliberately broad search produce no unrelated privacy prompts.
2. Deliberately selected project access requests only relevant consent.
3. Native sandbox behavior, cancellation, keychain use, and normal builds work.
4. Restart and signed A-to-B upgrade preserve intended behavior. Test persistent
   Files & Folders consent separately from process-lifetime App Data consent.

Synthetic denial tests do not establish zero TCC prompts on a clean signed
launchd installation. An already-authorized developer account is not that
acceptance evidence.

References:

- [Codex permission configuration](https://learn.chatgpt.com/docs/config-file/config-reference).
- [Apple WWDC23: What's new in privacy](https://developer.apple.com/videos/play/wwdc2023/10053/).
- [Apple DTS: repeated App Data prompts](https://developer.apple.com/forums/thread/742147).
- [Issue 166](https://github.com/timmyagentic/cc-connect-next/issues/166).
