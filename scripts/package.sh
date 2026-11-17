#!/usr/bin/env bash
# Build the portable InkFerry releases for every supported target.
#
# Usage:
#   ./scripts/package.sh v1.0.0
#   VERSION=v1.0.0 ./scripts/package.sh
#
# Output in dist/release/, no archives:
#   InkFerry-<version>-<platform>-gui[.exe]   desktop app
#   InkFerry-<version>-<platform>-cli[.exe]   ferry command-line converter
#   SHA256SUMS.txt
#
# Every file is a single self-contained executable: no installer, no DLL, no
# data folder, no log file, no documentation.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-${VERSION:-}}"
if [[ -z "$VERSION" ]]; then
  VERSION="$(git describe --tags --exact-match 2>/dev/null || echo dev)"
fi
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "warning: '$VERSION' is not a semver tag (expected e.g. v1.0.0)" >&2
fi

OUT="$ROOT/dist/release"
rm -rf "$OUT"
mkdir -p "$OUT"

ASSETS=()

# build <goos> <goarch> <asset name> <package> <gui|cli>
# Windows GUI builds need the windowsgui subsystem: without it the linker
# marks the binary as a console program and Windows opens a console window
# beside the app. -s -w strips the symbol table and DWARF data.
build() {
  local goos="$1" goarch="$2" output="$3" pkg="$4" kind="$5"
  local ldflags="-s -w"
  if [[ "$kind" == "gui" ]]; then
    ldflags="-s -w -X main.version=$VERSION"
    if [[ "$goos" == "windows" ]]; then
      ldflags="-s -w -H windowsgui -X main.version=$VERSION"
    fi
  fi
  echo "  building $output"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$ldflags" -o "$OUT/$output" "$pkg"
  chmod 0755 "$OUT/$output"
  ASSETS+=("$output")
}

target() {
  local goos="$1" goarch="$2" label="$3" suffix=""
  if [[ "$goos" == "windows" ]]; then
    suffix=".exe"
  fi
  build "$goos" "$goarch" "InkFerry-$VERSION-$label-gui$suffix" . gui
  build "$goos" "$goarch" "InkFerry-$VERSION-$label-cli$suffix" ./cmd/ferry cli
}

echo "InkFerry $VERSION"
echo "Building portable single-file releases..."

target windows amd64 windows-amd64
target windows arm64 windows-arm64
target linux amd64 linux-amd64
target darwin arm64 macos-arm64

(
  cd "$OUT"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${ASSETS[@]}" > SHA256SUMS.txt
  else
    shasum -a 256 "${ASSETS[@]}" > SHA256SUMS.txt
  fi
)

echo
echo "Release files in dist/release:"
ls -lh "$OUT"
