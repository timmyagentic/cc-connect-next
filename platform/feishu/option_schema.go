package feishu

import "github.com/timmyagentic/cc-connect-next/core"

func init() {
	core.RegisterPlatformConfigOptions("feishu", feishuConfigOptions())
	core.RegisterPlatformConfigOptions("lark", feishuConfigOptions())
}

func feishuConfigOptions() []core.ConfigOption {
	return core.DescribePlatformOptions([]string{
		"allow_chat", "allow_from", "app_id", "app_secret", "callback_path", "domain", "done_emoji",
		"enable_feishu_card", "encrypt_key", "group_only", "group_reply_all", "group_reply_all_chats",
		"image_batch_window_ms", "mention_map", "peer_bots", "port", "progress_style", "reaction_emoji",
		"reply_to_trigger", "require_mention", "resolve_mentions", "respond_to_at_everyone_and_here",
		"share_session_in_channel", "thread_isolation",
	})
}
