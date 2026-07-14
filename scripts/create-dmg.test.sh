#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-create-dmg-test.XXXXXX")"

cleanup() {
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

SOURCE_DIR="$TEST_DIR/source"
FAKE_HDIUTIL="$TEST_DIR/hdiutil"
mkdir -p "$SOURCE_DIR"
printf 'fixture\n' >"$SOURCE_DIR/file.txt"

cat >"$FAKE_HDIUTIL" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail

command_name="${1:-}"
shift || true

if [ "$command_name" = "verify" ]; then
  exit "${FAKE_VERIFY_STATUS:-0}"
fi

if [ "$command_name" != "create" ]; then
  printf 'unexpected fake hdiutil command: %s\n' "$command_name" >&2
  exit 90
fi

count=0
if [ -f "$FAKE_HDIUTIL_STATE" ]; then
  count="$(cat "$FAKE_HDIUTIL_STATE")"
fi
count=$((count + 1))
printf '%s\n' "$count" >"$FAKE_HDIUTIL_STATE"

output_path="${!#}"
case "${FAKE_HDIUTIL_MODE:-success}" in
  transient)
    if [ "$count" -lt 3 ]; then
      printf 'hdiutil: create failed - Resource busy\n' >&2
      exit 1
    fi
    ;;
  always_busy)
    printf 'hdiutil: create failed - Resource busy\n' >&2
    exit 1
    ;;
  permanent)
    printf 'hdiutil: create failed - invalid source\n' >&2
    exit 23
    ;;
esac

printf 'fake dmg\n' >"$output_path"
FAKE
chmod +x "$FAKE_HDIUTIL"

run_create() {
  local mode="$1"
  local output_path="$2"
  local state_path="$3"
  local verify_status="${4:-0}"

  FAKE_HDIUTIL_MODE="$mode" \
  FAKE_HDIUTIL_STATE="$state_path" \
  FAKE_VERIFY_STATUS="$verify_status" \
  HDIUTIL_BIN="$FAKE_HDIUTIL" \
  DMG_CREATE_MAX_ATTEMPTS=5 \
  DMG_CREATE_RETRY_DELAY_SECONDS=0 \
    bash "$ROOT_DIR/scripts/create-dmg.sh" "$SOURCE_DIR" "ssh-man" "$output_path"
}

transient_output="$TEST_DIR/transient.dmg"
transient_state="$TEST_DIR/transient.state"
run_create transient "$transient_output" "$transient_state" >"$TEST_DIR/transient.log" 2>&1
[ -f "$transient_output" ]
[ "$(cat "$transient_state")" -eq 3 ]
grep -Fq 'retrying in 0 second(s) (2/5)' "$TEST_DIR/transient.log"

permanent_output="$TEST_DIR/permanent.dmg"
permanent_state="$TEST_DIR/permanent.state"
set +e
run_create permanent "$permanent_output" "$permanent_state" >"$TEST_DIR/permanent.log" 2>&1
permanent_status=$?
set -e
[ "$permanent_status" -eq 23 ]
[ "$(cat "$permanent_state")" -eq 1 ]
[ ! -e "$permanent_output" ]
if grep -Fq 'retrying' "$TEST_DIR/permanent.log"; then
  printf 'permanent hdiutil failures must not be retried\n' >&2
  exit 1
fi

busy_output="$TEST_DIR/busy.dmg"
busy_state="$TEST_DIR/busy.state"
set +e
run_create always_busy "$busy_output" "$busy_state" >"$TEST_DIR/busy.log" 2>&1
busy_status=$?
set -e
[ "$busy_status" -eq 1 ]
[ "$(cat "$busy_state")" -eq 5 ]
[ ! -e "$busy_output" ]
grep -Fq 'remained busy after 5 attempts' "$TEST_DIR/busy.log"

verify_output="$TEST_DIR/verify.dmg"
verify_state="$TEST_DIR/verify.state"
set +e
run_create success "$verify_output" "$verify_state" 17 >"$TEST_DIR/verify.log" 2>&1
verify_status=$?
set -e
[ "$verify_status" -eq 1 ]
[ "$(cat "$verify_state")" -eq 1 ]
[ ! -e "$verify_output" ]

printf 'create-dmg tests passed\n'
