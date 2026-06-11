# Source Adapters

agent-cockpit reads local files and normalizes them into usage events. It does
not call provider APIs.

## Claude Code

Default path:

```text
~/.claude/projects/**/*.jsonl
```

Relevant fields:

```json
{
  "type": "assistant",
  "timestamp": "...",
  "sessionId": "...",
  "cwd": "...",
  "message": {
    "model": "...",
    "usage": {
      "input_tokens": 0,
      "output_tokens": 0,
      "cache_creation_input_tokens": 0,
      "cache_read_input_tokens": 0
    }
  }
}
```

## Codex

Default paths:

```text
~/.codex/sessions/**/*.jsonl
~/.codex/archived_sessions/**/*.jsonl
```

Codex sessions include metadata and token count events:

```json
{"type":"session_meta","payload":{"id":"...","cwd":"...","model":"..."}}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{}}}}
```

`last_token_usage` is treated as turn-level usage.

Some Codex logs do not include a model name. agent-cockpit normalizes those
events to model `codex` so they can still be grouped and priced with a local
estimate.

## Gemini

Default path:

```text
~/.gemini/tmp/**/chats/session-*.json
```

Gemini session files include a message array. Assistant messages can include:

```json
{
  "type": "gemini",
  "timestamp": "...",
  "model": "...",
  "tokens": {
    "input": 0,
    "output": 0,
    "cached": 0,
    "thoughts": 0,
    "tool": 0,
    "total": 0
  }
}
```

`tool` tokens are counted as output. `thoughts` tokens are counted as reasoning.

## OpenCode

Default paths:

```text
~/.local/share/opencode/opencode.db
~/.local/share/opencode/opencode-*.db
~/.local/share/opencode/storage/message/*.json
~/.opencode/opencode.db
~/.opencode/opencode-*.db
```

`OPENCODE_DATA_DIR` can override these roots with one path or a comma-separated
list of paths. The source reads local files only; it does not use provider APIs
or OpenCode credentials.

OpenCode database rows store message JSON in `message.data`:

```json
{
  "id": "...",
  "sessionID": "...",
  "providerID": "...",
  "modelID": "...",
  "time": {"created": 1767312000000},
  "tokens": {
    "input": 0,
    "output": 0,
    "total": 0,
    "cache": {"read": 0, "write": 0}
  }
}
```

Token mapping:

- `input` -> input tokens
- `output` -> output tokens
- `cache.read` -> cache-read input tokens
- `cache.write` -> cache-creation input tokens
- `total` fills missing output tokens when provider-specific parts are absent

Legacy OpenCode SQLite databases with aggregate `sessions` rows are read as
session-level events when message JSON is unavailable.
