# 未读提醒已改为即时私聊完成通知

原会话中的额外完成通知及延迟未读提醒已移除。现在任务成功、答复送达后，由同一机器人立即私聊发起人一次，并附原对话链接。

配置、旧字段兼容、持久去重和验证边界见[私聊完成通知](completion-notifications.md)。旧 `unread_reminder_delay_mins` 已忽略；历史待提醒队列不会补发。

# Unread reminders replaced by immediate private completion notifications

The same bot now privately notifies the turn owner immediately after a successful final answer, with a link to the original conversation. No extra topic notice, read-status checks or delayed reminders remain.

See [private completion notifications](completion-notifications.md) for configuration, legacy compatibility and verification boundaries. The old delay option is ignored; historical pending reminders are never replayed.
