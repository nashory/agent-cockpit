# Integrations

`agent-cockpit` exposes two integration surfaces for bars, prompts, and local
automation: `cockpit statusline` for one-shot rendering and `cockpit serve` for
localhost JSON.

## Statusline

```bash
cockpit statusline --compact
cockpit statusline --json
cockpit statusline --format '{{model}} {{context}} {{today_cost}}'
```

Useful placeholders for `--format`:

| Placeholder | Value |
| --- | --- |
| `{{model}}` | Active Claude Code model display name, when stdin context is available |
| `{{context}}` | Active Claude Code context usage percentage |
| `{{context_left}}` | Active Claude Code context remaining percentage |
| `{{tokens_compact}}` | Total tokens in compact form |
| `{{today_cost}}` | Estimated cost for today's filtered events |
| `{{block_left}}` | Remaining configured Claude 5h token budget |

Claude Code can run `cockpit statusline` directly as a `statusLine` command. If
Claude Code sends statusLine JSON on stdin, cockpit uses the active model,
session, and context-window data. If stdin is empty, cockpit falls back to local
logs.

## Localhost API

```bash
cockpit serve --addr 127.0.0.1:8765
```

Endpoints:

| Endpoint | Description |
| --- | --- |
| `GET /health` | Lightweight health check |
| `GET /api/summary` | Summary JSON envelope |
| `GET /api/daily` | Daily trend JSON envelope |
| `GET /api/blocks` | Claude-style activity block JSON envelope |
| `GET /api/sessions` | Session aggregate JSON envelope |
| `GET /api/statusline` | Statusline JSON payload |

The server binds `127.0.0.1` by default. It reads the same local logs and config
as the CLI, and does not upload data or call remote APIs. Endpoint responses
include normalized usage metadata only; they do not include raw prompts or
assistant text.

## tmux

One-shot prompt/status segment:

```tmux
set -g status-right "#(cockpit statusline --compact)"
```

With the localhost API:

```tmux
set -g status-right "#(curl -fsS http://127.0.0.1:8765/api/statusline | jq -r '\"tok \" + (.totals.total_tokens|tostring)')"
```

## Waybar

Start the server in your session manager:

```bash
cockpit serve --addr 127.0.0.1:8765
```

Waybar custom module:

```json
{
  "custom/agent-cockpit": {
    "exec": "curl -fsS http://127.0.0.1:8765/api/statusline | jq -r '\"tok \" + (.totals.total_tokens|tostring)'",
    "interval": 10,
    "return-type": "text"
  }
}
```

Use `--no-cost` on the `cockpit serve` command when cost fields should be
omitted from API responses.
