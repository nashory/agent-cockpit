# Development

agent-cockpit is a native Go CLI/TUI. The shipped command is `ac`; the repo and
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
go build ./cmd/ac
GOOS=linux GOARCH=amd64 go build ./cmd/ac
GOOS=windows GOARCH=amd64 go build ./cmd/ac
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

## Release Flow

1. Make sure CI passes on macOS, Linux, and Windows.
2. Tag a version:

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. The release workflow uploads native archives and `checksums.txt`.
4. The Homebrew tap should point its formula at the release tarball.

## Homebrew Formula Shape

The tap formula should install the shipped binary as `ac`:

```ruby
class AgentCockpit < Formula
  desc "Terminal cockpit for usage, cost, and speed across coding agents"
  homepage "https://github.com/nashory/agent-cockpit"
  url "https://github.com/nashory/agent-cockpit/releases/download/v0.1.0/ac-v0.1.0-darwin-arm64.tar.gz"
  sha256 "..."

  def install
    bin.install "ac"
  end
end
```
