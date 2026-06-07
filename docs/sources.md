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

