# CC Connect Feishu Plus

[![CI](https://github.com/timmyagentic/cc-connect-feishu-plus/actions/workflows/ci.yml/badge.svg)](https://github.com/timmyagentic/cc-connect-feishu-plus/actions/workflows/ci.yml)
![状态](https://img.shields.io/badge/status-foundation-orange)
![npm](https://img.shields.io/badge/npm-publication%20gated-lightgrey)

[English](./README.md)

一个专注于飞书/Lark 的 CC Connect 独立维护兼容增强发行版。它保留原生适配器、
配置和会话数据，再通过可选的 `plus_*` 开关增加能力，不依赖原仓库合并 PR。

## 为什么这样做

原生飞书能力本身有价值，应该继续保留。如果另起一个机器人或代理进程，会产生竞争
连接、重复事件和额外迁移成本。因此 Feishu Plus 直接增强现有适配器：

- Plus 能力编译进兼容版 CC Connect 二进制。
- 继续使用现有 `~/.cc-connect` 配置和会话数据。
- 深入飞书链路的改动全部放在明确的 `plus_*` 开关后。
- 关闭 Plus 时，原生飞书路径及默认值保持不变。
- 更新检查和下载只指向本仓库，避免升级时静默丢失 Plus 能力。

## 基础版功能

### 可自愈的机器人身份 fail-closed

适配器必须先获得机器人的 `open_id`，才能判断群消息是否真的 @了机器人。原生路径
在查询失败后会继续启动，但此时无法进行 @过滤，无关群消息可能被接收，直到手动重启。

在现有飞书平台配置中启用首个 Plus 功能：

```toml
[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "your-feishu-app-id"
app_secret = "your-feishu-app-secret"
plus_enabled = true
plus_identity_mode = "retry"
```

可选模式：

| 模式 | 无法获得机器人身份时的行为 |
| --- | --- |
| `retry` | 默认值。阻止需要 @的群消息，私聊继续可用；按上限指数退避重试，成功后无需重启即可恢复。 |
| `fail_closed` | 阻止需要 @的群消息直到进程重启；私聊继续可用。 |
| `legacy` | 为兼容保留原生 fail-open 行为。 |

这项保护目前应用于 WebSocket 模式，现有 webhook 行为不变。完整兼容边界见
[Feishu Plus 使用说明](./docs/feishu-plus.zh-CN.md)。

## npm 引导工具

计划中的公开入口是：

```bash
npx cc-connect-feishu-plus@latest install
```

在签名发布物和自动回滚能力完成前，基础版 npm 包有意保持私有且不执行写入。当前可从
仓库检出中检查原生安装，并预览安装计划：

```bash
node npm/cli.js doctor
node npm/cli.js doctor --json
node npm/cli.js install --dry-run
node npm/cli.js install --dry-run --json
```

这些命令不会下载、替换、停止或重启当前服务。doctor 只报告路径、二进制版本、文件
权限和 Plus 状态，不返回配置内容或凭证。

## 当前状态与发布门禁

`0.1.0` 基础版包含：

- 第一个可独立测试的 Feishu Plus 行为；
- 独立二进制身份和固定到本仓库的更新源；
- 只读 npm doctor 与安装计划器；
- 原生兼容、fail-closed、重试自愈、更新源隔离和无写入安装器的回归测试。

公开 npm 和修改后二进制的发布仍需满足：

- 确认上游许可证与署名要求；
- 签名清单与 SHA-256 校验；
- 不可变版本目录和原子切换；
- 服务/配置备份、切换后健康检查和自动回滚。

详情见[固定上游基线](./docs/upstream-baseline.md)。

## 开发

环境要求：Go 1.25、Node.js 18 或更高版本；Web UI 使用 pnpm 10。

```bash
corepack pnpm@10.28.2 --dir web install --frozen-lockfile
corepack pnpm@10.28.2 --dir web build
go test ./...
npm run check --prefix npm
```

为了与固定上游基线保持源码兼容，Go module 路径仍为
`github.com/chenhg5/cc-connect`；发行版身份、更新源、发布流程和维护工作都在本仓库。

## 上游基线

Feishu Plus 当前固定在
[`chenhg5/cc-connect@3fc360e`](https://github.com/chenhg5/cc-connect/tree/3fc360ee6acc9bab13ab1b48ddde3af44062903b)。
原始文档和完整 Git 历史仍可从上游及本 fork 的历史中查看。后续基线更新必须显式审阅，
不会自动追随上游 `main`。
