# ccusage Workflow Mapping

This page maps common ccusage-style workflows to `cockpit` commands.

## Daily, Weekly, Monthly

```bash
cockpit today
cockpit weekly
cockpit monthly
```

Use `--json` for scripts and `--no-cost` when token counts matter more than
estimated USD values.

```bash
cockpit today --json
cockpit monthly --json --no-cost
```

## Date Windows

```bash
cockpit trends --days 30
cockpit trends --since 7d
cockpit trends --since 2w --until 2026-06-11
cockpit report --since 2026-06-01 --until 2026-06-11
```

`--since` accepts `YYYY-MM-DD`, day/week shorthands such as `7d` or `2w`, and
Go-style durations such as `168h`. `--until` accepts `YYYY-MM-DD` and includes
the whole selected day.

## Timezone-Aware Reports

```bash
cockpit today --timezone Europe/Zurich
cockpit weekly --timezone America/Los_Angeles
cockpit export --group daily --timezone Asia/Seoul
```

The timezone changes date windows and buckets. Stored timestamps are not
rewritten.

## Breakdowns

```bash
cockpit report --breakdown source
cockpit report --breakdown model
cockpit report --breakdown project
cockpit export --group daily --breakdown project
```

`--breakdown` narrows aggregate output to the selected dimension. For CSV
exports, it overrides the default daily aggregate with source/model/project rows.

## Ordering

```bash
cockpit trends --order asc
cockpit monthly --order desc
cockpit export --group daily --order asc
cockpit export --group event --order desc
```

Use ascending order for spreadsheets and charts, and descending order for
interactive inspection of recent usage.

## Source, Project, Model Filters

```bash
cockpit today --source claude
cockpit trends --source claude,codex --project agent-cockpit
cockpit agents --model sonnet
cockpit report --source gemini --no-cost
```

Filters can be combined with date windows, ordering, timezone, JSON, and CSV
export flags.

## Statusline

```bash
cockpit statusline
cockpit statusline --compact
cockpit statusline --json
cockpit statusline --no-cost
cockpit statusline --format '{{model}} {{context}} {{today_cost}}'
```

Use the compact form for shell prompts and tmux-style integrations. Use JSON
when another tool owns the final rendering. Use `--format` for simple prompt
templates; common placeholders include `{{model}}`, `{{context}}`,
`{{context_left}}`, `{{tokens_compact}}`, `{{today_cost}}`, and
`{{block_left}}`.
