# 写给官方 CC Connect 用户：cc-connect-next 有什么不同，怎么直接迁移

[English](coming-from-cc-connect.md)

如果你正在用 [官方 CC Connect](https://github.com/chenhg5/cc-connect)，并且对它满意，你不需要换——它依然是一个很好的项目，cc-connect-next 也正是从它 fork 出来的（v1.4.1，MIT）。这篇文章写给另一类用户：想要原生飞书卡片体验、更严格的隐私边界、或者"任务跑着的时候还能补一句"的用户。下面每一条差异都可以自己验证，文末给出不需要第二个飞书应用的直接迁移与安全回退路径。

## 先说清楚这是什么

cc-connect-next 是一个**独立后继**，不是补丁、代理或伴生插件：

- 单个 Go 二进制，拥有自己的命令（`cc-connect-next`）、数据目录（`~/.cc-connect-next`）、daemon 服务名和 npm 包；
- 普通安装和只复制迁移不修改官方 CC Connect；显式生产割接只停止并禁用其服务，binary 与数据始终保留；
- 对上游保持尊重：不做盲目 merge，而是对每个上游变更做逐条审计后决定采纳或刻意偏离（[审计策略与记录](upstream-v1.5.0-beta.3-audit.md)）。

## 五个真正不同的地方

### 1. 飞书回答是一张原生卡片，不是消息流

一次 Agent 回合从头到尾停留在**同一张**引用你提问的 Card 2.0 卡片里：`⏳ 正在思考` → `⏳ 正在调用工具` → `✍️ 正在回答`（CardKit 打字机流式）→ `✅ 已完成`。不刷屏、不拼接多条消息，最终答案和中间进度在同一个位置演进。精确的状态机和验证命令写在[飞书回答卡片契约](feishu-card-contract.md)里。

### 2. 隐私边界：进度元数据保持最小化

处理中，卡片只渲染“推理 N 次 · 工具 M 次”这样的匿名进度。推理文本、工具名称、参数、结果、模型名、token 数、工作目录，在事件层和渲染层**两层**被丢弃，卡片也没有可展开的详情面板。最终回答仍会作为可见正文发送并存储在飞书；如果回答引用了代码或 patch，这些内容也会经过飞书。这里的隐私保证针对隐藏的中间过程，不包括你要求 Agent 交付的最终答案。

### 3. 任务跑着的时候，可以直接补一句

官方的模型是 FIFO 排队：Agent 忙时来的消息等下一个回合。cc-connect-next 默认是 **steer**：新消息经 Codex 原生 `turn/steer` **并入正在运行的这个回合**，旧卡片冻结为灰色"已转到更新的消息"（保留已显示的内容），后续进度只在回复最新消息的卡片上继续，每个回合只有一张卡片到达"已完成"。不支持 steer 的 Agent 自动回退为排队，`busy_message_mode = "queue"` 可恢复官方行为，`/ps` 在任何模式下显式 steer。

### 4. 安装会自己照顾自己

`cc-connect-next update [stable|beta]` 显式选择通道并默认使用 Stable。独立安装使用 immutable Plan、同 Release checksum、双版本探针、锁、备份和回滚。daemon 跟随 `update_channel`，按通道/版本向配置的管理员**只提醒一次**，并要求先查看精确计划再确认。`doctor` 一条命令检查配置、Agent CLI 登录态、平台、依赖和网络。

### 5. 出了问题，保持短流程反馈

聊天里明确执行 `/feedback <描述>` 或点击 Feedback 动作，会立即经匿名作者中继提交有界、脱敏的 Foundation Draft，不展示预览、不要求二次确认，最终也不回显 Issue 链接。回合失败时 daemon 的自动提示在用户点击前始终零请求；一次点击即可提交，且不需要 GitHub 账号。

其余能力（14 种 Agent × 15 个平台、cron 与 webhook、语音输入输出、Web 管理台、五种语言 i18n）见[主 README](../README.md)。

## 迁移不需要第二个飞书应用

两套系统的边界完全隔离：

| 边界 | 官方 | cc-connect-next |
|---|---|---|
| 命令 | `cc-connect` | `cc-connect-next` |
| 数据 | `~/.cc-connect` | `~/.cc-connect-next` |
| macOS 服务 | `com.cc-connect.service` | `com.cc-connect-next.service` |
| Linux 服务 | `cc-connect.service` | `cc-connect-next.service` |
| API socket | `~/.cc-connect/run/api.sock` | `~/.cc-connect-next/run/api.sock` |

默认迁移直接复用现有飞书应用凭证：`migrate --switch` 会先停止并禁用官方 daemon，再启动 Next，因此不会主动制造两个同凭证消费者。你不需要创建测试应用。两套身份仍保持独立，官方 binary 和数据不会删除；只有主动做高级并行试用时才需要第二个应用。

## 一键迁移为什么敢用

`cc-connect-next migrate` 不是“复制目录”脚本。它先清点官方安装中受支持且当前可访问的持久化来源（`config.toml`、生效的 `data_dir`、已发现的项目本地 `.cc-connect` 目录），对**清点到的每个源文件计算 SHA-256**，在旁路暂存目录构建并验证结果，激活前重新核对源清单——清单内任何新增、删除、内容或权限变化都会**拒绝激活**。如果项目状态、binding 或项目目录不可读，可选项目发现仍可能被跳过；命令会打印每条 `skipped_project_discovery` 并写入 `migration-manifest.json`。必须先解决这些跳过项并重跑，才能把项目本地数据视为完整。激活采用原子重命名，已有目标会先做带时间戳的备份。担心的话，先跑：

```bash
cc-connect-next migrate --dry-run
```

它会打印完整计划而不写任何东西。支持的官方版本与配置项见[迁移兼容矩阵](migration-compatibility.md)（当前覆盖 v1.4.1 与 v1.5.0-beta.1 ~ 稳定版 v1.5.0）；完整语义在[迁移指南](migration.zh-CN.md)。

## 一条命令切换与回滚

先预览完整计划，再执行直接割接：

```bash
cc-connect-next migrate --dry-run
cc-connect-next migrate --switch
cc-connect-next daemon status
```

`--switch` 从已连接 CC Agent 之外的终端执行，并要求没有已安装的 Next 服务。它停止并禁用官方服务、最终同步、安装启动 Next，等待本地 API 与所有配置平台真实 Ready，再私聊唯一或显式飞书/Lark 操作者。激活失败时，只有 Next 已被证明解除注册并停止才恢复官方服务。手工回滚先解除 Next 服务注册，再恢复官方自启：

```bash
cc-connect-next daemon uninstall
# 先按平台重新启用官方自启（launchctl/systemctl/Task Scheduler）
cc-connect daemon start
```

## 三分钟迁移

```bash
npm install -g cc-connect-next
cc-connect-next migrate --dry-run
cc-connect-next migrate --switch
cc-connect-next doctor
cc-connect-next daemon status
```

## 常见疑虑

- **会删我的官方安装吗？** 不会。`--switch` 会停止并禁用官方服务，但 binary、配置和数据始终保留，失败恢复和手工回滚都有来源。
- **上游以后更新了怎么办？** 逐条审计上游变更，采纳有价值的、记录刻意偏离的，不做整体 merge。
- **我不用飞书，值得看吗？** steer、自更新、`/feedback`、doctor 与平台无关；飞书卡片契约只是差异最大的一块。
- **许可证？** MIT，与上游一致；代码保留上游署名。感谢 [chenhg5](https://github.com/chenhg5) 和 CC Connect 的所有贡献者——没有上游就没有这个项目。
