#!/usr/bin/env bash
# Checks that the app deploy script, the standalone installer and the server
# agree on the compatibility contract.
set -euo pipefail

readonly TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPOSITORY_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
readonly CONTRACT="$REPOSITORY_ROOT/server-installer/compatibility-contract.env"
readonly INSTALLER="$REPOSITORY_ROOT/server-installer/install.sh"
readonly RELEASE_BUNDLE="$REPOSITORY_ROOT/server-installer/release_bundle.sh"
readonly DEPLOY="$REPOSITORY_ROOT/client/internal/deploy/deploy.sh"
readonly DEPLOY_GO="$REPOSITORY_ROOT/client/internal/deploy/deploy.go"
readonly SERVER="$REPOSITORY_ROOT/server/server.go"

fail() {
    printf 'FAIL: совместимость деплоя из приложения и standalone: %s\n' "$*" >&2
    exit 1
}

require_exact_line() {
    local file="$1" expected="$2"
    grep -Fqx "$expected" "$file" ||
        fail "в $(basename "$file") нет обязательной строки: $expected"
}

require_text() {
    local file="$1" expected="$2"
    grep -Fq "$expected" "$file" ||
        fail "в $(basename "$file") отсутствует обязательная защита: $expected"
}

[[ -f "$CONTRACT" && ! -L "$CONTRACT" ]] || fail "нет канонического compatibility-contract.env"
# shellcheck source=/dev/null
source "$CONTRACT"

for key in \
    WDTT_DEPLOY_CONTRACT_VERSION \
    WDTT_SERVER_VERSION \
    WDTT_SERVER_BINARY_PATH \
    WDTT_SYSTEMD_UNIT_PATH \
    WDTT_CONFIG_DIRECTORY \
    WDTT_ACCESS_DATABASE \
    WDTT_WG_KEYS_FILE \
    WDTT_APP_DEPLOY_MARKER \
    WDTT_APP_CONTRACT_MARKER \
    WDTT_APP_PRESERVED_MARKER; do
    [[ -n "${!key:-}" ]] || fail "в контракте пустое поле $key"
done

require_exact_line "$INSTALLER" "readonly SUPPORTED_SERVER_VERSION=\"$WDTT_SERVER_VERSION\""
require_exact_line "$INSTALLER" "readonly DEPLOY_COMPATIBILITY_VERSION=\"$WDTT_DEPLOY_CONTRACT_VERSION\""
require_exact_line "$INSTALLER" "readonly ANDROID_DEPLOY_MARKER=\"$WDTT_APP_DEPLOY_MARKER\""
require_exact_line "$INSTALLER" "readonly ANDROID_PRESERVED_MARKER=\"$WDTT_APP_PRESERVED_MARKER\""
require_exact_line "$INSTALLER" \
    "readonly ANDROID_DEPLOY_COMPATIBILITY_MARKER=\"WDTT deploy compatibility: \$DEPLOY_COMPATIBILITY_VERSION\""
require_text "$INSTALLER" 'install.sh handoff-android [--dry-run] [--yes]'
require_text "$INSTALLER" 'write_android_handoff_unit()'
require_text "$INSTALLER" '# $ANDROID_DEPLOY_MARKER'
require_text "$INSTALLER" '# $ANDROID_DEPLOY_COMPATIBILITY_MARKER'
require_text "$INSTALLER" '[[ "$initial_kind" == "owned" ]]'
require_text "$INSTALLER" 'confirm_handoff'
[[ -f "$RELEASE_BUNDLE" && ! -L "$RELEASE_BUNDLE" && -x "$RELEASE_BUNDLE" ]] ||
    fail "нет безопасного исполняемого release_bundle.sh"
bash -n "$RELEASE_BUNDLE" || fail "release_bundle.sh содержит синтаксическую ошибку"
require_exact_line "$DEPLOY" "readonly WDTT_DEPLOY_CONTRACT_VERSION=\"$WDTT_DEPLOY_CONTRACT_VERSION\""
require_exact_line "$DEPLOY" "readonly WDTT_SERVER_VERSION=\"$WDTT_SERVER_VERSION\""
require_exact_line "$DEPLOY" "readonly WDTT_SERVER_BINARY_PATH=\"$WDTT_SERVER_BINARY_PATH\""
require_exact_line "$DEPLOY" "readonly WDTT_SYSTEMD_UNIT_PATH=\"$WDTT_SYSTEMD_UNIT_PATH\""
require_exact_line "$DEPLOY" "readonly WDTT_CONFIG_DIR=\"$WDTT_CONFIG_DIRECTORY\""
require_exact_line "$DEPLOY" "readonly WDTT_ACCESS_DB=\"$WDTT_ACCESS_DATABASE\""
require_exact_line "$DEPLOY" "readonly WDTT_WG_KEYS=\"$WDTT_WG_KEYS_FILE\""
require_exact_line "$DEPLOY" "readonly WDTT_APP_DEPLOY_MARKER=\"$WDTT_APP_DEPLOY_MARKER\""
require_exact_line "$DEPLOY" "readonly WDTT_APP_CONTRACT_MARKER=\"$WDTT_APP_CONTRACT_MARKER\""
require_exact_line "$DEPLOY" "readonly WDTT_APP_PRESERVED_MARKER=\"$WDTT_APP_PRESERVED_MARKER\""

grep -Fq 'ExecStart=${WDTT_SERVER_BINARY_PATH} -listen ' "$DEPLOY" ||
    fail "unit деплоя не запускает бинарник из контракта"
grep -Fq '# ${WDTT_APP_DEPLOY_MARKER}' "$DEPLOY" ||
    fail "unit деплоя не записывает метку владельца"
grep -Fq '# ${WDTT_APP_CONTRACT_MARKER}' "$DEPLOY" ||
    fail "unit деплоя не записывает версию контракта"
grep -Fq 'server_version="$("$WDTT_SERVER_BINARY_PATH" --version' "$DEPLOY" ||
    fail "deploy.sh не проверяет версию установленного бинарника"
grep -Fq '[ "$server_version" = "$WDTT_SERVER_VERSION" ]' "$DEPLOY" ||
    fail "deploy.sh не сравнивает версию бинарника с контрактом"

grep -Eq "wdttServerVersion[[:space:]]*=[[:space:]]*\"$WDTT_SERVER_VERSION\"" "$SERVER" ||
    fail "Go-сервер не соответствует версии из контракта"
bash "$TEST_DIR/version_sync_test.sh" >/dev/null ||
    fail "версии приложения, сервера и установщиков не синхронизированы"

# The app must refuse to manage standalone installs and must detect app-owned
# installs by the same markers the installer writes.
require_text "$DEPLOY_GO" "Managed by WDTT Plus standalone server installer"
require_text "$DEPLOY_GO" "# $WDTT_APP_DEPLOY_MARKER"
require_text "$DEPLOY_GO" "$WDTT_APP_PRESERVED_MARKER"
require_text "$INSTALLER" 'is_preserved_android_data()'
require_text "$INSTALLER" 'android-preserved)'
require_text "$INSTALLER" 'adopt-preserved'

actual_contract="$($INSTALLER compatibility --machine)"
grep -Fxq "contract_version=$WDTT_DEPLOY_CONTRACT_VERSION" <<<"$actual_contract" ||
    fail "standalone не сообщает версию контракта"
grep -Fxq "server_version=$WDTT_SERVER_VERSION" <<<"$actual_contract" ||
    fail "standalone сообщает другую версию сервера"
grep -Fxq "binary=$WDTT_SERVER_BINARY_PATH" <<<"$actual_contract" ||
    fail "standalone сообщает другой путь бинарника"
grep -Fxq "unit=$WDTT_SYSTEMD_UNIT_PATH" <<<"$actual_contract" ||
    fail "standalone сообщает другой путь unit"
grep -Fxq "config=$WDTT_CONFIG_DIRECTORY" <<<"$actual_contract" ||
    fail "standalone сообщает другой каталог конфигурации"
grep -Fxq "database=$WDTT_ACCESS_DATABASE" <<<"$actual_contract" ||
    fail "standalone сообщает другую базу"
grep -Fxq "wg_keys=$WDTT_WG_KEYS_FILE" <<<"$actual_contract" ||
    fail "standalone сообщает другой файл ключей"
grep -Fxq "preserved_marker=$WDTT_APP_PRESERVED_MARKER" <<<"$actual_contract" ||
    fail "standalone сообщает другую метку сохранённых данных приложения"

printf 'Контракт совместимости %s согласован: деплой из приложения, standalone-установщик и wdtt-server v%s.\n' \
    "$WDTT_DEPLOY_CONTRACT_VERSION" "$WDTT_SERVER_VERSION"
