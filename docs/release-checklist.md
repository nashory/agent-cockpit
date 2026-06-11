# Release Checklist

Use this checklist before publishing a user-facing release. Do not create a tag
until the release version and notes are final.

## Preflight

```bash
make ci
make bench
python3 scripts/release-notes.py --check HEAD
cockpit pricing update --check
```

If pricing is stale:

```bash
make pricing
go test ./internal/usage ./internal/cli
```

Commit and push any pricing refresh before tagging.

## Screenshot Refresh

Refresh screenshots when the TUI layout, colors, tab content, table columns, or
README hero content changes.

Current assets:

| Asset | Purpose |
| --- | --- |
| `docs/imgs/demo.gif` | README dashboard demo |
| `docs/imgs/overview.png` | overview tab |
| `docs/imgs/breakdown.png` | breakdown tab |
| `docs/imgs/trends.png` | trends tab |
| `docs/imgs/activity.png` | activity tab |

Checklist:

- Use a deterministic fixture or sanitized local log set.
- Capture at a normal terminal size, such as `120x36`, unless the image is
  intentionally showing compact mode.
- Show the current source list and feature set.
- Avoid real usernames, project names, prompts, and proprietary paths.
- Verify light/dark or no-color behavior if the visual change affects theme
  selection.
- Run `git diff --check` after replacing image assets.
- Confirm README image links render locally or in the GitHub preview.

Useful manual smoke path:

```bash
cockpit --source claude,codex,gemini,opencode,amp,copilot,kimi
cockpit live --refresh 2s
cockpit report --svg usage.svg
```

## Release Notes

Generate compact, human-readable release notes from commits:

```bash
version=vX.Y.Z
python3 scripts/release-notes.py "$version"
```

Check the generated notes for:

- user-visible feature bullets first
- compact wording
- no internal process-only bullets
- no duplicated commit subjects
- no breaking-change language unless behavior actually changed

## Homebrew Tap Audit

The Homebrew tap is `nashory/homebrew-tap`; the formula should be
`Formula/agent-cockpit.rb`.

Before updating the tap, verify release assets:

```bash
version=vX.Y.Z
gh release view "$version" --repo nashory/agent-cockpit
gh release download "$version" --repo nashory/agent-cockpit --pattern checksums.txt
cat checksums.txt
```

Expected release archive names:

```text
cockpit-vX.Y.Z-darwin-arm64.tar.gz
cockpit-vX.Y.Z-darwin-amd64.tar.gz
cockpit-vX.Y.Z-linux-amd64.tar.gz
cockpit-vX.Y.Z-linux-arm64.tar.gz
cockpit-vX.Y.Z-windows-amd64.zip
cockpit-vX.Y.Z-windows-arm64.zip
checksums.txt
```

Formula audit:

- `desc` matches the current product description.
- `homepage` is `https://github.com/nashory/agent-cockpit`.
- macOS arm64, macOS amd64, Linux arm64, and Linux amd64 URLs use the new tag.
- each `sha256` matches `checksums.txt`.
- `bin.install "cockpit"` installs the binary name users run.
- `test do` checks `cockpit --version`.
- `brew audit --strict --online agent-cockpit` passes from the tap checkout.
- `brew install --build-from-source Formula/agent-cockpit.rb` succeeds.
- `cockpit --version` reports the release tag.

Windows archives are not used by Homebrew but should still be present on the
GitHub release.

## Post-Tag Checks

After the release workflow completes:

```bash
version=vX.Y.Z
gh run list --workflow Release --limit 3
gh release view "$version" --repo nashory/agent-cockpit
```

Install and smoke test one archive:

```bash
version=vX.Y.Z
tmp="$(mktemp -d)"
gh release download "$version" \
  --repo nashory/agent-cockpit \
  --pattern "cockpit-${version}-$(go env GOOS)-$(go env GOARCH).*" \
  --dir "$tmp"
ls "$tmp"
```

Then update the Homebrew tap and test the formula before announcing the release.
