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
| `tools` | Agent 可调用的本机 CLI 工具，如能力查询、发送附件、Feedback、Cron、Timer、Relay、延迟 daemon 重启 |
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
- Agent 调用的 `cc-connect-next daemon restart` 同样需要管理员权限，且只在持有由 Agent session 私有 secret 派生的精确 HMAC 回合凭证时条件可用；客户端不提交 project、session、user 或 platform 路由；
- `local-agent` Feedback 工具使用同一 HMAC 凭证解析可信身份；preview 返回完整 Draft 且零网络，只有绑定发起用户/session 的独立一次性 token 才能进入共享 Relay 提交路径；
- `/commands addexec`、`/cron addexec` 和 `/timer addexec` 仅对应 Shell 注册子操作需要管理员权限；
- 所有带 `Exec` 正文的已注册自定义命令在实际调用时同样需要管理员；Prompt 自定义命令保持 member 权限；
- 项目级 `disabled_commands` 会反映为 `unavailable`；用户角色级禁用和具体调用者身份仍由真实命令分发在调用时检查，因此 Manifest 将其保留为权限条件而不猜测身份；
- 自定义 Prompt/Exec 命令和 Skills 按当前真实调用规则展示，并明确其 Agent turn 或 Shell 副作用。Agent 命令文件只有显式 frontmatter `description` 可进入 Manifest，Markdown Prompt 正文的首行也不会被当作说明发布。

Manifest 不把“命令存在”误写成“当前一定能执行”：模型、Provider、TTS、多工作区、Cron/Timer、Relay、Web、活动回合等都会检查对应运行态接口或组件。

## 本聊天模型与全局默认

`model` 命令公布 `/model [<name|alias|number>|reset|default [<name|alias|number>]]`。
唯一子命令是 `default`，动作允许值与 `privileged_when` 均仅包含 `default`。
权限为 `conditional`：`/model` 和 `/model default` 是成员可用的只读查询；
设置或重置本聊天模型也是成员操作。只有 `/model default <name|alias|number>`
修改全局默认需要管理员权限。原有角色和禁用命令策略继续生效；分发时检查具体调用，
因此查看默认值不要求管理员权限。

`/model default` 查询或修改当前机器人/项目 Agent 的全局默认模型，
不针对工作区 Agent 或其他机器人。只需 `ModelSwitcher` 即可提供这些操作，
不要求后端支持逐回合模型。不再有独立的工作区默认模型层：未绑定模型的聊天跟随该全局默认。

`/model` 查询、固定或重置已验证私聊或群聊的模型。固定模型另要求配置的当前后端
实现 `PerTurnModelSupporter` 并返回 true（目前 Codex `exec` 与 `app_server` 支持），
模型属于当前可选列表，且持久化存储可用。操作不切换 Provider。调用时必须验证聊天身份，
不能仅凭 session key 形式推断；尚未识别的聊天可先在此发送 `/model`。
飞书/Lark 中绑定已验证私聊用户的卡片回调也能建立私聊路由，无需先收到新的文字事件。
后端不再支持固定模型时，只要聊天身份已验证且存储有效，reset 仍可清除已有绑定。
命令级可用性原因分别说明全局默认值操作与聊天操作条件。

参数契约明确在各机器人/项目内隔离：

- 群使用 `platform:groupID` key（平台 + 稳定群 ID），全部成员和新旧话题共享绑定。
- 已验证私聊使用 `platform:dm:userID`，按私聊用户隔离，与其他用户、群聊、平台和
  机器人/项目互不影响，共用工作区目录的机器人也不共享绑定。

两种绑定均不按话题、会话或工作区目录划分，跨 `/reset`、`/new` 及重启持久保留。
项目状态的 `chat_model_bindings` 保存 `ChatModelBinding`；会话 `ChatKey`
对应 JSON 字段 `chat_key`。聊天设置声明 `persistent_state` 副作用；
`configuration`、`agent_process` 描述全局默认模型变更。整体命令因同时包含设置操作
和只读查询而保持可写。模型和推理强度独立持久化，`/model reset` 只清除模型绑定。
不需要新增 `config.toml` 字段。

旧模型状态字段不迁移；此前保存的聊天模型绑定和工作区默认模型不会沿用，
需要通过上述命令重新选择。

全部三条发送路径在执行时分别解析各字段，包括排队请求。变更在下次新回合生效，
不打断当前回复。`/fast`/回答预设或任务显式指定的单次模型只覆盖模型字段；
没有显式覆盖时，普通消息使用本聊天固定模型，没有绑定时跟随当前机器人/项目的全局默认。
未显式指定模型的预设仍继承聊天模型。`reject` 契约覆盖逐回合能力不支持、
后端/Provider 不匹配、不可选模型、存储及模型应用错误；固定模型的请求不能静默改用其他模型。
恢复时应执行 `/model <name>` 显式重新选择，或执行 `/model reset` 清除模型绑定。
无效参数和过期卡片操作返回当前用法，不改变设置。

只读会话能力 `model_provider_identity` 检查 `ModelProviderSession`，不公开其记录的
Provider 名称。可切换 Provider 的 Agent 执行聊天固定模型请求时需要此能力；引擎同时校验
实际会话和当前 Agent 的身份。已有对话仍连接其他 Provider 时，应通过
`/provider switch <name>` 重新连接，或使用 `/new`，然后重试。

本聊天卡片显示实际模型及固定/跟随默认状态，只有一个选择框，使用 `cmd:/model MODEL`
和 `cmd:/model reset`。二级按钮 `cmd:/model default` 打开独立的全局默认卡片，
其唯一选择框使用 `cmd:/model default MODEL`。模型值使用稳定标识，不使用列表序号。
命令和卡片选择均返回对应卡片及结果提示；不支持卡片的平台接收文本渲染。
`nav:/model` 打开本聊天菜单。本聊天模型状态及错误提示使用 `MsgModelChat*` i18n 键。

推理强度卡片仍使用 `cmd:/reasoning chat ...`（含 `reset`），`nav:/reasoning` 仍打开
强度菜单；`/reasoning` 和 `/effort` 的语义不变。

## 聊天推理强度

`reasoning`/`effort` 命令公布 `/reasoning [<level>|chat [<level>|reset]]`。
`/effort` 是 `/reasoning` 的别名，`group` 保留为 `chat` 的子命令别名，在已验证私聊和
群聊中都可使用。`ReasoningEffortSwitcher` 保留原默认值切换；直接指定等级更改
项目/工作区 Agent 默认强度并重置本对话，已固定强度的聊天保持原设置。
聊天固定强度另需 `PerTurnReasoningEffortSupporter`、非空强度列表、已验证聊天身份、
持久化存储及实际会话/Provider 的逐回合兼容性。保持 `member` 权限，声明 `persistent_state`。
不支持的强度、缺少身份/存储、后端/Provider 或会话不匹配均明确失败，不静默回退。
恢复时应执行 `/reasoning chat <level>` 重新选择，或执行 `/reasoning chat reset`
清除强度绑定；即使后端不再支持固定强度，只要身份已验证且存储有效，仍可 reset。

聊天 effort 独立于模型保存，使用项目内相同的群或私聊 key。群内覆盖全部成员及话题，
私聊按已验证用户和机器人/项目隔离，跨 `/new`、`/reset` 和重启保留。
排队请求执行时分别解析模型和强度，任务/预设显式指定的字段分别优先。
仅指定模型的预设仍继承聊天强度，仅指定强度的预设仍继承聊天模型。
重置一项保留另一项，进行中的回复不被打断。命令元数据明确默认值切换会重置对话，
聊天操作则作用于独立范围并保留历史。

## Cron 任务模型

`cron` Agent CLI 工具契约包含可选 `model` 参数：仅 Prompt 任务可设置，省略继承，`cron edit <job-id> model ""` 恢复继承。注册/编辑写入任务持久状态；执行产生 Agent 回合及网络请求，但不切换 Provider、不修改项目或 Agent 默认模型。Shell 任务和不支持的 Agent 会拒绝显式模型，不能自动退回默认模型。

同一工具契约还包含可选字符串参数 `reasoning_effort`：Prompt 任务使用 `cron add --reasoning-effort <value>` 或 `cron edit <job-id> reasoning_effort <value>`，可独立于模型覆盖设置。省略时继承；`cron edit <job-id> reasoning_effort ""` 恢复继承。有效强度与 Agent 支持由服务端校验，通用 CLI 元数据不枚举特定 Agent 的取值。Shell 任务、不支持的 Agent 和无效强度会拒绝显式覆盖。注册/编辑仅修改该任务持久状态，强度作用于它的 Agent 回合，不修改项目/Agent 默认值。CLI 列表和单字段 info 查询显示指定强度或继承状态，完整 info 保留可选 JSON 字段。

运行态能力 `per_turn_reasoning_effort` 仅在 Agent 实现 `PerTurnReasoningEffortSupporter` 并返回 true，且通过 `ReasoningEffortSwitcher.AvailableReasoningEfforts()` 提供非空目录时可用。缺少支持或目录缺失、为空时仍显示为 `unavailable`；具体会话能力在执行时检查。运行态 `cron` 工具参数通过 `allowed_values` 提供该 Agent 的目录，通用元数据不硬编码 Provider 专属列表。按能力简报指引执行 `cc-connect-next capabilities --search "cron reasoning effort"` 可查询参数及运行态契约。Web 编辑器同时查询 `runtime` 和 `tools`，提供继承选项及已声明的强度；无可用列表时退回由服务端校验的文本输入。

Agent 运行态条目 `per_turn_model` 通过 `PerTurnModelSupporter` 探测，不依赖 Agent 名称；缺失接口或返回 false 时仍保留为 `unavailable` 并说明拒绝原因。声明支持后，实际执行仍检查会话的 `TurnOptionsSession`；已有 `turn_options` 会话条目展示该能力。无活动会话不等于支持或不支持：会话条目保持 `conditional`，Agent 声明供创建任务前检查。当前 Codex `exec` 与 `app_server` 支持该契约。具体模型 ID 的可用性仍由当前 Agent/Provider 执行时决定，Manifest 不承诺模型目录或凭证有效性。

## 近期讨论背景

可选 Platform 能力 `recent_conversation_context` 在已核验的用户回合前，使用已有平台权限只读补全有界、不可信的讨论。引用记录不执行命令、不授予工具权限。可用性区分配置未启用、目标不适用以及仍需已有历史访问权限的条件状态；不支持的适配器保留条目和仅使用当前请求的降级说明。选中的历史支持带归因的转发文字、图片和文件，附件在去重及文字预算筛选后有界下载。读取失败、截断和附件不可用分别提示；缺失内容保留标记，附件失败仍可重试。

对应的 `group_context` 配置默认关闭，只为通过群回复规则的授权请求按需读取，不轮询；去重 ID 只在回合成功后持久化，忙时请求排队以保持顺序。

## 会话速度

`/speed` 查询后续回合速度、当前模型支持的档位，并在忙碌时显示当前回合速度。`/speed fast` 保存当前会话的速度；`/speed default` 明确选择标准速度。切换不改变模型和推理强度，不重启桥接，也不打断当前回合。跨速度档位的消息会排队开启新回合，即使启用了忙碌消息 steering。已排队消息在开始执行时读取当时的设置。`/fast`、`/quality` 仍只覆盖自己的单次回合，普通回答随后恢复当前会话的速度。

选择写入会话存储，桥接重启后保留；`/new` 继承项目默认值，切回旧会话恢复其选择。未启用存储时，回复明确提示仅本次进程有效。命令需要成员访问权限，遵循用户角色和禁用命令策略。Manifest 声明会话状态和持久状态副作用；不带参数时只查询。

已连接的 Codex app-server 通过实时 `model/list` 目录校验档位。会话建立前，本地 Codex 根据其实际 `CODEX_HOME` 中的模型目录校验：优先使用 `model_catalog_json`，否则读取 `models_cache.json`。目录中的 `priority` 对外显示为 `fast`，也兼容旧目录的 `additional_speed_tiers`。模型未知、目录缺失、档位不支持、尚未建立会话而无法核验的远程服务或自定义 CLI 参数，以及保存失败都会明确报错。目录缺失或过期时可通过 Codex CLI 刷新。模型/Provider 改变后，后续回合会重新校验已保存的选择，不静默降级。

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

平台自动动作也可带有 `permission`、`read_only` 和 `side_effects`。例如 `completion_notification` 会明确报告未启用或条件可用，不暴露配置中的收件人 ID；其[配置与投递契约](completion-notifications.md)包含持久去重记录和机器人即时私聊消息副作用，不查询已读、不安排延迟提醒。

## 图片图集

批量发送本地图片时，在**同一次** `cc-connect-next send` 调用中重复传入
`--image`。运行态 `image_batches` 能力说明当前平台是否支持图集合并。
飞书/Lark 将两张及以上图片按输入顺序组织为 Card 2.0 `img_combination`
图集：双列布局，每条消息最多五张原图；十张图只发送两条消息。
单张图片仍为原生图片消息，最后一组可不足五张，不合成或裁剪上传的原图。

`enable_feishu_card` 开启时默认使用图集；该现有选项为 false 或平台不支持
`image_batches` 时仍逐张发送。附件开关、大小限制、出站限流、工作目录、
聊天/话题及引用回复边界保持生效。此功能不改变远程 Markdown 图片解析，
Markdown 中的本地路径不会自动上传，应使用 `send --image`。

每批先上传全部图片，再发布一条卡片；上传失败不会发布残缺图集。
消息瞬时失败重试复用同一飞书 UUID，不重发前面已成功的批次。
最终失败时命令返回错误和已确认发送的图片数量，并停止后续批次及文件。
再次执行 CLI 是新投递；应先核对失败批次是否已被远端接收，再仅补发未确认
的剩余图片，避免响应丢失导致重复。当前没有跨调用持久重发队列。

## macOS doctor 进程身份与隐私权限边界

`cc-connect-next doctor` 和聊天中的 `/doctor` 同时保留 Agent CLI 的解析路径和版本。
在 macOS 上还会依次列出诊断进程及最多七级祖先进程的 PID、PPID、可执行文件名称或路径。
只读取 `ps` 可执行文件元数据，不读取进程参数或环境变量。检查最多持续两秒；进程消失、
超时、循环或深度限制均标记为信息不完整。

CLI 展示自己的进程链；聊天中的 doctor 在服务进程中执行。二者都不代表所有 Agent/工具
进程清单，也不能确定 macOS 将 TCC 权限归属于哪个应用。屏幕录制、辅助功能、Apple Events
均明确标为**未知（未探测）**，Apple Events 还取决于目标应用。警告不等于权限被拒绝。
不读取 TCC 数据库，不触发授权弹窗或启动应用，不重置权限；请勿仅凭此报告重复授权或重启。
无需新增配置或原生编译依赖，其他操作系统保留原有检查。公开分享前请移除私人可执行文件路径。
