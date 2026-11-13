# Custom card hooks

A card button can invoke a named command or HTTP handler configured by the operator. The Feishu adapter authenticates the callback and checks project, sender, chat and explicit user bindings. The hook receives the button action, original card route, opaque data and submitted form values.

## Configure a handler

The following command reads one event JSON object from stdin.

```toml
[[hooks]]
name = "reply_confirmation"
project = "example"
event = "card.action"
type = "command"
command = "python3 /path/to/handle_reply.py"
input = "json"
timeout = 10
```

Names are case-sensitive and contain up to 64 characters, starting with a letter, followed by letters, digits, underscores, dots or hyphens. Names are unique within a project. A card action requires an explicit project and handler name. Command handlers use `input = "json"`. HTTP handlers use the existing JSON POST transport.

Hook commands and URLs are selected from local configuration. Card values and form contents are passed as data. Configure a different command or URL to change the action without rebuilding cc-connect-next. Configuration takes effect after restart.

## Bind a button

Place this value on a Feishu button with `name = "confirmed"`. Use the actual project and intended operator bindings for the card.

```json
{
  "action": "hook:reply_confirmation",
  "cc_project": "example",
  "cc_direct_user": "ou_owner",
  "cc_card_schema": "2.0",
  "data": {
    "request_id": "opaque-business-record",
    "choice": "text"
  }
}
```

The button name becomes the action name. Ordinary buttons and form-submit buttons use the same route. A form-submit button must use Feishu's form-submit behavior so the callback includes its final form values. Each action may supply its own opaque `data` object. Group cards may omit `cc_direct_user` and use the existing sender/chat admission rules. Private cards outside a group allowlist require the explicit user binding.

Only explicit named handlers receive these actions. Lifecycle wildcard hooks receive their existing lifecycle events. A missing or invalid handler produces an error toast. Accepted dispatch produces a localized processing toast. The handler owns subsequent business results and card updates.

## Event data

Command stdin and HTTP POST carry the same envelope.

```json
{
  "event": "card.action",
  "timestamp": "2026-11-13T03:00:00Z",
  "project": "example",
  "platform": "feishu",
  "user_id": "ou_owner",
  "session_key": "feishu:oc_card",
  "extra": {
    "hook": "reply_confirmation",
    "action": "confirmed",
    "message_id": "om_card",
    "chat_id": "oc_card",
    "data": {"request_id": "opaque-business-record", "choice": "text"},
    "form_values": {"reply_text": "The user's final edited text"}
  }
}
```

Metadata comes from the admitted platform callback. The callback's extra data is limited to 64 KiB of serialized JSON. The handler should resolve its business record using the opaque ID and verify the expected operator, card and allowed operation before performing side effects.

Card hooks execute asynchronously even when `async = false`, preserving the platform callback response deadline. The processing toast acknowledges dispatch. Completion, failure feedback, deduplication and recovery belong to the handler. Delivery is best-effort and callbacks may recur.

Existing navigation, command and form-receipt cards retain their callback behavior. Integrations migrating existing cards should retain their current receipt/recovery path while issuing new cards with explicit named bindings.
