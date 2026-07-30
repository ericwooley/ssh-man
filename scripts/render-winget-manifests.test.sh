#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RENDERER="$ROOT_DIR/scripts/render-winget-manifests.rb"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-winget-manifest-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local file_path="$1"
  local expected="$2"

  grep -Fq "$expected" "$file_path" ||
    fail "expected $file_path to contain: $expected"
}

assert_not_contains() {
  local file_path="$1"
  local unexpected="$2"

  if grep -Fq -- "$unexpected" "$file_path"; then
    fail "expected $file_path not to contain: $unexpected"
  fi
}

assert_rejected() {
  local name="$1"
  shift
  local destination="$TEST_ROOT/rejected-$name"

  if ruby "$RENDERER" "$@" "$destination" >"$TEST_ROOT/$name.log" 2>&1; then
    fail "$name should have been rejected"
  fi
  [ ! -e "$destination" ] ||
    fail "$name should not leave a destination directory"
}

stable_sha="$(printf 'a%.0s' {1..64})"
stable_dir="$TEST_ROOT/stable"
ruby "$RENDERER" stable 2.3.4 "$stable_sha" "$stable_dir"

stable_version="$stable_dir/EricWooley.SSHMan.yaml"
stable_locale="$stable_dir/EricWooley.SSHMan.locale.en-US.yaml"
stable_installer="$stable_dir/EricWooley.SSHMan.installer.yaml"

for manifest_path in "$stable_version" "$stable_locale" "$stable_installer"; do
  [ -f "$manifest_path" ] || fail "missing stable manifest: $manifest_path"
  assert_contains "$manifest_path" 'ManifestVersion: 1.12.0'
done

ruby -ryaml -e '
  manifests = ARGV.map { |path| YAML.safe_load(File.read(path)) }
  abort "wrong stable package id" unless manifests.all? { |item| item["PackageIdentifier"] == "EricWooley.SSHMan" }
  abort "wrong stable version" unless manifests.all? { |item| item["PackageVersion"] == "2.3.4" }
  abort "wrong manifest types" unless manifests.map { |item| item["ManifestType"] } == %w[version defaultLocale installer]
' "$stable_version" "$stable_locale" "$stable_installer"

assert_contains "$stable_locale" 'PackageName: SSH Man'
assert_contains "$stable_locale" 'ReleaseNotesUrl: https://github.com/ericwooley/ssh-man/releases/tag/2.3.4'
assert_not_contains "$stable_locale" 'Experimental'
assert_contains "$stable_installer" 'InstallerType: nullsoft'
assert_contains "$stable_installer" 'Scope: machine'
assert_contains "$stable_installer" 'InstallerUrl: https://github.com/ericwooley/ssh-man/releases/download/2.3.4/ssh-man-windows-amd64-installer.exe'
assert_contains "$stable_installer" "InstallerSha256: $stable_sha"
assert_contains "$stable_installer" 'ProductCode: tech.moonpixels.ssh-man'
assert_contains "$stable_installer" 'DisplayName: SSH Man'

experimental_sha="$(printf 'b%.0s' {1..64})"
experimental_dir="$TEST_ROOT/experimental"
ruby "$RENDERER" experimental 2.4.0 "$experimental_sha" "$experimental_dir"

experimental_version="$experimental_dir/EricWooley.SSHMan.Experimental.yaml"
experimental_locale="$experimental_dir/EricWooley.SSHMan.Experimental.locale.en-US.yaml"
experimental_installer="$experimental_dir/EricWooley.SSHMan.Experimental.installer.yaml"

for manifest_path in "$experimental_version" "$experimental_locale" "$experimental_installer"; do
  [ -f "$manifest_path" ] || fail "missing experimental manifest: $manifest_path"
  assert_contains "$manifest_path" 'PackageIdentifier: EricWooley.SSHMan.Experimental'
done

assert_contains "$experimental_locale" 'PackageName: SSH Man Experimental'
assert_contains "$experimental_locale" 'The experimental release channel for SSH Man.'
assert_contains "$experimental_installer" 'InstallerUrl: https://github.com/ericwooley/ssh-man/releases/download/2.4.0/ssh-man-experimental-windows-amd64-installer.exe'
assert_contains "$experimental_installer" "InstallerSha256: $experimental_sha"
assert_contains "$experimental_installer" 'ProductCode: tech.moonpixels.ssh-man.experimental'
assert_contains "$experimental_installer" 'DisplayName: SSH Man Experimental'

assert_rejected "unknown-channel" nightly 2.3.4 "$stable_sha"
assert_rejected "invalid-version" stable v2.3.4 "$stable_sha"
assert_rejected "prerelease-version" experimental 2.3.4-beta.1 "$stable_sha"
assert_rejected "invalid-sha" stable 2.3.4 abc123

workflow="$ROOT_DIR/.github/workflows/release.yml"
promotion_workflow="$ROOT_DIR/.github/workflows/promote-release.yml"

assert_contains "$workflow" 'build-windows:'
assert_contains "$workflow" 'bash ./scripts/package-windows-release.sh'
assert_contains "$workflow" 'ssh-man-windows-amd64-installer.exe'
assert_contains "$workflow" 'ssh-man-experimental-windows-amd64-installer.exe'
assert_contains "$workflow" 'submit-experimental-winget:'
assert_contains "$workflow" 'ruby ./scripts/render-winget-manifests.rb'
assert_contains "$workflow" 'WINGET_CREATE_GITHUB_TOKEN: ${{ secrets.WINGET_CREATE_GITHUB_TOKEN }}'
assert_contains "$workflow" '24042bd37915805615e6cf969ac57c6439124c3fe85823327f5f3fb24bd9ffea'
assert_not_contains "$workflow" '--token $env:WINGET_CREATE_GITHUB_TOKEN'

assert_contains "$promotion_workflow" 'ssh-man-windows-amd64-installer.exe'
assert_contains "$promotion_workflow" 'submit-stable-winget:'
assert_contains "$promotion_workflow" 'ruby ./scripts/render-winget-manifests.rb'
assert_contains "$promotion_workflow" 'WINGET_CREATE_GITHUB_TOKEN: ${{ secrets.WINGET_CREATE_GITHUB_TOKEN }}'
assert_contains "$promotion_workflow" '24042bd37915805615e6cf969ac57c6439124c3fe85823327f5f3fb24bd9ffea'
assert_not_contains "$promotion_workflow" '--token $env:WINGET_CREATE_GITHUB_TOKEN'

printf 'WinGet manifest tests passed.\n'
