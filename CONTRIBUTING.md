# Contributing to CC Connect Feishu Plus

[中文](#为-cc-connect-feishu-plus-做贡献) | [English](#contributing-to-cc-connect-feishu-plus)

## Contributing to CC Connect Feishu Plus

Please use this repository's
[Issues](https://github.com/timmyagentic/cc-connect-feishu-plus/issues) and
[Pull requests](https://github.com/timmyagentic/cc-connect-feishu-plus/pulls).
The project is maintained independently; delivery must not depend on an
upstream pull request being accepted.

Before opening a change:

- search this repository for an existing report or proposal;
- include the Feishu Plus binary/bootstrap version, OS, agent, installation
  method, and a minimal reproduction;
- redact application secrets, tokens, message contents, and user identifiers;
- state whether the behavior occurs with `plus_enabled = false`.

Implementation rules:

- preserve the native `feishu`/`lark` path when Plus is disabled;
- put deep Feishu behavior behind an explicit `plus_*` option;
- never create a second connection for the same Feishu application;
- add a focused regression test and document rollback behavior;
- keep update and download sources on this repository.

Minimum validation:

```bash
corepack pnpm@10.28.2 --dir web install --frozen-lockfile
corepack pnpm@10.28.2 --dir web build
go test ./...
npm run check --prefix npm
```

Public npm or binary publication also requires the release gates documented in
[docs/upstream-baseline.md](./docs/upstream-baseline.md).

---

## 为 CC Connect Feishu Plus 做贡献

请使用本仓库的
[Issues](https://github.com/timmyagentic/cc-connect-feishu-plus/issues) 和
[Pull requests](https://github.com/timmyagentic/cc-connect-feishu-plus/pulls)。
这是一个独立维护项目，交付不能依赖上游接受 PR。

提交前请：

- 先搜索本仓库是否已有相同问题或方案；
- 提供 Feishu Plus 二进制/引导工具版本、操作系统、Agent、安装方式和最小复现；
- 对应用密钥、Token、消息内容和用户标识进行脱敏；
- 说明关闭 `plus_enabled` 后问题是否仍然存在。

实现要求：

- Plus 关闭时保留原生 `feishu`/`lark` 路径；
- 深入飞书链路的行为必须放在明确的 `plus_*` 开关后；
- 同一个飞书应用不得创建第二条连接；
- 增加聚焦的回归测试并说明回滚方式；
- 更新检查和下载源只能指向本仓库。

最低验证：

```bash
corepack pnpm@10.28.2 --dir web install --frozen-lockfile
corepack pnpm@10.28.2 --dir web build
go test ./...
npm run check --prefix npm
```

公开 npm 或二进制发布还必须满足
[docs/upstream-baseline.md](./docs/upstream-baseline.md) 中的发布门禁。
