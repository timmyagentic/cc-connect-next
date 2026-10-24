# 从零安装 cc-connect-next

[English](getting-started.md) · [返回 README](../README.md) · [安装方式与维护参考](../INSTALL.md)

本教程面向**没有安装过官方 CC Connect、没有旧配置、也没有飞书机器人**的用户。以 Codex + 飞书/Lark 为例，从空环境走到第一条真实回答，再转为后台服务。已有官方安装请走[迁移指南](migration.zh-CN.md)。

整个链路是：**准备 Agent → 安装 Next → 设置项目目录 → 创建机器人 → 验证回复 → 后台运行**。每一步都有自己的完成标志，安装包下载成功只是其中一步。

## 交给 Agent 一键安装

推荐把下面整段复制到 Codex、Claude Code、Cursor 等**能执行本机命令**的 Agent 中。无需先手工安装 Next 或编辑配置。Agent 负责完成安装流程，只在需要你登录、扫码、确认应用发布或发验证消息时提示你。

```text
请从 https://github.com/timmyagentic/cc-connect-next 为我完成首次安装，使用已发布稳定版并接入飞书/Lark。
优先复用当前可用的编码 Agent 和当前项目目录；
请直接执行依赖检查与安装、生成和填写配置（含真实 work_dir）、扫码创建机器人的引导、首条回复验证，以及后台服务安装和启动。
只有 Agent/目录无法确定，或需要我登录、扫码、确认应用发布、发送验证消息时再叫我；
先识别并保留已有配置和服务，不执行迁移或覆盖。
完成后报告版本、配置路径、Agent/项目目录、消息验证和后台状态，未验证项明确说明。
```

这段 Prompt 本身说明了完整目标，不依赖你已经装好 Agent 之外的任何 Next 环境。安装者按下面的[执行流程](#给安装-agent-的执行流程)继续；想自己操作可直接看[手动步骤](#手动安装步骤)。

## 给安装 Agent 的执行流程

收到上面的请求后，**实际执行安装、配置和检查**。用户已经要求首次安装和后台运行；能从上下文和本机检查确定的步骤直接完成，避免逐条把命令或配置编辑交回用户。需要用户参与时保留进度，收到结果后继续。

### A. 确定安装对象

检查当前系统、架构、运行用户、Node/npm、可用 Agent CLI、当前项目目录及 CC Connect/Next 的安装和服务状态。优先复用用户正在使用且可工作的 Agent，以及当前任务明确对应的项目目录。不要把临时目录、用户主目录或工具控制目录误当成代码项目；只有意图无法确定时，合并询问 Agent 与项目目录这两项。

若用户没有可用 Agent，默认准备 Codex，用户已指定其他 Agent 时遵循其选择；安装所选 CLI 并引导完成其官方登录。账号、付费访问或组织权限需要用户自行完成，认证内容留在官方登录流程中。

发现已有配置时先读取结构并判断能否续装；保留已完成步骤。已有官方服务或 Next 在运行时，不把“首次安装”擅自变成迁移、覆盖或替换服务；涉及改变已有安装时先说明现状并确认具体目标。

### B. 安装稳定版并验证程序

按 [INSTALL.md](../INSTALL.md#published-release) 安装已发布稳定版。npm 路径缺少 Node.js 时完成当前 LTS 的安装和 PATH 设置；也可使用经同版本 `checksums.txt` 验证的独立二进制。使用所选 Agent 已登录的同一个系统用户。

实际运行 `cc-connect-next --version` 和相关 `--help`，记录安装来源与版本。以已安装版本的参数为准；不要因为 main 文档较新而切到 Beta 或源码构建。

### C. 由你生成并填写配置

选择该系统下明确的配置绝对路径，并在后续启动、setup、doctor、daemon 命令中一直使用它。对尚不存在的配置，按[生成配置步骤](#3-生成并编辑配置)运行一次 Next；首次退出是正常结果。

随后**由你直接编辑生成的 TOML**：填写已确定的真实 `work_dir` 和 Agent 类型，保持同一个项目名，移除不属于所选 Agent 的选项。权限模式遵循用户已确认的选择；没有偏好时采用本文首次试用模式（Codex `suggest`，Claude Code `default`）。不用让用户去找文件、替换路径或拼 TOML。已有配置只修改本次需要的字段。

在绑定机器人前检查配置可解析、目录存在、Agent CLI 可启动。不要在此时把尚未填写的飞书凭证当成安装成功。

### D. 创建机器人并把二维码交给用户

全新用户没有 App ID/App Secret，默认运行 `feishu setup` 创建新机器人，使用 C 步的配置路径和项目名。Starter 已有卡片样式，首次聊天使用 `--no-recommended --no-lark-cli`；之后按需要再启用可选扩展。

通过可保持运行的终端会话启动 setup。需要展示图片时使用 `--qr-image` 保存到本次安装的私有临时位置，将二维码或本次登录链接展示给用户，**保持 setup 进程继续等待扫码**。二维码、凭证和临时登录链接不进入仓库或交付日志。扫码完成后读取命令结果，并确认凭证已保存；超时后先检查完成到哪一步，再决定是否重试，避免重复创建应用。

若扫码流程不可用，转入[手动应用创建参考](feishu.md)，由你继续完成本地配置，只把开放平台必须由用户操作的步骤交给用户。确认机器人启用、长连接消息事件、卡片回调、发布状态和应用可用范围。首次验证使用只对该用户可用的应用范围。

### E. 验证消息与 Agent，写入用户范围

运行 `doctor`，修复配置、目录和 Agent 的必要失败项；可选语音依赖等警告按实际需求判断。然后以前台进程启动 Next，并观察真实连接状态。保留进程供验证；`doctor` 通过或进程存活不代表能聊天。

请用户在新机器人的私聊中依次发 `/version`、`/whoami` 和「请简要介绍当前项目，不要修改文件」。这几条需要真实用户身份的消息由用户发送；你读取可访问的回复或等用户反馈后核对，不伪造已完成的验证。根据 `/whoami` 返回的该应用用户 ID，**由你写入**项目的 `admin_from` 与平台的 `allow_from`，首次群聊使用 `group_reply_all = false`。沿用用户已有的明确访问范围。

`/version` 失败先排消息/权限/连接；普通提问失败则排 Agent 登录、模型访问和工作目录。自动完成可确定的修复，避免把整套排错流程丢给用户。

### F. 安装后台服务并收尾

首次回复正常后，停止本次安装由你启动的前台进程。按[后台运行步骤](#6-转为后台运行)，以同一系统用户安装并启动 daemon，显式传正确配置路径；有相对路径时保留原来的运行工作目录。不要留下前台与后台两个相同配置的消费者。

检查 `daemon status` 中的 Service 和 Runtime/Platforms，读取日志，再验证后台运行后的一条实际回复。平台持续未就绪时继续定位具体问题，不靠反复重装或 `--force` 结束任务。

最后给用户简短结果：版本和安装来源、配置文件、Agent 与项目目录、消息验证结果、后台服务状态、常用状态/日志/停止命令。登录、扫码、发布或真实回复尚缺一步时，明确停在哪一步和用户需要完成的动作，记为 **UNVERIFIED**；不要仅凭文件存在或 doctor 通过宣称整条安装完成。

## 手动安装步骤

以下是上述 Agent 流程的命令与配置参考，也可自行按步骤执行。

## 1. 准备机器和 Agent

需要准备：

- 一台保持开机、联网的 macOS、Linux 或 Windows 机器，x64 或 ARM64。
- [Node.js 当前 LTS](https://nodejs.org/en/download)（包含 npm）。npm 安装的 Next 命令需要 Node.js；使用独立二进制可省去 Next 自身的 Node.js 依赖。
- 飞书或 Lark 账号与手机 App。组织可能限制自建应用，需要有创建、发布和使用该应用的权限。
- 一个真实存在的项目目录，以及已经安装、登录并能工作的 Agent CLI。Next 不包含模型账号或额度，也不替 Agent 完成认证。

在你准备运行 Next 的同一个终端、同一个系统用户下检查：

```bash
node --version
npm --version
```

本教程采用模板默认的 Codex。按 [Codex CLI 官方文档](https://learn.chatgpt.com/docs/codex/cli)安装，也可以通过 npm：

```bash
npm install -g @openai/codex
codex --version
```

进入你希望远程操作的项目目录，运行 `codex`，按提示登录，然后问「请简要介绍这个目录，不要修改文件」。成功后退出 Codex，再继续。桌面 App 可用不等于终端里的 CLI 已安装或已登录。

已有 Claude Code 的用户可按其[官方安装与登录说明](https://code.claude.com/docs/en/overview)确认 `claude` 在同一终端能正常回答；第 3 步会说明模板如何切换。其他 Agent 的准备方式见各自文档。

**完成标志：** Agent 在目标目录内能独立回答问题。登录、模型权限或额度错误应先在 Agent 自身解决。

## 2. 安装 Next

在终端执行，默认安装稳定版：

```bash
npm install -g cc-connect-next
cc-connect-next --version
```

无需安装 Go，也无需克隆这个仓库。安装器从 GitHub Release 下载与 npm 包同版本的原生程序，并校验 `checksums.txt`；因此需要同时能访问 npm 和 GitHub Release 下载。

Windows PowerShell 如果拒绝运行 `npm.ps1`，可用 `npm.cmd`；Next 的 npm 启动器也可写成 `cc-connect-next.cmd`。如果在 WSL 中使用，请把 Node.js、Agent、Next、配置与后台服务都安装在同一 WSL 环境中，不要混用 Windows 和 Linux 路径。

**完成标志：** `--version` 输出 `cc-connect-next v…`。这还不代表机器人已经配置。

Beta、独立二进制和源码构建见 [INSTALL.md](../INSTALL.md)。第一次使用优先选择稳定版；仓库 main 文档可能包含新于稳定版的能力，以所装版本的 `--help` 为准。

## 3. 生成并编辑配置

先运行一次，明确指定配置文件位置，避免当前目录同名文件影响选择。

macOS / Linux：

```bash
cc-connect-next --config "$HOME/.cc-connect-next/config.toml"
```

Windows PowerShell：

```powershell
cc-connect-next --config "$env:USERPROFILE\.cc-connect-next\config.toml"
```

第一次会打印 `Created default config at …` 并退出，这是预期行为。打开打印出的文件，用文本编辑器修改**已有区块**，不要把下面示例直接追加到末尾。

保留 `[[projects]]` 下的 `name = "my-project"`，确认 Agent 区块：

```toml
[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/absolute/path/to/your/project"
mode = "suggest"
backend = "app_server"
app_server_url = "stdio"
```

- `work_dir` 必须换成你的真实项目目录。macOS/Linux 可在目标目录运行 `pwd` 查看绝对路径。
- Windows 可用 TOML 单引号避免反斜杠转义，例如 `work_dir = 'C:\Users\you\Projects\demo'`；替换为 `(Get-Location).Path` 显示的真实路径。
- 模板原本是 `mode = "yolo"`，允许 Agent 不经审批、无沙箱限制地执行。首次试用可改为上例的 `suggest`：Codex app-server 使用只读沙箱并支持交互审批。需要自动写入项目时再选择其他[权限模式](usage.zh-CN.md#权限模式)。
- `app_id`、`app_secret` 暂时保留占位值，第 4 步会写入。

如果使用 Claude Code，把 `type` 改为 `claudecode`，把 `mode` 改为 `default`，并移除 Codex 专属的 `backend`、`app_server_url` 行；`work_dir` 仍填写实际项目路径。

**完成标志：** 项目目录已填写，所选 Agent 与第 1 步一致。配置包含凭证，不要提交到代码仓库。`feishu setup` 不会替已有项目修改 `work_dir`，所以这一步不能跳过。

## 4. 创建飞书 / Lark 机器人

在终端运行对应命令：

macOS / Linux：

```bash
cc-connect-next feishu setup --config "$HOME/.cc-connect-next/config.toml" --project my-project --no-recommended --no-lark-cli
```

Windows PowerShell：

```powershell
cc-connect-next feishu setup --config "$env:USERPROFILE\.cc-connect-next\config.toml" --project my-project --no-recommended --no-lark-cli
```

用手机上的飞书/Lark 扫终端二维码，按页面提示创建应用。命令将凭证保存到刚才的配置文件。若第 3 步改过项目名，`--project` 也必须使用同一个名称，否则会创建另一个项目，原来的占位项目仍会阻止启动。

这两个跳过选项让第一次配置只处理机器人：模板已经有推荐卡片样式，`--no-recommended` 跳过重复套用；`--no-lark-cli` 跳过可选扩展。以后需要让 Agent 调用飞书 API 时，再看 [lark-cli companion](feishu.md#可选把同一个机器人设为-lark-cli-默认-profile)。

扫码可能已预配权限与事件，但仍需在[飞书开放平台](https://open.feishu.cn/)或 [Lark Developer](https://open.larksuite.com/)确认：

- 机器人能力已启用，应用已发布，可用范围包括你本人。
- 消息事件 `im.message.receive_v1` 使用长连接接收，具备私聊接收和机器人发送消息权限。
- 要使用卡片按钮，回调配置中还需要 `card.action.trigger` 长连接回调。

**没有二维码、扫码失败或没有创建权限：** 按[手动创建指南](feishu.md#第一步创建飞书企业自建应用)创建并发布机器人，然后在现有 `[projects.platforms.options]` 中填写 App ID、App Secret。使用 Lark 时将该平台的 `type` 改为 `lark`。不要把真实 App Secret 粘贴到 issue、聊天或命令示例中。

**完成标志：** setup 提示机器人已配置，配置中不再有 `your-feishu-app-id`、`your-feishu-app-secret`，且应用可供你使用。这不等于已经验证消息收发。

## 5. 在前台验证第一条回答

macOS / Linux：

```bash
cc-connect-next doctor --config "$HOME/.cc-connect-next/config.toml"
cc-connect-next --config "$HOME/.cc-connect-next/config.toml"
```

Windows PowerShell：

```powershell
cc-connect-next doctor --config "$env:USERPROFILE\.cc-connect-next\config.toml"
cc-connect-next --config "$env:USERPROFILE\.cc-connect-next\config.toml"
```

先处理 `doctor` 中的失败项，再启动；警告需按实际使用能力判断。`doctor` 检查配置、Agent、依赖和网络，不建立平台长连接，也不能证明机器人已成功回复。

保持终端打开，在飞书中搜索新机器人的名称并打开私聊，依次发送：

1. `/version` —— 收到 Next 版本，确认消息能进也能出。
2. `/whoami` —— 获取当前机器人的应用范围内的用户 ID。
3. 「请简要介绍当前项目，不要修改文件」—— 收到完整 Agent 回答，确认目录、登录和模型调用链路可用。

首次先在只对你可用的应用范围内使用私聊。把 `/whoami` 得到的**用户 ID**写入配置，明确谁能使用、谁能管理：

在已有 `[[projects]]` 区块的 `name` 后添加管理员：

```toml
[[projects]]
name = "my-project"
admin_from = "ou_your_user_id"
```

再到该项目已有的 `[projects.platforms.options]` 区块，保留 setup 写入的凭证，只添加或修改下面两项：

```toml
[projects.platforms.options]
# 保留此处已有的 app_id、app_secret 和其他设置
allow_from = "ou_your_user_id"
group_reply_all = false
```

将两处 `ou_your_user_id` 都替换为 `/whoami` 的结果。不要重复添加表头；`admin_from` 属于项目，`allow_from` 属于平台。模板默认会回复群里的所有消息；上述配置让第一次接入群聊通过 @ 机器人触发。更多用户、群范围与 `allow_chat` 见[飞书指南](feishu.md)。

修改后用 `Ctrl+C` 停止前台进程，再重新运行，检查 `/version` 和一条普通提问仍能回复。`admin_from` 为空时，普通聊天可用，但 `/upgrade`、`/restart`、`/shell` 等管理命令不可用。

**完成标志：** `/version` 与普通提问都成功；没有只靠“进程正在运行”或 `doctor` 的结果判断完成。

## 6. 转为后台运行

在已经登录 Agent 的同一个系统用户下操作。先用 `Ctrl+C` 停止前台进程，然后：

macOS / Linux：

```bash
cc-connect-next daemon install --config "$HOME/.cc-connect-next/config.toml"
```

Windows PowerShell：

```powershell
cc-connect-next daemon install --config "$env:USERPROFILE\.cc-connect-next\config.toml"
```

安装命令会同时启动服务。检查状态和日志：

```bash
cc-connect-next daemon status
cc-connect-next daemon logs -f
```

确认服务运行、Runtime/Platforms 就绪，再从飞书发送一次普通提问。日志跟随用 `Ctrl+C` 退出，不会停止后台服务。macOS 使用 launchd；Linux 使用 systemd，若提示未启用 linger，按提示处理退出 SSH 后的持续运行；Windows 使用任务计划程序。机器休眠、断网或关机时，机器人无法正常工作。

若配置依赖相对路径，安装时还应传 `--work-dir` 指向原来的启动目录；本教程使用绝对项目路径。不要在后台服务运行时另开前台实例，也不要使用 `--force` 掩盖重复启动问题。

**完成标志：** 关闭前台终端后，机器人仍能回复，daemon 状态与实际回复一致。

## 遇到问题时

| 现象 | 下一步 |
|---|---|
| `npm` / `node` 不存在 | 安装 Node.js LTS，重新打开终端；再检查版本 |
| npm 全局安装 `EACCES` | 使用当前用户可写的 Node.js 安装/prefix，见 [npm 安装说明](../INSTALL.md#published-release) |
| 安装后找不到 `cc-connect-next` | 检查 `npm prefix -g` 对应命令目录是否在 PATH，重新开终端；Windows 可尝试 `.cmd` 启动器 |
| 下载失败或 checksum 不匹配 | 检查 npm 和 GitHub Release 网络；不要跳过校验，独立下载也要核对同版本 checksums |
| `feishu setup` 报配置不存在 | 先完成第 3 步；setup 与启动使用同一个 `--config` |
| 启动报告 `REPLACE` 占位值 | 按错误列出的键修改；常见遗漏是 `work_dir`，或 setup 创建了不同名称的项目 |
| `/version` 没有回复 | 检查应用发布/可用范围、消息权限、长连接事件、`allow_from`、重复服务；查看终端/daemon 日志 |
| `/version` 有回复，普通提问失败 | 回到同一系统用户的终端验证 Agent 登录、目录、模型权限和额度，运行 `doctor` |
| 卡片按钮超时 | 检查 `card.action.trigger` 回调和应用发布状态；见[卡片回调配置](feishu.md#53-选择回调配置) |
| 前台可用，后台失败 | 检查 daemon 的 Binary、Config、WorkDir、PATH 与 Agent 登录用户；按 [INSTALL.md](../INSTALL.md) 排查 |

日常使用见[使用指南](usage.zh-CN.md)；更新、停止服务与卸载见 [INSTALL.md](../INSTALL.md)。报告问题时提供操作系统、安装来源、`--version` 和脱敏错误信息，不上传整个配置或凭证。
