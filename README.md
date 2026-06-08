<div align="center">

# 🛩️ agent-cockpit

[![CI](https://github.com/nashory/agent-cockpit/actions/workflows/ci.yml/badge.svg)](https://github.com/nashory/agent-cockpit/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nashory/agent-cockpit)](https://goreportcard.com/report/github.com/nashory/agent-cockpit)
[![Go Version](https://img.shields.io/github/go-mod/go-version/nashory/agent-cockpit)](go.mod)
[![License](https://img.shields.io/github/license/nashory/agent-cockpit)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-555)](#-platforms)

**A live terminal cockpit for token usage, cost, and speed across your coding agents.**

Claude Code, Codex, and Gemini burn tokens all day — `agent-cockpit` reads their
**local** logs and turns them into a glass-cockpit dashboard: token burn, USD
estimates, observed throughput, trends, and a GitHub-style year of activity.
No cloud upload. No API keys. No background daemon.

</div>

<!--
  TODO(demo): drop a real terminal screenshot or GIF here, e.g.
  <p align="center"><img src="docs/demo.gif" alt="agent-cockpit demo" width="800"></p>
  The ASCII preview below is the fallback until then.
-->

```text
FOCUS  TREND · ENGINES · MODELS · ACTIVITY    ↑↓ select · enter zoom

╔════════════════════════════════════════════════════════════════════════════════════╗
║ ✈ AGENT COCKPIT                                                                    ║
║ TOKENS   COST        EVENTS   CACHE   CLOCK                                        ║
║ 14.4M    73.66 USD   446      1.8M    14:32:09                                     ║
╚════════════════════════════════════════════════════════════════════════════════════╝
╭───────────────────────────────────────╮ ╭───────────────────────────────────────╮
│ ◈ ENGINES                             │ │ ◈ TREND · 30d tokens                  │
│ █████████████████████████████████████ │ │ 54850│⢣    ⢰⡀                         │
│ ████████████████████▊                 │ │      │ ⢣   ⡇⢱   ⢸⢆   ⢀⢦    ⡀          │
│ ▏                                     │ │ 36567│  ⢣ ⢰⠁⠘⡄  ⡇ ⠣⡀ ⡸ ⠱⡀ ⢠⠛⢄   ⡰⢄    │
│ ● codex    63.9%  9.2M                │ │      │   ⠱⡎  ⡇⡸⣤⠃  ⠑⣄⠇  ⢇ ⡜  ⠑⢄⢠⠃ ⠑⢄  │
│ ● claude   35.8%  5.2M                │ │ 18283│       ⢇⡇⠈    ⠈   ⢸⢠⠃   ⠈⠃    ⠑ │
│ ● gemini    0.3%  44.8K               │ │      │       ⢸⠁         ⠈⡞            │
╰───────────────────────────────────────╯ │     0└─────────────────────────────── │
                                           │      '26 05/10   05/21   05/28        │
                                           ╰───────────────────────────────────────╯
╭───────────────────────────────────────╮ ╭───────────────────────────────────────╮
│ ◈ MODELS                              │ │ ◈ ACTIVITY                            │
│ gpt-5-codex    █████████████     9.2M │ │ claude               █▇ ▅█▇▆▆▅▇▇▆▅▅   │
│ claude-opus-4… ███████░░░░░░     5.2M │ │ codex                ▇▅ ▂█▆▅▃▂▇▅▄▃▂   │
│ gemini-2.5-pro ░░░░░░░░░░░░░    44.8K │ │ gemini                █  █  █  █  █   │
╰───────────────────────────────────────╯ ╰───────────────────────────────────────╯
```

<sub>Colors render in your terminal; the snapshot above is monochrome.</sub>

---

## Contents

[Why](#-why-agent-cockpit) · [Features](#-features) · [Install](#-install) ·
[Quick start](#-quick-start) · [Dashboards](#-dashboards) · [Privacy](#-what-it-reads) ·
[Configuration](#-configuration) · [How it works](#-how-it-works) ·
[Platforms](#-platforms) · [Development](#-development) · [License](#-license)

## ✨ Why agent-cockpit?

- **🔒 Private by design.** It only reads log files already on your disk. Nothing
  is uploaded, no service runs in the background, no keys are required.
- **🛩️ One cockpit for every agent.** Claude Code, Codex, and Gemini in a single
  normalized view — compare engines, models, and projects side by side.
- **💸 Know the cost.** Per-model pricing turns raw tokens into USD estimates,
  cache savings, effective `$/1M output`, and a daily burn rate.
- **⚡ Live.** `ac live` refreshes the instant an agent writes a log, via fsnotify
  (with a polling backstop).
- **🧰 Zero setup.** Sensible defaults discover your logs automatically; a config
  file is optional.

## 🎛️ Features

- **8 instrument tabs** — Overview, Agents, Models, Trends, Speed, Insights,
  Activity, and a GitHub-style year **Calendar**.
- **Derived insights** — cache hit rate, cache savings, input:output ratio,
  reasoning share, effective rate, spending cadence, and activity streaks.
- **Focus & zoom** — arrow-key focus across widgets, `enter` to blow one up
  fullscreen for detail.
- **Expert / compact modes** — pack every instrument in, or a clean, light view.
- **Caution annunciators** — `HIGH BURN`, `OPUS HEAVY`, `STALE`, `LIVE` lamps.
- **Static reports & statusline** — pipeable output for scripts, tmux, and CI.
- **Fast** — a cold scan of hundreds of logs parses in parallel across cores.
- **Single binary** — CGO-free, cross-platform, ~6 MB. The command is just `ac`.

## 🚀 Install

> **Homebrew** is the planned default once the first release is tagged:
>
> ```bash
> brew install nashory/tap/agent-cockpit
> ```

**With Go:**

```bash
go install github.com/nashory/agent-cockpit/cmd/ac@latest
```

**From source:**

```bash
git clone https://github.com/nashory/agent-cockpit.git
cd agent-cockpit
make build && ./ac
```

## ⚡ Quick Start

```bash
ac                       # open the dashboard
ac live --refresh 2s     # live mode (refreshes on file changes)

# static, pipeable reports
ac today
ac trends --days 30
ac agents
ac speed
ac statusline            # one line for tmux / your shell prompt

# filter by source, project, or model
ac monthly --source claude
ac trends  --source claude,codex --project myrepo --days 30
ac agents  --model sonnet

# JSON for scripts
ac today --json

# config helpers
ac config init
ac doctor                # show detected log locations
```

## 🧭 Dashboards

Four tabs, each packed with instruments — press `enter` to zoom any widget
fullscreen for detail:

| Tab | Shows |
| --- | --- |
| **Overview** | headline token/cost readouts, per-agent bars, 30-day trend |
| **Breakdown** | engine share, model load, and output speed per lane |
| **Trends** | token/cost time-series plus efficiency, economics, and cadence |
| **Activity** | year contribution calendar, hour-of-day, day-of-week, projects |

### Keys

| Key | Action |
| --- | --- |
| `1`–`4` | jump to a tab |
| `tab` / `shift+tab` | next / previous tab |
| arrows / `hjkl` | move widget focus |
| `enter` / `esc` | zoom the focused widget / exit zoom |
| `e` | toggle expert (dense) / compact (light) mode |
| `r` | refresh now |
| `q` | quit |

On **Activity**, zoom the calendar (`enter`) and then arrows move the day cursor
(left/right by week, up/down by day) with a tooltip for the selected day.

## 🔒 What It Reads

`agent-cockpit` reads **local log files only** — no network calls, ever.

| Agent | Default path |
| --- | --- |
| Claude Code | `~/.claude/projects/**/*.jsonl` |
| Codex | `~/.codex/sessions/**/*.jsonl`, `~/.codex/archived_sessions/**/*.jsonl` |
| Gemini | `~/.gemini/tmp/**/chats/session-*.json` |

## ⚙️ Configuration

Configuration is optional. To customize paths or pricing, create a config file:

| OS | Path |
| --- | --- |
| macOS / Linux | `~/.config/agent-cockpit/config.toml` |
| Windows | `%APPDATA%\agent-cockpit\config.toml` |

```toml
timezone = "local"
refresh_interval = "3s"
currency = "USD"

[paths]
claude = ["~/.claude/projects"]
codex  = ["~/.codex/sessions", "~/.codex/archived_sessions"]
gemini = ["~/.gemini/tmp"]

# Prices are USD per million tokens; keys match a model-name substring.
[pricing."claude-sonnet"]
input_per_million       = 3
output_per_million      = 15
cache_read_per_million  = 0.30
cache_write_per_million = 3.75
```

Run `ac config init` to drop a starter file in place.

## 🏗️ How It Works

Each adapter parses its agent's logs into a normalized `usage.Event`; events are
aggregated, priced, and rendered. Scanning runs in parallel across CPUs, live
mode is driven by an fsnotify watcher, and the TUI is built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea).

```text
cmd/ac/                 entry point (the `ac` binary)
internal/source/        Claude / Codex / Gemini log adapters
internal/scan/          parallel directory walk + file parsing
internal/watch/         fsnotify watcher for live refresh
internal/usage/         normalized events, pricing, aggregation, insights
internal/report/        static terminal reports
internal/tui/           Bubble Tea glass-cockpit dashboard
```

See [docs/architecture.md](docs/architecture.md) for the full design.

## 🖥️ Platforms

Ships as a CGO-free native binary:

| OS | Architectures |
| --- | --- |
| macOS | Apple Silicon, Intel |
| Linux | amd64, arm64 |
| Windows | amd64, arm64 |

## 🛠️ Development

```bash
make ci        # gofmt + go vet + full test suite (mirrors GitHub CI)
make test      # go test ./...
make race      # go test -race ./...
make build     # build ./ac
make run       # go run ./cmd/ac
```

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md),
[SECURITY.md](SECURITY.md), and [docs/](docs/).

## 📄 License

[Apache-2.0](LICENSE) © agent-cockpit contributors.

## 🙌 Acknowledgements

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss),
[ntcharts](https://github.com/NimbleMarkets/ntcharts),
[Cobra](https://github.com/spf13/cobra), and
[fsnotify](https://github.com/fsnotify/fsnotify).
