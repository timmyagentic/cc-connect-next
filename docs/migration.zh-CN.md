# 从官方 CC Connect 迁移

这是完整、权威的迁移与共存指南，保留迁移命令做出的每一项保证；README 中只保留摘要。另见[迁移兼容矩阵](migration-compatibility.md)。

## 一条命令完成生产迁移

正常用户不需要第二个飞书应用。先做只读预检，再用一个显式命令完成生产割接：

```bash
cc-connect-next migrate --dry-run
cc-connect-next migrate --switch
cc-connect-next daemon status
```

`--switch` 会重新执行完整预检，停止并禁用官方 CC Connect，确认源已静止后完成
最终迁移，再以迁移配置和官方原运行目录安装、启动 cc-connect-next。它不会把服务
管理器的 `Running` 当作成功：本地 API 与所有配置平台都必须真实 Ready，才宣布完成
并发送私聊。最终同步或激活/就绪失败时，只有 Next 已解除注册、runtime socket 不再
应答且迁移配置锁已释放，才恢复官方服务；否则保持官方禁用，避免双消费者。官方
binary 与数据目录始终保留。

迁移成功后，交互终端会提供官方 `lark-cli` companion：复用迁移后的机器人创建或
复用独立 profile，将它设为默认 profile 和默认 bot 身份，旧 profile 与用户 OAuth
完整保留。脚本模式不会隐式写入，使用 `--lark-cli` 明确启用；若配置中有多个不同
机器人，再加 `--lark-cli-project <项目名>`（同一项目内多个平台用
`--lark-cli-platform-index`）。`--dry-run --lark-cli` 只报告计划，绝不安装或改 profile。

不带 `--switch` 的 `cc-connect-next migrate` 仍是只复制、不切换流量的高级操作，
适用于备份或主动配置的并行试用，不是默认用户旅程。

这一条命令会在写入前覆盖三类来源：只读取官方的 `config.toml`、清点配置实际生效的 `data_dir`（包括自定义路径），以及从项目配置、多工作区根目录、项目状态和 workspace bindings 中发现的每个项目内 `.cc-connect`。当配置文件与实际数据目录分离时，它不会清点配置文件旁边的任何兄弟文件或目录。因此配置、会话、项目覆盖、cron/timer/heartbeat、绑定、本地 provider 配置以及项目内暂存的图片和附件都会迁移，但项目仓库、`.env`、备份目录或整个 service home 不会被误带走。Codex、Claude 等 Agent 自己的外部会话库保持原位，原有 session ID 继续有效。

迁移会先为每个源文件计算 SHA-256，在同级 staging 目录构建并校验完整结果；正式启用前还会重新生成一遍完整源清单，任何新增、删除、内容变化、项目发现变化或访问权限变化都会让本次迁移安全失败，不会启用一个不完整的目标。已有目标也会在 staging 前建立包含内容、类型、权限和属主的快照，复制后重新校验，在每个目标正式切换前再次校验，并在原子 rename 后通过备份路径做最后一次比对；如果另一个 cc-connect-next 进程在迁移期间新建或改写目标状态，即使是已打开的写入句柄恰好在 rename 边界落盘，尤其是 `--force` 合并期间，命令也会恢复并原样保留较新的目标，而不会启用陈旧的 staging 副本。清单稳定后才以原子 rename 逐个启用目标；如果后续目标启用失败，每个已经启用的目标都会先完整移动到唯一的 `.failed-migration-*/preserved` 恢复目录，再恢复迁移前备份。回滚绝不会删除可能包含切换后新写入的数据，错误信息会列出全部恢复路径。所有目标都会先解析真实路径并按文件系统 identity 比较；只要与任一官方源目录重叠就会拒绝执行，符号链接父目录和大小写不敏感卷上的仅大小写别名也不例外。实际生效的全局 `data_dir` 与项目内 `.cc-connect-next` 都会保留源目录/文件权限与属主，兼容 `run_as_user` 穿透读取；若自定义目标的中间父目录不存在，迁移新建的父目录会继承对应源根目录的穿透权限与属主，已经存在的父目录绝不改动。重写后的 `config.toml` 仍是单独生成的 `0600` 文件。日志、socket、锁、重启通知和 daemon 元数据不会复制，源符号链接会跳过。目标非空时默认拒绝；显式使用 `--force` 表示有意合并。只要目标原先存在，即使它是空目录，也会完整保留为带时间戳的 `*.pre-migration-*` 备份，并同时记录在报告和 manifest 中。最终生成 `migration-manifest.json`，逐项记录源、目标、大小和 SHA-256。只有明确不需要项目内图片和附件时才使用 `--skip-project-data`。

只复制迁移时，官方实例仍可保持安装和运行；如果它恰好在迁移窗口持续写入持久数据，命令会要求你在较安静的时刻重跑，而不是静默漏掉新文件。生产 `--switch` 会先停止官方实例，再做最终扫描与同步。

自定义路径：

```bash
cc-connect-next migrate \
  --source /path/to/official-data \
  --target /path/to/next-data \
	  --source-version v1.5.0 \
  --dry-run
```

当前兼容矩阵覆盖官方 v1.4.1 与 v1.5.0-beta.1 至稳定版 v1.5.0 的已知持久化布局。默认 `--source-version auto` 不执行 daemon 元数据中记录的二进制，而是直接校验实际 TOML schema、正常启动所需的语义、当前构建已注册的 Agent/平台和持久化目录；已知版本也可以显式记录。缺失 Agent/平台、无效展示模式、无法保留行为的新字段或未安装插件都会在写入前失败，详见[迁移兼容矩阵](migration-compatibility.md)。

相对形式的 `data_dir`、`work_dir` 和 `base_dir` 会优先按官方 daemon 记录的工作目录解析。即使 `--source` 指向单独的配置目录，迁移仍会从官方 `$HOME/.cc-connect/daemon.json` 读取这份元数据，不会信任任意配置目录旁边的同名文件。元数据格式损坏，或其中记录的工作目录已经不存在、无法访问时，预检会直接失败，不会改用另一个目录解析相对路径。如果没有配置 `data_dir`，迁移也会严格沿用 `$HOME/.cc-connect` 默认值，即使 `--source` 指向 `/etc/cc-connect` 等自定义配置根也不会漏掉真实状态；自定义配置根中只复制 `config.toml`。若 daemon 元数据已过期，或官方版本一直由手工命令启动，请显式传入 `--runtime-work-dir /原运行目录的绝对路径`；该参数优先级最高。

出于安全考虑，如果实际生效的 `data_dir` 包含官方配置根目录（例如 `data_dir = "~"`），迁移会直接拒绝执行。即使自定义 `data_dir` 与配置根完全分离，也只会按当前支持的官方版本明确拥有的持久路径清点：会话、项目状态与模型缓存、cron/timer、绑定、heartbeat/目录历史、MiniMax 本地配置、微信状态、Agent prompt 和 Matrix 加密状态。只要出现意外的普通文件或目录，预检就会直接失败；因此配置放在 `/etc`、数据却误指向整个 service home 时，也不会把 SSH 密钥、浏览器资料或其他无关数据递归带走。请先把官方安装指向专用数据目录并确认状态，再重新迁移；命令不会静默生成不完整目标。

配置路径中的环境变量沿用官方 CC Connect 的 `${NAME}` 占位符语法，但迁移进程必须实际拥有每个被引用的变量；变量未设置时会安全拒绝，而不会用空字符串误选其他目录。配置了但尚未创建的 `data_dir` 会按空目录处理，已有配置文件仍能正常迁移。如果可选的项目内数据无法读取，或项目状态 / binding 元数据已损坏，全局迁移仍会继续，且这些元数据文件仍会原样复制；每个跳过的发现来源都会输出并写入 `migration-manifest.json`。授权或修复元数据后应重新迁移，确认项目内数据完整。

## 与官方版本并存

| 隔离边界 | 官方 CC Connect | cc-connect-next |
|---|---|---|
| 命令 | `cc-connect` | `cc-connect-next` |
| 数据 | `~/.cc-connect` | `~/.cc-connect-next` |
| macOS 服务 | `com.cc-connect.service` | `com.cc-connect-next.service` |
| Linux 服务 | `cc-connect.service` | `cc-connect-next.service` |
| API socket | `~/.cc-connect/run/api.sock` | `~/.cc-connect-next/run/api.sock` |

两者可以同时安装，但不要让它们同时使用同一个飞书应用凭证建立 WebSocket：两个消费者可能争抢或重复处理消息。默认 `--switch` 会在启动 Next 前停止并禁用官方 daemon，所以直接复用迁移过来的原凭证，不需要测试应用。只有主动选择高级并行试用时才需要单独应用。普通 `lark-cli` HTTP API 命令可以共用同一机器人；不要在 Next 运行时用该 profile 执行 `lark-cli event consume`，因为它会再建立一条事件连接。

这条规则由产品层强制执行，而不只是写在文档里：

- **启动守卫。** 官方 daemon 正在运行且与当前配置共用平台凭证时，`cc-connect-next` 拒绝启动，并给出确切的停止/解除自启命令。官方自启只是"待命"（服务已注册 `RunAtLoad`/`enabled` 但 daemon 未运行）时记录显著警告——冲突离触发只差一次重启。`CC_NEXT_ALLOW_OFFICIAL_CONFLICT=1` 可跳过拒绝（不推荐）。
- **迁移收尾引导。** 只复制迁移结束后，报告会检测官方 binary、自启注册、运行态和凭证重叠，并把直接 `migrate --switch` 作为默认下一步；并行试用降为高级选项。
- **`doctor`。** 新增"official CC Connect coexistence"检查区，报告 binary/自启/运行态/共享凭证四项；官方 daemon 运行且凭证重叠记为失败，自启待命记为警告。凭证值一律脱敏。
- **`migrate --switch`。** 一条命令完成首次割接：若已有 Next 服务则在变更官方服务前拒绝；否则停止并禁用官方 daemon，执行最终 `--force` 同步，用迁移配置和原运行目录安装启动 Next，等待本地 runtime 与平台真实 Ready，再由 CLI 直接私聊唯一或显式操作者。`--switch` 不能与 `--dry-run` 同用。

正式切换不要求先运行测试实例，也不要求手工拼接配置路径和工作目录。预检成功后直接执行：

```bash
cc-connect-next migrate --switch
cc-connect-next daemon status
```

命令必须从官方 daemon 生命周期之外的终端执行；检测到 `CC_SESSION_KEY` 时会在任何停服动作前拒绝。

`--switch` 必须重复 dry-run 用过的所有自定义路径参数。若配置只有一个可确认的飞书/Lark 操作者，CLI 会私聊“迁移完成，cc-connect-next 已运行”；否则用 `--notify-project` 与 `--notify-user` 明确指定。不会搜索最近会话或向群广播。

回滚时先解除 Next 服务注册，再重新启用官方服务自启并启动官方 daemon；两边数据始终保留：

```bash
cc-connect-next daemon uninstall
# macOS: launchctl enable gui/$(id -u)/com.cc-connect.service
# Linux user: systemctl --user enable cc-connect.service
# Linux system: sudo systemctl enable cc-connect.service
# Windows: Enable-ScheduledTask -TaskName cc-connect
cc-connect daemon start
```

官方数据始终保留。

## 交给 Agent 一键执行

```text
	请从 https://github.com/timmyagentic/cc-connect-next 安装已发布稳定版。
	先确认操作系统、CPU 架构以及 cc-connect 是否正在运行；不要卸载、覆盖或删除
	官方 CC Connect。执行 `cc-connect-next migrate --dry-run` 并报告完整计划、跳过项、
	旧路径引用与目标目录。预检通过后执行 `cc-connect-next migrate --switch`；不要创建
	第二个飞书应用，也不要手工并行启动两套同凭证服务。确认命令报告官方 daemon 已
	停止并禁用、最终 migration-manifest.json 与备份可读、cc-connect-next daemon 已
	安装并 Running，配置路径和 work_dir 精确，并确认机器人私聊完成消息已发送或如实报告跳过原因。若已有 `CC_SESSION_KEY`，只交付外部终端命令，不在该 Agent 会话内执行割接。
	在交互终端确认安装官方 lark-cli companion；脚本执行时使用 `--lark-cli`，多个机器人
	显式传 `--lark-cli-project`。确认 profile 默认身份是 bot、原 profile 未删除、密钥只走
	stdin；不要执行 auth login、真实测试消息或 event consume。
```
