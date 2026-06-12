# Privacy

agent-cockpit is designed for local usage accounting. Reports, the TUI, and the
localhost API read files already present on your machine and do not upload
usage, prompts, completions, file names, or credentials.

## Network Behavior

Most commands do not use the network:

```bash
cockpit
cockpit today
cockpit trends --json
cockpit statusline --compact
cockpit serve --addr 127.0.0.1:8765
```

The explicit maintenance command below fetches LiteLLM's public pricing table
to compare it with the vendored snapshot:

```bash
cockpit pricing update --check
```

Release engineering can also run:

```bash
make pricing
```

That script downloads the same public pricing table and rewrites the vendored
pricing JSON files. Ordinary reports never fetch pricing at runtime.

## Local Files Read

Default source paths:

| Source | Default paths |
| --- | --- |
| Claude Code | `~/.claude/projects/**/*.jsonl` |
| Codex | `~/.codex/sessions/**/*.jsonl`, `~/.codex/archived_sessions/**/*.jsonl` |
| Gemini | `~/.gemini/tmp/**/chats/session-*.json` |
| OpenCode | `~/.local/share/opencode/opencode*.db`, `~/.local/share/opencode/storage/message/*.json`, `~/.opencode/opencode*.db` |
| Amp | `~/.local/share/amp/threads/**/*.json` |
| GitHub Copilot CLI | `~/.copilot/otel/**/*.jsonl`, plus `COPILOT_OTEL_FILE_EXPORTER_PATH` |
| Kimi | `~/.kimi/sessions/*/*/wire.jsonl` |
| Qwen Code | `~/.qwen/projects/*/chats/*.jsonl` |
| Codebuff | `~/.config/manicode*/projects/*/chats/*/chat-messages.json` |
| Kilo Code | `~/.local/share/kilo/kilo.db` |
| Goose | `~/.local/share/goose/sessions/sessions.db`, `~/Library/Application Support/goose/sessions/sessions.db`, `~/.local/share/Block/goose/sessions/sessions.db` |

Use config paths to narrow scanning to specific directories:

```toml
[paths]
claude = ["~/work/.claude/projects"]
codex = []
gemini = []
opencode = []
amp = []
copilot = []
kimi = []
qwen = []
codebuff = []
kilo = []
goose = []
```

## Data Extracted

Adapters extract usage metadata:

- source name
- session id
- project or working directory
- model name
- input, output, cache, and reasoning token counts
- timestamp

agent-cockpit does not expose raw prompts or assistant text in reports or
localhost API responses. Source files can contain more data than agent-cockpit
uses; see [Source Adapters](sources.md) for source-specific parsing notes.

## Localhost API

`cockpit serve` binds to `127.0.0.1:8765` by default:

```bash
cockpit serve --addr 127.0.0.1:8765
```

Keep the bind address on loopback unless you intentionally want another local
network process to read usage aggregates. The API returns normalized summaries,
daily rows, blocks, sessions, and statusline payloads. It does not include raw
message text.

## Cost Estimates

Cost is estimated from token counts and local pricing data. Use `--no-cost` to
omit estimated cost fields:

```bash
cockpit today --json --no-cost
cockpit export --group daily --no-cost
cockpit serve --no-cost
```

## Files Written

Configuration and UI preferences are stored under the agent-cockpit config
directory:

| OS | Path |
| --- | --- |
| macOS / Linux | `~/.config/agent-cockpit/` |
| Windows | `%APPDATA%\agent-cockpit\` |

`cockpit config init` writes `config.toml`. The TUI may write `ui.json` to
remember local display preferences.
