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
		options[i].Type = "string | boolean (legacy)"
		options[i].Default = "off when omitted; new Starter/recommended profile writes topics_only; legacy true maps to topic_per_message"
		options[i].Values = []string{"off", "topics_only", "topic_per_message"}
		options[i].Description = "Choose Feishu/Lark topic isolation scope. off keeps legacy per-user/channel sessions. topics_only isolates only real topics whose events carry thread_id; ordinary group messages stay in the main chat. topic_per_message gives every top-level group message its own topic/session. Real topics get an independent Agent session and workspace binding in both enabled modes. Omitting the key maps to off; legacy true maps to topic_per_message and false maps to off. New Starter and recommended profiles write topics_only."
		options[i].DescriptionZH = "选择飞书/Lark 话题隔离范围。off 沿用旧版按用户/频道会话；topics_only 只隔离事件携带 thread_id 的真实话题，普通群消息留在群主会话；topic_per_message 让每条群主会话消息都拥有独立话题/session。两种启用模式都会给真实话题独立 Agent 会话和工作区绑定。省略该键映射 off；旧 true 映射 topic_per_message，旧 false 映射 off；新 Starter 和推荐 Profile 写入 topics_only。"
		options[i].Keywords = []string{"topic isolation", "multiple topics", "话题隔离", "话题独立", "多个话题"}
	}
	return core.ConfigureOption(options, "port", "8080")
}
