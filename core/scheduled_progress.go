package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const scheduledProgressInterval = 1500 * time.Millisecond

// scheduledProgress observes one execution. It owns no task, session, result or
// delivery state. Only its worker performs Hook I/O; producers replace a single
// pending snapshot, so a slow consumer cannot queue work behind the Agent.
type scheduledProgress struct {
	mu       sync.Mutex
	wake     chan struct{}
	done     chan struct{}
	pending  bool
	finished bool
	current  scheduledProgressSnapshot
	base     HookEvent
	send     func(HookEvent)
}

type scheduledProgressSnapshot struct {
	runID, kind, jobID, model, effort string
	started, updated                  time.Time
	sequence, thinking, tools         int64
	phase, status                     string
}

func (hm *HookManager) scheduledProgressHooks() []HookConfig {
	var hooks []HookConfig
	for _, hook := range hm.Hooks() {
		// This higher-frequency stream requires explicit opt-in. In particular,
		// existing wildcard observers must not acquire new traffic or processes.
		if strings.EqualFold(hook.Event, string(HookEventScheduledProgress)) {
			hooks = append(hooks, hook)
		}
	}
	return hooks
}

func newScheduledProgress(hm *HookManager, spec scheduledJobSpec) *scheduledProgress {
	hooks := hm.scheduledProgressHooks()
	if len(hooks) == 0 {
		return nil
	}
	runID := spec.historyRun.runID()
	if runID == "" {
		var err error
		runID, err = scheduledHistoryID()
		if err != nil {
			slog.Warn("scheduled progress: allocate run identity", "error", err)
			return nil
		}
	}
	now := time.Now()
	p := &scheduledProgress{
		wake: make(chan struct{}, 1), done: make(chan struct{}),
		base: HookEvent{Event: HookEventScheduledProgress, Project: hm.project, SessionKey: spec.sessionKey},
		current: scheduledProgressSnapshot{
			runID: runID, kind: spec.kind, jobID: spec.id,
			model: spec.model, effort: spec.reasoningEffort, started: now,
			phase: "started", status: "running",
		},
		send: func(event HookEvent) {
			for _, hook := range hooks {
				// Wait in this worker even for async hooks. Calling Emit here
				// would spawn another goroutine per event and defeat the bound.
				_ = hm.execute(&hook, event)
			}
		},
	}
	p.publishLocked()
	go p.run()
	return p
}

func (p *scheduledProgress) observe(event Event) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finished {
		return
	}
	switch event.Type {
	case EventThinking:
		if strings.TrimSpace(event.Content) == "" || isEllipsisOnly(event.Content) {
			return
		}
		p.current.thinking++
		p.current.phase = "thinking"
	case EventToolUse:
		p.current.tools++
		p.current.phase = "tool"
	case EventText:
		if strings.TrimSpace(event.Content) == "" {
			return
		}
		p.current.phase = "answer"
	default:
		return
	}
	p.publishLocked()
}

func (p *scheduledProgress) finish(err error) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finished {
		return
	}
	p.finished = true
	p.current.phase = "finished"
	p.current.status = "succeeded"
	if err != nil {
		p.current.status = "failed"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			p.current.status = "interrupted"
		}
	}
	p.publishLocked()
}

// publishLocked is called under mu, except before the worker starts.
func (p *scheduledProgress) publishLocked() {
	p.current.sequence++
	p.current.updated = time.Now()
	p.pending = true
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *scheduledProgress) run() {
	defer close(p.done)
	var next time.Time
	for range p.wake {
		for {
			p.mu.Lock()
			if !p.pending {
				p.mu.Unlock()
				break
			}
			if delay := time.Until(next); delay > 0 && !p.finished {
				p.mu.Unlock()
				timer := time.NewTimer(delay)
				select {
				case <-timer.C:
				case <-p.wake:
				}
				timer.Stop()
				continue
			}
			snapshot, terminal := p.current, p.finished
			p.pending = false
			p.mu.Unlock()
			p.send(p.event(snapshot))
			if terminal {
				return
			}
			next = time.Now().Add(scheduledProgressInterval)
		}
	}
}

func (p *scheduledProgress) event(s scheduledProgressSnapshot) HookEvent {
	event := p.base
	event.Timestamp = s.updated
	event.Extra = map[string]any{
		"schema_version": 1, "run_id": s.runID, "kind": s.kind, "job_id": s.jobID,
		"sequence": s.sequence, "started_at": s.started.UTC().Format(time.RFC3339Nano),
		"elapsed_ms": max(int64(0), s.updated.Sub(s.started).Milliseconds()),
		"phase":      s.phase, "status": s.status,
		"thinking_count": s.thinking, "tool_count": s.tools,
	}
	if s.model != "" {
		event.Extra["model"] = s.model
	}
	if s.effort != "" {
		event.Extra["reasoning_effort"] = s.effort
	}
	return event
}

func (r *scheduledTurnResult) observe(event Event) {
	if r != nil && !r.completed {
		r.progress.observe(event)
	}
}

func (e *Engine) scheduledProgressCapability() RuntimeFeatureCapability {
	availability := unavailable("No explicit scheduled.progress hook is configured.", "未显式配置 scheduled.progress Hook。")
	if len(e.hooks.scheduledProgressHooks()) > 0 {
		availability = conditional("An explicit hook receives coalesced Cron/Timer execution snapshots; ordinary chat is excluded.", "显式 Hook 接收合并后的 Cron/Timer 执行快照；不包含普通聊天。")
	}
	capability := feature("scheduled_progress_hook",
		"Publish execution identity, phase, anonymous counts and timing through a bounded non-blocking observer. Finished execution does not prove business completion or notification delivery.",
		"通过有界非阻塞观察器发布执行身份、阶段、匿名计数和耗时。执行结束不代表业务完成或通知送达。", availability,
		CapabilityFallback{Mode: "skip", Description: "Without an explicit subscription no observer runs. Hook failures and coalesced snapshots do not affect execution; consumers retain their existing result verification and recovery.", DescriptionZH: "未显式订阅时不运行观察器。Hook 失败及进度合并不影响执行；消费者保留已有结果校验与恢复流程。"})
	capability.Permission = CapabilityPermissionConditional
	readOnly := false
	capability.ReadOnly = &readOnly
	capability.SideEffects = expandSideEffects([]string{"shell_execution", "network"})
	return capability
}
