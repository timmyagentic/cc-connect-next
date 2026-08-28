package core

import (
	"strings"
	"testing"
)

func TestExecuteTimerJobDeliversPromptAndReply(t *testing.T) {
	platform := &stubCronReplyTargetPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "discord"},
	}
	agentSession := newResultAgentSession("timer complete")
	engine := NewEngine("test", &resultAgent{session: agentSession}, []Platform{platform}, "", LangEnglish)
	t.Cleanup(func() { _ = engine.Stop() })

	job := &TimerJob{
		ID:          "timer-1",
		SessionKey:  "discord:channel-1:user-1",
		Prompt:      "summarize timer activity",
		Description: "Timer summary",
	}
	if err := engine.ExecuteTimerJob(job); err != nil {
		t.Fatalf("ExecuteTimerJob() error = %v", err)
	}
	if platform.resolvedSessionKey != job.SessionKey || platform.resolveTitle != job.Description {
		t.Fatalf("resolved target = (%q, %q), want (%q, %q)",
			platform.resolvedSessionKey, platform.resolveTitle, job.SessionKey, job.Description)
	}
	if sent := platform.getSent(); len(sent) != 2 || sent[0] != "⏰ Timer summary" || sent[1] != "timer complete" {
		t.Fatalf("sent messages = %#v", sent)
	}
	if len(agentSession.sentPrompts) != 1 || !strings.Contains(agentSession.sentPrompts[0], job.Prompt) {
		t.Fatalf("agent prompts = %#v", agentSession.sentPrompts)
	}
}
