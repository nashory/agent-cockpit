# Command Reference

## Dashboard

```bash
cockpit
cockpit live --refresh 2s
```

`cockpit` opens the TUI. `live` starts the same dashboard with file watching and
an interval backstop.

## Reports

```bash
cockpit today
cockpit weekly
cockpit monthly
cockpit trends --days 30
cockpit agents
cockpit speed
cockpit sessions
cockpit report
```

Common filters:

```bash
cockpit trends --source claude,codex --project agent-cockpit --since 7d
cockpit monthly --timezone Europe/Zurich --order asc
cockpit report --breakdown project --no-cost
```

## JSON Output

```bash
cockpit today --json
cockpit trends --json
cockpit sessions --json
cockpit report --json
```

Typical JSON report shape:

```json
{
  "schema_version": 1,
  "generated_at": "2026-06-11T00:00:00Z",
  "range": {"since": "...", "until": "...", "timezone": "Europe/Zurich"},
  "filters": {"source": ["claude"]},
  "cost_mode": "estimated",
  "rows": [],
  "totals": {}
}
```

With `--no-cost`, cost fields are omitted and `cost_mode` is `disabled`.

## CSV Export

```bash
cockpit export --group daily > daily.csv
cockpit export --group session --order asc > sessions.csv
cockpit export --group event --source claude --no-cost > events.csv
```

Supported groups are `daily`, `session`, `model`, `project`, and `event`.

## Statusline

```bash
cockpit statusline
cockpit statusline --compact
cockpit statusline --json
cockpit statusline --format '{{model}} {{context}} {{today_cost}}'
```

Typical JSON statusline shape:

```json
{
  "schema_version": 1,
  "generated_at": "2026-06-11T00:00:00Z",
  "cost_mode": "estimated",
  "totals": {},
  "status": {}
}
```

## Localhost API

```bash
cockpit serve --addr 127.0.0.1:8765
curl -fsS http://127.0.0.1:8765/api/statusline
```

Endpoints:

| Endpoint | Description |
| --- | --- |
| `/health` | health check |
| `/api/summary` | summary JSON envelope |
| `/api/daily` | daily trend JSON envelope |
| `/api/blocks` | activity block JSON envelope |
| `/api/sessions` | session aggregate JSON envelope |
| `/api/statusline` | statusline JSON payload |

## Configuration

```bash
cockpit config path
cockpit config init
cockpit config schema
cockpit config validate
cockpit config validate --json
```

## Pricing

```bash
cockpit pricing status
cockpit pricing status --json
cockpit pricing update --check
cockpit pricing update --check --json
```

`pricing status` uses current local logs to show pricing coverage. `pricing
update --check` is the only ordinary command that fetches remote pricing data.

## Diagnostics

```bash
cockpit doctor
cockpit --version
cockpit completion zsh
```
