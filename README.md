<div align="center">

# agent-cockpit

**A live terminal cockpit for usage, cost, and speed across your coding agents.**

Track Claude Code, Codex, Gemini, OpenCode, Copilot, and more from one local
TUI. See token burn, USD estimates, observed throughput, and 30-day trends
without uploading your logs.

</div>

> Early MVP: Claude Code and Codex local log parsing are implemented first.
> More agent adapters and live speed dashboards are planned.

## Install

Homebrew distribution is planned as the default install path:

```bash
brew install nashory/tap/agent-cockpit
ac
```

Until the first release is cut, install from source:

```bash
go install github.com/nashory/agent-cockpit/cmd/ac@latest
```

Or build from source:

```bash
git clone https://github.com/nashory/agent-cockpit.git
cd agent-cockpit
make build
./ac
```

## Quick Start

Open the TUI:

```bash
ac
```

Open the live TUI with periodic refresh:

```bash
ac live --refresh 2s
```

Print a static report:

```bash
ac today
ac trends --days 30
ac agents
ac sessions
ac speed
```

Filter by source, project, or model:

```bash
ac monthly --source claude
ac trends --source claude,codex --project agx --days 30
ac agents --model sonnet
```

Use it in tmux or a statusline:

```bash
ac statusline
```

Create a local config file:

```bash
ac config init
ac config path
```

## What It Reads

agent-cockpit reads local logs only:

| Agent | Default path |
| --- | --- |
| Claude Code | `~/.claude/projects/**/*.jsonl` |
| Codex | `~/.codex/sessions/**/*.jsonl`, `~/.codex/archived_sessions/**/*.jsonl` |

No cloud upload, no background service, no API keys.

## Dashboards

| View | Shows |
| --- | --- |
| Overview | token totals, estimated cost, top agents, top models |
| Agents | cross-agent usage share |
| Models | model-level usage and cost |
| Trends | 30-day token and cost trend |
| Speed | observed output tokens/sec by agent and model |

## Native Platforms

agent-cockpit ships as a CGO-free native binary for:

| OS | Architectures |
| --- | --- |
| macOS | Apple Silicon, Intel |
| Linux | amd64, arm64 |
| Windows | amd64, arm64 |

The command name is intentionally short:

```text
repo:    agent-cockpit
binary:  ac
```

## Configuration

Default config paths:

| OS | Path |
| --- | --- |
| macOS/Linux | `~/.config/agent-cockpit/config.toml` |
| Windows | `%APPDATA%\agent-cockpit\config.toml` |

Example:

```toml
timezone = "local"
refresh_interval = "3s"
currency = "USD"

[paths]
claude = ["~/.claude/projects"]
codex = ["~/.codex/sessions", "~/.codex/archived_sessions"]

[pricing."claude-sonnet"]
input_per_million = 3
output_per_million = 15
cache_read_per_million = 0.30
cache_write_per_million = 3.75
```

## Development

```bash
make test
make build
make run
```

See [docs/development.md](docs/development.md) for native macOS, Linux, and
Windows build/release details.

Project layout:

```text
cmd/ac/                     main entry
internal/source/claude/     Claude Code JSONL adapter
internal/source/codex/      Codex JSONL adapter
internal/usage/             normalized events and aggregation
internal/report/            static terminal reports
internal/tui/               Bubble Tea TUI
```

## Roadmap

- Live file watching and active session detection
- Observed throughput dashboards
- Gemini, OpenCode, and Copilot adapters
- Configurable pricing
- Homebrew distribution
- Windows zip releases
