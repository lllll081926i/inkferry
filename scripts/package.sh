#!/usr/bin/env bash
# Build and package every supported InkFerry target.
#
# Usage:
#   ./scripts/package.sh v1.0.0
#   VERSION=v1.0.0 ./scripts/package.sh
#
# Output: dist/release/*.zip, *.tar.gz and SHA256SUMS.txt
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
STAGE="$ROOT/dist/stage"
rm -rf "$OUT" "$STAGE"
mkdir -p "$OUT" "$STAGE"

GUI_LDFLAGS=(-trimpath -ldflags "-s -w -X main.version=$VERSION")
WIN_LDFLAGS=(-trimpath -ldflags "-s -w -H windowsgui -X main.version=$VERSION")
CLI_LDFLAGS=(-trimpath -ldflags "-s -w")

build() {
  local goos="$1" goarch="$2" output="$3" pkg="$4" kind="$5"
  local flags=("${GUI_LDFLAGS[@]}")
  if [[ "$kind" == "windows-gui" ]]; then
    flags=("${WIN_LDFLAGS[@]}")
  elif [[ "$kind" == "cli" ]]; then
    flags=("${CLI_LDFLAGS[@]}")
  fi
  echo "  building $output"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build "${flags[@]}" -o "$output" "$pkg"
}

archive_dir() {
  local dir="$1" ext="$2" archive="$3"
  case "$ext" in
    zip)
      if command -v zip >/dev/null 2>&1; then
        (cd "$(dirname "$dir")" && zip -qr "$archive" "$(basename "$dir")")
      elif command -v powershell.exe >/dev/null 2>&1; then
        local win_dir win_archive
        win_dir="$(cygpath -w "$dir")"
        win_archive="$(cygpath -w "$archive")"
        powershell.exe -NoProfile -Command \
          "Compress-Archive -LiteralPath '$win_dir' -DestinationPath '$win_archive' -Force"
      else
        echo "error: zip is required to package Windows builds" >&2
        exit 1
      fi
      ;;
    tar.gz)
      tar -C "$(dirname "$dir")" -czf "$archive" "$(basename "$dir")"
      ;;
    *)
      echo "error: unknown archive type $ext" >&2
      exit 1
      ;;
  esac
}

package_target() {
  local goos="$1" goarch="$2" label="$3" ext="$4"
  local name="InkFerry-${VERSION}-${label}"
  local dir="$STAGE/$name"
  local suffix="" gui="inkferry"
  if [[ "$goos" == windows ]]; then
    suffix=".exe"
    gui="InkFerry"
  fi
  mkdir -p "$dir"

  build "$goos" "$goarch" "$dir/$gui$suffix" . gui
  build "$goos" "$goarch" "$dir/ferry$suffix" ./cmd/ferry cli
  build "$goos" "$goarch" "$dir/ferryinstall$suffix" ./cmd/ferryinstall cli
  cp README.md LICENSE CHANGELOG.md "$dir/"
  cp -R docs "$dir/"

  archive_dir "$dir" "$ext" "$OUT/$name.$ext"
  echo "packaged $name.$ext"
}

echo "InkFerry $VERSION"
echo "Building release archives..."
package_target windows amd64 windows-amd64 zip
package_target windows arm64 windows-arm64 zip
package_target linux amd64 linux-amd64 tar.gz
package_target darwin arm64 macos-arm64 tar.gz

(
  cd "$OUT"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.zip ./*.tar.gz > SHA256SUMS.txt
  else
    shasum -a 256 ./*.zip ./*.tar.gz > SHA256SUMS.txt
  fi
)

echo
echo "Release files in dist/release:"
ls -lh "$OUT"
