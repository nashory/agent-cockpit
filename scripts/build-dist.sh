#!/usr/bin/env bash
set -euo pipefail

version="${1:-dev}"
out_dir="${OUT_DIR:-dist}"

mkdir -p "$out_dir"
rm -f "$out_dir"/cockpit-* "$out_dir"/checksums.txt

build() {
  local goos="$1"
  local goarch="$2"
  local ext=""
  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
  fi

  local bin="cockpit${ext}"
  local dir="cockpit-${version}-${goos}-${goarch}"
  local archive

  rm -rf "$out_dir/$dir"
  mkdir -p "$out_dir/$dir"

  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X github.com/nashory/agent-cockpit/internal/cli.version=${version}" \
    -o "$out_dir/$dir/$bin" \
    ./cmd/cockpit

  cp README.md LICENSE "$out_dir/$dir/"

  if [[ "$goos" == "windows" ]]; then
    archive="$out_dir/${dir}.zip"
    (cd "$out_dir" && zip -qr "$(basename "$archive")" "$dir")
  else
    archive="$out_dir/${dir}.tar.gz"
    tar -C "$out_dir" -czf "$archive" "$dir"
  fi

  rm -rf "$out_dir/$dir"
}

build darwin arm64
build darwin amd64
build linux amd64
build linux arm64
build windows amd64
build windows arm64

(cd "$out_dir" && shasum -a 256 cockpit-* > checksums.txt)

