# Agent Capability Manifest

Agent Capability Manifest 是 cc-connect-next 当前版本、项目和会话的统一只读能力契约。它解决的不是“源码里大概有什么”，而是让连接中的 Agent 能准确回答：

- 当前项目能做什么、怎样调用；
- 哪些配置项真正存在，应该写在哪里；
- 当前有哪些内置/自定义聊天命令和 Skills；
- 活动 Agent、Agent session 与 Platform 实际实现了哪些可选接口；
- 每个操作需要什么参数和权限；
- 操作是否只读，会写入什么或触发哪些外部副作用；
- 缺少能力时会如何退化；
- 当前为何可用、条件可用或不可用。

## Agent 查询入口

运行中的 Agent 会在首个回合收到有界能力摘要，并被要求在回答具体能力问题或调用 cc-connect-next 操作前查询：

```bash
cc-connect-next capabilities --search "2 到 4 个关键词"
```

示例：

```bash
cc-connect-next capabilities --search "群聊 已有话题 隔离" --lang zh
cc-connect-next capabilities --search "切换模型" --lang zh
cc-connect-next capabilities --search "发送原生音频" --format json
cc-connect-next capabilities --all --search "Slack"
```

默认查询聚焦当前项目的活动适配器；询问未配置但已编译的 Agent/Platform 时使用 `--all`。这会加入全部适配器配置契约，并为未激活适配器返回 `compiled but not active` 与 `configure-and-restart` 原因/退化说明。

`--project` 和 `--session-key` 默认从 `CC_PROJECT` / `CC_SESSION_KEY` 获取；只有一个项目或活动会话时，runtime 也可自动选择。CLI 通过权限为 `0600` 的本机 Unix Socket 查询正在运行的 daemon，不会连接消息平台或修改状态。

daemon 未运行时，运行态 Manifest 无法成立。此时只能使用下面的静态构建期配置契约：

```bash
cc-connect-next config capabilities --search "关键词"
```

## Manifest 组成

JSON Schema 标识为：

```text
cc-connect-next.agent-capabilities/v1
```

顶层区段：

| 字段 | 内容 |
|------|------|
| `configuration` | 合并自配置契约的活动项目配置能力；不包含当前值 |
| `tools` | Agent 可调用的本机 CLI 工具，如能力查询、发送附件、Cron、Timer、Relay |
| `commands` | 内置命令及当前项目自定义命令 |
| `skills` | 当前 Agent Skill 目录实际发现的 Skills |
| `runtime` | 活动 Agent、Agent session 和各 Platform 的接口能力 |

每个可执行能力统一说明：

- `parameters`：参数名、类型、是否必填、允许值；
- `permission`：`member`、`admin`、`conditional` 或 `local-agent`；
- `read_only`：整个操作是否只读；
- `side_effects`：可能发生的配置/文件/进程/网络/外部消息等副作用；
- `fallback`：能力缺失时的退化或拒绝契约；
- `availability`：当前状态和原因。

可用性状态：

| 状态 | 含义 |
|------|------|
| `available` | 当前运行态和已知上下文满足前提 |
| `conditional` | 需要活动回合、调用者身份或执行时参数才能最终判断 |
| `unavailable` | 已确认缺少接口、运行组件、权限或配置；`reason` 说明原因 |

Manifest 会保留不支持的运行态能力条目，而不是静默删除。例如 Platform 不实现原生视频时，`video` 会显示 `unavailable`，同时说明存在 `FileSender` 时退化为文件投递。

## 权限与副作用是真实运行契约

内置命令权限直接遵循 Engine dispatch：

- `/shell`、`/show`、`/dir`、`/diff`、`/web`、`/upgrade`、`/restart` 需要 `projects.admin_from`；
- `/commands addexec`、`/cron addexec` 和 `/timer addexec` 仅对应 Shell 注册子操作需要管理员权限；
- 所有带 `Exec` 正文的已注册自定义命令在实际调用时同样需要管理员；Prompt 自定义命令保持 member 权限；
- 项目级 `disabled_commands` 会反映为 `unavailable`；用户角色级禁用和具体调用者身份仍由真实命令分发在调用时检查，因此 Manifest 将其保留为权限条件而不猜测身份；
- 自定义 Prompt/Exec 命令和 Skills 按当前真实调用规则展示，并明确其 Agent turn 或 Shell 副作用。Agent 命令文件只有显式 frontmatter `description` 可进入 Manifest，Markdown Prompt 正文的首行也不会被当作说明发布。

Manifest 不把“命令存在”误写成“当前一定能执行”：模型、Provider、TTS、多工作区、Cron/Timer、Relay、Web、活动回合等都会检查对应运行态接口或组件。

## 安全边界

Manifest 只返回能力元数据，明确不返回：

- 当前配置值或凭证；
- Skill 指令正文与本地来源路径；
- 自定义 Prompt/Exec 正文或 Shell 命令正文；
- management token、bridge token、Provider key 等秘密。

动态描述和运行态错误会经过与反馈通道同等级的凭证、长 blob、飞书 ID 和主目录路径脱敏。查询本身只读；Manifest 中的 `side_effects` 描述未来调用该能力的后果，不表示查询已经执行了这些操作。

## 其他消费入口

- 本机 API：`GET /capabilities?project=...&session_key=...&search=...`
- Management API：`GET /api/v1/projects/{name}/capabilities`
- Web Chat：命令面板从 Manifest 读取内置/自定义命令和 Skills；暂时不可用的项会禁用并显示原因
- Bridge `capabilities_snapshot_v1`：旧命令投影由 Manifest 命令契约生成；协议 v1 尚未直接内嵌完整 Skills/配置对象

配置项的完整逐项参考仍由同一个配置契约生成，见[配置能力参考](configuration.zh-CN.md)。
