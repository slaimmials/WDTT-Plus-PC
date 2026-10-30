#!/usr/bin/env bash
# Builds the standalone server bundle: dist/WDTT-Plus-server-v<N>-installer-<V>-linux-amd64.tar.gz
set -Eeuo pipefail
umask 022

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly DIST_DIR="$ROOT_DIR/dist"

die() {
    printf 'Ошибка сборки серверного комплекта: %s\n' "$*" >&2
    exit 1
}

# shellcheck source=/dev/null
source "$SCRIPT_DIR/compatibility-contract.env"
installer_version="$(bash "$SCRIPT_DIR/install.sh" --version)"
[[ "$installer_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "некорректная версия install.sh: $installer_version"
bundle="WDTT-Plus-server-v${WDTT_SERVER_VERSION}-installer-${installer_version}-linux-amd64"

work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT
mkdir -m 0755 "$work/$bundle"

(cd "$ROOT_DIR/server" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=false -trimpath -ldflags="-s -w" -o "$work/$bundle/wdtt-server" .)
chmod 0755 "$work/$bundle/wdtt-server"
if [[ "$(uname -s)-$(uname -m)" == Linux-x86_64 ]]; then
    version="$("$work/$bundle/wdtt-server" --version)"
    [[ "$version" == "$WDTT_SERVER_VERSION" ]] || die "сервер сообщил версию $version вместо $WDTT_SERVER_VERSION"
fi
install -m 0755 "$SCRIPT_DIR/install.sh" "$work/$bundle/install.sh"
(cd "$work/$bundle" && sha256sum install.sh wdtt-server >SHA256SUMS && chmod 0644 SHA256SUMS)

epoch="${SOURCE_DATE_EPOCH:-$(git -C "$ROOT_DIR" log -1 --format=%ct)}"
mkdir -p "$DIST_DIR"
(cd "$work" && LC_ALL=C tar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner \
    --format=posix --pax-option=delete=atime,delete=ctime -cf - "$bundle" | gzip -n >"$DIST_DIR/$bundle.tar.gz")
(cd "$DIST_DIR" && sha256sum "$bundle.tar.gz" >"$bundle.tar.gz.sha256")
printf 'Готово: dist/%s.tar.gz\n' "$bundle"
