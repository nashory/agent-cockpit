# Contributing

agent-cockpit is early, but contributions are welcome. The highest-value areas
are new agent adapters, parser fixtures, terminal UX, and native packaging.

## Development Setup

```bash
git clone https://github.com/nashory/agent-cockpit.git
cd agent-cockpit
go mod download
go build ./cmd/ac
```

Run the TUI locally:

```bash
go run ./cmd/ac
```

On machines with binary allow-listing, `go run` and `go test` may require local
approval. Compile-only checks are still useful:

```bash
go build ./cmd/ac
GOOS=darwin GOARCH=arm64 go build ./cmd/ac
GOOS=linux GOARCH=amd64 go build ./cmd/ac
GOOS=windows GOARCH=amd64 go build ./cmd/ac
```

## Before Opening a PR

```bash
gofmt -w cmd internal
go test ./...
bash scripts/build-dist.sh dev
```

CI runs on macOS, Linux, and Windows. If local test execution is blocked by a
machine policy, say that in the PR and make sure compile-only checks pass.

## Adding an Agent Adapter

1. Add a package under `internal/source/<agent>/`.
2. Implement `Name()` and `Collect(context.Context, config.Config)`.
3. Parse logs into `usage.Event`.
4. Register the adapter in `internal/source/source.go`.
5. Document the default log path in `README.md`.
6. Add parser fixtures or focused tests without committing real logs.

Adapter rules:

- Read local files only.
- Do not shell out to agent CLIs for normal parsing.
- Ignore malformed final JSONL lines; active writers can leave partial lines.
- Preserve enough metadata for filtering: source, session, cwd/project, model,
  timestamp, and token categories.
- Treat cost as an estimate unless the source provides authoritative billing.

## Commit Style

Keep commits focused. Good examples:

```text
Add OpenCode JSONL adapter
Improve Windows config path handling
Add live dashboard refresh
```

## Security

Never include real agent logs, API keys, bot tokens, config files, screenshots
with secrets, or private repo paths in fixtures or docs.

