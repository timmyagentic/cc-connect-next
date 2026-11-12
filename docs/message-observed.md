# Observing human mentions

`message.observed` delivers message routing metadata before the Agent's reply
filters. Feishu and Lark reuse their existing event connection; enabling this
hook does not open another connection or make an Agent answer ordinary messages.
Configure an exact hook or a wildcard hook in the existing `[[hooks]]` section.

```toml
[[hooks]]
event = "message.observed"
type = "command"
command = "python3 /path/to/persist_mention.py"
async = false
timeout = 2
```

The source accepts human senders with explicit mentioned open IDs and applies
the existing `allow_from`, `allow_chat` and `group_only` restrictions. Bots and
`@all` without explicit user mentions do not produce observations. The source
does not choose an owner or notification recipient; the consumer must check the
mentioned IDs against its configured recipient. Bot mention requirements for
Agent replies continue to apply independently.

Only events delivered under the application's existing memberships, permissions
and subscriptions can be observed. A bot event connection does not cover all
of a human user's groups or person-to-person private conversations. This hook
does not discover missed history by polling.

## Payload and acknowledgement

Command hooks receive the ordinary `CC_HOOK_EVENT`, `CC_HOOK_PLATFORM`,
`CC_HOOK_PROJECT`, `CC_HOOK_USER_ID`, `CC_HOOK_TIMESTAMP` variables, plus
`CC_HOOK_EXTRA` containing JSON. HTTP hooks receive the same metadata in the
event's `extra` object. The source supplies no body or display names.

| Field in `extra` | Meaning |
| --- | --- |
| `message_id`, `chat_id`, `chat_type` | Original message and conversation; type is `group` or `p2p` |
| `created_at_ms` | Original creation timestamp as an integer in milliseconds |
| `thread_id`, `root_id`, `parent_id` | Original routing IDs; empty when absent |
| `sender_type` | `user` |
| `mentioned_user_ids` | Explicit mentioned open IDs, deduplicated in source order |

`message.observed` uses synchronous delivery even if a matching hook has
`async = true`; this includes wildcard hooks. Every matching hook runs and any
failure is returned to the Feishu/Lark SDK instead of acknowledging success.
The existing `Emit` behavior for other events is unchanged.

Persist the pending message idempotently before returning success. Keep the
hook short and move API reads, investigation and notifications to the existing
worker. Duplicate delivery, a retry after another project's failure and an old
message replayed after restart can all invoke the observer again. Use the
original message ID as the deduplication key; do not use delivery timestamps.
Observation happens before the Agent's in-memory deduplication and startup-time
filter, so neither can swallow a persistence retry. An observer failure also
prevents that delivery from starting an Agent turn until retry succeeds.

The hook does not own event retention. Transport retries remain bounded by the
platform's delivery behavior; downtime or missing event access is not repaired
by this interface. With no matching hook, the observer stays disconnected and
the normal message path is unchanged.

## 中文说明

`message.observed` 复用飞书/Lark 已有连接，在 `@bot` 回复过滤之前同步提供真人提及的路由元数据，
遵守现有发送人、群范围及仅群聊限制，不改变 Agent 的回复触发条件。用户/通知接收人由消费者筛选，
Go 平台不硬编码特定用户。事件仅覆盖应用已有权限和成员关系实际允许收到的消息。

命令通过 `CC_HOOK_EXTRA` 读取上表 JSON，HTTP 通过 `extra` 读取；不传正文和显示名称。
该事件会等待所有匹配 Hook（包括通配符）完成，不受 `async` 选项影响；任一失败都会返回 SDK，
使本次投递不能被当成成功确认。其他事件仍使用原有同步/异步行为。

消费者须先按原 `message_id` 幂等持久化 pending，再返回成功，网络读取和模型处理交给后续 worker。
重复投递、多个项目共享连接时其他项目失败后的重投、进程重启后的旧消息都可能再次触发。
没有匹配 Hook 时不安装观察器；该接口不提供历史轮询，也不承诺平台无限重试。
