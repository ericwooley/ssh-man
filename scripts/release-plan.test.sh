#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLANNER="$ROOT_DIR/scripts/plan-release.sh"
RANGE_VALIDATOR="$ROOT_DIR/scripts/validate-commit-range.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-release-plan-test.XXXXXX")"

cleanup() {
  rm -rf "$TEST_ROOT"
}

trap cleanup EXIT

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

new_repo() {
  local name="$1"
  local repo="$TEST_ROOT/$name"

  mkdir -p "$repo"
  git -C "$repo" init -q
  git -C "$repo" config user.name "Release Test"
  git -C "$repo" config user.email "release-test@example.com"
  git -C "$repo" config commit.gpgsign false
  printf 'initial\n' >"$repo/file.txt"
  git -C "$repo" add file.txt
  git -C "$repo" commit -q -m "chore: initialize repository"
  printf '%s\n' "$repo"
}

commit_message() {
  local repo="$1"
  local subject="$2"
  local body="${3:-}"

  printf '%s\n' "$subject" >>"$repo/file.txt"
  git -C "$repo" add file.txt
  if [ -n "$body" ]; then
    git -C "$repo" commit -q -m "$subject" -m "$body"
  else
    git -C "$repo" commit -q -m "$subject"
  fi
}

assert_plan() {
  local label="$1"
  local repo="$2"
  local expected_required="$3"
  local expected_current="$4"
  local expected_type="$5"
  local expected_next="$6"
  local output

  output="$(cd "$repo" && bash "$PLANNER")" || fail "$label planner execution failed"
  grep -Fxq "release_required=$expected_required" <<<"$output" || fail "$label release_required mismatch: $output"
  grep -Fxq "current_version=$expected_current" <<<"$output" || fail "$label current_version mismatch: $output"
  grep -Fxq "release_type=$expected_type" <<<"$output" || fail "$label release_type mismatch: $output"
  grep -Fxq "next_version=$expected_next" <<<"$output" || fail "$label next_version mismatch: $output"
}

patch_repo="$(new_repo patch)"
git -C "$patch_repo" tag 1.2.3
commit_message "$patch_repo" "fix: close a tunnel leak"
assert_plan "patch" "$patch_repo" true 1.2.3 patch 1.2.4

minor_repo="$(new_repo minor)"
git -C "$minor_repo" tag 1.2.3
commit_message "$minor_repo" "fix: close a tunnel leak"
commit_message "$minor_repo" "feat(browser): add quick switching"
assert_plan "minor outranks patch" "$minor_repo" true 1.2.3 minor 1.3.0

major_repo="$(new_repo major)"
git -C "$major_repo" tag 1.2.3
commit_message "$major_repo" "feat(api)!: replace the control protocol"
assert_plan "breaking header" "$major_repo" true 1.2.3 major 2.0.0

footer_repo="$(new_repo footer)"
git -C "$footer_repo" tag 1.2.3
commit_message "$footer_repo" "refactor(storage): replace preferences" "BREAKING CHANGE: preferences require migration"
assert_plan "breaking footer" "$footer_repo" true 1.2.3 major 2.0.0

no_release_repo="$(new_repo no-release)"
git -C "$no_release_repo" tag 1.2.3
commit_message "$no_release_repo" "docs: explain browser profiles"
commit_message "$no_release_repo" "test: cover profile ordering"
assert_plan "no release" "$no_release_repo" false 1.2.3 none ""

first_release_repo="$(new_repo first-release)"
commit_message "$first_release_repo" "feat: add the first user-facing feature"
assert_plan "first release" "$first_release_repo" true 0.0.0 minor 0.1.0

range_repo="$(new_repo commit-range)"
range_base="$(git -C "$range_repo" rev-parse HEAD)"
commit_message "$range_repo" "fix: validate a commit range"
range_head="$(git -C "$range_repo" rev-parse HEAD)"
(cd "$range_repo" && bash "$RANGE_VALIDATOR" "$range_base" "$range_head") >/dev/null || fail "valid commit range should pass"
commit_message "$range_repo" "Invalid commit subject"
invalid_head="$(git -C "$range_repo" rev-parse HEAD)"
if (cd "$range_repo" && bash "$RANGE_VALIDATOR" "$range_head" "$invalid_head") >"$TEST_ROOT/range-output" 2>&1; then
  fail "invalid commit range should fail"
fi
grep -Fq 'Invalid commit:' "$TEST_ROOT/range-output" || fail "invalid range should identify the commit"

printf 'Release planning tests passed.\n'
