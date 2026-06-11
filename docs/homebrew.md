# Homebrew Distribution

The public repo is `nashory/agent-cockpit`. The Homebrew tap should live at
`nashory/homebrew-tap`.

The formula name should be `agent-cockpit`, but the installed executable should
be `cockpit`.

## User Install

Before the first stable release:

```bash
brew tap nashory/tap
brew install --HEAD agent-cockpit
cockpit
```

After a stable release is added to the formula:

```bash
brew tap nashory/tap
brew install agent-cockpit
cockpit
```

Or:

```bash
brew install nashory/tap/agent-cockpit
```

## Formula

The tap currently supports `--HEAD` source builds. After the first GitHub
Release, add stable URLs for macOS and Linux archives.

```ruby
class AgentCockpit < Formula
  desc "Terminal cockpit for usage, cost, and speed across coding agents"
  homepage "https://github.com/nashory/agent-cockpit"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/nashory/agent-cockpit/releases/download/v0.1.2/cockpit-v0.1.2-darwin-arm64.tar.gz"
      sha256 "..."
    else
      url "https://github.com/nashory/agent-cockpit/releases/download/v0.1.2/cockpit-v0.1.2-darwin-amd64.tar.gz"
      sha256 "..."
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/nashory/agent-cockpit/releases/download/v0.1.2/cockpit-v0.1.2-linux-arm64.tar.gz"
      sha256 "..."
    else
      url "https://github.com/nashory/agent-cockpit/releases/download/v0.1.2/cockpit-v0.1.2-linux-amd64.tar.gz"
      sha256 "..."
    end
  end

  def install
    bin.install "cockpit"
  end

  test do
    system "#{bin}/cockpit", "--version"
  end
end
```

## Release Steps

1. Tag and push a release:

   ```bash
   git tag v0.1.2
   git push origin v0.1.2
   ```

2. Wait for the GitHub Release workflow.
3. Copy artifact checksums from `checksums.txt`.
4. Update `Formula/agent-cockpit.rb` in `nashory/homebrew-tap`.
5. Test locally:

   ```bash
   brew install --build-from-source Formula/agent-cockpit.rb
   cockpit --version
   ```

See [Release Checklist](release-checklist.md) for the full tap audit: asset
names, checksum checks, formula fields, `brew audit`, and local install smoke.
