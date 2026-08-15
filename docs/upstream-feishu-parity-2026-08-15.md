# Upstream Feishu parity — 2026-08-15

This follow-up audits official CC Connect through upstream `main` commit `6c8607980016c5eccaf5a3aa1f53dbe78e4f9c5d`. No additional Feishu change landed between official `v1.5.0-beta.3` (`ad196294`) and that pinned main commit. cc-connect-next therefore adapts the five previously deferred Feishu changes below without merging the diverged upstream branch wholesale.

| Official change | Next status | Adaptation |
|---|---|---|
| [#1551](https://github.com/chenhg5/cc-connect/pull/1551) / `129753c` — topic workspace isolation | Implemented | A topic receives `chat:topic:root` as its workspace key. An existing chat-level binding is copied as the topic default without deleting the source or overwriting a topic override. Project and shared bindings use the same rule. |
| [#1588](https://github.com/chenhg5/cc-connect/pull/1588) / `c22d5e2` — quoted-file download | Implemented | Reply-chain file metadata is collected without downloading bytes. Download happens only when the current message explicitly @mentions this bot and the quoted uploader is the same Feishu user. |
| [#1627](https://github.com/chenhg5/cc-connect/pull/1627) / `94e2154` — first mention in an existing topic | Implemented | The first accepted mention activates the isolated topic and injects its pre-existing root/reply context once. Later turns do not repeat that history. |
| [#1413](https://github.com/chenhg5/cc-connect/pull/1413) / `97b96ad` — relay visibility | Implemented | Relay request/response visibility echoes reuse the caller's topic session key. Other platforms and non-topic Feishu sessions keep the legacy channel-level target. |
| [#1341](https://github.com/chenhg5/cc-connect/pull/1341) / `6fc59a3` — outbound bot mentions | Implemented and hardened | `mention_map` overrides same-name group members, validates bot `ou_` identifiers, and requires `resolve_mentions = true`. Ordinary sends use native text mention syntax. In Rich mode, a resolved final @ is delivered as a tracked quoted text replacement because Feishu Card/Post at-tags render but do not emit the bot mention event. |

The earlier configurable 500 ms multi-image batch behavior is already present in Next and remains unchanged.

## Protected successor contracts

- Core does not contain Feishu-specific session parsing. Optional capability interfaces let the platform supply topic relay targets and the rare Rich Card terminal-text replacement.
- A terminal native-mention replacement is sent successfully and tracked before the lifecycle card is deleted. A concurrent trigger recall can therefore delete the exact replacement instead of leaving an untracked answer.
- Native mention resolution ignores inline, fenced, and indented Markdown code, so examples cannot switch the final transport or emit a notification.
- Topic binding migration is non-destructive and idempotent. It retains the chat default for other topics and rollback to older binaries.
- First-topic bootstrap holds a short per-topic FIFO until root recovery completes and the current turn is accepted by Core; transient API, parsing, media-download, or recall failures release the bootstrap reservation for the next queued message without revoking attachment admission.
- Quoted-file bytes never enter the Agent request until the explicit-mention and same-uploader privacy checks both pass.
- Reasoning text, tool details, model/token/context metadata, and local runtime state remain outside Feishu answer-card payloads.

## Verification boundary

Repository tests cover platform routing, migration persistence, privacy gates, terminal mention transport, recallable handles, and a six-action topic workspace CUJ. Run:

```bash
go test ./platform/feishu -count=1
go test ./core -count=1
go test ./... -count=1
go test -race ./core ./platform/feishu -count=1
```

Real Feishu desktop/mobile rendering, tenant permissions, quoted-file API access, and bot-to-bot event delivery are external release gates and remain **UNVERIFIED** until exercised with a test tenant.
