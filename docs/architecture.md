# Architecture

agent-cockpit has a small pipeline:

```text
local agent logs
  -> source adapters
  -> normalized usage events
  -> filters and aggregation
  -> static reports or Bubble Tea TUI
```

## Packages

```text
cmd/ac/                     binary entrypoint
internal/cli/               Cobra commands and flags
internal/config/            config path, TOML loading, defaults
internal/source/            adapter registry
internal/source/claude/     Claude Code JSONL parser
internal/source/codex/      Codex JSONL parser
internal/source/gemini/     Gemini session parser
internal/scan/              parallel directory walk + file parsing
internal/usage/             normalized event model, pricing, grouping, insights
internal/report/            text reports and charts
internal/tui/               glass-cockpit TUI (tabs, theme, async load)
```

## Normalized Event

Every adapter emits `usage.Event`:

```go
type Event struct {
    Source      string
    SessionID   string
    Project     string
    CWD         string
    Model       string
    Input       int64
    Output      int64
    CacheRead   int64
    CacheCreate int64
    Reasoning   int64
    Timestamp   time.Time
}
```

The model intentionally separates cache and reasoning tokens because providers
price and expose those categories differently.

## Cost

Cost is estimated locally from model-name substring matching:

1. user config pricing overrides
2. built-in defaults
3. zero cost when no pricing is known

The UI should always say estimated cost unless an adapter gains authoritative
billing data.

## Loading Pipeline

`source.Collect` runs the three adapters concurrently. Each adapter delegates to
`scan.Parallel`, which first walks its roots to gather matching file paths, then
parses them across a worker pool sized to the CPU count. Per-file parse errors
are skipped so one corrupt log cannot abort a scan. Event order is therefore
non-deterministic; all downstream consumers aggregate into maps and sort their
own output, so they do not depend on order.

The TUI launches immediately and performs the first load in a background
`tea.Cmd`, showing a loading state and then the dashboard. Derived insights are
computed once per load and cached on the model, not recomputed per frame.

## Live Mode

`ac live` uses a non-blocking polling refresh loop in the TUI: each tick kicks
off a background reload rather than blocking the render. This is deliberately
portable across macOS, Linux, and Windows. A future fsnotify watcher can be
layered in without changing adapters or report aggregation.

## Native Distribution

Release artifacts are CGO-free binaries:

```text
ac-vX.Y.Z-darwin-arm64.tar.gz
ac-vX.Y.Z-darwin-amd64.tar.gz
ac-vX.Y.Z-linux-amd64.tar.gz
ac-vX.Y.Z-linux-arm64.tar.gz
ac-vX.Y.Z-windows-amd64.zip
ac-vX.Y.Z-windows-arm64.zip
```

