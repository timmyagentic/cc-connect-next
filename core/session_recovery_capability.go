package core

// AutomaticSessionRecoverySupport reports structural support without reading
// history, probing the backend, or exposing paths or configured values. Actual
// recovery must independently validate the failed turn and complete history.
type AutomaticSessionRecoverySupport interface {
	SupportsAutomaticSessionRecovery() bool
}

func automaticSessionRecoveryCapability(session AgentSession) RuntimeFeatureCapability {
	availability := conditional("An active session is required to determine support.", "需要活动会话才能确定支持情况。")
	if session != nil {
		provider, ok := session.(AutomaticSessionRecoverySupport)
		availability = unavailable("This session does not support local automatic context recovery.", "当前会话不支持本地自动上下文恢复。")
		if ok && provider.SupportsAutomaticSessionRecovery() {
			availability = conditional("Local transport supports recovery; the failed turn, readable history, tool outcomes, backend RPCs, and durable binding must pass validation at recovery time.", "本地传输支持恢复；恢复时仍须验证失败回合、可读历史、工具结果、后端 RPC 和持久化绑定。")
		}
	}
	capability := feature("automatic_session_recovery",
		"Automatically attempt encrypted-context recovery once per user Send after a confirmed signature failure. Import readable history and recorded tool results into a new thread, persist its binding, then continue under existing instructions and permissions. Historical tools are not replayed; subsequent model tool choices are not an exactly-once guarantee.",
		"确认签名校验失败后，每次用户 Send 最多自动恢复一次加密上下文。将可读历史及工具结果导入新会话，持久化绑定后按已有指令和权限继续。不会回放历史工具；不保证模型后续工具选择恰好执行一次。",
		availability, CapabilityFallback{Mode: "fail-closed", Description: "Stop with the original failure if history, tool outcomes, required RPCs, or persistence are unavailable or ambiguous. Accepted or uncertain steer input disables recovery. User stop cancels recovery; no automatic retry after recovery fails.", DescriptionZH: "历史、工具结果、必要 RPC 或持久化不可用或不明确时保留原失败并停止。已接受或结果不明的 steer 输入禁止恢复；用户停止会取消恢复，恢复失败不自动重试。"})
	readOnly := false
	capability.ReadOnly = &readOnly
	capability.Permission = CapabilityPermissionConditional
	capability.SideEffects = expandSideEffects([]string{"filesystem_read", "persistent_state", "agent_turn", "network"})
	return capability
}
