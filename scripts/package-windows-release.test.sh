#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGER="$ROOT_DIR/scripts/package-windows-release.sh"
RELEASE_WORKFLOW="$ROOT_DIR/.github/workflows/release.yml"
PROMOTION_WORKFLOW="$ROOT_DIR/.github/workflows/promote-release.yml"
VALIDATION_SCRIPT="$ROOT_DIR/scripts/validate.sh"
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

assert_not_contains_case_insensitive() {
  local file_path="$1"
  local unexpected="$2"

  if grep -Fiq -- "$unexpected" "$file_path"; then
    fail "expected $file_path not to contain, ignoring case: $unexpected"
  fi
}

fixture_root="$TEST_ROOT/project"
fake_bin="$TEST_ROOT/bin"
mkdir -p "$fixture_root/scripts" "$fake_bin"
cp "$PACKAGER" "$fixture_root/scripts/package-windows-release.sh"

cat >"$fake_bin/go" <<'FAKE_GO'
#!/usr/bin/env bash
set -euo pipefail
printf 'args=%s\n' "$*" >>"$TEST_COMMAND_LOG"
case "$*" in
  "install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0")
    mkdir -p "$GOBIN"
    cat >"$GOBIN/wails" <<'FAKE_WAILS'
#!/usr/bin/env bash
set -euo pipefail
printf 'cgo=%s cc=%s args=%s\n' \
  "${CGO_ENABLED:-}" \
  "${CC:-}" \
  "$*" >>"$TEST_COMMAND_LOG"
mkdir -p "$TEST_PROJECT_ROOT/build/bin"
printf 'windows executable\n' >"$TEST_PROJECT_ROOT/build/bin/ssh-man.exe"
FAKE_WAILS
    chmod +x "$GOBIN/wails"
    ;;
  *)
    printf 'unexpected go command: %s\n' "$*" >&2
    exit 1
    ;;
esac
FAKE_GO

cat >"$fake_bin/x86_64-w64-mingw32-gcc" <<'FAKE_GCC'
#!/usr/bin/env bash
exit 0
FAKE_GCC

chmod +x "$fake_bin/go" "$fake_bin/x86_64-w64-mingw32-gcc"

command_log="$TEST_ROOT/commands.log"
(
  cd "$fixture_root"
  PATH="$fake_bin:$PATH" \
    TEST_COMMAND_LOG="$command_log" \
    TEST_PROJECT_ROOT="$fixture_root" \
    ./scripts/package-windows-release.sh 2.3.4
)

release_executable="$fixture_root/dist/ssh-man-windows-amd64.exe"
[ "$(cat "$release_executable")" = "windows executable" ] ||
  fail "Windows executable was not copied to dist"

assert_contains "$command_log" 'args=install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0'
assert_contains "$command_log" 'cgo=1 cc=x86_64-w64-mingw32-gcc args=build'
assert_contains "$command_log" '-platform windows/amd64'
assert_contains "$command_log" '-clean'
assert_contains "$command_log" '-ldflags -X ssh-man/internal/buildinfo.Version=2.3.4'
assert_not_contains "$command_log" '-nsis'

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

assert_contains "$RELEASE_WORKFLOW" 'build-windows:'
assert_contains "$RELEASE_WORKFLOW" 'bash ./scripts/package-windows-release.sh'
assert_contains "$RELEASE_WORKFLOW" 'dist/ssh-man-windows-amd64.exe'
assert_not_contains "$RELEASE_WORKFLOW" 'test-windows-installers:'
assert_not_contains "$RELEASE_WORKFLOW" 'installer.exe'
assert_not_contains_case_insensitive "$RELEASE_WORKFLOW" 'winget'
assert_not_contains "$RELEASE_WORKFLOW" 'WINGET_CREATE_GITHUB_TOKEN'

assert_contains "$PROMOTION_WORKFLOW" 'ssh-man-windows-amd64.exe'
assert_not_contains "$PROMOTION_WORKFLOW" 'installer.exe'
assert_not_contains_case_insensitive "$PROMOTION_WORKFLOW" 'winget'
assert_not_contains "$PROMOTION_WORKFLOW" 'WINGET_CREATE_GITHUB_TOKEN'

assert_not_contains "$VALIDATION_SCRIPT" 'render-winget-manifests.test.sh'

printf 'Windows release packaging tests passed.\n'
