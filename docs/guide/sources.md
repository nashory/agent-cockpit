# Source Reference

agent-cockpit reads supported agents from local files and normalizes them into
usage events. It does not call provider APIs for reports.

| Source | Default paths | Credentials | Network |
| --- | --- | --- | --- |
| Claude Code | `~/.claude/projects/**/*.jsonl` | Not read | No |
| Codex | `~/.codex/sessions/**/*.jsonl`, `~/.codex/archived_sessions/**/*.jsonl` | Not read | No |
| Gemini | `~/.gemini/tmp/**/chats/session-*.json` | Not read | No |
| OpenCode | `~/.local/share/opencode/opencode*.db`, `~/.local/share/opencode/storage/message/*.json`, `~/.opencode/opencode*.db` | Not read | No |
| Amp | `~/.local/share/amp/threads/**/*.json` | Not read | No |
| GitHub Copilot CLI | `~/.copilot/otel/**/*.jsonl`, `COPILOT_OTEL_FILE_EXPORTER_PATH` | Not read | No |
| Kimi | `~/.kimi/sessions/*/*/wire.jsonl` | Not read | No |
| Qwen Code | `~/.qwen/projects/*/chats/*.jsonl` | Not read | No |
| Codebuff | `~/.config/manicode*/projects/*/chats/*/chat-messages.json` | Not read | No |
| Kilo Code | `~/.local/share/kilo/kilo.db` | Not read | No |

Check what exists on your machine:

```bash
cockpit doctor
```

Limit or disable sources through config:

```toml
[paths]
claude = ["~/.claude/projects"]
codex = []
gemini = []
opencode = []
amp = []
copilot = []
kimi = []
qwen = []
codebuff = []
kilo = []
```

Extracted fields are usage metadata only: source, session id, project or working
directory, model, token counts, and timestamp. Reports do not expose raw prompt
or assistant text.

For parser details and source-specific examples, see
[Source Adapters](../sources.md).
