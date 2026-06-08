# Security

agent-cockpit is local-first. It reads local coding-agent logs and renders
terminal dashboards. It should not upload logs or require API keys.

## Sensitive Data

Agent logs can contain:

- prompts and responses
- local file paths
- repository names
- command output
- API keys or tokens accidentally pasted into sessions

Do not attach raw logs to GitHub issues. If a parser bug needs a fixture,
minimize and redact it before sharing.

## Reporting a Vulnerability

Open a private security advisory on GitHub if possible. If that is unavailable,
open a minimal issue without secrets and say that details can be shared
privately.

## Maintainer Checklist

Before a release:

- Confirm no real logs or local config files are committed.
- Confirm release archives contain only `cockpit`, `README.md`, and `LICENSE`.
- Confirm GitHub Actions artifacts are built from the tagged commit.
- Confirm Homebrew checksums match the release artifacts.
- Review new adapters for network calls or shell execution.

