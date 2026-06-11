# ccusage Parity Design

This document defines how agent-cockpit should mature from a strong local
CLI/TUI into a production-quality usage monitor that can stand beside ccusage
for reporting quality and CodexBar for product polish.

## Goal

The target is not feature-count parity for its own sake. The target is that a
developer can trust agent-cockpit as their default local usage monitor because
it is:

- broad enough to cover the coding agents they actually use
- fast enough to run in a statusline or shell prompt
- stable enough for scripts and dashboards
- clear enough to understand cost, quota, and burn without reading docs
- private enough to explain exactly what local files or credentials are read

The product thesis remains:

```text
local coding-agent logs -> normalized usage events -> fast reports + live TUI
```

agent-cockpit should keep its differentiators:

- one static Go binary
- local-first, no backend
- rich terminal cockpit, not only static reports
- multi-agent normalized usage model
- native release artifacts for macOS, Linux, and Windows

## Competitive Baseline

### ccusage

ccusage is the primary parity target for CLI/reporting quality.

Strengths:

- Many source adapters: Claude Code, Codex, OpenCode, Amp, Droid, Codebuff,
  Hermes, pi-agent, Goose, OpenClaw, Kilo, Kimi, Qwen, Copilot, Gemini.
- Clear command model: `daily`, `weekly`, `monthly`, `session`, `blocks`,
  `statusline`, plus source-focused commands.
- Script-friendly flags: `--json`, `--no-cost`, `--compact`, `--timezone`,
  `--since`, `--until`, project filters, custom paths, pricing overrides.
- Configuration schema for autocomplete and validation.
- Strong output tests using snapshots for text and JSON stability.
- Large-fixture and performance benchmarking in CI.
- Native packages plus npm wrapper packages for easy `npx` / `bunx` usage.
- Offline pricing snapshot and scheduled pricing update workflow.
- Documentation site with guide pages per command and source.

Implications for agent-cockpit:

- The CLI output contract needs to become explicit and tested.
- Source adapters need a repeatable contract, fixture style, and scaffold.
- Config needs schema support.
- Performance needs to be measured, not assumed.
- Statusline must be optimized as a first-class fast path.

### CodexBar

CodexBar is the primary target for product surface and provider breadth.

Strengths:

- Always-visible menu bar surface with provider status, bars, countdowns, and
  notifications.
- Very broad provider set, with provider-specific documentation.
- Multiple data-source strategies: local files, CLI probes, OAuth, API keys,
  browser cookies, provider dashboards.
- CLI companion for scripts and Linux integrations.
- Clear privacy and permission docs.
- Strong packaging: macOS app, CLI tarballs, Homebrew cask/formula, Linux CLI,
  AUR ecosystem.
- Display controls: labels, icons, merged provider icons, refresh cadence, reset
  styles.

Implications for agent-cockpit:

- The terminal TUI is not enough for always-on monitoring. The near-term bridge
  is a stronger `statusline` and a local `serve` endpoint. Desktop companion
  work should come later.
- Provider/source docs must say exactly how data is found and what is read.
- Optional credential-based integrations must be explicit, isolated, and never
  silently added to the local-log path.

### Claude-Code-Usage-Monitor

This project is narrower but strong in live quota UX.

Strengths:

- Real-time terminal monitor with predictions and warnings.
- Plan and custom-limit handling.
- P90-style historical limit detection.
- Saved command preferences.
- Rich terminal UI with burn-rate and cost projections.

Implications for agent-cockpit:

- Budget and quota state should be visible at a glance.
- Burn rate should be shown in both token and cost terms.
- Warnings need thresholds and deterministic behavior.
- Limit discovery should be added after the base warning model is stable.

### Secondary References

CodeZeno's Windows taskbar monitor is relevant for always-on display behavior:

- very small glanceable surface
- OS-native affordances
- low refresh overhead
- clear differentiation between current block, day, and plan limits

Implications for agent-cockpit:

- The first always-on surface should be protocol-based, not OS-specific:
  `statusline --json` and `cockpit serve` should let other shells, bars, and
  launchers consume the same data.
- A native desktop/menu-bar companion can be built later without changing the
  core aggregation model.

aimonitor is relevant for account and credential handling:

- multi-account awareness
- keyring usage
- explicit account switching
- no-telemetry positioning

Implications for agent-cockpit:

- Credential-backed providers should not be added until local-log support is
  stable.
- If credential-backed providers are added, credentials must live in the OS
  keyring or an explicit external secret provider, not in the main config file.
- Multi-account reporting should be modeled as metadata on normalized events,
  not as separate report implementations.

## Current agent-cockpit State

Implemented:

- Sources: Claude Code, Codex, Gemini.
- Source orchestration: `internal/source` already collects the built-in sources
  concurrently.
- Static reports: today, weekly, monthly, trends, agents, sessions, speed,
  statusline, report, SVG.
- Exports: CSV by daily, session, model, project, event.
- TUI tabs: Overview, Breakdown, Trends, Activity, Daily, Blocks, Sessions.
- TUI interactions: zoom, help, row drill-downs, chart windows, table sorting,
  Daily day/week/month periods, project drill-down.
- Monitoring: optional USD budgets and Claude 5h / 7d token limits.
- Pricing: vendored LiteLLM-derived pricing plus config overrides.
- Distribution: native archives for macOS, Linux, Windows with release smoke
  tests.

Gaps:

- Only three source adapters.
- Source registry is static; adding a source still requires editing
  `internal/source/source.go`.
- No config schema.
- No stable JSON schema documentation.
- Limited output golden tests.
- Performance benchmarks exist for statusline rendering, source collection, and
  usage aggregation; p95 tracking and cache-vs-scan comparison are still open.
- No large synthetic fixture.
- Statusline does not consume Claude Code stdin payload yet.
- No `--timezone`, `--no-cost`, `--breakdown`, `--order`.
- No docs site or command-by-command guide.
- Privacy docs are high-level, not source-specific.
- No provider/source plugin guide.

## Product Principles

### Local-first by default

The default data path should read local logs only. Network or credential-based
data sources can be added later, but must be opt-in and clearly labeled.

### Script contracts are product contracts

Any `--json`, CSV, statusline, or table format used by users should have tests
that catch accidental breakage.

### TUI and CLI share the same model

The TUI should not invent parallel aggregation logic where possible. Reports,
exports, statusline, and TUI panels should use shared usage-domain helpers.

### Fast path matters

`cockpit statusline` should be treated as a latency-sensitive command. It should
avoid full scans when an incremental cache exists, and it should have benchmark
coverage.

### Adapter addition should be boring

Adding a new source should require:

- one parser package
- fixtures
- source docs
- tests
- registration

No source should force changes across report/TUI aggregation code unless the
normalized event model is genuinely insufficient.

## Target Architecture

### Package Shape

Current packages are good, but need a few additions and some tightening:

```text
internal/source/          existing source orchestration, registry, contracts
internal/source/<name>/   source parser + fixtures + docs link
internal/schema/          config and JSON schema generation
internal/export/          CSV/JSON shape helpers, if report grows too large
internal/cache/           optional incremental cache for statusline/serve
internal/benchdata/       synthetic fixture generation helpers
docs/guide/               command and source docs
```

`internal/usage` remains the normalized domain model:

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

Before adding more sources, decide whether we need optional fields:

```go
type Event struct {
    // existing fields
    RequestID    string
    Conversation string
    CostUSD      *float64 // authoritative provider cost, when present
    Metadata     map[string]string
}
```

Do not add this until a real adapter needs it. The current event model is still
enough for local log usage reports.

### Source Contracts

The project already has this high-level source interface:

```go
type Source interface {
    Name() string
    Collect(context.Context, config.Config) ([]usage.Event, error)
}
```

Keep that interface for command-level collection, but make registration
extensible instead of hard-coding all sources in `All()`. The registry should
eventually allow:

```go
source.Register(claude.Source{})
source.Register(codex.Source{})
source.Register(gemini.Source{})
```

For file-backed local log sources, add a lower-level helper contract so each
source can share scanning behavior without duplicating path walking:

```go
type FileAdapter interface {
    Name() string
    Roots(config.Config) []string
    Match(path string) bool
    Parse(path string, r io.Reader) ([]usage.Event, error)
}
```

This should be an internal helper, not necessarily the public source contract.
Some future sources may use CLI probes, APIs, browser stores, or keyrings and
will not fit a file-adapter model cleanly.

Source requirements:

- Parse errors should be file-local unless the adapter configuration itself is
  invalid.
- Missing optional fields should produce usable events, not dropped files.
- Project name extraction must be deterministic and Unicode-safe.
- Timestamps must preserve timezone information when available.
- Tests must include malformed input and at least one realistic fixture.

### Report Contracts

Define output shapes for:

- `summary`
- `daily`
- `weekly`
- `monthly`
- `sessions`
- `blocks`
- `models`
- `projects`
- `statusline`

Each report should have:

- text output golden test
- JSON output golden test
- empty-data test
- filter test
- no-cost test once `--no-cost` lands

Recommended internal shape:

```go
type ReportEnvelope struct {
    SchemaVersion string       `json:"schema_version"`
    GeneratedAt   time.Time    `json:"generated_at"`
    Range         Range        `json:"range"`
    Filters       Filters      `json:"filters"`
    Totals        usage.Totals `json:"totals"`
    Rows          any          `json:"rows"`
}
```

Keep JSON additive. Removing or renaming JSON fields should require an explicit
breaking-change decision.

Cost fields need an explicit policy. Existing totals use a numeric
`estimated_cost_usd`; `--no-cost` should not silently emit zero values that look
authoritative. Either omit cost through report-specific response structs or
include a `cost_available` / `cost_mode` field that makes the absence explicit.

### Config Schema

Add:

```bash
cockpit config schema
cockpit config validate
```

Schema should cover:

- timezone
- refresh interval
- currency
- budget
- limits
- paths
- pricing overrides
- UI preferences if they become user-editable
- source-specific settings

The schema can be generated from Go structs or maintained manually. Generated is
preferable if the code remains simple. The file should be published into release
artifacts and linked from README.

### Pricing

Current pricing is good enough for local estimates. Next steps:

- `cockpit pricing status` should report unmatched models clearly.
- Add `cockpit pricing update --check` to compare vendored snapshot metadata.
- Add a scheduled workflow to refresh pricing data and open a PR.
- Store pricing snapshot provenance: upstream source, commit/date, generated at.

Do not add runtime network pricing lookup to ordinary reports. Keep reports
offline and deterministic by default.

### Statusline

Statusline should become a first-class command, not a summary shortcut.

Inputs:

- no stdin: use latest local logs
- Claude Code statusLine JSON on stdin: identify current session/model/context
- flags/config: thresholds, formatting, cost source

Output modes:

```bash
cockpit statusline
cockpit statusline --compact
cockpit statusline --json
cockpit statusline --format '{{model}} {{today_cost}} {{block_left}}'
```

Target fields:

- active model
- current session cost
- today cost
- active 5h block cost
- active 5h block time left
- burn rate tokens/hour
- burn rate USD/hour
- context tokens and context percent when stdin provides it
- budget/quota worst state

Latency target:

- under 200 ms on normal local logs
- under 500 ms on large synthetic fixture

If full scans cannot meet that target, add `internal/cache`.

### Optional Incremental Cache

Cache is only justified when statusline or `serve` needs it.

Design:

```text
~/.cache/agent-cockpit/index.db or index.json
  file path
  file size
  file mtime
  parser version
  event hashes or serialized events
```

Rules:

- Cache must be disposable.
- `--no-cache` must exist.
- Corrupt cache falls back to full scan.
- Parser version changes invalidate entries.
- Cache must not contain raw prompts or message text. Store only normalized
  usage events.

### Local Serve Mode

`cockpit serve` is the bridge toward CodexBar-like surfaces without building a
desktop app first.

Phase 1:

```bash
cockpit serve --addr 127.0.0.1:8765
```

Endpoints:

```text
GET /health
GET /api/summary
GET /api/daily
GET /api/blocks
GET /api/sessions
GET /api/statusline
```

Rules:

- Bind localhost only by default.
- No auth needed for localhost phase.
- Never expose raw prompts.
- Reuse report JSON shapes.

Later:

- SSE or WebSocket for live refresh events.
- Small web dashboard.
- Integrations for SketchyBar, Waybar, tmux, Zellij.

## Feature Specifications

### Config Schema

User story:

- As a user, I want editor autocomplete and validation for
  `config.toml`, so configuration mistakes are caught before reports run.

Commands:

```bash
cockpit config schema
cockpit config validate
cockpit config validate --config ./config.toml
```

Implementation notes:

- Keep the current TOML config as the source of truth.
- Add a JSON schema that describes the TOML-equivalent shape.
- Print schema to stdout by default.
- `validate` should report all known errors when feasible, not only the first
  error.

Done when:

- Schema output is deterministic.
- README links to schema usage.
- CI verifies schema freshness or at least verifies the command emits valid
  JSON.

### Stable Report JSON

User story:

- As a script author, I want stable JSON reports, so dashboards and automation
  do not break between patch releases.

Implementation notes:

- Introduce typed response structs for each report instead of returning ad hoc
  maps.
- Wrap report rows in `ReportEnvelope`.
- Include versioned metadata:

```json
{
  "schema_version": "1",
  "generated_at": "2026-06-11T00:00:00Z",
  "range": {},
  "filters": {},
  "totals": {},
  "rows": []
}
```

Rules:

- Additive fields are allowed in minor releases.
- Field removal or type changes require a major schema version bump.
- JSON golden tests must cover empty, normal, and filtered output.
- `--no-cost` JSON must have an explicit representation for unavailable cost,
  not an implicit zero.

### Source Adapter Registry

User story:

- As a maintainer, I want a repeatable adapter contract, so adding OpenCode or
  Amp does not require touching every report.

Implementation notes:

- Extend existing `internal/source`.
- Move the hard-coded built-in source list behind registration.
- Add a file-adapter helper for local log sources that can reuse `internal/scan`.
- Keep parser packages small and file-format-specific.
- Make the scanner collect adapter-level warnings without failing the whole
  command.

Done when:

- Existing three sources use the registry.
- Report and TUI code consume only normalized `usage.Event` values.
- One parser fixture can fail without aborting unrelated source scans.
- Source packages can be added without editing unrelated aggregation code.

### CLI Parity Flags

User story:

- As a ccusage user, I want familiar flags, so switching tools does not require
  relearning basic report workflows.

Flags:

```bash
--timezone <iana-name>
--no-cost
--breakdown model|project|source
--order asc|desc
--since <date-or-duration>
--until <date>
```

Rules:

- Timezone affects bucketing, not stored timestamps.
- `--no-cost` must remove cost calculations from rows and totals, not print
  zero-cost values that look authoritative.
- `--order` should be consistent across table, JSON, and CSV output.
- Existing shorthand workflows should keep working.

### Statusline Fast Path

User story:

- As a Claude Code user, I want a statusline command that is fast enough to run
  continuously and useful enough to replace hand-written scripts.

Command examples:

```bash
cockpit statusline
cockpit statusline --compact
cockpit statusline --json
cockpit statusline --format '{{model}} {{block_left}} {{today_cost}}'
```

Behavior:

- If stdin contains Claude Code statusLine JSON, use it to identify the active
  model/session/context.
- If stdin is empty, infer state from the latest local logs.
- If cost is disabled or unavailable, token and time fields should still render.

Done when:

- Normal data p95 is under 200 ms.
- Large fixture p95 is under 500 ms.
- Output has golden tests for default, compact, JSON, and custom format.

### Local API

User story:

- As a power user, I want a localhost API, so I can build tmux, menu-bar, or
  dashboard integrations without parsing terminal tables.

Command:

```bash
cockpit serve --addr 127.0.0.1:8765
```

Endpoint rules:

- Reuse report JSON envelopes.
- Never return raw prompts or assistant text.
- Bind localhost by default.
- Log refresh errors without crashing the server when possible.

Initial endpoints:

```text
GET /health
GET /api/summary
GET /api/daily
GET /api/blocks
GET /api/sessions
GET /api/statusline
```

### Documentation Site Readiness

User story:

- As a new user, I want command and source docs that answer basic setup and
  privacy questions without reading source code.

Minimum guide set:

- install
- quick start
- configuration
- command reference
- source reference
- privacy
- troubleshooting

Rules:

- Every command page needs at least one realistic example.
- Every source page must list default paths and whether credentials/network are
  used.
- README should remain a product overview, not the full manual.

## Source Expansion Plan

Priority should follow value divided by implementation risk.

### P1 Sources

OpenCode:

- High overlap with ccusage users.
- Local log parsing likely similar to existing adapters.

Amp:

- Visible in ccusage and CodexBar.
- Useful for broader coding-agent positioning.

Copilot CLI:

- Important brand coverage.
- Validate whether reliable local logs exist before committing.

Cursor:

- High user value, but may require browser/app data or API access. Keep opt-in.

### P2 Sources

Kimi, Qwen, Kilo, Goose, Codebuff.

Add only after adapter contract, fixtures, and docs template are in place.

### Source Documentation Template

Each source doc should include:

- supported versions, if known
- default paths
- file patterns
- data fields extracted
- whether network or credentials are used
- privacy notes
- troubleshooting
- fixture examples without real user data

## Testing Strategy

### Unit Tests

Required for:

- every parser
- path expansion
- filters
- pricing resolution
- budget/quota status
- time bucketing
- JSON envelopes

### Golden Tests

Add `testdata/golden`.

Targets:

- table output for every report
- JSON output for every report
- CSV output for every export group
- statusline default/compact/json
- TUI render smoke for representative widths

Golden tests should normalize:

- timestamps
- current date
- terminal color when needed
- path separators for Windows

### Fixture Tests

Each adapter gets:

```text
internal/source/<name>/testdata/
  minimal.<ext>
  realistic.<ext>
  malformed.<ext>
```

No real logs.

### Performance Tests

Add fixture generator:

```bash
go test ./internal/benchdata -run TestGenerateLargeFixture
```

Benchmark targets:

```bash
go test -bench=. ./internal/scan ./internal/source ./internal/report
```

Scenarios:

- 1k files, 10k events
- 10k files, 100k events
- mixed source roots
- cold scan
- warm cache scan, once cache exists
- statusline latency

CI should initially record benchmarks as artifacts. Later, add PR comparison
comments like ccusage.

## CI and Release Quality

Current CI is solid for a small Go project. To reach parity:

Add:

- output golden tests in normal `go test`
- race tests remain
- coverage artifact remains
- benchmark job, non-blocking at first
- action pinning or periodic action update policy
- generated schema freshness check
- pricing snapshot freshness check
- release notes check for human-readable bullets

Release workflow should continue to:

- build all native artifacts
- verify artifacts on native runners
- run `cockpit --version`
- run JSON smoke command
- run `doctor`

Add later:

- smoke `statusline --json`
- smoke `export --group daily`
- smoke `pricing status`
- verify `config schema` emits valid JSON

## Documentation Plan

Move from a single README-heavy doc style to guide pages.

Proposed structure:

```text
docs/
  architecture.md
  development.md
  privacy.md
  guide/
    installation.md
    configuration.md
    commands.md
    daily.md
    weekly.md
    monthly.md
    sessions.md
    blocks.md
    statusline.md
    export.md
    pricing.md
    tui.md
    sources/
      claude.md
      codex.md
      gemini.md
      opencode.md
```

README should remain concise:

- what it is
- install
- quick start
- core features
- screenshots
- link to docs

Docs acceptance criteria:

- every command has at least one copy-paste example
- every JSON command has a short JSON shape example
- every source explains what it reads
- privacy page lists network behavior and local paths

## Implementation Backlog

### Epic A: Contract Stabilization

Tasks:

- Add typed JSON structs for existing reports.
- Add golden tests for text output.
- Add golden tests for JSON output.
- Add golden tests for CSV export groups.
- Document JSON compatibility policy.
- Add release note quality check for compact human-readable bullets.

Suggested commit split:

- `report: add stable json envelopes`
- `test: add report golden fixtures`
- `docs: document report contracts`

### Epic B: Config and Pricing

Tasks:

- Add `cockpit config schema`.
- Add `cockpit config validate`.
- Add pricing snapshot metadata.
- Add `cockpit pricing status` unmatched-model diagnostics.
- Add `cockpit pricing update --check`.
- Add scheduled pricing refresh PR workflow.
- Add CI smoke for schema and pricing commands.

Suggested commit split:

- `config: add schema and validation commands`
- `pricing: expose snapshot diagnostics`
- `ci: smoke config and pricing commands`
- `ci: automate pricing refresh prs`

### Epic C: Source Registry

Tasks:

- Add source registration to existing `internal/source`.
- Add a reusable `FileAdapter` helper for local log parsers.
- Move Claude source behind the registry.
- Move Codex source behind the registry.
- Move Gemini source behind the registry.
- Add adapter fixture template.
- Add adapter docs template.
- Preserve current CLI/TUI behavior.

Suggested commit split:

- `source: add adapter registry`
- `source: migrate existing parsers`
- `docs: add source adapter template`

### Epic D: ccusage Workflow Parity

Tasks:

- Add `--timezone`.
- Add `--no-cost`.
- Add `--breakdown`.
- Add `--order`.
- Normalize `--since` and `--until` behavior across reports.
- Add examples mapping common ccusage workflows to cockpit commands.

Suggested commit split:

- `cli: add report ordering and timezone flags`
- `cli: add no-cost and breakdown modes`
- `docs: add ccusage workflow examples`

### Epic E: Statusline and Cache

Tasks:

- Parse Claude Code statusLine JSON from stdin.
- Add statusline compact/JSON/custom format modes.
- Add statusline golden tests.
- Add latency benchmark.
- Add cache only if benchmark data shows full scans are too slow.

Suggested commit split:

- `statusline: support claude stdin context`
- `statusline: add json and custom formats`
- `perf: benchmark statusline latency`
- `cache: add disposable scan index`, if needed

### Epic F: Source Expansion

Tasks:

- Add OpenCode adapter.
- Add Amp adapter.
- Validate Copilot CLI local data availability.
- Add Kimi adapter.
- Add Qwen adapter.
- Pick additional Cursor, Kilo, Goose, or Codebuff adapters based on available
  local fixture quality.
- Add docs and fixtures for each new source.

Suggested commit split:

- one adapter per commit
- docs and tests in the same commit as each adapter

### Epic G: Always-on Integrations

Tasks:

- Add `cockpit serve`.
- Add localhost JSON endpoints.
- Add tmux example.
- Add Waybar or SketchyBar example.
- Add privacy docs for serve mode.

Suggested commit split:

- `serve: add localhost report api`
- `docs: add bar integration examples`

## Roadmap

### Phase 0: Stabilize Contracts

Deliverables:

- config schema command
- JSON envelope definitions
- golden tests for existing reports
- statusline golden tests
- CSV golden tests
- release smoke additions

Acceptance criteria:

- `go test ./...` covers text/JSON/CSV output stability
- `cockpit config schema` emits valid JSON schema
- README links to schema and docs

### Phase 1: ccusage CLI Parity

Deliverables:

- `--timezone`
- `--no-cost`
- `--breakdown`
- `--order asc|desc`
- richer `statusline` with Claude Code stdin support
- unmatched model pricing diagnostics

Acceptance criteria:

- command examples mirror the common ccusage workflows
- statusline works as a Claude Code statusLine command
- no network calls in default reports

### Phase 2: Adapter Expansion

Deliverables:

- explicit adapter interface
- source docs template
- OpenCode adapter
- Amp adapter
- one more high-value adapter after validation

Acceptance criteria:

- each source has fixtures and docs
- no parser can abort the whole scan on one corrupt file
- source registry keeps report/TUI code unchanged

### Phase 3: Performance and Cache

Deliverables:

- synthetic benchmark fixture helpers
- benchmark suite and CI artifact
- statusline latency budget
- optional incremental cache if needed

Acceptance criteria:

- statusline p95 under target on large fixture
- cache stores only normalized usage events
- `--no-cache` works

### Phase 4: Always-on Surface

Deliverables:

- `cockpit serve`
- local JSON API
- tmux/Waybar/SketchyBar examples
- optional notification support in live mode

Acceptance criteria:

- serve mode binds localhost by default
- no raw prompt text exposed
- status endpoints reuse tested report envelopes

### Phase 5: Product Polish

Deliverables:

- docs guide pages
- privacy page
- screenshot refresh checklist
- release note quality check
- Homebrew tap automation audit

Acceptance criteria:

- new user can install, configure, and read reports from docs alone
- every release has compact human-readable notes
- source privacy model is auditable

## Non-goals

Near-term non-goals:

- cloud backend
- team account management
- prompt/eval observability platform
- browser cookie scraping by default
- storing raw prompts or assistant content
- desktop menu bar app in the core repo

These can be revisited later, but they should not block CLI/reporting parity.

## Risks

Source churn:

- Agent log formats are private and can change. Mitigation: fixture tests,
  parser versioning, tolerant parsing, docs for supported versions.

Statusline latency:

- Full scans may be too slow. Mitigation: benchmark first, then add cache only
  if measured latency requires it.

JSON contract lock-in:

- Once users script against JSON, changes are costly. Mitigation: explicit
  envelopes and additive changes only.

Scope creep:

- CodexBar-style provider breadth can pull the project toward credential and
  browser-cookie complexity. Mitigation: local-log sources first, opt-in
  credential sources later.

TUI complexity:

- Adding features directly into Bubble Tea views can create duplicated
  aggregation logic. Mitigation: shared report/usage helpers and tests outside
  the TUI.

## Success Metrics

Technical:

- 8+ supported local-log sources
- statusline p95 under 200 ms on normal data
- golden coverage for every report/export/statusline mode
- config schema generated and tested
- benchmark suite running in CI

Product:

- README quick start stays under one screen
- docs cover every command and source
- releases have compact human-written changelogs
- users can explain what files are read without inspecting code

Competitive:

- CLI workflows match ccusage for common daily/weekly/monthly/session/block
  reports.
- TUI remains a differentiator rather than a maintenance liability.
- Statusline is good enough to use continuously inside Claude Code.
