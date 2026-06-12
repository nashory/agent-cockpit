# Source Adapter Template

Use this checklist when adding or reviewing a source adapter.

## Scope

- Source name:
- Upstream agent/tool:
- Local-only data paths:
- Environment overrides:
- File or database format:
- Network access: no, unless explicitly documented and opt-in
- Credentials read: no, unless explicitly documented and opt-in

## Implementation

Create:

```text
internal/source/<name>/<name>.go
internal/source/<name>/<name>_test.go
```

Register the adapter in `internal/source/builtin/builtin.go`, then add the
source to:

- `config.Paths` defaults, validation, and schema
- `cockpit doctor` path output
- live-mode watch roots when the source is local file or database backed
- `--source` flag help text
- CI smoke config
- README source list
- `docs/sources.md`
- `docs/privacy.md`
- `docs/guide/sources.md`
- `docs/guide/configuration.md`

## Parser Rules

- Treat malformed files or rows as local parse misses where possible.
- Do not let one bad source file abort unrelated source scans.
- Normalize usage into `usage.Event`; avoid report or TUI source branches.
- Keep token components disjoint for pricing:
  - input tokens in `Input`
  - output-billed tokens in `Output`
  - cache reads in `CacheRead`
  - cache writes in `CacheCreate`
  - reasoning in `Reasoning` as a subset of `Output`
- If a provider exposes only total tokens, map the missing remainder to
  `Output`.
- Preserve provider timestamps in UTC. If a fallback timestamp is used, cover it
  in tests and docs.

## Tests

Cover:

- one minimal valid event
- total-token fallback
- cache-read and cache-write mapping when supported
- reasoning token mapping when supported
- malformed input
- zero-token input
- path discovery and environment override
- database adapters with an in-test SQLite fixture
- deduplication if the source can expose the same message through multiple
  files or roots

Run before commit:

```bash
go test ./internal/source/<name> ./internal/source ./internal/config ./internal/cli
make ci
git diff --check
```

## Source Docs

Add a `docs/sources.md` section with:

- default paths
- environment override
- credentials/network behavior
- representative sanitized record
- token field mapping
- timestamp/session/project derivation
- skip behavior for incomplete rows
