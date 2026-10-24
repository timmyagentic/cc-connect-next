<div align="center">

<img src="docs/images/banner-next.svg" alt="cc-connect-next" width="720">

**在飞书里使用你电脑上的 AI 编程 Agent。**

连接 Codex、Claude Code、Cursor 等本机 Agent，把消息、执行进度和回答带到飞书/Lark、Telegram、Slack、Discord 等聊天平台。

[![Release](https://img.shields.io/github/v/release/timmyagentic/cc-connect-next?color=0284c7&label=release)](https://github.com/timmyagentic/cc-connect-next/releases/latest)
[![npm](https://img.shields.io/npm/v/cc-connect-next?color=cb3837&logo=npm&logoColor=white)](https://www.npmjs.com/package/cc-connect-next)
[![License: MIT](https://img.shields.io/github/license/timmyagentic/cc-connect-next?color=blue)](LICENSE)

[English](README.en.md) · **[一段 Prompt 安装](docs/getting-started.zh-CN.md#交给-agent-一键安装)** · [使用指南](docs/usage.zh-CN.md) · [飞书配置](docs/feishu.md) · [迁移指南](docs/migration.zh-CN.md)

</div>

cc-connect-next 是 [CC Connect](https://github.com/chenhg5/cc-connect) 的独立后继项目，重点维护飞书原生卡片和本机 Agent 的使用体验。首次使用不需要安装原版 CC Connect，也不需要已有机器人或旧配置。

它负责连接聊天平台和 Agent。**安装 Agent 会检查并准备所需 CLI，你按提示完成账号登录，并使用自己的模型访问权限与额度**。代码和 Agent 进程运行在你的机器上，发出的消息和最终回答会经过所选聊天平台；模型请求由 Agent 发给其配置的提供商。运行机器需要保持开机、联网。

| 你的情况 | 从这里开始 |
|---|---|
| 从未用过 CC Connect / Next | **[复制安装 Prompt，让 Agent 完成首次安装](docs/getting-started.zh-CN.md#交给-agent-一键安装)** |
| 已在使用官方 CC Connect | [迁移指南](docs/migration.zh-CN.md)，复用已有配置与机器人 |
| 已安装 cc-connect-next | [更新与后台服务](INSTALL.md)，或查看[使用指南](docs/usage.zh-CN.md) |

## 🚀 快速开始

**推荐：把下面整段 Prompt 复制给能执行本机命令的编码 Agent，例如 Codex、Claude Code 或 Cursor。** Agent 会负责安装和配置，你按提示完成登录、扫码和验证消息即可。

```text
请从 https://github.com/timmyagentic/cc-connect-next 为我完成首次安装，使用已发布稳定版并接入飞书/Lark。
优先复用当前可用的编码 Agent 和当前项目目录；
请直接执行依赖检查与安装、生成和填写配置（含真实 work_dir）、扫码创建机器人的引导、首条回复验证，以及后台服务安装和启动。
只有 Agent/目录无法确定，或需要我登录、扫码、确认应用发布、发送验证消息时再叫我；
先识别并保留已有配置和服务，不执行迁移或覆盖。
完成后报告版本、配置路径、Agent/项目目录、消息验证和后台状态，未验证项明确说明。
```

这个任务从没有旧配置、没有机器人的状态开始，包含首次安装和后台运行。执行细节见[给安装 Agent 的执行流程](docs/getting-started.zh-CN.md#给安装-agent-的执行流程)。普通网页聊天如果没有本机终端权限，无法直接执行安装。

需要自己操作或检查某一步时，查看[手动安装步骤与排错](docs/getting-started.zh-CN.md#手动安装步骤)。已有官方 CC Connect 用户使用[迁移安装 Prompt](docs/migration.zh-CN.md#交给-agent-一键执行)。

## ✨ 亮点

- **飞书原生回答卡片。** 进度、流式正文和完成状态在同一张引用卡片中呈现；处理中只展示匿名计数，隐藏推理文本与工具明细。默认 footer 展示模型、推理强度和耗时，可关闭。[卡片契约](docs/feishu-card-contract.md)
- **回合中途补充要求。** Codex 的默认 app-server 后端支持原生 steer；其他 Agent 按能力回退到队列。用 `/ps` 显式并入当前回合，也可设置始终排队。[排队与 steer](docs/usage.zh-CN.md#忙时消息排队-vs-并入steer)
- **多项目与日常操作。** 每个项目绑定自己的 Agent、目录和平台；支持会话切换、权限确认、模型/provider 切换、定时任务、语音和 Web 管理台（Beta）。[使用指南](docs/usage.zh-CN.md)
- **可诊断、可维护。** `doctor` 检查环境，daemon 管理后台运行，更新区分 Stable/Beta；迁移保留备份与校验记录。[安装与维护](INSTALL.md)
- **可查询的能力。** 直接询问连接的 Agent，或运行 `cc-connect-next capabilities --search "关键词"`，查询当前版本、项目和会话可用的配置、命令及权限。[能力参考](docs/agent-capability-manifest.zh-CN.md)

## 🎬 实际效果

<div align="center">
<img src="docs/images/turn-demo.svg" alt="示意图：提问、执行进度、流式回答和完成卡片" width="820">
</div>

上图是交互示意。忙时补充通过 steer 加入同一个 Agent 回合时，旧卡片保留已显示内容，后续进度和最终回答交接到回复最新消息的卡片；具体边界见[飞书卡片契约](docs/feishu-card-contract.md)。

## 🥇 飞书是一等公民

支持 WebSocket 长连接，无需公网 IP、域名或反向代理；支持权限与模型切换卡片、真实话题隔离、群聊 @ 与免 @ 范围、文件与图片输入。第一次先跑通私聊，再按需要启用群聊和高级功能：[飞书接入指南](docs/feishu.md)。

## 🧩 支持的 Agent 与平台

常见组合包括 Codex / Claude Code / Cursor / Gemini CLI，以及飞书/Lark / Telegram / Slack / Discord。默认发行包包含项目支持的适配器；精确列表与构建裁剪以 [Makefile](Makefile) 的 `ALL_AGENTS`、`ALL_PLATFORMS` 为准。外部平台也可使用 [Bridge 协议](docs/bridge-protocol.zh-CN.md)接入。

## 🔄 从官方 CC Connect 迁移

已有用户可以复用机器人和配置。先执行只读检查：

```bash
cc-connect-next migrate --dry-run
```

确认迁移计划后，按[迁移指南](docs/migration.zh-CN.md)执行切换、验证或回滚。两个服务不能同时消费同一个飞书应用的事件。Next 使用独立的命令、`~/.cc-connect-next` 数据目录和后台服务；迁移会保留官方二进制与数据。差异说明见[写给官方 CC Connect 用户](docs/coming-from-cc-connect.zh-CN.md)。

## 📚 文档

| 你要做什么 | 文档 |
|---|---|
| 让 Agent 完成首次安装 | [复制安装 Prompt](docs/getting-started.zh-CN.md#交给-agent-一键安装) · [手动步骤与排错](docs/getting-started.zh-CN.md#手动安装步骤) |
| npm / 独立二进制 / 源码安装、更新、服务、卸载 | [安装与维护参考](INSTALL.md) |
| 创建飞书应用、设置权限与事件、配置群聊 | [飞书指南](docs/feishu.md) |
| 会话、权限、模型、定时任务与日常命令 | [使用指南](docs/usage.zh-CN.md) |
| 查询配置项、默认值与可用能力 | [配置参考](docs/configuration.zh-CN.md) · [Agent Capability Manifest](docs/agent-capability-manifest.zh-CN.md) |
| 从官方版本迁移 | [迁移指南](docs/migration.zh-CN.md) · [兼容矩阵](docs/migration-compatibility.md) |
| 接入其他聊天平台 | [Telegram](docs/telegram.md) · [Slack](docs/slack.md) · [Discord](docs/discord.md) · [更多](docs/) |
| 开发集成 | [Bridge 协议](docs/bridge-protocol.zh-CN.md) · [管理 API](docs/management-api.zh-CN.md) |

遇到问题先按入门教程的排错表运行 `doctor`、检查 daemon 日志。需要报告问题时，可使用聊天中的 `/feedback` 或 [GitHub Issues](https://github.com/timmyagentic/cc-connect-next/issues)。提交材料前移除凭证和私人内容。

## 🛠️ 开发与验证

```bash
make web            # 构建 Web 管理台资源
make test-full      # 构建、vet、race、smoke 与 regression
make build-noweb    # 不含 Web 面板的开发构建
```

源码要求见 [INSTALL.md](INSTALL.md#current-source)。贡献遵循 [AGENTS.md](AGENTS.md)：core 不依赖具体 Agent/平台，适配器通过能力接口扩展，行为修复附带回归测试。

## 🙏 来源与许可证

cc-connect-next 始于 CC Connect v1.4.1，保留其 Git 历史，并通过逐项审计跟进上游。感谢 [@chenhg5](https://github.com/chenhg5) 与所有贡献者。归属声明见 [NOTICE](NOTICE)，许可证见 [LICENSE](LICENSE)。
