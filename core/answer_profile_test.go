package core

import "testing"

func TestParseAnswerProfilePrefix(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantProfile AnswerProfileName
		wantPrompt  string
		wantMatched bool
	}{
		{name: "fast command", input: "/fast finish this task", wantProfile: AnswerProfileFast, wantPrompt: "finish this task", wantMatched: true},
		{name: "fast command case insensitive", input: "/FAST finish this task", wantProfile: AnswerProfileFast, wantPrompt: "finish this task", wantMatched: true},
		{name: "quality command", input: "/quality\n检查这个问题", wantProfile: AnswerProfileQuality, wantPrompt: "检查这个问题", wantMatched: true},
		{name: "use fast mode", input: "用快速模式完成这个任务", wantProfile: AnswerProfileFast, wantPrompt: "完成这个任务", wantMatched: true},
		{name: "polite quality mode", input: "请使用高质量模式来完成这个任务", wantProfile: AnswerProfileQuality, wantPrompt: "完成这个任务", wantMatched: true},
		{name: "task verb is preserved", input: "用快速模式处理器是什么", wantProfile: AnswerProfileFast, wantPrompt: "处理器是什么", wantMatched: true},
		{name: "fast label", input: "快速模式：处理这个任务", wantProfile: AnswerProfileFast, wantPrompt: "处理这个任务", wantMatched: true},
		{name: "quality label", input: "高质量模式: 回答这个问题", wantProfile: AnswerProfileQuality, wantPrompt: "回答这个问题", wantMatched: true},
		{name: "empty command still recognized", input: "/fast", wantProfile: AnswerProfileFast, wantPrompt: "", wantMatched: true},
		{name: "empty natural prefix still recognized", input: "使用高质量模式", wantProfile: AnswerProfileQuality, wantPrompt: "", wantMatched: true},
		{name: "ordinary message", input: "完成这个任务", wantPrompt: "完成这个任务"},
		{name: "negative intent", input: "不要用快速模式完成这个任务", wantPrompt: "不要用快速模式完成这个任务"},
		{name: "comparison", input: "比较快速模式和高质量模式", wantPrompt: "比较快速模式和高质量模式"},
		{name: "embedded command", input: "请解释 /fast 是什么", wantPrompt: "请解释 /fast 是什么"},
		{name: "balanced command is not supported", input: "/balanced 完成这个任务", wantPrompt: "/balanced 完成这个任务"},
		{name: "default natural mode is not supported", input: "用默认模式完成这个任务", wantPrompt: "用默认模式完成这个任务"},
		{name: "abbreviated command is not supported", input: "/f 完成这个任务", wantPrompt: "/f 完成这个任务"},
		{name: "longer command is not a match", input: "/fastest 完成这个任务", wantPrompt: "/fastest 完成这个任务"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, prompt, matched := parseAnswerProfilePrefix(tt.input)
			if profile != tt.wantProfile || prompt != tt.wantPrompt || matched != tt.wantMatched {
				t.Fatalf("parseAnswerProfilePrefix(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.input, profile, prompt, matched, tt.wantProfile, tt.wantPrompt, tt.wantMatched)
			}
		})
	}
}
