# 写给官方 CC Connect 用户：cc-connect-next 有什么不同，怎么零风险试用

[English](coming-from-cc-connect.md)

如果你正在用 [官方 CC Connect](https://github.com/chenhg5/cc-connect)，并且对它满意，你不需要换——它依然是一个很好的项目，cc-connect-next 也正是从它 fork 出来的（v1.4.1，MIT）。这篇文章写给另一类用户：想要原生飞书卡片体验、更严格的隐私边界、或者"任务跑着的时候还能补一句"的用户。下面每一条差异都可以自己验证，文末给出零风险的试用与回退路径。

## 先说清楚这是什么

cc-connect-next 是一个**独立后继**，不是补丁、代理或伴生插件：

- 单个 Go 二进制，拥有自己的命令（`cc-connect-next`）、数据目录（`~/.cc-connect-next`）、daemon 服务名和 npm 包；
- 安装、迁移、试用全程**不停止、不卸载、不修改**官方 CC Connect，两者可以长期并存；
- 对上游保持尊重：不做盲目 merge，而是对每个上游变更做逐条审计后决定采纳或刻意偏离（[审计策略与记录](upstream-v1.5.0-beta.3-audit.md)）。

## 五个真正不同的地方

### 1. 飞书回答是一张原生卡片，不是消息流

一次 Agent 回合从头到尾停留在**同一张**引用你提问的 Card 2.0 卡片里：`⏳ 正在思考` → `⏳ 正在调用工具` → `✍️ 正在回答`（CardKit 打字机流式）→ `✅ 已完成`。不刷屏、不拼接多条消息，最终答案和中间进度在同一个位置演进。精确的状态机和验证命令写在[飞书回答卡片契约](feishu-card-contract.md)里。

### 2. 隐私边界：聊天里只有匿名计数

卡片上只渲染"推理 N 次 · 工具 M 次"这样的匿名进度。推理文本、工具名称、参数、结果、模型名、token 数、工作目录，在事件层和渲染层**两层**被丢弃，卡片也没有可展开的详情面板。你的代码和 Agent 的中间过程不会经过飞书的消息存储。

### 3. 任务跑着的时候，可以直接补一句

官方的模型是 FIFO 排队：Agent 忙时来的消息等下一个回合。cc-connect-next 默认是 **steer**：新消息经 Codex 原生 `turn/steer` **并入正在运行的这个回合**，旧卡片冻结为灰色"已转到更新的消息"（保留已显示的内容），后续进度只在回复最新消息的卡片上继续，每个回合只有一张卡片到达"已完成"。不支持 steer 的 Agent 自动回退为排队，`busy_message_mode = "queue"` 可恢复官方行为，`/ps` 在任何模式下显式 steer。

### 4. 安装会自己照顾自己

`cc-connect-next update` 跟随 stable 渠道，npm 和独立二进制都带校验和验证；新版本发布后，daemon 会在每个项目最近的会话里**只提醒一次**（`update_notice = false` 关闭）。`doctor` 一条命令检查配置、Agent CLI 登录态、平台、依赖和网络。

### 5. 出了问题，一条命令反馈

聊天里 `/feedback` 直接把问题作为 GitHub issue 报给作者：自动脱敏、匿名中继、不需要 GitHub 账号；回合失败时 daemon 也会主动提示这个入口。

其余能力（14 种 Agent × 15 个平台、cron 与 webhook、语音输入输出、Web 管理台、五种语言 i18n）见[主 README](../README.md)。

## 试用不需要"下决心"

两套系统的边界完全隔离：

| 边界 | 官方 | cc-connect-next |
|---|---|---|
| 命令 | `cc-connect` | `cc-connect-next` |
| 数据 | `~/.cc-connect` | `~/.cc-connect-next` |
| macOS 服务 | `com.cc-connect.service` | `com.cc-connect-next.service` |
| Linux 服务 | `cc-connect.service` | `cc-connect-next.service` |
| API socket | `~/.cc-connect/run/api.sock` | `~/.cc-connect-next/run/api.sock` |

唯一的注意事项：**不要让两个进程同时连接同一个飞书应用凭证**（两个 WebSocket 消费者会竞争消息）。试用时建一个测试用飞书应用即可，官方 daemon 完全不用停。

## 一键迁移为什么敢用

`cc-connect-next migrate` 不是"复制目录"脚本。它先清点官方安装的全部持久化来源（`config.toml`、生效的 `data_dir`、每个项目本地的 `.cc-connect` 目录），对**每个源文件计算 SHA-256**，在旁路暂存目录构建并验证完整结果，激活前重新核对源清单——任何新增、删除、内容或权限变化都会**拒绝激活**而不是产出半成品。激活是原子重命名，已存在的目标先做带时间戳的备份，结果附带记录每个路径与哈希的 `migration-manifest.json`。担心的话，先跑：

```bash
cc-connect-next migrate --dry-run
```

它会打印完整计划而不写任何东西。支持的官方版本与配置项见[迁移兼容矩阵](migration-compatibility.md)（当前覆盖 v1.4.1 与 v1.5.0-beta.1 ~ beta.3）；完整语义在[迁移指南](migration.zh-CN.md)。

## 正式切换与回滚

切换生产流量时，先停官方 daemon，再做最终同步迁移（把测试期间官方新写的会话、绑定、定时器也带过来），然后装服务：

```bash
cc-connect daemon stop
cc-connect-next migrate --dry-run --force
cc-connect-next migrate --force
cc-connect-next daemon install --config ~/.cc-connect-next/config.toml
```

回滚永远只有两条命令，官方数据目录从头到尾没有被动过：

```bash
cc-connect-next daemon stop
cc-connect daemon start
```

## 三分钟上手

```bash
npm install -g cc-connect-next
cc-connect-next                # 生成 ~/.cc-connect-next/config.toml 并引导
cc-connect-next feishu setup   # 交互式飞书应用配置（内置推荐档）
cc-connect-next doctor
cc-connect-next
```

## 常见疑虑

- **会动我的官方安装吗？** 不会。迁移不停止、不卸载、不修改官方；并存边界见上表。
- **上游以后更新了怎么办？** 逐条审计上游变更，采纳有价值的、记录刻意偏离的，不做整体 merge。
- **我不用飞书，值得看吗？** steer、自更新、`/feedback`、doctor 与平台无关；飞书卡片契约只是差异最大的一块。
- **许可证？** MIT，与上游一致；代码保留上游署名。感谢 [chenhg5](https://github.com/chenhg5) 和 CC Connect 的所有贡献者——没有上游就没有这个项目。
