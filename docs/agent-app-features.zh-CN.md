# Awesome Agent App Features 接入说明

[English](agent-app-features.md)

CC Connect Next 固定使用
`github.com/timmyagentic/awesome-agent-app-features v0.1.0`，对应源码提交
`1634667face06c20ba1e71d1b1599c959e882376`。没有本地 `replace`、Git
submodule 或浮动 `main` 依赖。

## Feedback

CC Connect Next 继续负责命令、卡片、文本回退、本地化、最近错误选择、能力缺口
提示和公开 fallback；Foundation 负责结构化报告、环境白名单、脱敏与限长、opaque
批准值以及拒绝重定向的 HTTP Client。

1. `/feedback <描述>` 生成完整脱敏 Draft。
2. 宿主展示 `Draft.Report()` 的全部字段，并保存这份精确 Draft 十分钟。
3. 独立按钮或 `/feedback confirm` 才调用 `Approve(true)` 并发送同一份 Draft；取消、
   过期和未批准始终零请求。
4. Relay 在服务端固定 GitHub 仓库，负责 title/body、label、Token、限流和尽力去重。

## Update

daemon 提醒只负责发现。`/upgrade` 准备 immutable Plan，展示同一 Release 的 notes
和选定产物；`/upgrade confirm` 只 Apply 这份 Plan，不会重新解析 latest。

- macOS/Linux 独立安装使用 Foundation checksum、staging、双版本探针、目标锁、
  no-clobber backup、替换与回滚。
- npm 宿主 adapter 安装已审阅的精确 stable package version，并验证 package metadata
  与二进制版本。
- Windows 保持显式宿主替换 adapter，但同样消费固定 Release、精确 archive/checksum、
  staged/installed 探针、no-clobber backup 与回滚边界。
- 重启、重启后回执、卡片、自然语言意图、授权和本地化仍由 CC Connect Next 负责。

## Relay source-subtree

`feedback-relay/` 来自同一 Foundation 提交的 `relay/cloudflare`。只有
`wrangler.jsonc` 与生成的 `worker-configuration.d.ts` 允许变化。Worker 名称和服务端
目标仓库是宿主映射；Rate Limiting namespace 在单独授权部署前保持 dry-run 占位值。

所有 Relay 命令必须进入 `feedback-relay/` 后执行，不能用从其他 cwd 指向外部绝对
目录的 `npm --prefix` 代替最终目标验证。

## Lock 验证

`agent-app-features.lock.json` 记录精确来源、module delivery、subtree 目标、改动文件、
检查和未验证的生产边界。使用同一提交的临时完整源码解压目录运行：

```bash
GOWORK=off go run \
  github.com/timmyagentic/awesome-agent-app-features/cmd/feature-lock@v0.1.0 \
  validate \
  --source "$EXACT_SOURCE_ROOT" \
  --source-commit 1634667face06c20ba1e71d1b1599c959e882376 \
  --host "$CC_CONNECT_NEXT_ROOT" \
  --lock "$CC_CONNECT_NEXT_ROOT/agent-app-features.lock.json"
```

Lock 只是维护元数据，不是运行配置，也不能证明公开 Relay endpoint 已部署新协议。
