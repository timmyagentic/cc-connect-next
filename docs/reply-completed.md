# Reply completion hooks / 答复送达 Hook

## English

Configure `reply.completed` (or `*`) in `[[hooks]]` to receive routing metadata
when a supported adapter confirms a non-silent final group reply in
`display.card_mode = "rich"`. Feishu and Lark implement this route. This includes
an updated rich card, a replacement card, and a tracked native-text replacement.
It does not cover private chats, legacy/plain reply mode, silent answers, failed
or canceled terminal delivery. Intermediate streaming updates do not emit it.
All three routing IDs must be present; incomplete routes emit no event.
Delivery means the platform send/update succeeded, not that a human read the
answer or that the agent's task succeeded.

```toml
[[hooks]]
event = "reply.completed"
type = "command"
command = "python3 /path/to/handle_reply.py"
async = true
timeout = 2
```

HTTP hooks receive a JSON event; command hooks receive metadata in `CC_HOOK_EXTRA`.
The event includes the project, platform, session key, timestamp and these fields:

| `extra` field | Meaning |
| --- | --- |
| `source_id` | Triggering message ID from the delivered reply route |
| `answer_id` | Actual delivered message ID, including replacement messages |
| `chat_id` | Group ID, checked against the delivered handle |
| `completed_at_ms` | Local completion timestamp in milliseconds |

No message body, display name, hook command, or credentials are added to this
payload. Existing inbound authorization and reply routing remain unchanged.
Hook consumers receive only the configured command/HTTP delivery; the event does
not authorize additional data access or choose a notification recipient.

Unlike `message.observed`, this event uses ordinary best-effort Hook delivery:
`hooks.async` is honored (default true); false waits for the hook. Hook failures
are logged without undoing the answer, retrying the hook, or requesting platform
redelivery. There is no durable outbox, replay after restart, or exactly-once
guarantee. Consumers should deduplicate by routing IDs rather than timestamps
and arrange their own durable processing if needed. The runtime capability
`reply_completion_hook` describes availability, side effects and fallback without
exposing configured hook bodies.

## 中文

在 `[[hooks]]` 中配置 `reply.completed`（或 `*`），可在支持的平台确认群聊
`display.card_mode = "rich"` 非静默最终答复送达后接收路由元数据。飞书/Lark
支持此能力，包括富卡更新、替换卡片，以及可追踪的原生文本替换。私聊、legacy/
普通文本模式、静默答复、失败或取消的最终投递不触发；中间流式更新不触发。
三个路由 ID 必须齐全，缺失时不触发。
送达仅表示平台发送/更新成功，不表示用户已读，也不表示 Agent 任务成功。

HTTP Hook 接收 JSON，命令 Hook 从 `CC_HOOK_EXTRA` 读取上表字段。
事件还包含项目、平台、会话键和时间戳，不附加正文、显示名称、Hook 命令或凭据。
`source_id` 为本次投递路由的触发消息 ID，`answer_id` 为实际送达（含替换）的消息
ID，`chat_id` 与投递句柄的群 ID 校验一致，`completed_at_ms` 为本机完成时间。
原有入站权限与回复路由保持不变；事件不授予额外数据访问权限，也不选择通知接收人。

与 `message.observed` 不同，本事件遵循普通 Hook 的尽力交付语义：遵循
`hooks.async`（默认 true），false 时等待 Hook 完成。Hook 失败记录日志，但不
撤回答复、不重试 Hook、不请求平台重投。没有持久化 outbox、重启补发或严格一次
保证。消费者应按路由 ID 而非时间戳幂等处理，需要时自行安排持久处理。
运行时 `reply_completion_hook` 能力描述可用条件、副作用和回退，不暴露配置的
Hook 内容。真实飞书群仍需独立端到端验收。
