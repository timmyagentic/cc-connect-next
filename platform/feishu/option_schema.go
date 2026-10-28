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
		"allow_chat", "allow_from", "app_id", "app_secret", "callback_path", "cc_data_dir", "cc_project", "domain", "done_emoji",
		"enable_feishu_card", "encrypt_key", "group_only", "group_reply_all", "group_reply_all_chats",
		"group_context", "group_context_include_peer_bots", "group_context_max_messages", "group_context_window_minutes", "group_context_max_chars",
		"image_batch_window_ms", "mention_map", "peer_bots", "port", "progress_style", "reaction_emoji",
		"reply_to_trigger", "require_mention", "resolve_mentions", "respond_to_at_everyone_and_here",
		"share_session_in_channel", "thread_isolation", "completion_notify", "completion_notify_users", "unread_reminder", "unread_reminder_users", "unread_reminder_delay_mins",
	})
	for i := range options {
		option := &options[i]
		switch option.Key {
		case "completion_notify", "unread_reminder":
			option.Type = "boolean"
			option.Default = "false"
			option.Requires = []string{"non-empty completion_notify_users (or legacy unread_reminder_users)"}
			option.Description = "Immediately send one private completion notification from the same bot to the configured turn owner after a successful visible answer, with a link to the original conversation. No topic notice, read-status check or delay."
			option.DescriptionZH = "成功投递可见答复后，由同一机器人立即私聊配置名单中的回合发起人一次，附原对话链接。不在原话题追加通知，不查询已读或延迟发送。"
			option.Keywords = []string{"private completion notification", "bot direct message", "完成通知私聊", "机器人完成通知"}
			if option.Key == "unread_reminder" {
				option.Deprecated = true
				option.Description = "Deprecated alias for completion_notify, used only when completion_notify is absent. Now sends immediate private completion notifications, with no unread checks or delay."
				option.DescriptionZH = "completion_notify 的废弃兼容别名，仅在新选项未设置时生效。现改为立即私聊完成通知，不再查询已读或等待。"
			}
		case "completion_notify_users", "unread_reminder_users":
			option.Type = "string[]"
			option.Default = "empty"
			option.Requires = []string{"completion_notify = true (or legacy unread_reminder = true)"}
			option.Description = "Explicit app-scoped user open_ids eligible for private completion notifications. Notify only the owner of the completed turn, using the same bot; no wildcard or implicit recipient."
			option.DescriptionZH = "允许接收私聊完成通知的应用范围用户 open_id；只通知完成回合的发起人，由同一机器人发送。不支持通配符或隐式收件人。"
			option.Keywords = []string{"completion notification users", "完成通知接收人"}
			if option.Key == "unread_reminder_users" {
				option.Deprecated = true
				option.Description = "Deprecated alias for completion_notify_users, used only when completion_notify_users is absent. IDs remain scoped to this bot app."
				option.DescriptionZH = "completion_notify_users 的废弃兼容别名，仅在新选项未设置时生效。ID 仍限定于当前机器人应用。"
			}
		case "unread_reminder_delay_mins":
			option.Type = "integer"
			option.Default = "ignored"
			option.Unit = "minutes"
			minimum, maximum := float64(1), float64(10079)
			option.Minimum, option.Maximum = &minimum, &maximum
			option.Deprecated = true
			option.Description = "Deprecated and ignored; remove this option. Valid old beta values remain accepted, but completion notifications are sent immediately with no read-status queries or delayed reminders."
			option.DescriptionZH = "已废弃并忽略，请删除。旧 beta 的合法值仍可解析，但完成通知改为立即私聊，不再查询已读或延迟提醒。"
			option.Keywords = []string{"unread reminder delay", "未读提醒等待时长", "迁移完成通知"}
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
		case "group_context":
			option.Type = "boolean"
			option.Default = "false"
			option.Description = "On first participation, read bounded recent group discussion as untrusted background. Later requests and busy follow-ups read only messages since the last delivered input, regardless of sender. Save the read position on input delivery, not task completion. Images/files are downloaded only within the text and media budgets. Groups/topics stay isolated; existing access only, no polling."
			option.DescriptionZH = "首次参与时补读有界近期群聊背景；后续请求及忙时追加只读取上次交付之后的新消息，不区分发送者。输入交付后保存阅读位置，不等待任务完成。图片和文件仅在文字与媒体预算内下载。群和话题分别隔离；仅使用已有权限，不轮询。"
			option.Keywords = []string{"recent discussion", "group context", "catch up", "补读近期讨论", "群聊上下文", "未@讨论", "群聊补读"}
		case "group_context_include_peer_bots":
			option.Type = "boolean"
			option.Default = "false"
			option.Requires = []string{"group_context = true", "non-empty peer_bots with other bot app IDs and aliases"}
			option.Description = "Include new public messages and readable card snapshots from the exact peer_bots app IDs in the same group/topic. Preserve sender and card status. Edits to messages behind the read position are not automatically replayed. Ignore bot-originated triggers, own/unknown bots and private conversations; native Agent sessions remain separate."
			option.DescriptionZH = "将 peer_bots 中精确 app ID 的新增公开消息和可读卡片快照纳入同群、同话题背景，保留发送者和卡片状态。已读位置之前的消息编辑不自动重放。忽略机器人触发消息，排除自身、未知机器人和私聊；原生 Agent 会话保持独立。"
			option.Keywords = []string{"shared bot context", "peer bot", "机器人上下文互通", "同群同话题", "接续对话"}
		case "group_context_max_messages", "group_context_window_minutes", "group_context_max_chars":
			option.Type = "integer"
			option.Requires = []string{"group_context = true"}
			minimum, maximum, fallback := 1.0, float64(maxGroupContextMessages), defaultGroupContextMessages
			option.Unit = "messages"
			option.Description = "Maximum background messages per accepted input, after excluding messages at or before the delivered read position. Keep the newest messages and disclose truncation."
			option.DescriptionZH = "每次输入最多补读的背景消息条数；先排除已读位置及之前的消息，再保留最新消息，明确提示截断。"
			switch option.Key {
			case "group_context_window_minutes":
				maximum, fallback = float64(maxGroupContextMinutes), defaultGroupContextMinutes
				option.Unit = "minutes"
				option.Description = "Initial catch-up window before the first trigger in an Agent conversation. Later inputs read from the last delivered position to their trigger timestamp, even after a longer gap. Message, page and character budgets still apply."
				option.DescriptionZH = "Agent 会话首次参与时向前补读的分钟数。后续输入从上次已交付位置读到本次触发时间，间隔较长时也不重置为半小时窗口；仍受条数、页数和字符预算限制。"
			case "group_context_max_chars":
				minimum, maximum, fallback = float64(minGroupContextChars), float64(maxGroupContextChars), defaultGroupContextChars
				option.Unit = "Unicode characters"
				option.Description = "Bound the complete JSON background including message text, sender, time and reply metadata; preserve the newest discussion and disclose truncation."
				option.DescriptionZH = "限制完整 JSON 背景的字符数，包含正文、发送者、时间及回复元数据；优先保留最新讨论并提示截断。"
			}
			option.Minimum, option.Maximum = &minimum, &maximum
			option.Default = strconv.Itoa(fallback)
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
			option.Description = "Map each peer bot app_id to a friendly alias for quoted-reply attribution. Also acts as the exact sender allowlist only when group_context_include_peer_bots is explicitly enabled; this table alone never enables sharing."
			option.DescriptionZH = "将每个对端机器人 app_id 映射为友好别名，用于引用回复归因。仅显式开启 group_context_include_peer_bots 时兼作发送者白名单；单独配置此表不会开启共享。"
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
			option.Default = strconv.FormatBool(defaultShareSessionInChannel)
			option.Description = "Share one Agent session among users in the same non-isolated group by default. Explicit false keeps per-user group sessions; isolated topics remain shared within each topic. Omission preserves ordinary private-chat keys; explicit true retains the legacy channel-wide key. Existing histories are not merged when this setting changes."
			option.DescriptionZH = "默认让同一非隔离群聊内的用户共享一个 Agent 会话。显式 false 保留按用户的群会话；隔离话题仍在各自话题内共享。省略该键时普通私聊保留原会话键；显式 true 延用历史频道级会话键。切换设置不会自动合并已有历史。"
			option.Example = "share_session_in_channel = true"
			option.PresetValues = []core.ConfigPresetValue{{Preset: "starter/recommended-feishu", Value: "true", Description: "Share ordinary group conversations across users.", DescriptionZH: "普通群聊中的不同用户共享会话。"}}
		case "thread_isolation":
			option.Type = "string | boolean (legacy)"
			option.Default = defaultThreadIsolation.String()
			option.Values = []string{"off", "topics_only", "topic_per_message"}
			option.Description = "Choose Feishu/Lark topic isolation scope. off uses the configured per-user/group session. topics_only isolates every real topic whose event carries thread_id, including P2P topics; ordinary group messages stay in the main chat. topic_per_message additionally gives every top-level group message its own topic/session. Real topics get an independent Agent session and workspace binding in both enabled modes. Omitting the key now maps to topics_only, including existing configs; legacy true maps to topic_per_message and false maps to off. New Starter and recommended profiles write topics_only. Existing histories are retained, not merged."
			option.DescriptionZH = "选择飞书/Lark 话题隔离范围。off 使用配置指定的用户/群会话；topics_only 隔离事件携带 thread_id 的所有真实话题（包括 P2P 私聊话题），普通群消息留在群主会话；topic_per_message 还会让每条群主会话消息拥有独立话题/session。两种启用模式都会给真实话题独立 Agent 会话和工作区绑定。省略该键现在映射 topics_only，也适用于已有配置；旧 true 映射 topic_per_message，旧 false 映射 off；新 Starter 和推荐 Profile 写入 topics_only。已有历史保留，不会自动合并。"
			option.Example = `thread_isolation = "topics_only"`
			option.Keywords = []string{"topic isolation", "multiple topics", "话题隔离", "话题独立", "多个话题"}
			option.PresetValues = []core.ConfigPresetValue{{Preset: "starter/recommended-feishu", Value: "topics_only", Description: "Isolate real topics without promoting ordinary group messages.", DescriptionZH: "隔离真实话题，不把普通群消息提升为话题。"}}
		}
		if option.DefaultSource == core.ConfigDefaultAdapter && option.Default != "unset" {
			option.DefaultSource = core.ConfigDefaultBuiltin
		}
	}
	options = core.ConfigureOptionExample(options, "group_reply_all_chats", `group_reply_all_chats = "oc_chat_a,oc_chat_b"`)
	options = core.ConfigureOptionExample(options, "completion_notify", "completion_notify = true")
	options = core.ConfigureOptionExample(options, "completion_notify_users", `completion_notify_users = ["ou_your_open_id"]`)
	options = core.ConfigureOptionExample(options, "unread_reminder", "unread_reminder = true")
	options = core.ConfigureOptionExample(options, "unread_reminder_users", `unread_reminder_users = ["ou_your_open_id"]`)
	options = core.ConfigureOptionExample(options, "unread_reminder_delay_mins", "unread_reminder_delay_mins = 60")
	options = core.ConfigureOptionExample(options, "mention_map", `mention_map = { Reviewer-Bot = "ou_bot_open_id" }`)
	options = core.ConfigureOptionExample(options, "peer_bots", `peer_bots = { cli_peer_app_id = "Reviewer-Bot" }`)
	options = core.ConfigureOptionExample(options, "group_context", "group_context = true")
	options = core.ConfigureOptionExample(options, "group_context_include_peer_bots", "group_context_include_peer_bots = true")
	options = core.ConfigureOptionExample(options, "group_context_max_messages", "group_context_max_messages = 20")
	options = core.ConfigureOptionExample(options, "group_context_window_minutes", "group_context_window_minutes = 30")
	options = core.ConfigureOptionExample(options, "group_context_max_chars", "group_context_max_chars = 8000")
	return options
}
