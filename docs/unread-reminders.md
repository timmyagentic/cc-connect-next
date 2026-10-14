# 未读完成通知提醒 / Unread completion reminders

## 中文

此功能由共用 Engine 和飞书适配器实现，Codex、Claude Code 使用同一条链路。默认关闭，按每个项目的飞书机器人单独启用。

### 行为

1. 用户回合成功，且最终答复已成功送达后，在原会话/话题额外发送一条短完成通知。即使此前看过流式过程卡片，这条新通知仍有独立的已读状态。
2. 从新通知成功发送时计时，一小时后查询该用户是否已读。后台每分钟检查一次，因此正常情况下在一小时至一小时一分钟之间处理到期记录；断网、积压或机器休眠会延后。
3. 未读时由**同一个机器人**向回合发起人的私聊发送一次提醒，带摘要和原会话入口。同一检查批次最多聚合 20 条，避免每条单独发送。
4. 已读、在原话题继续发消息、或原触发消息/完成通知被撤回时，取消待提醒。其他人或其他话题的回复不会取消。继续回复的判断限于机器人实际收到的消息事件。

仅提醒 `unread_reminder_users` 中明确列出的发起人，并继续遵守现有 `allow_from` / `allow_chat`。不会向全部管理员或最近活跃会话发送。静默 `NO_REPLY`、失败/取消的回合、发送失败的最终答复、未到终态的结果事件，以及定时通知本身，都不会产生完成提醒。

### 配置两个机器人

在**各自已有**的 `[projects.platforms.options]` 下添加：

```toml
# Codex 项目的飞书机器人
unread_reminder = true
unread_reminder_users = ["ou_your_id_in_codex_bot_app"]
```

```toml
# Claude Code 项目的飞书机器人
unread_reminder = true
unread_reminder_users = ["ou_your_id_in_claude_bot_app"]
```

这两个片段分别属于两个项目的已有选项表，不要粘贴到同一张表。修改后重启 daemon。`open_id` 属于应用范围：即使是同一个人，在两个机器人应用里的 ID 也可能不同。使用各应用已有的正确 ID，不要互相复制，不支持 `*`。

机器人依赖已有的消息已读查询、消息读取和私聊发送权限；不新增用户 OAuth 流程或事件订阅。权限/API 错误不代表未读：检查失败会有限退避重试，最终停止该条提醒并记录日志。

### 持久化与限制

记录保存在 `data_dir/feishu-unread-reminders/`，按项目、机器人应用和平台域隔离，文件权限 `0600`。内容包括路由 ID、短摘要、完成时间和状态，不包含应用密钥或访问令牌。队列最多保留 1000 条记录，七天后清理；启动时恢复，停用功能不会新增或发送提醒。

私聊发送前先持久标记已尝试，网络重试使用同一个 UUID。若进程在标记后、实际发送前崩溃，或发送结果不确定，该条可能漏提醒，但不会在重启后重复发送。运行中存储失败时停止发送，防止失去去重记录；启动时已有记录无法读取或解析，会报告错误并拒绝启动该平台，避免覆盖去重状态。已读查询只证明完成通知被客户端标记已读，不证明最终答复已被理解或事情已推进。

本功能不调用模型、不启动额外 Agent 回合，不修改安装版本，也不会自动为现有配置启用。

可通过 `cc-connect-next capabilities --search "未读提醒"` 检查运行态能力，或用 `cc-connect-next config capabilities --search "unread reminder"` 查询配置契约。

## English

This opt-in feature uses the shared Engine completion path, including Codex and Claude Code. Add `unread_reminder = true` and an explicit `unread_reminder_users = ["ou_user_in_this_app"]` list to **each existing** `[projects.platforms.options]` table and restart the daemon. User open IDs are app-scoped; configure each bot separately. The omitted default is disabled.

After a successful, visible final answer, the same bot posts a short, fresh completion notice in the original conversation/topic. This avoids treating a previously read streaming progress card as proof that the final answer was seen. After one hour, a minute-based worker checks the recipient's read status and sends at most one private reminder with a summary and conversation link. Due items for the same recipient are grouped in batches of up to 20. Network outages, backlog, or sleep may delay delivery.

Only explicitly configured turn initiators are eligible, still subject to existing sender/chat restrictions. Reading the notice, continuing in the original topic (when the bot receives that event), or recalling the trigger/notice cancels the pending reminder. Other users/topics cannot cancel it. Silent, failed, canceled, unsuccessfully delivered, and nonterminal turns do not schedule reminders. Reminders do not recursively schedule more reminders or invoke the agent.

The adapter uses existing bot access to read messages/readers and send privately. Failed read queries are **unknown**, never unread; bounded exponential retries stop after six failed checks. No new OAuth flow or event subscription is introduced.

Records survive restarts under `data_dir/feishu-unread-reminders/`, isolated by project, app and domain, with owner-only file permissions. The queue is bounded to 1000 records with a seven-day retention window. It stores routing IDs, short summaries and state, never app credentials. Disable the option to stop new notices and reminders.

A durable claim precedes each private send, with a stable UUID for in-flight HTTP retries. An ambiguous send or a crash between the claim and the send can lose a reminder; it is deliberately not resent after restart. Runtime storage failures stop sending rather than lose deduplication. An unreadable or corrupt existing store rejects platform startup with an error instead of overwriting deduplication state. A read receipt only means the completion notice was marked read, not that the work was acted upon.
