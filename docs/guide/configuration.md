# Configuration

Configuration is optional. Defaults discover supported agent logs in the normal
local paths.

Print the config path:

```bash
cockpit config path
```

Create a starter config:

```bash
cockpit config init
```

Validate it:

```bash
cockpit config validate
cockpit config validate --json
```

Generate JSON Schema for editor integration:

```bash
cockpit config schema > agent-cockpit.schema.json
```

Example config:

```toml
timezone = "local"
refresh_interval = "3s"
currency = "USD"

[budget]
daily_usd = 25
weekly_usd = 100
monthly_usd = 300
warn_pct = 80
critical_pct = 95

[limits]
claude_5h_tokens = 88000
claude_7d_tokens = 500000
warn_pct = 80
critical_pct = 95

[paths]
claude = ["~/.claude/projects"]
codex = ["~/.codex/sessions", "~/.codex/archived_sessions"]
gemini = ["~/.gemini/tmp"]
opencode = ["~/.local/share/opencode", "~/.opencode"]
amp = ["~/.local/share/amp"]
copilot = ["~/.copilot/otel"]
kimi = ["~/.kimi"]
qwen = ["~/.qwen"]
codebuff = ["~/.config/manicode", "~/.config/manicode-dev", "~/.config/manicode-staging"]
kilo = ["~/.local/share/kilo"]

[pricing."claude-sonnet"]
input_per_million = 3
output_per_million = 15
cache_read_per_million = 0.30
cache_write_per_million = 3.75
```

Set a source path to an empty array to disable that source:

```toml
[paths]
copilot = []
```
