# Feishu Plus

CC Connect Feishu Plus keeps the native `feishu` and `lark` platform adapters.
Enhancements are compiled into the compatible distribution and stay behind
explicit platform options so an existing configuration retains native behavior.

## Foundation feature: recovering fail-closed identity

The native adapter needs the bot's own `open_id` to decide whether a group
message mentions the bot. The upstream behavior continues after a lookup
failure with mention filtering disabled, which admits unrelated group traffic
until the daemon is restarted.

Enable the Plus behavior in the existing platform options:

```toml
[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "your-feishu-app-id"
app_secret = "your-feishu-app-secret"
plus_enabled = true
plus_identity_mode = "retry"
```

Available identity modes:

| Mode | Behavior when bot identity is unavailable |
| --- | --- |
| `retry` | Default Plus behavior. Block protected group traffic, keep direct messages available, retry with capped exponential backoff, and self-recover. |
| `fail_closed` | Block protected group traffic until the process restarts; direct messages remain available. |
| `legacy` | Preserve the upstream fail-open behavior. Intended only for compatibility. |

When `plus_enabled` is absent or false, the adapter follows the native behavior.
The identity protection applies to WebSocket mode. Existing private/self-hosted
webhook behavior is unchanged because those deployments may not expose the same
bot-info API.

## Compatibility rules

- Never open a second connection for the same Feishu application.
- Keep the existing config and session data directory.
- Add one independently testable feature flag at a time.
- Default native behavior must remain unchanged when Plus is disabled.
- Every deep Feishu change needs a regression test and a documented rollback.
