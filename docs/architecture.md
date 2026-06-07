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
internal/usage/             normalized event model, pricing, grouping
internal/report/            text reports and charts
internal/tui/               sidebar TUI and live refresh loop
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

## Live Mode

`ac live` uses a polling refresh loop in the TUI. This is deliberately portable
across macOS, Linux, and Windows. A future fsnotify watcher can be layered in
without changing adapters or report aggregation.

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

