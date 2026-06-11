# Development

agent-cockpit is a native Go CLI/TUI. The shipped command is `cockpit`; the repo and
project name stay `agent-cockpit`.

## Requirements

- Go 1.26 or newer
- macOS, Linux, or Windows
- A terminal with ANSI support for the TUI

## Common Commands

```bash
make test
make build
make run
```

During local development on machines with binary allow-listing, it can be
useful to run compile-only checks first:

```bash
go build ./cmd/cockpit
GOOS=linux GOARCH=amd64 go build ./cmd/cockpit
GOOS=windows GOARCH=amd64 go build ./cmd/cockpit
```

Build the distributable archives for all native targets:

```bash
bash scripts/build-dist.sh dev
```

Targets:

```text
darwin/arm64
darwin/amd64
linux/amd64
linux/arm64
windows/amd64
windows/arm64
```

## Local Data

The app reads local JSONL logs:

```text
Claude Code:
  ~/.claude/projects/**/*.jsonl

Codex:
  ~/.codex/sessions/**/*.jsonl
  ~/.codex/archived_sessions/**/*.jsonl

Gemini:
  ~/.gemini/tmp/**/chats/session-*.json
```

Do not commit real agent logs, API keys, bot tokens, or local config files.

## Source Adapters

Command-level collection uses `internal/source.Source`:

```go
type Source interface {
	Name() string
	Collect(context.Context, config.Config) ([]usage.Event, error)
}
```

Built-in sources are registered from `internal/source/builtin`; CLI packages
import that package for its registration side effect. Report and TUI code should
depend only on normalized `usage.Event` values and should not import individual
source packages.

File-backed local log adapters can use `source.CollectFiles` by implementing:

```go
type FileAdapter interface {
	Name() string
	Roots(config.Config) []string
	Match(path string) bool
	Parse(path string, r io.Reader) ([]usage.Event, error)
}
```

Adapter rules:

- Keep parser packages focused on one source format.
- Treat malformed files as file-local errors where possible; one bad file should
  not abort unrelated source scans.
- Add realistic fixtures and malformed-input tests for new adapters.
- Preserve timestamps and normalize into `usage.Event`; do not add report-level
  source branches unless the shared event model is insufficient.

## Release Flow

1. Make sure CI passes on macOS, Linux, and Windows.
2. Tag a version:

   ```bash
   git tag v0.1.2
   git push origin v0.1.2
   ```

3. The release workflow uploads native archives and `checksums.txt`.
4. The Homebrew tap should point its formula at the release tarball.

Pre-1.0 versioning is numeric, not feature-gated:

- Patch releases advance as `0.x.1`, `0.x.2`, through `0.x.10`.
- After `0.x.10`, the next release is `0.(x+1).0`.
- Example: after `v0.1.10`, the next release is `v0.2.0`; after `v0.2.10`,
  the next release is `v0.3.0`.

## Homebrew Formula Shape

The tap formula should install the shipped binary as `cockpit`:

```ruby
class AgentCockpit < Formula
  desc "Terminal cockpit for usage, cost, and speed across coding agents"
  homepage "https://github.com/nashory/agent-cockpit"
  url "https://github.com/nashory/agent-cockpit/releases/download/v0.1.2/cockpit-v0.1.2-darwin-arm64.tar.gz"
  sha256 "..."

  def install
    bin.install "cockpit"
  end
end
```
