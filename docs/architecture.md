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
cmd/cockpit/                     binary entrypoint
internal/cli/               Cobra commands and flags
internal/config/            config path, TOML loading, defaults
internal/source/            adapter registry
internal/source/claude/     Claude Code JSONL parser
internal/source/codex/      Codex JSONL parser
internal/source/gemini/     Gemini session parser
internal/source/opencode/   OpenCode SQLite and message JSON parser
internal/source/amp/        Amp thread JSON parser
internal/source/copilot/    Copilot OpenTelemetry JSONL parser
internal/scan/              parallel directory walk + file parsing
internal/watch/             fsnotify log-dir watcher with debounced refresh
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

`source.Collect` runs the built-in adapters concurrently. File-backed adapters
delegate to `scan.Parallel`, which first walks roots to gather matching file
paths, then parses them across a worker pool sized to the CPU count. Per-file
parse errors are skipped so one corrupt log cannot abort a scan. Event order is
therefore non-deterministic; all downstream consumers aggregate into maps and
sort their own output, so they do not depend on order. Non-file adapters, such
as OpenCode's SQLite reader, keep the same normalized `usage.Event` boundary.

The TUI launches immediately and performs the first load in a background
`tea.Cmd`, showing a loading state and then the dashboard. Derived insights are
computed once per load and cached on the model, not recomputed per frame.

## Live Mode

`cockpit live` refreshes on file-system events. `internal/watch` recursively watches
the configured log roots with fsnotify, auto-watches new subdirectories as they
appear, and debounces bursts of writes into a single refresh signal that the TUI
consumes via a Bubble Tea command. A non-blocking interval tick remains as a
backstop, and if the OS watcher cannot start (e.g. inotify limits) the TUI falls
back to polling alone. Watching is layered on top of the adapters and report
aggregation without changing them.

## Native Distribution

Release artifacts are CGO-free binaries:

```text
cockpit-vX.Y.Z-darwin-arm64.tar.gz
cockpit-vX.Y.Z-darwin-amd64.tar.gz
cockpit-vX.Y.Z-linux-amd64.tar.gz
cockpit-vX.Y.Z-linux-arm64.tar.gz
cockpit-vX.Y.Z-windows-amd64.zip
cockpit-vX.Y.Z-windows-arm64.zip
```
