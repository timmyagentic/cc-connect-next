# Feishu Plus

CC Connect Feishu Plus 保留原生 `feishu` 和 `lark` 平台适配器。增强能力直接编译进
兼容发行版，并放在明确的飞书平台配置开关后；现有配置不启用 Plus 时继续使用原生行为。

## 基础版功能：可自愈的身份 fail-closed

原生适配器需要先获得机器人的 `open_id`，才能判断群消息是否真的 @了机器人。
上游在查询失败后会继续启动并关闭 @过滤，导致无关群消息也可能被接收，直到手动重启。

在现有飞书平台配置中启用：

```toml
[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "your-feishu-app-id"
app_secret = "your-feishu-app-secret"
plus_enabled = true
plus_identity_mode = "retry"
```

身份模式：

| 模式 | 无法获得机器人身份时的行为 |
| --- | --- |
| `retry` | Plus 默认值。阻止需要 @的群消息，私聊继续可用；后台指数退避重试，成功后自动恢复，无需重启。 |
| `fail_closed` | 阻止需要 @的群消息直到进程重启；私聊继续可用。 |
| `legacy` | 保留上游 fail-open 行为，只用于兼容。 |

不设置 `plus_enabled` 或将其设为 false 时，所有行为与原生路径一致。身份保护当前只应用于
WebSocket 模式；私有部署的 webhook 模式可能没有相同的机器人信息 API，因此保持原行为。

## 兼容规则

- 同一个飞书应用绝不创建第二条竞争连接。
- 继续使用现有配置和会话数据目录。
- 每次只增加一个可独立测试的功能开关。
- Plus 关闭时，默认原生行为必须不变。
- 所有深入飞书链路的改动必须有回归测试和明确回滚路径。
