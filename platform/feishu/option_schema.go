package feishu

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("feishu", feishuConfigOptions())
	core.RegisterPlatformConfigOptions("lark", feishuConfigOptions())
}

func feishuConfigOptions() []core.ConfigOption {
	options := core.DescribePlatformOptions([]string{
		"allow_chat", "allow_from", "app_id", "app_secret", "callback_path", "domain", "done_emoji",
		"enable_feishu_card", "encrypt_key", "group_only", "group_reply_all", "group_reply_all_chats",
		"image_batch_window_ms", "mention_map", "peer_bots", "port", "progress_style", "reaction_emoji",
		"reply_to_trigger", "require_mention", "resolve_mentions", "respond_to_at_everyone_and_here",
		"share_session_in_channel", "thread_isolation",
	})
	for i := range options {
		if options[i].Key != "thread_isolation" {
			continue
		}
		options[i].Default = "false when omitted; new Starter/recommended profile writes true"
		options[i].Description = "Use a separate Agent session and workspace binding for each Feishu/Lark topic. Omitting the key keeps the false compatibility fallback; new Starter configs and accepted recommended profiles explicitly set true."
		options[i].DescriptionZH = "为每个飞书/Lark 话题使用独立 Agent 会话和工作区绑定。省略该键时保留 false 兼容回落；新 Starter 配置和用户接受的推荐 Profile 会显式写入 true。"
		options[i].Keywords = []string{"topic isolation", "multiple topics", "话题隔离", "话题独立", "多个话题"}
	}
	return core.ConfigureOption(options, "port", "8080")
}
