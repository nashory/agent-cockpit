# Installation

## Homebrew

```bash
brew install nashory/tap/agent-cockpit
cockpit
```

To track the latest source build:

```bash
brew install --HEAD nashory/tap/agent-cockpit
```

## Release Archives

Download the matching archive from the latest GitHub release:

```bash
ver="$(gh release view --repo nashory/agent-cockpit --json tagName --jq .tagName)"
gh release download "$ver" \
  --repo nashory/agent-cockpit \
  --pattern "cockpit-${ver}-linux-amd64.tar.gz"
tar -xzf "cockpit-${ver}-linux-amd64.tar.gz"
```

Windows PowerShell:

```powershell
$ver = (Invoke-RestMethod https://api.github.com/repos/nashory/agent-cockpit/releases/latest).tag_name
Invoke-WebRequest "https://github.com/nashory/agent-cockpit/releases/download/$ver/cockpit-$ver-windows-amd64.zip" -OutFile cockpit.zip
Expand-Archive cockpit.zip -DestinationPath . -Force
.\cockpit-$ver-windows-amd64\cockpit.exe
```

## Go Install

```bash
go install github.com/nashory/agent-cockpit/cmd/cockpit@latest
cockpit --version
```

## From Source

```bash
git clone https://github.com/nashory/agent-cockpit.git
cd agent-cockpit
make build
./cockpit
```

Run the local validation gate before changing source:

```bash
make ci
```
