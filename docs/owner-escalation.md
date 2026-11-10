# 回复后自动 @ 负责人 / Post-answer owner escalation

启用后，cc-connect-next 在正常答复送达后，调用独立的 GPT-6 Luna 判断是否需要负责人介入。需要介入时，在原群聊或原话题中发送一条真实 @ 消息，包含问题摘要和需要负责人处理的事。不发送私聊，也不追加“已经通知”的第二条回执。

## 配置

在目标项目中添加以下配置，并在启动服务的环境中提供模型密钥。不同机器人的飞书 open_id 不通用，负责人必须属于当前机器人可访问的原群。

```toml
[projects.owner_escalation]
enabled = true
owner_id = "ou_owner"
owner_name = "负责人"
base_url = "https://api.openai.com/v1"
api_key = "${OWNER_ESCALATION_API_KEY}"
model = "gpt-6-luna"
timeout_seconds = 20
```

默认关闭，修改后重启生效。可以使用兼容 Responses API 的服务；只有回环地址允许明文 HTTP。所有项目独立配置，不会从管理员名单或最近会话猜测接收人。完整字段说明见[配置目录](configuration.zh-CN.md)。

## 判断和投递

- `resolved`：不需要负责人介入，不发送通知。
- `waiting`：正常追问、等待请求人验证、任务仍在执行，不发送通知。等待负责人作决定属于 `needs_owner`。
- `needs_owner`：无法合理继续、需要负责人决定或协调，或请求人明确要求人工接手；发送原会话 @。

判断输入是最近 12 条会话记录和本轮最近 8 条工具结果。内容经过截断和已有的敏感信息过滤；不提供执行工具或隐藏推理。模型需输出具体依据、摘要和需要负责人做的事。近期记录显示同一事项已转交且没有新进展时，应避免重复通知。这是模型判断，不能保证完全没有误报或漏报。

负责人自己发起的请求、定时任务、静默答复、已取消回合和未能送达的答复不触发判断。模型/适配器报错回合也不自动转交，避免将服务故障直接变为人工提醒；正常答复中报告的业务受阻可以触发转交。

当前投递实现支持飞书群聊和话题。发送前验证原消息路由及负责人当前群成员身份，使用原生文本 @ 和原消息回复接口。P2P、负责人不在群中、成员读取失败或消息撤回时，不会改成私聊或其他群。发送失败会保留原答复并尽可能说明未能确认 @ 成功；模型超时或输出无效则记录判断失败，不伪装成 `resolved`。

同一个原消息的发送尝试使用固定幂等键，并在 session 文件旁的 `.owner-escalation.json` 中保留 7 天的哈希回执。无法持久化时不发送。发送结果不确定时不自动重放；这避免重复打扰，但不提供保证必达的后台重试队列。原答复先送达；判断受配置的超时限制，后续发送另有 15 秒上限，失败提示最多 5 秒，然后继续排队消息。

## English

This opt-in project feature independently judges a delivered foreground answer with GPT-6 Luna and mentions the configured owner in the original Feishu group/topic when human intervention is needed. It never sends a DM. The owner must already be a member of that group. Configuration changes require a restart.

The judge receives bounded recent conversation and execution evidence and returns `resolved`, `waiting`, or `needs_owner`. Normal clarification, verification and ongoing work stay quiet; waiting for an owner's decision requires escalation. The owner’s own requests, scheduled turns, silent/cancelled/failed deliveries and agent-error turns are excluded. Invalid judgments are logged as failures, not successful decisions. Durable per-message claims suppress duplicate and uncertain sends for seven days; semantic duplicate detection relies on the recent conversation. No background delivery/retry queue is added.
