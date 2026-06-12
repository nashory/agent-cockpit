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

## Amp

Default path:

```text
~/.local/share/amp/threads/**/*.json
```

`AMP_DATA_DIR` can override the root with one path or a comma-separated list of
paths. The source reads local thread JSON files only; it does not call Amp or
provider APIs.

Amp thread files include an `id`, `messages`, and sometimes `usageLedger`:

```json
{
  "id": "T-...",
  "usageLedger": {
    "events": [
      {
        "timestamp": "2026-01-02T00:00:00.000Z",
        "model": "gpt-5",
        "toMessageId": 123,
        "tokens": {"input": 0, "output": 0, "total": 0}
      }
    ]
  },
  "messages": [
    {
      "role": "assistant",
      "messageId": 123,
      "usage": {
        "model": "...",
        "timestamp": "...",
        "inputTokens": 0,
        "outputTokens": 0,
        "cacheCreationInputTokens": 0,
        "cacheReadInputTokens": 0,
        "totalTokens": 0
      }
    }
  ]
}
```

`usageLedger.events[]` is preferred when present. `messages[].usage` is used as
the current-schema fallback. Ledger events use `messages[].usage` to recover
cache creation/read counts for the matching `toMessageId`.

## GitHub Copilot CLI

Default path:

```text
~/.copilot/otel/**/*.jsonl
```

`COPILOT_OTEL_FILE_EXPORTER_PATH` can add one explicit OpenTelemetry JSONL
export file. Copilot CLI does not provide a stable session-log directory like
Claude Code or Codex; reliable local usage requires OpenTelemetry file export to
be enabled before starting or resuming Copilot sessions. The source reads local
JSONL files only; it does not call GitHub APIs or read credentials.

Copilot OpenTelemetry records include spans and logs with `attributes`:

```json
{
  "type": "span",
  "traceId": "...",
  "spanId": "...",
  "name": "chat ...",
  "endTime": [1775934264, 967317833],
  "attributes": {
    "gen_ai.operation.name": "chat",
    "gen_ai.response.model": "...",
    "gen_ai.conversation.id": "...",
    "gen_ai.usage.input_tokens": 0,
    "gen_ai.usage.output_tokens": 0,
    "gen_ai.usage.cache_read.input_tokens": 0,
    "gen_ai.usage.cache_creation.input_tokens": 0,
    "gen_ai.usage.reasoning.output_tokens": 0
  }
}
```

Token mapping:

- `input_tokens` minus cache-read tokens -> input tokens
- `output_tokens` plus reasoning tokens -> output tokens
- `cache_read.input_tokens` -> cache-read input tokens
- `cache_creation.input_tokens` or `cache_write.input_tokens` -> cache-creation input tokens
- `reasoning.output_tokens` -> reasoning tokens, surfaced as a subset of output

When multiple Copilot OpenTelemetry records describe the same response,
agent-cockpit keeps the highest-priority record in this order: chat span,
inference log, agent turn log, agent summary span.

## Kimi

Default path:

```text
~/.kimi/sessions/*/*/wire.jsonl
```

`KIMI_DATA_DIR` can override the root with one path or a comma-separated list of
paths. The source reads local wire JSONL files only; it does not call Kimi APIs
or read credentials.

Kimi wire files include `StatusUpdate` messages with `payload.token_usage`:

```json
{
  "timestamp": 1770983427.123,
  "message": {
    "type": "StatusUpdate",
    "payload": {
      "message_id": "msg-1",
      "token_usage": {
        "input_other": 0,
        "output": 0,
        "input_cache_read": 0,
        "input_cache_creation": 0,
        "total": 0
      }
    }
  }
}
```

Token mapping:

- `input_other` -> input tokens
- `output` -> output tokens
- `input_cache_read` -> cache-read input tokens
- `input_cache_creation` -> cache-creation input tokens
- `total` fills missing output tokens when provider-specific parts are absent

The model name is read from `~/.kimi/config.json` when present, otherwise it
falls back to `kimi-for-coding`.

## Qwen Code

Default path:

```text
~/.qwen/projects/*/chats/*.jsonl
```

`QWEN_DATA_DIR` can override the root with one path or a comma-separated list of
paths. The source reads local chat JSONL files only; it does not call Qwen APIs
or read credentials.

Qwen chat files include assistant records with `usageMetadata`:

```json
{
  "type": "assistant",
  "model": "qwen3-coder-plus",
  "timestamp": "2026-02-23T14:24:56.857Z",
  "sessionId": "session-json",
  "usageMetadata": {
    "promptTokenCount": 100,
    "candidatesTokenCount": 50,
    "cachedContentTokenCount": 5,
    "thoughtsTokenCount": 10,
    "totalTokenCount": 165
  }
}
```

Token mapping:

- `promptTokenCount` -> input tokens
- `candidatesTokenCount` plus `thoughtsTokenCount` -> output tokens
- `cachedContentTokenCount` -> cache-read input tokens
- `thoughtsTokenCount` -> reasoning tokens, surfaced as a subset of output
- `totalTokenCount` fills missing output tokens when provider-specific parts
  are absent

The project name is derived from the path segment after `projects/`. Missing
session IDs fall back to `<project>-<file_stem>`, and missing model names fall
back to `unknown`.

## Codebuff

Default paths:

```text
~/.config/manicode/projects/*/chats/*/chat-messages.json
~/.config/manicode-dev/projects/*/chats/*/chat-messages.json
~/.config/manicode-staging/projects/*/chats/*/chat-messages.json
```

`CODEBUFF_DATA_DIR` can override the roots with one path or a comma-separated
list. Roots may point at either a channel directory or directly at its
`projects` directory. The source reads local chat JSON files only; it does not
call Codebuff APIs or read credentials.

Codebuff chat files are JSON arrays. Assistant messages can expose usage under
`metadata.usage`, `metadata.codebuff.usage`, or the latest assistant entry in
`metadata.runState.sessionState.mainAgentState.messageHistory`:

```json
{
  "id": "msg-1",
  "variant": "assistant",
  "timestamp": "2026-02-23T14:24:56.857Z",
  "metadata": {
    "model": "claude-sonnet-4-5",
    "usage": {
      "inputTokens": 100,
      "outputTokens": 50,
      "cacheReadInputTokens": 5,
      "cacheCreationInputTokens": 7,
      "totalTokens": 162
    }
  }
}
```

Token mapping:

- `inputTokens`, `input_tokens`, `promptTokens`, or `prompt_tokens` -> input
  tokens
- `outputTokens`, `output_tokens`, `completionTokens`, or `completion_tokens`
  -> output tokens
- `cacheReadInputTokens`, `cache_read_input_tokens`, or cached token details
  -> cache-read input tokens
- `cacheCreationInputTokens`, `cache_creation_input_tokens`,
  `cacheCreationTokens`, `cache_creation_tokens`, `cachedTokensCreated`, or
  `cached_tokens_created` -> cache-creation input tokens
- `totalTokens`, `total_tokens`, or `total` fills missing output tokens when
  provider-specific parts are absent

The project name is derived from the path segment after `projects/`. Session
IDs use `<channel>/<project>/<chat_id>/<message_id>` when the message has an
ID, or the message ordinal otherwise. Missing model names fall back to
`codebuff-unknown`.

## Kilo Code

Default path:

```text
~/.local/share/kilo/kilo.db
```

`KILO_DATA_DIR` can override the root with one path or a comma-separated list of
paths. The source reads local SQLite databases only; it does not call Kilo APIs
or read credentials.

Kilo stores assistant messages in the `message` table as JSON in the `data`
column:

```json
{
  "id": "msg-1",
  "role": "assistant",
  "providerID": "anthropic",
  "modelID": "claude-sonnet-4-20250514",
  "time": {"created": 1767312000000},
  "tokens": {
    "input": 100,
    "output": 50,
    "reasoning": 5,
    "cache": {"read": 10, "write": 20}
  }
}
```

Token mapping:

- `tokens.input` -> input tokens
- `tokens.output` plus `tokens.reasoning` -> output tokens
- `tokens.cache.read` -> cache-read input tokens
- `tokens.cache.write` -> cache-creation input tokens
- `tokens.reasoning` -> reasoning tokens, surfaced as a subset of output
- `tokens.total` fills missing output tokens when provider-specific parts are
  absent

Session IDs use `<row_session_id>/<message_id>` when the embedded message has
an ID, or `<row_session_id>/<row_id>` otherwise. Missing timestamps or models
are skipped because Kilo rows need both to be useful in time-series reports.

## Goose

Default paths:

```text
~/.local/share/goose/sessions/sessions.db
~/Library/Application Support/goose/sessions/sessions.db
~/.local/share/Block/goose/sessions/sessions.db
```

`GOOSE_PATH_ROOT` can override discovery with a Goose root; agent-cockpit then
reads `<root>/data/sessions/sessions.db`. Config paths point at directories
that contain `sessions.db`, or directly at a `sessions.db` file. The source
reads local SQLite databases only; it does not call Goose APIs or read
credentials.

Goose stores session-level token totals in the `sessions` table:

```json
{
  "id": "session-a",
  "model_config_json": {"model_name": "claude-sonnet-4-20250514"},
  "created_at": "2026-05-01 01:02:03",
  "accumulated_total_tokens": 180,
  "accumulated_input_tokens": 100,
  "accumulated_output_tokens": 50
}
```

Token mapping:

- `accumulated_input_tokens`, falling back to `input_tokens` -> input tokens
- `accumulated_output_tokens`, falling back to `output_tokens` -> output tokens
- `accumulated_total_tokens`, falling back to `total_tokens`, fills reasoning
  or otherwise missing output tokens when it exceeds input plus output
- `model_config_json.model_name` -> model name

Session IDs use the Goose session row ID. Missing timestamps, models, or token
totals are skipped because Goose rows are session-level aggregates.
