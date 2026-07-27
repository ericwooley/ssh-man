#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UPDATER="$ROOT_DIR/scripts/update-homebrew-cask.rb"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-homebrew-cask-test.XXXXXX")"

cleanup() {
  rm -rf "$TEST_ROOT"
}

trap cleanup EXIT

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local expected="$2"

  grep -Fqx "$expected" "$file" || fail "$file is missing: $expected"
}

assert_before() {
  local file="$1"
  local first="$2"
  local second="$3"
  local first_line
  local second_line

  first_line="$(grep -nFx "$first" "$file" | cut -d: -f1)"
  second_line="$(grep -nFx "$second" "$file" | cut -d: -f1)"
  [ "$first_line" -lt "$second_line" ] ||
    fail "$file should place '$first' before '$second'"
}

assert_rejected() {
  local label="$1"
  shift

  if ruby "$UPDATER" "$@" >"$TEST_ROOT/rejected-output" 2>&1; then
    fail "$label should be rejected"
  fi
}

STABLE_SOURCE="$TEST_ROOT/ssh-man.rb"
EXPERIMENTAL_CASK="$TEST_ROOT/ssh-man@experimental.rb"
PROMOTED_CASK="$TEST_ROOT/promoted-ssh-man.rb"
EXPERIMENTAL_SHA="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
STABLE_SHA="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

cp "$ROOT_DIR/scripts/testdata/ssh-man.cask.rb" "$STABLE_SOURCE"

ruby "$UPDATER" \
  experimental \
  1.7.0 \
  "$EXPERIMENTAL_SHA" \
  "$STABLE_SOURCE" \
  "$EXPERIMENTAL_CASK"

assert_contains "$EXPERIMENTAL_CASK" 'cask "ssh-man@experimental" do'
assert_contains "$EXPERIMENTAL_CASK" '  version "1.7.0"'
assert_contains "$EXPERIMENTAL_CASK" "  sha256 \"$EXPERIMENTAL_SHA\""
assert_contains "$EXPERIMENTAL_CASK" '  url "https://github.com/ericwooley/ssh-man/releases/download/1.7.0/ssh-man.dmg"'
assert_contains "$EXPERIMENTAL_CASK" '  desc "Manage your SSH tunnels and SOCKS5 proxies"'
assert_contains "$EXPERIMENTAL_CASK" '  depends_on :macos'
assert_contains "$EXPERIMENTAL_CASK" '  conflicts_with cask: "ssh-man"'
assert_contains "$EXPERIMENTAL_CASK" '  binary "#{appdir}/ssh-man.app/Contents/MacOS/ssh-man"'
assert_before \
  "$EXPERIMENTAL_CASK" \
  '  conflicts_with cask: "ssh-man"' \
  '  depends_on :macos'
cmp -s "$STABLE_SOURCE" "$ROOT_DIR/scripts/testdata/ssh-man.cask.rb" ||
  fail "rendering the experimental cask should not modify the stable source"

cp "$EXPERIMENTAL_CASK" "$PROMOTED_CASK"
ruby "$UPDATER" \
  stable \
  1.6.0 \
  "$STABLE_SHA" \
  "$PROMOTED_CASK" \
  "$PROMOTED_CASK"

assert_contains "$PROMOTED_CASK" 'cask "ssh-man" do'
assert_contains "$PROMOTED_CASK" '  version "1.6.0"'
assert_contains "$PROMOTED_CASK" "  sha256 \"$STABLE_SHA\""
assert_contains "$PROMOTED_CASK" '  url "https://github.com/ericwooley/ssh-man/releases/download/1.6.0/ssh-man.dmg"'
assert_contains "$PROMOTED_CASK" '  conflicts_with cask: "ssh-man@experimental"'
assert_before \
  "$PROMOTED_CASK" \
  '  conflicts_with cask: "ssh-man@experimental"' \
  '  depends_on :macos'

assert_rejected "unknown channel" \
  nightly 1.7.0 "$EXPERIMENTAL_SHA" "$STABLE_SOURCE" "$TEST_ROOT/nightly.rb"
assert_rejected "non-semantic version" \
  stable latest "$STABLE_SHA" "$STABLE_SOURCE" "$TEST_ROOT/latest.rb"
assert_rejected "invalid SHA256" \
  stable 1.7.0 not-a-sha "$STABLE_SOURCE" "$TEST_ROOT/bad-sha.rb"

MALFORMED_SOURCE="$TEST_ROOT/malformed.rb"
grep -v '^  url ' "$STABLE_SOURCE" >"$MALFORMED_SOURCE"
assert_rejected "missing URL stanza" \
  stable 1.7.0 "$STABLE_SHA" "$MALFORMED_SOURCE" "$TEST_ROOT/malformed-output.rb"

printf 'Homebrew cask update tests passed.\n'
