<div align="center">

# agent-cockpit

**A live terminal cockpit for usage, cost, and speed across your coding agents.**

Track Claude Code, Codex, Gemini, OpenCode, Copilot, and more from one local
TUI. See token burn, USD estimates, observed throughput, and 30-day trends
without uploading your logs.

</div>

> Early MVP: Claude Code, Codex, and Gemini local log parsing are implemented first.
> More agent adapters (OpenCode, Copilot) are planned.

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
| Gemini | `~/.gemini/tmp/**/chats/session-*.json` |

No cloud upload, no background service, no API keys.

## Dashboards

The TUI is a glass-cockpit dashboard with eight instrument tabs:

| View | Shows |
| --- | --- |
| Overview | headline token/cost readouts, per-agent bars, 30-day trend |
| Agents | per-engine clusters: tokens, cost, share, speed, activity |
| Models | model-level load, cost, and share |
| Trends | 30-day token and cost time-series |
| Speed | airspeed tape and observed output tokens/sec per lane |
| Insights | cache efficiency, economics, and spending cadence |
| Activity | hour-of-day heatmap, day-of-week, and top projects |
| Calendar | GitHub-style year contribution graph of tokens/day |

### Keys

| Key | Action |
| --- | --- |
| `1`–`8` | jump to a tab |
| `tab` / `shift+tab` | next / previous tab |
| arrows / `hjkl` | move widget focus (the focus bar shows the selection) |
| `enter` | zoom the focused widget fullscreen |
| `esc` | exit zoom |
| `e` | toggle expert (dense) / compact (light) mode |
| `r` | refresh now |
| `q` | quit |

On the Calendar tab, zoom in (`enter`) and then arrows / `hjkl` move the day
cursor (left/right by week, up/down by day) with a tooltip for the selected day.

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
gemini = ["~/.gemini/tmp"]

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

Contribution guide:

- [CONTRIBUTING.md](CONTRIBUTING.md)
- [SECURITY.md](SECURITY.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/homebrew.md](docs/homebrew.md)
- [docs/sources.md](docs/sources.md)

Project layout:

```text
cmd/ac/                     main entry
internal/source/claude/     Claude Code JSONL adapter
internal/source/codex/      Codex JSONL adapter
internal/source/gemini/     Gemini session adapter
internal/scan/              parallel directory walk + file parsing
internal/usage/             normalized events, aggregation, derived insights
internal/report/            static terminal reports
internal/tui/               Bubble Tea glass-cockpit TUI
```

## Roadmap

- Live file watching and active session detection
- OpenCode and Copilot adapters
- Homebrew distribution
- Windows zip releases
