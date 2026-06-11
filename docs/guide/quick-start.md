# Quick Start

Open the dashboard:

```bash
cockpit
```

Use live mode when an agent is actively writing logs:

```bash
cockpit live --refresh 2s
```

Run static reports:

```bash
cockpit today
cockpit weekly
cockpit monthly
cockpit trends --days 30
cockpit sessions
```

Filter by source, project, model, date, and timezone:

```bash
cockpit today --source claude --timezone Europe/Zurich
cockpit trends --source claude,codex --project agent-cockpit --since 7d
cockpit sessions --model sonnet --order desc
```

Use JSON for scripts:

```bash
cockpit today --json
cockpit statusline --json
cockpit today --json --no-cost
```

Export CSV for spreadsheets:

```bash
cockpit export --group daily > usage.csv
cockpit export --group session --order asc > sessions.csv
```

Check detected sources:

```bash
cockpit doctor
```

See [Command Reference](commands.md) for the full command list.
