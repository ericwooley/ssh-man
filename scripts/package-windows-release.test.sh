#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGER="$ROOT_DIR/scripts/package-windows-release.sh"
RELEASE_WORKFLOW="$ROOT_DIR/.github/workflows/release.yml"
PROMOTION_WORKFLOW="$ROOT_DIR/.github/workflows/promote-release.yml"
VALIDATION_SCRIPT="$ROOT_DIR/scripts/validate.sh"
ARTIFACT_VALIDATOR="$ROOT_DIR/scripts/test-windows-release-artifact.ps1"
WINDOWS_VERSION_INFO="$ROOT_DIR/build/windows/info.json"
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
cp "$ROOT_DIR/wails.json" "$fixture_root/wails.json"
original_wails_config_sha="$(
  shasum -a 256 "$fixture_root/wails.json" | awk '{print $1}'
)"

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
product_version="$(
  node -e '
    const fs = require("fs");
    const config = JSON.parse(fs.readFileSync("wails.json", "utf8"));
    process.stdout.write(config.info.productVersion);
  '
)"
printf 'cgo=%s cc=%s productVersion=%s args=%s\n' \
  "${CGO_ENABLED:-}" \
  "${CC:-}" \
  "$product_version" \
  "$*" >>"$TEST_COMMAND_LOG"
if [ "${TEST_WAILS_FAIL:-false}" = "true" ]; then
  exit 42
fi
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
assert_contains "$command_log" 'cgo=1 cc=x86_64-w64-mingw32-gcc productVersion=2.3.4 args=build'
assert_contains "$command_log" '-platform windows/amd64'
assert_contains "$command_log" '-clean'
assert_contains "$command_log" '-ldflags -X ssh-man/internal/buildinfo.Version=2.3.4'
assert_not_contains "$command_log" '-nsis'

restored_product_version="$(
  node -e '
    const fs = require("fs");
    const config = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
    process.stdout.write(config.info.productVersion);
  ' "$fixture_root/wails.json"
)"
[ "$restored_product_version" = "1.0.0" ] ||
  fail "packager did not restore the original Wails product version"
[ "$(shasum -a 256 "$fixture_root/wails.json" | awk '{print $1}')" = "$original_wails_config_sha" ] ||
  fail "packager did not restore the exact Wails configuration after a successful build"

failed_build_log="$TEST_ROOT/failed-build.log"
if (
  cd "$fixture_root"
  PATH="$fake_bin:$PATH" \
    TEST_COMMAND_LOG="$command_log" \
    TEST_PROJECT_ROOT="$fixture_root" \
    TEST_WAILS_FAIL=true \
    ./scripts/package-windows-release.sh 2.3.5
) >"$failed_build_log" 2>&1; then
  fail "simulated Wails build failure should have failed packaging"
fi

restored_after_failure="$(
  node -e '
    const fs = require("fs");
    const config = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
    process.stdout.write(config.info.productVersion);
  ' "$fixture_root/wails.json"
)"
[ "$restored_after_failure" = "1.0.0" ] ||
  fail "packager did not restore Wails configuration after a build failure"
[ "$(shasum -a 256 "$fixture_root/wails.json" | awk '{print $1}')" = "$original_wails_config_sha" ] ||
  fail "packager did not restore the exact Wails configuration after a failed build"

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
assert_contains "$RELEASE_WORKFLOW" 'test-windows-release-artifact:'
assert_contains "$RELEASE_WORKFLOW" 'pwsh ./scripts/test-windows-release-artifact.ps1'
assert_contains "$RELEASE_WORKFLOW" '- test-windows-release-artifact'
assert_not_contains "$RELEASE_WORKFLOW" 'installer.exe'
assert_not_contains_case_insensitive "$RELEASE_WORKFLOW" 'winget'
assert_not_contains "$RELEASE_WORKFLOW" 'WINGET_CREATE_GITHUB_TOKEN'

publish_step="$(
  awk '
    /- name: Publish experimental GitHub release/ { capture = 1 }
    capture { print }
    capture && /- name: Checkout tap repository/ { exit }
  ' "$RELEASE_WORKFLOW"
)"
grep -Fq 'dist/ssh-man-windows-amd64.exe' <<<"$publish_step" ||
  fail "release publishing step does not attach the Windows executable"

assert_contains "$ARTIFACT_VALIDATOR" '[System.Diagnostics.FileVersionInfo]::GetVersionInfo'
assert_contains "$ARTIFACT_VALIDATOR" '$versionInfo.FileMajorPart'
assert_contains "$ARTIFACT_VALIDATOR" '$versionInfo.FileMinorPart'
assert_contains "$ARTIFACT_VALIDATOR" '$versionInfo.FileBuildPart'
assert_contains "$ARTIFACT_VALIDATOR" '$versionInfo.FilePrivatePart'
assert_contains "$ARTIFACT_VALIDATOR" '$versionInfo.ProductMajorPart'
assert_contains "$ARTIFACT_VALIDATOR" '$versionInfo.ProductMinorPart'
assert_contains "$ARTIFACT_VALIDATOR" '$versionInfo.ProductBuildPart'
assert_contains "$ARTIFACT_VALIDATOR" '$versionInfo.ProductPrivatePart'
assert_not_contains "$ARTIFACT_VALIDATOR" '$versionInfo.FileVersion'
assert_contains "$ARTIFACT_VALIDATOR" "Start-Process"
assert_contains "$ARTIFACT_VALIDATOR" "'--help'"
assert_contains "$ARTIFACT_VALIDATOR" 'DesktopStartupSeconds'
assert_contains "$ARTIFACT_VALIDATOR" 'desktop application exited during its startup smoke check'
assert_contains "$ARTIFACT_VALIDATOR" 'Windows release artifact validation passed.'

assert_contains "$WINDOWS_VERSION_INFO" '"file_version": "{{.Info.ProductVersion}}"'
assert_contains "$WINDOWS_VERSION_INFO" '"product_version": "{{.Info.ProductVersion}}"'
assert_contains "$WINDOWS_VERSION_INFO" '"0409": {'
assert_not_contains "$WINDOWS_VERSION_INFO" '"0000": {'
assert_contains "$WINDOWS_VERSION_INFO" '"ProductVersion": "{{.Info.ProductVersion}}"'

assert_contains "$PROMOTION_WORKFLOW" 'ssh-man-windows-amd64.exe'
assert_not_contains "$PROMOTION_WORKFLOW" 'installer.exe'
assert_not_contains_case_insensitive "$PROMOTION_WORKFLOW" 'winget'
assert_not_contains "$PROMOTION_WORKFLOW" 'WINGET_CREATE_GITHUB_TOKEN'

assert_not_contains "$VALIDATION_SCRIPT" 'render-winget-manifests.test.sh'

printf 'Windows release packaging tests passed.\n'
