#!/usr/bin/env bash
# Checks that the server, both installers and the installer README agree on versions.
set -euo pipefail

readonly TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPOSITORY_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
readonly CONTRACT="$REPOSITORY_ROOT/server-installer/compatibility-contract.env"
readonly SERVER="$REPOSITORY_ROOT/server/server.go"
readonly DEPLOY="$REPOSITORY_ROOT/client/internal/deploy/deploy.sh"
readonly INSTALLER="$REPOSITORY_ROOT/server-installer/install.sh"
readonly INSTALLER_README="$REPOSITORY_ROOT/server-installer/README.md"

fail() {
    printf 'FAIL: синхронизация версий: %s\n' "$*" >&2
    exit 1
}

[[ -f "$CONTRACT" && ! -L "$CONTRACT" ]] || fail "нет канонического контракта"
# shellcheck source=/dev/null
source "$CONTRACT"
[[ "${WDTT_SERVER_VERSION:-}" =~ ^[0-9]+$ ]] || fail "в контракте некорректна версия сервера"

grep -Eq "wdttServerVersion[[:space:]]*=[[:space:]]*\"$WDTT_SERVER_VERSION\"" "$SERVER" ||
    fail "server.go не соответствует версии $WDTT_SERVER_VERSION"
grep -Fxq "readonly WDTT_SERVER_VERSION=\"$WDTT_SERVER_VERSION\"" "$DEPLOY" ||
    fail "deploy.sh клиента не соответствует версии $WDTT_SERVER_VERSION"
grep -Fxq "readonly SUPPORTED_SERVER_VERSION=\"$WDTT_SERVER_VERSION\"" "$INSTALLER" ||
    fail "ручной установщик не соответствует версии $WDTT_SERVER_VERSION"

installer_version="$(sed -n 's/^readonly INSTALLER_VERSION="\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)"$/\1/p' "$INSTALLER")"
[[ -n "$installer_version" && "$installer_version" != *$'\n'* ]] ||
    fail "не удалось прочитать версию ручного установщика"
installer_intro="$(sed -n '1,16p' "$INSTALLER_README")"
grep -Fq "\`wdtt-server\` версии \`$WDTT_SERVER_VERSION\`" <<<"$installer_intro" ||
    fail "инструкция ручной установки указывает другую версию сервера"
grep -Fq "автономный установщик \`$installer_version\`" <<<"$installer_intro" ||
    fail "инструкция ручной установки указывает другую версию скрипта"
bundle_name="WDTT-Plus-server-v${WDTT_SERVER_VERSION}-installer-${installer_version}-linux-amd64.tar.gz"
grep -Fxq "$bundle_name" "$INSTALLER_README" ||
    fail "имя серверного архива в инструкции не соответствует выпуску"
grep -Fxq "$bundle_name.sha256" "$INSTALLER_README" ||
    fail "имя контрольной суммы в инструкции не соответствует выпуску"

printf 'Версии согласованы: сервер v%s, ручной установщик %s.\n' "$WDTT_SERVER_VERSION" "$installer_version"
