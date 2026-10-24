# 首次安装 cc-connect-next

[English](getting-started.md) · [README](../README.md) · [安装参考](../INSTALL.md)

面向没有官方 CC Connect、旧配置或机器人的用户。已有官方用户使用[迁移指南](migration.zh-CN.md)。

## 交给 Agent 一键执行

把下面整段复制给能执行本机命令的编码 Agent；安装、命令执行和配置编辑由 Agent 完成。

```text
请从 https://github.com/timmyagentic/cc-connect-next 安装已发布稳定版，按首次安装流程接入飞书/Lark。
先确认操作系统、CPU 架构、Agent CLI 及登录状态和项目绝对路径；优先复用当前 Agent 与项目，
缺少必要信息再问我。检查并保留已有 CC Connect/Next 配置和服务，不执行迁移或覆盖。
安装后选择明确的配置路径；文件不存在时运行 `cc-connect-next --config <配置路径>` 生成配置，
由你填写真实 work_dir、Agent 类型和对应选项，保持项目名一致，不让我手工编辑配置。
执行 `cc-connect-next feishu setup --config <配置路径> --project <项目名>`，展示二维码并保持
进程等待我扫码。接受推荐配置，并按提示确认官方 lark-cli companion；非交互执行时使用
`--recommended --lark-cli`，保留已有 profile，让同一机器人作为默认 bot profile。
执行 `cc-connect-next doctor --config <配置路径>` 并前台启动；引导我在机器人私聊发送
/version、/whoami 和一条普通提问，核对回复并按实际用户 ID 配置访问范围。
验证后停止本次前台进程，执行 `cc-connect-next daemon install --config <配置路径>`，检查
`cc-connect-next daemon status` 的 Service 与 Runtime/Platforms，并验证后台仍能回复。
报告版本、配置路径、work_dir 和验证结果；需要我登录、扫码或发布应用时再提示，未完成的
真实消息验证明确标为 UNVERIFIED。
```

## 必要说明

- `<配置路径>` 与 `<项目名>` 由 Agent 换成实际值，并在初始化、setup、doctor 和 daemon 中保持一致。现有配置先检查，不重复生成或创建第二个占位项目。
- 没有 Agent CLI 时由安装 Agent 准备；账号登录需要你在官方流程中完成。项目目录不明确时再询问，不把临时目录当成项目。
- setup 需要保持运行直到扫码完成；可用 `--qr-image` 展示二维码，临时登录信息不写入仓库。扫码不代替应用发布、权限和可用范围检查，细节见[飞书指南](feishu.md)。
- `lark-cli` 复用该机器人，不另建机器人；保留旧 profile，不执行用户 `auth login` 或第二条 `event consume` 连接。仅需基本聊天时可使用 `--no-lark-cli` 跳过 companion。
- `/version` 验证消息收发，普通提问验证 Agent 能执行；`doctor` 通过不代表真实消息已验证。需要用户发送的验证消息由用户发送，未完成时如实说明。
- macOS/Linux/Windows 的安装方式、独立二进制、更新和迁移参考 [INSTALL.md](../INSTALL.md)；日常命令见[使用指南](usage.zh-CN.md)。
