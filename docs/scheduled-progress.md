# Scheduled execution progress

An explicit `scheduled.progress` Hook receives anonymous execution snapshots for
Cron and Timer jobs. It is an optional side channel: execution, result storage,
ordinary chat and platform card rendering keep their existing responsibilities.
No observer is created without this exact subscription; `*` does not opt in.

```toml
[[hooks]]
project = "my-project"
event = "scheduled.progress"
type = "command"
command = "python3 /path/to/update_progress.py"
input = "json"
timeout = 3
```

Both command and HTTP handlers use the existing Hook transport. `async` does not
change this event's behavior: one worker per execution performs at most one Hook
call at a time, retaining only the latest pending snapshot. Intermediate updates
are spaced by at least 1.5 seconds after the previous call finishes. The first
available snapshot and the terminal snapshot bypass this interval. A terminal
snapshot replaces pending progress and cannot be overwritten by late events.
Handlers keep their configured timeout. A blocked or failed handler never blocks
the Agent or another execution; failures are logged without restarting the task
or replaying the snapshot. Multiple handlers for one run execute sequentially.

The envelope contains `event`, `project`, `timestamp` and the original scheduled
`session_key`. Its `extra` object has the following versioned fields:

| Field | Meaning |
| --- | --- |
| `schema_version` | `1` |
| `run_id` | Unique execution ID; reuses history ID when available, otherwise allocated independently |
| `kind`, `job_id` | `cron` or `timer`, and the persisted job ID |
| `sequence` | Increasing integer within this run; gaps are normal when updates coalesce |
| `started_at`, `elapsed_ms` | Execution observer start in RFC3339 UTC, and elapsed milliseconds |
| `phase` | `started`, `thinking`, `tool`, `answer`, or `finished` |
| `status` | `running`, `succeeded`, `failed`, or `interrupted` |
| `thinking_count`, `tool_count` | Anonymous event counts; no reasoning, tool names, arguments, outputs or answer text |
| `model`, `reasoning_effort` | Explicit scheduled overrides when set; absent values are unknown/inherited |

Shell jobs emit start/end snapshots with zero Agent-event counts. Codex prompt
jobs count completed reasoning items even when their summaries are empty.
The app-server supplies item IDs directly. The exec adapter also reads its
current turn's local transcript because the CLI can omit these items from JSON
output. When that transcript is unavailable, exec counts the reasoning items
present in its output. Intermediate agent messages are excluded. Other agents
count meaningful thinking events from their own streams. Tool-use events have
their own count. The `answer` phase marks the start of answer text. The Hook
publishes only execution metadata.

Successful prompt completion is emitted after the turn's normal result commit.
It is detached before a reused session drains ordinary messages. Start/setup
errors produce `failed`; cancellations and execution deadlines produce
`interrupted`. A scheduler waiting timeout does not stop the Agent by itself and
does not emit a fictitious successful completion: progress can continue until
the actual execution ends. Scheduler/history finalization can follow the Hook.

Consumers must bind the exact project, job and `run_id`, retain the latest
sequence, and keep one owner for a visible card. A `finished` event can wake the
existing result verifier; it does not prove business completion, scheduler
success or message delivery. Keep the existing notification gates, result
verification and recovery path. Snapshots are best-effort and not persisted or
replayed across restart, so correctness must not depend on every frame arriving.
The `scheduled_progress_hook` runtime capability advertises availability and
side effects without publishing handler commands, URLs or runtime data.

## 中文

在现有 `[[hooks]]` 中显式订阅 `scheduled.progress`，即可接收 Cron/Timer 的
执行身份、阶段、匿名计数和耗时。它是可选旁路，未配置时不创建观察器；`*` 不会
收到进度。普通聊天、任务执行、结果保存和原有卡片渲染保持原职责。

每次执行使用独立工作器，最多一个正在执行的 Hook 调用和一个最新待发送快照。
中间进度在上次调用结束后至少间隔 1.5 秒；首个可用快照和终态无需等该间隔。
终态覆盖待发进度，迟到事件不能覆盖终态。`async` 不改变这一行为，现有 Hook
超时继续有效。慢回调或失败不会拖住 Agent 或其他任务；失败只记录日志，不重跑
任务、不重投该快照。同一执行配置多个处理器时依次调用。

事件格式及字段见上表。`run_id` 优先复用历史记录 ID；历史关闭或写入失败时，仍
为本次执行独立生成 ID。`session_key` 始终是任务原目标。计数来自真实 Agent
事件；不传思考正文、工具名、参数、结果或回答。模型和强度仅在任务显式设置时
提供，省略表示未知或继承。Shell 任务只有开始与结束，Agent 计数为零。

Prompt 成功终态在原有结果提交后发布，并在复用会话继续普通消息前解绑。取消或
执行截止时间记为 `interrupted`。调度器等待超时不一定停止 Agent，不能据此宣布
调查完成；进度可持续到实际执行结束。Hook 到达时调度历史可能尚未最终提交。

业务侧应校验项目、任务和本次执行 ID，以序号忽略旧更新，并独占卡片写入。
`finished` 只用于唤醒原有结果校验，不能当成业务已完成或消息已送达。保留已有
通知条件、结果校验和恢复机制。进度尽力交付，不持久化、不跨重启重放；正确性
不能依赖每一帧到达。运行能力 `scheduled_progress_hook` 会描述可用性、副作用
及降级方式，不公开处理器命令、URL 或运行数据。
