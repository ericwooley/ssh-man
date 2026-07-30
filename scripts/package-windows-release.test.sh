#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGER="$ROOT_DIR/scripts/package-windows-release.sh"
INSTALLER_TEMPLATE="$ROOT_DIR/build/windows/installer/project.nsi"
LIFECYCLE_TEST="$ROOT_DIR/scripts/test-windows-installer-lifecycle.ps1"
RELEASE_WORKFLOW="$ROOT_DIR/.github/workflows/release.yml"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-windows-package-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local file_path="$1"
  local expected="$2"

  grep -Fq -- "$expected" "$file_path" ||
    fail "expected $file_path to contain: $expected"
}

assert_not_contains() {
  local file_path="$1"
  local unexpected="$2"

  if grep -Fq -- "$unexpected" "$file_path"; then
    fail "expected $file_path not to contain: $unexpected"
  fi
}

fixture_root="$TEST_ROOT/project"
fake_bin="$TEST_ROOT/bin"
mkdir -p "$fixture_root/scripts" "$fixture_root/build/windows/installer" "$fake_bin"
cp "$PACKAGER" "$fixture_root/scripts/package-windows-release.sh"
cp "$INSTALLER_TEMPLATE" "$fixture_root/build/windows/installer/project.nsi"

cat >"$fake_bin/go" <<'FAKE_GO'
#!/usr/bin/env bash
set -euo pipefail
printf 'channel=%s version=%s args=%s\n' \
  "${SSH_MAN_CHANNEL:-}" \
  "${SSH_MAN_VERSION:-}" \
  "$*" >>"$TEST_COMMAND_LOG"
case "$*" in
  "install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0")
    mkdir -p "$GOBIN"
    cat >"$GOBIN/wails" <<'FAKE_WAILS'
#!/usr/bin/env bash
set -euo pipefail
printf 'channel=%s version=%s cgo=%s cc=%s args=%s\n' \
  "${SSH_MAN_CHANNEL:-}" \
  "${SSH_MAN_VERSION:-}" \
  "${CGO_ENABLED:-}" \
  "${CC:-}" \
  "$*" >>"$TEST_COMMAND_LOG"
mkdir -p "$TEST_PROJECT_ROOT/build/bin"
printf 'windows binary\n' >"$TEST_PROJECT_ROOT/build/bin/ssh-man.exe"
printf 'stable installer\n' >"$TEST_PROJECT_ROOT/build/bin/ssh-man-windows-amd64-installer.exe"
FAKE_WAILS
    chmod +x "$GOBIN/wails"
    ;;
  *)
    printf 'unexpected go command: %s\n' "$*" >&2
    exit 1
    ;;
esac
FAKE_GO

cat >"$fake_bin/makensis" <<'FAKE_MAKENSIS'
#!/usr/bin/env bash
set -euo pipefail
printf 'channel=%s version=%s args=%s\n' \
  "${SSH_MAN_CHANNEL:-}" \
  "${SSH_MAN_VERSION:-}" \
  "$*" >>"$TEST_COMMAND_LOG"
mkdir -p "$TEST_PROJECT_ROOT/build/bin"
case "$*" in
  *"-DSSH_MAN_CHANNEL=stable"*)
    printf 'stable installer\n' >"$TEST_PROJECT_ROOT/build/bin/ssh-man-windows-amd64-installer.exe"
    ;;
  *"-DSSH_MAN_CHANNEL=experimental"*)
    printf 'experimental installer\n' >"$TEST_PROJECT_ROOT/build/bin/ssh-man-experimental-windows-amd64-installer.exe"
    ;;
  *)
    printf 'unexpected makensis channel: %s\n' "$*" >&2
    exit 1
    ;;
esac
FAKE_MAKENSIS

cat >"$fake_bin/x86_64-w64-mingw32-gcc" <<'FAKE_GCC'
#!/usr/bin/env bash
exit 0
FAKE_GCC

chmod +x "$fake_bin/go" "$fake_bin/makensis" "$fake_bin/x86_64-w64-mingw32-gcc"

command_log="$TEST_ROOT/commands.log"
(
  cd "$fixture_root"
  PATH="$fake_bin:$PATH" \
    TEST_COMMAND_LOG="$command_log" \
    TEST_PROJECT_ROOT="$fixture_root" \
    ./scripts/package-windows-release.sh 2.3.4
)

stable_installer="$fixture_root/dist/ssh-man-windows-amd64-installer.exe"
experimental_installer="$fixture_root/dist/ssh-man-experimental-windows-amd64-installer.exe"

[ "$(cat "$stable_installer")" = "stable installer" ] ||
  fail "stable installer was not copied to dist"
[ "$(cat "$experimental_installer")" = "experimental installer" ] ||
  fail "experimental installer was not copied to dist"

assert_contains "$command_log" 'channel= version= args=install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0'
assert_contains "$command_log" 'channel=stable version=2.3.4 cgo=1 cc=x86_64-w64-mingw32-gcc args=build'
assert_contains "$command_log" '-platform windows/amd64'
assert_contains "$command_log" '-nsis'
assert_contains "$command_log" '-ldflags -X ssh-man/internal/buildinfo.Version=2.3.4'
assert_contains "$command_log" 'channel=stable version=2.3.4 args=-DSSH_MAN_CHANNEL=stable'
assert_contains "$command_log" 'channel=experimental version=2.3.4 args='
assert_contains "$command_log" 'project.nsi'

invalid_log="$TEST_ROOT/invalid.log"
if (
  cd "$fixture_root"
  PATH="$fake_bin:$PATH" \
    TEST_COMMAND_LOG="$command_log" \
    TEST_PROJECT_ROOT="$fixture_root" \
    ./scripts/package-windows-release.sh v2.3.4
) >"$invalid_log" 2>&1; then
  fail "invalid version should have been rejected"
fi
assert_contains "$invalid_log" 'Version must be a plain semantic version'

assert_contains "$INSTALLER_TEMPLATE" '!define STABLE_PRODUCT_CODE "tech.moonpixels.ssh-man"'
assert_contains "$INSTALLER_TEMPLATE" '!define EXPERIMENTAL_PRODUCT_CODE "tech.moonpixels.ssh-man.experimental"'
assert_contains "$INSTALLER_TEMPLATE" 'SSH_MAN_CHANNEL'
assert_contains "$INSTALLER_TEMPLATE" 'SSH_MAN_VERSION'
assert_contains "$INSTALLER_TEMPLATE" 'File "/oname=${PRODUCT_EXECUTABLE}.new"'
assert_contains "$INSTALLER_TEMPLATE" 'Rename "$INSTDIR\${PRODUCT_EXECUTABLE}" "$INSTDIR\${PRODUCT_EXECUTABLE}.old"'
assert_contains "$INSTALLER_TEMPLATE" 'IfErrors binaryInstallFailed'
assert_contains "$INSTALLER_TEMPLATE" 'SetErrorLevel 1'
assert_not_contains "$INSTALLER_TEMPLATE" '!insertmacro wails.files'
assert_not_contains "$INSTALLER_TEMPLATE" '!insertmacro wails.associateCustomProtocols'
assert_not_contains "$INSTALLER_TEMPLATE" '!insertmacro wails.unassociateCustomProtocols'

assert_contains "$RELEASE_WORKFLOW" 'test-windows-installers:'
assert_contains "$RELEASE_WORKFLOW" 'pwsh ./scripts/test-windows-installer-lifecycle.ps1'
assert_contains "$RELEASE_WORKFLOW" '- test-windows-installers'
assert_contains "$LIFECYCLE_TEST" 'function Test-InstallerChannel'
assert_contains "$LIFECYCLE_TEST" 'Silent upgrade changed the installed executable after reporting failure.'
assert_contains "$LIFECYCLE_TEST" 'Installer lifecycle tests passed.'

printf 'Windows release packaging tests passed.\n'
