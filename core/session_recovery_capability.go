package core

// AutomaticSessionRecoverySupport reports local transport support without
// reading history or contacting the backend.
type AutomaticSessionRecoverySupport interface {
	SupportsAutomaticSessionRecovery() bool
}

func automaticSessionRecoveryCapability(session AgentSession) RuntimeFeatureCapability {
	availability := conditional("An active session is required to determine support.", "需要活动会话才能确定支持情况。")
	if session != nil {
		provider, ok := session.(AutomaticSessionRecoverySupport)
		availability = unavailable("This session does not support local automatic context recovery.", "当前会话不支持本地自动上下文恢复。")
		if ok && provider.SupportsAutomaticSessionRecovery() {
			availability = conditional("Local transport supports recovery after the failed turn ends and the native process releases its history.", "本地传输支持在失败回合结束、原生进程释放历史后恢复。")
		}
	}
	capability := feature("automatic_session_recovery",
		"After a confirmed signature failure, clear encrypted history and retry once per user Send in the same native thread. Preserve plain conversation and recorded tool results. The first failure is handled internally.",
		"确认签名错误后清理加密历史，在同一个原生会话内重试，每次用户 Send 最多一次。保留普通对话和已记录的工具结果，首次错误在内部处理。",
		availability, CapabilityFallback{Mode: "fail-closed", Description: "Report failure when safe history cleanup, persistence or retry fails. User stop cancels recovery.", DescriptionZH: "安全清理历史、持久化或重试失败时报告错误。用户停止会取消恢复。"})
	readOnly := false
	capability.ReadOnly = &readOnly
	capability.Permission = CapabilityPermissionConditional
	capability.SideEffects = expandSideEffects([]string{"filesystem_read", "persistent_state", "agent_turn", "network"})
	return capability
}
