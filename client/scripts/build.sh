#!/usr/bin/env bash
# Builds WDTT Plus for Windows. Works on Windows (Git Bash), Linux and macOS.
# Requires Go, Node.js and the Wails CLI (go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0).
set -euo pipefail

readonly CLIENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly ROOT_DIR="$(cd "$CLIENT_DIR/.." && pwd)"
readonly WINTUN_VERSION="0.14.1"
readonly WINTUN_SHA256="07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51"
readonly CACHE_DIR="$CLIENT_DIR/build/cache"
readonly ARCH="${ARCH:-amd64}"

version="$(cd "$CLIENT_DIR" && node -p "require('./wails.json').info.productVersion")"

sha256() {
    if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

echo "==> wdtt-server для деплоя"
for server_arch in amd64 arm64; do
    (cd "$ROOT_DIR/server" && CGO_ENABLED=0 GOOS=linux GOARCH="$server_arch" \
        go build -trimpath -buildvcs=false -ldflags="-s -w" \
        -o "$CLIENT_DIR/internal/deploy/assets/wdtt-server-linux-$server_arch" .)
done

echo "==> Wintun $WINTUN_VERSION"
mkdir -p "$CACHE_DIR"
archive="$CACHE_DIR/wintun-$WINTUN_VERSION.zip"
[[ -f "$archive" ]] || curl -fsSL -o "$archive" "https://www.wintun.net/builds/wintun-$WINTUN_VERSION.zip"
if [[ "$(sha256 "$archive")" != "$WINTUN_SHA256" ]]; then
    rm -f "$archive"
    echo "Контрольная сумма Wintun не совпала" >&2
    exit 1
fi
extracted="$CACHE_DIR/wintun-$WINTUN_VERSION"
rm -rf "$extracted"
if command -v unzip >/dev/null; then
    unzip -q "$archive" -d "$extracted"
else
    powershell.exe -NoProfile -Command "Expand-Archive -LiteralPath '$(cygpath -w "$archive")' -DestinationPath '$(cygpath -w "$extracted")'"
fi
for wintun_arch in amd64 arm64; do
    cp "$extracted/wintun/bin/$wintun_arch/wintun.dll" "$CLIENT_DIR/internal/tunnel/wintun/wintun-$wintun_arch.dll"
done

echo "==> WDTT Plus $version для windows/$ARCH"
cd "$CLIENT_DIR"
wails build -clean -trimpath -platform "windows/$ARCH" -ldflags "-s -w -X main.version=$version"

mkdir -p "$ROOT_DIR/dist"
cp "build/bin/WDTT Plus.exe" "$ROOT_DIR/dist/WDTT-Plus-$version-windows-$ARCH.exe"
echo "Готово: dist/WDTT-Plus-$version-windows-$ARCH.exe"
