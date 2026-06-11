# Troubleshooting

## No Usage Appears

Check detected paths:

```bash
cockpit doctor
```

Then run a source-specific report:

```bash
cockpit today --source claude
cockpit today --source codex
```

If a source lives outside the default path, add it to config:

```toml
[paths]
claude = ["/absolute/path/to/claude/projects"]
```

## Unexpected Cost

Show pricing coverage for local logs:

```bash
cockpit pricing status
cockpit pricing status --json
```

Omit estimates when you only need token counts:

```bash
cockpit today --no-cost
cockpit export --group daily --no-cost
```

Override a model price locally:

```toml
[pricing."custom-model"]
input_per_million = 1
output_per_million = 5
```

## Date Window Looks Wrong

Set an explicit timezone:

```bash
cockpit today --timezone Europe/Zurich
cockpit trends --since 7d --timezone Asia/Seoul
```

Use ascending order for spreadsheet exports:

```bash
cockpit export --group daily --order asc > usage.csv
```

## JSON Consumer Breaks

Inspect the command shape directly:

```bash
cockpit report --json
cockpit statusline --json
cockpit config validate --json
```

JSON fields are intended to be additive. Prefer reading named fields instead of
depending on object ordering.

## Live Mode Does Not Refresh

Run live mode with a polling interval:

```bash
cockpit live --refresh 2s
```

If filesystem watching is unavailable, the interval still refreshes the TUI.

## Copilot Usage Is Missing

Copilot CLI requires local OpenTelemetry file export before usage can be read.
Set `COPILOT_OTEL_FILE_EXPORTER_PATH` or configure the directory you use:

```bash
export COPILOT_OTEL_FILE_EXPORTER_PATH="$HOME/.copilot/otel/copilot.jsonl"
cockpit today --source copilot
```
