# 私聊完成通知 / Private completion notifications

## 中文

启用后，任务成功且最终答复已送达时，由**处理该任务的同一个飞书/Lark 机器人**立即私聊发起人一次，附简短摘要和原对话链接。Codex、Claude Code 共用 Engine 完成链路。

- 原会话/话题只保留正常答复，不再追加短完成通知。
- 不等待一小时，不查询已读，也不在之后重复提醒；私聊通知本身不会启动模型或新的 Agent 回合。
- 只通知配置名单中的当前回合发起人，并继续遵守 `allow_from` / `allow_chat`。不会群发给所有管理员或名单中的其他人。
- 失败、取消、静默 `NO_REPLY`、未到终态和最终答复投递失败的回合不通知。通知失败不影响已经成功送达的答复。

在各项目已有的 `[projects.platforms.options]` 中配置并重启 daemon：

```toml
completion_notify = true
completion_notify_users = ["ou_your_id_in_this_bot_app"]
```

默认关闭。用户 ID 是应用范围的 open_id，不支持 `*`；不同机器人须使用各自应用下的正确 ID。未在名单里的发起人不会收到通知。私聊发起的任务也由当前机器人发送一条完成通知，链接指向原私聊/话题。

通知示例：

```text
✅ 已完成：项目 · 原对话名称 · 结果摘要
点击回到原对话：
https://applink.feishu.cn/client/chat/open?openChatId=oc_example
```

链接优先定位原话题及触发消息位置；无法查询消息元数据时，使用已有的会话/话题 ID 打开原对话。使用机器人租户身份发送，无需用户 OAuth。机器人须能向接收人私聊；可用的消息读取权限用于补全定位信息，不再依赖已读查询权限。飞书/Lark 客户端仍要求接收人有权访问原对话。

### 从 v0.3.2-beta.1 迁移

- `unread_reminder` 是 `completion_notify` 的兼容别名。
- `unread_reminder_users` 是 `completion_notify_users` 的兼容别名。
- 新字段分别优先于对应旧字段，包括显式 `false` 和空名单。
- `unread_reminder_delay_mins` 已废弃：旧合法值仍可解析，但完全不延迟发送，请删除。
- 旧 `data_dir/feishu-unread-reminders/` 队列不再加载或发送，文件保持原样，避免升级后补发历史通知。

### 去重与验证边界

`data_dir/feishu-completion-notifications/` 仅保存完成通知尝试的散列 ID 和时间，不保存摘要、用户 ID 或凭证；按项目、机器人应用和平台域隔离，文件权限 `0600`。最多保留七天内的 1000 条记录，新通知到来时清理过期记录，没有后台轮询。

私聊前先持久记录尝试，API 重试沿用同一个 UUID。网络结果不确定或记录后崩溃可能漏通知，但重启不会补发。存储不可写时停止通知；已有去重文件不可读或损坏时拒绝启动，避免重复发送。达到记录上限时跳过新通知并记录错误。

本地自动化覆盖即时私聊、原话题无追加消息、接收人/机器人身份、链接和降级、关闭/权限/取消、并发去重、重启和存储失败。真实飞书/Lark 收发及客户端点击跳转仍为 **UNVERIFIED**。

## English

Opt in to one immediate private completion notification from the same Feishu/Lark bot after a successful final answer is delivered. The notification contains a short summary and a link to the original conversation. Codex and Claude Code share the same Engine completion path.

Configure `completion_notify = true` and explicit app-scoped `completion_notify_users` in each existing platform options table, then restart. Disabled by default; only the completed turn's owner is notified, subject to sender/chat access rules. No wildcard, implicit recipients, topic notification, read-status lookup, waiting period or later reminder. Failed, canceled, silent, nonterminal and unsuccessfully delivered turns do not notify. Private conversations are eligible too.

The bot sends using tenant authentication, without user OAuth. Links use the original topic and trigger position when available, falling back to the known conversation when metadata lookup fails. The recipient still needs access to that conversation. Existing read access improves link positioning; read-receipt access is no longer needed.

Legacy `unread_reminder` and `unread_reminder_users` remain aliases, overridden individually by the corresponding new fields. Valid legacy `unread_reminder_delay_mins` values are accepted but ignored; remove this option. Old unread-reminder queues are left untouched and never replayed.

Only hashed attempt IDs and times are persisted, scoped by project/app/domain, with owner-only permissions. At most 1000 receipts are retained for seven days and pruned on new completions, without a worker. Durable claims and stable request UUIDs suppress duplicates; ambiguous sends or a crash after claiming may lose a notification. Storage failures stop notifications rather than lose deduplication. Full receipt stores skip new notices with a logged error.

Local tests cover routing, timing, links, access, cancellation, deduplication and persistence. Real Feishu/Lark delivery and client navigation remain **UNVERIFIED**.
