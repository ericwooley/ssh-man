#!/usr/bin/env bash

set -euo pipefail

SOURCE_FOLDER="${1:-}"
VOLUME_NAME="${2:-}"
OUTPUT_PATH="${3:-}"
HDIUTIL_BIN="${HDIUTIL_BIN:-hdiutil}"
MAX_ATTEMPTS="${DMG_CREATE_MAX_ATTEMPTS:-5}"
BASE_DELAY_SECONDS="${DMG_CREATE_RETRY_DELAY_SECONDS:-2}"
TEMP_ROOT="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"

if [ -z "$SOURCE_FOLDER" ] || [ -z "$VOLUME_NAME" ] || [ -z "$OUTPUT_PATH" ]; then
  printf 'Usage: %s <source-folder> <volume-name> <output-path>\n' "$(basename "$0")" >&2
  exit 2
fi

if [ ! -d "$SOURCE_FOLDER" ]; then
  printf 'DMG source folder does not exist: %s\n' "$SOURCE_FOLDER" >&2
  exit 2
fi

case "$MAX_ATTEMPTS" in
  ''|*[!0-9]*|0)
    printf 'DMG_CREATE_MAX_ATTEMPTS must be a positive integer, got: %s\n' "$MAX_ATTEMPTS" >&2
    exit 2
    ;;
esac

case "$BASE_DELAY_SECONDS" in
  ''|*[!0-9]*)
    printf 'DMG_CREATE_RETRY_DELAY_SECONDS must be a non-negative integer, got: %s\n' "$BASE_DELAY_SECONDS" >&2
    exit 2
    ;;
esac

if ! command -v "$HDIUTIL_BIN" >/dev/null 2>&1; then
  printf 'Missing required command: %s\n' "$HDIUTIL_BIN" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d "$TEMP_ROOT/ssh-man-dmg.XXXXXX")"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

mkdir -p "$(dirname "$OUTPUT_PATH")"
rm -f "$OUTPUT_PATH"

attempt=1
while [ "$attempt" -le "$MAX_ATTEMPTS" ]; do
  candidate_path="$WORK_DIR/ssh-man-attempt-$attempt.dmg"
  log_path="$WORK_DIR/hdiutil-attempt-$attempt.log"

  rm -f "$candidate_path" "$log_path"
  sync

  set +e
  "$HDIUTIL_BIN" create \
    -verbose \
    -volname "$VOLUME_NAME" \
    -srcfolder "$SOURCE_FOLDER" \
    -nospotlight \
    -format UDZO \
    "$candidate_path" >"$log_path" 2>&1
  status=$?
  set -e

  cat "$log_path"

  if [ "$status" -eq 0 ]; then
    mv "$candidate_path" "$OUTPUT_PATH"
    if ! "$HDIUTIL_BIN" verify "$OUTPUT_PATH"; then
      rm -f "$OUTPUT_PATH"
      exit 1
    fi
    exit 0
  fi

  rm -f "$candidate_path"
  if ! grep -Fq 'Resource busy' "$log_path"; then
    exit "$status"
  fi

  if [ "$attempt" -eq "$MAX_ATTEMPTS" ]; then
    printf 'hdiutil remained busy after %s attempts.\n' "$MAX_ATTEMPTS" >&2
    exit "$status"
  fi

  delay_seconds=$((BASE_DELAY_SECONDS * (1 << (attempt - 1))))
  printf 'hdiutil reported Resource busy; retrying in %s second(s) (%s/%s).\n' \
    "$delay_seconds" "$attempt" "$MAX_ATTEMPTS" >&2
  sleep "$delay_seconds"
  attempt=$((attempt + 1))
done
