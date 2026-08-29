package feishu

import (
	"strconv"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/timmyagentic/cc-connect-next/core"
)

func init() {
	core.RegisterPlatformConfigOptions("feishu", feishuConfigOptions(lark.FeishuBaseUrl))
	core.RegisterPlatformConfigOptions("lark", feishuConfigOptions(lark.LarkBaseUrl))
}

func feishuConfigOptions(defaultDomain string) []core.ConfigOption {
	options := core.DescribePlatformOptions([]string{
		"allow_chat", "allow_from", "app_id", "app_secret", "callback_path", "domain", "done_emoji",
		"enable_feishu_card", "encrypt_key", "group_only", "group_reply_all", "group_reply_all_chats",
		"image_batch_window_ms", "mention_map", "peer_bots", "port", "progress_style", "reaction_emoji",
		"reply_to_trigger", "require_mention", "resolve_mentions", "respond_to_at_everyone_and_here",
		"share_session_in_channel", "thread_isolation",
	})
	for i := range options {
		option := &options[i]
		switch option.Key {
		case "allow_chat":
			option.Default = "empty"
			option.Description = "Restrict access to a comma-separated list of Feishu/Lark chat IDs; empty or '*' allows every chat."
			option.DescriptionZH = "将访问限制为逗号分隔的飞书/Lark 会话 ID；留空或 '*' 表示允许所有会话。"
		case "app_id":
			option.Default = "none"
			option.DefaultSource = core.ConfigDefaultNone
			option.Requirement = core.ConfigRequirementRequired
			option.Description = "Identify the Feishu/Lark bot application; this option is required."
			option.DescriptionZH = "标识飞书/Lark 机器人应用；此配置必填。"
		case "app_secret":
			option.Default = "none"
			option.DefaultSource = core.ConfigDefaultNone
			option.Requirement = core.ConfigRequirementRequired
			option.Description = "Authenticate the Feishu/Lark bot application; this sensitive option is required."
			option.DescriptionZH = "认证飞书/Lark 机器人应用；此敏感配置必填。"
		case "callback_path":
			option.Default = defaultCallbackPath
			option.Description = "Set the inbound webhook callback path used when encrypt_key enables webhook mode."
			option.DescriptionZH = "设置 encrypt_key 启用 Webhook 模式后使用的入站回调路径。"
		case "domain":
			option.Default = defaultDomain
			option.Description = "Override the Feishu/Lark OpenAPI and WebSocket base URL; Feishu and Lark use different SDK defaults."
			option.DescriptionZH = "覆盖飞书/Lark OpenAPI 与 WebSocket 基础地址；飞书和 Lark 使用不同的 SDK 默认值。"
		case "done_emoji":
			option.Default = defaultDoneEmoji
			option.Description = "Choose the completion reaction. 'none' disables it; reaction_emoji = 'none' also disables the implicit completion reaction unless done_emoji is set explicitly."
			option.DescriptionZH = "选择完成时的表情回应。'none' 表示关闭；reaction_emoji = 'none' 也会关闭隐式完成回应，除非显式设置 done_emoji。"
			option.PresetValues = []core.ConfigPresetValue{{Preset: "starter/recommended-feishu", Value: defaultDoneEmoji, Description: "Pinned for completion notification.", DescriptionZH: "固定完成通知表情。"}}
		case "enable_feishu_card":
			option.Default = "true"
			option.Description = "Use Feishu/Lark interactive cards for replies; false falls back to non-card replies."
			option.DescriptionZH = "使用飞书/Lark 互动卡片回复；设为 false 时回退为非卡片回复。"
			option.PresetValues = []core.ConfigPresetValue{{Preset: "starter/recommended-feishu", Value: "true", Description: "Use interactive answer cards.", DescriptionZH: "使用互动回答卡片。"}}
		case "encrypt_key":
			option.Default = "unset"
			option.Description = "Leave unset to consume events over WebSocket; set the event encrypt key to select webhook mode and decrypt webhook events."
			option.DescriptionZH = "留空时通过 WebSocket 消费事件；设置事件 Encrypt Key 后切换为 Webhook 模式并解密 Webhook 事件。"
		case "group_only":
			option.Default = "false"
		case "group_reply_all":
			option.Default = "false"
			option.Description = "Reply to every group message without an explicit bot mention, unless a non-empty group_reply_all_chats allowlist takes precedence."
			option.DescriptionZH = "无需明确 @ 机器人即可回复所有群消息；但非空 group_reply_all_chats 白名单优先。"
			option.PresetValues = []core.ConfigPresetValue{{Preset: "starter/recommended-feishu", Value: "true", Description: "Answer group messages without an @mention; scope allow_from/allow_chat first.", DescriptionZH: "群聊无需 @ 即回复；应先配置 allow_from/allow_chat 范围。"}}
		case "group_reply_all_chats":
			option.Type = "string | string[]"
			option.Default = "empty"
			option.Description = "Allow mention-free replies only in selected chat IDs. Accepts a comma-separated string or string array; a non-empty list takes precedence over group_reply_all."
			option.DescriptionZH = "仅在指定会话 ID 中允许无需 @ 的回复。支持逗号分隔字符串或字符串数组；非空列表优先于 group_reply_all。"
			option.Example = `group_reply_all_chats = "oc_chat_a,oc_chat_b"`
		case "image_batch_window_ms":
			option.Type = "integer"
			option.Default = strconv.FormatInt(defaultImageBatchWindow.Milliseconds(), 10)
			option.Description = "Batch consecutive images from one session after this quiet window in milliseconds. Zero uses the 500 ms fallback; negative values are rejected."
			option.DescriptionZH = "同一会话的连续图片在该静默窗口（毫秒）后合并处理。0 仍使用 500 ms 回退值；负数会被拒绝。"
		case "mention_map":
			option.Type = "table"
			option.Default = "empty"
			option.Description = "Map a friendly bot name to its open_id for outbound native @ mentions; requires resolve_mentions = true."
			option.DescriptionZH = "将友好机器人名称映射到 open_id，以便出站消息使用原生 @；要求 resolve_mentions = true。"
			option.Requires = []string{"resolve_mentions = true"}
			option.Example = `mention_map = { Reviewer-Bot = "ou_bot_open_id" }`
		case "peer_bots":
			option.Type = "table"
			option.Default = "empty"
			option.Description = "Map each peer bot app_id to a friendly alias for quoted-reply attribution."
			option.DescriptionZH = "将每个对端机器人 app_id 映射为友好别名，用于引用回复归因。"
			option.Example = `peer_bots = { cli_peer_app_id = "Reviewer-Bot" }`
		case "port":
			option.Type = "string"
			option.Default = defaultWebhookPort
			option.Description = "Set the webhook-mode listening port as a quoted string."
			option.DescriptionZH = "以带引号的字符串设置 Webhook 模式监听端口。"
		case "progress_style":
			option.Default = defaultProgressStyle
			option.Values = []string{"legacy", "compact", "card"}
			option.Description = "Choose legacy, compact, or card progress rendering for Feishu/Lark replies."
			option.DescriptionZH = "选择飞书/Lark 回复的 legacy、compact 或 card 进度展示样式。"
		case "reaction_emoji":
			option.Default = defaultReactionEmoji
			option.Description = "Choose the processing reaction; 'none' disables it and also suppresses the implicit done reaction."
			option.DescriptionZH = "选择处理中表情回应；'none' 会关闭它，并同时抑制隐式完成回应。"
		case "reply_to_trigger":
			option.Default = "true"
			option.Description = "Reply using the triggering message as the target. When false, ordinary replies are created without quoting it; a real topic isolated by thread_isolation still targets that topic's thread_id to preserve topic locality."
			option.DescriptionZH = "以触发消息作为回复目标。设为 false 时普通回复不再引用该消息；由 thread_isolation 隔离的真实话题仍会指向该话题的 thread_id，以保持话题归属。"
			option.PresetValues = []core.ConfigPresetValue{{Preset: "starter/recommended-feishu", Value: "true", Description: "Quote the triggering message in ordinary chats.", DescriptionZH: "在普通会话中引用触发消息。"}}
		case "require_mention":
			option.Default = "true"
			option.Description = "Require an explicit bot mention in group chats. Setting false is a compatibility alias for group_reply_all = true; true does not override an explicit group_reply_all setting."
			option.DescriptionZH = "群聊中要求明确 @ 机器人。设为 false 是 group_reply_all = true 的兼容别名；true 不会覆盖显式 group_reply_all 配置。"
		case "resolve_mentions":
			option.Default = "false"
			option.Description = "Resolve inbound Feishu/Lark mentions to readable names and enable mention_map for outbound native bot mentions."
			option.DescriptionZH = "将入站飞书/Lark @ 解析为可读名称，并启用 mention_map 以发送原生机器人 @。"
		case "respond_to_at_everyone_and_here":
			option.Default = "false"
		case "share_session_in_channel":
			option.Default = "false"
			option.Description = "Share one Agent session among users in the same non-isolated channel; thread_isolation can still give real topics separate sessions."
			option.DescriptionZH = "让同一非隔离频道内的用户共享一个 Agent 会话；thread_isolation 仍可为真实话题建立独立会话。"
		case "thread_isolation":
			option.Type = "string | boolean (legacy)"
			option.Default = "off"
			option.Values = []string{"off", "topics_only", "topic_per_message"}
			option.Description = "Choose Feishu/Lark topic isolation scope. off keeps legacy per-user/channel sessions. topics_only isolates only real topics whose events carry thread_id; ordinary group messages stay in the main chat. topic_per_message gives every top-level group message its own topic/session. Real topics get an independent Agent session and workspace binding in both enabled modes. Omitting the key maps to off; legacy true maps to topic_per_message and false maps to off. New Starter and recommended profiles write topics_only."
			option.DescriptionZH = "选择飞书/Lark 话题隔离范围。off 沿用旧版按用户/频道会话；topics_only 只隔离事件携带 thread_id 的真实话题，普通群消息留在群主会话；topic_per_message 让每条群主会话消息都拥有独立话题/session。两种启用模式都会给真实话题独立 Agent 会话和工作区绑定。省略该键映射 off；旧 true 映射 topic_per_message，旧 false 映射 off；新 Starter 和推荐 Profile 写入 topics_only。"
			option.Keywords = []string{"topic isolation", "multiple topics", "话题隔离", "话题独立", "多个话题"}
			option.PresetValues = []core.ConfigPresetValue{{Preset: "starter/recommended-feishu", Value: "topics_only", Description: "Isolate real topics without promoting ordinary group messages.", DescriptionZH: "隔离真实话题，不把普通群消息提升为话题。"}}
		}
		if option.DefaultSource == core.ConfigDefaultAdapter && option.Default != "unset" {
			option.DefaultSource = core.ConfigDefaultBuiltin
		}
	}
	options = core.ConfigureOptionExample(options, "group_reply_all_chats", `group_reply_all_chats = "oc_chat_a,oc_chat_b"`)
	options = core.ConfigureOptionExample(options, "mention_map", `mention_map = { Reviewer-Bot = "ou_bot_open_id" }`)
	options = core.ConfigureOptionExample(options, "peer_bots", `peer_bots = { cli_peer_app_id = "Reviewer-Bot" }`)
	return options
}
