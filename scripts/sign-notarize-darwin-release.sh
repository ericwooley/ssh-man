#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-}"
APP_PATH="${2:-$ROOT_DIR/build/bin/ssh-man.app}"
DMG_PATH="${3:-$ROOT_DIR/dist/ssh-man.dmg}"
EXPECTED_BUNDLE_ID="tech.moonpixels.ssh-man"

CODESIGN_BIN="${CODESIGN_BIN:-codesign}"
HDIUTIL_BIN="${HDIUTIL_BIN:-hdiutil}"
PLISTBUDDY_BIN="${PLISTBUDDY_BIN:-/usr/libexec/PlistBuddy}"
PLUTIL_BIN="${PLUTIL_BIN:-plutil}"
SPCTL_BIN="${SPCTL_BIN:-spctl}"
XCRUN_BIN="${XCRUN_BIN:-xcrun}"

SIGNING_IDENTITY="${APPLE_SIGNING_IDENTITY:-}"
TEAM_ID="${APPLE_TEAM_ID:-}"
NOTARY_KEYCHAIN_PROFILE="${APPLE_NOTARY_KEYCHAIN_PROFILE:-}"
NOTARY_KEYCHAIN_PATH="${APPLE_NOTARY_KEYCHAIN_PATH:-}"

require_command() {
  local cmd="$1"

  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$cmd" >&2
    exit 1
  fi
}

require_value() {
  local name="$1"
  local value="$2"

  if [ -z "$value" ]; then
    printf 'Missing required environment value: %s\n' "$name" >&2
    exit 1
  fi
}

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Usage: %s <version> [app-path] [dmg-path]\n' "$(basename "$0")" >&2
  printf 'Version must match x.y.z, got: %s\n' "$VERSION" >&2
  exit 2
fi

if [ ! -d "$APP_PATH" ]; then
  printf 'App bundle does not exist: %s\n' "$APP_PATH" >&2
  exit 1
fi

INFO_PLIST="$APP_PATH/Contents/Info.plist"
APP_EXECUTABLE="$APP_PATH/Contents/MacOS/ssh-man"
if [ ! -f "$INFO_PLIST" ]; then
  printf 'App bundle Info.plist does not exist: %s\n' "$INFO_PLIST" >&2
  exit 1
fi
if [ ! -x "$APP_EXECUTABLE" ]; then
  printf 'App bundle executable is missing or not executable: %s\n' "$APP_EXECUTABLE" >&2
  exit 1
fi

require_value APPLE_SIGNING_IDENTITY "$SIGNING_IDENTITY"
require_value APPLE_TEAM_ID "$TEAM_ID"
require_value APPLE_NOTARY_KEYCHAIN_PROFILE "$NOTARY_KEYCHAIN_PROFILE"
require_value APPLE_NOTARY_KEYCHAIN_PATH "$NOTARY_KEYCHAIN_PATH"

case "$SIGNING_IDENTITY" in
  "Developer ID Application: "*"($TEAM_ID)") ;;
  *)
    printf 'Signing identity does not match Developer ID Application team %s: %s\n' \
      "$TEAM_ID" "$SIGNING_IDENTITY" >&2
    exit 1
    ;;
esac

if [ ! -f "$NOTARY_KEYCHAIN_PATH" ]; then
  printf 'Notary keychain does not exist: %s\n' "$NOTARY_KEYCHAIN_PATH" >&2
  exit 1
fi

require_command "$CODESIGN_BIN"
require_command "$HDIUTIL_BIN"
require_command "$PLISTBUDDY_BIN"
require_command "$PLUTIL_BIN"
require_command "$SPCTL_BIN"
require_command "$XCRUN_BIN"

BUNDLE_ID="$($PLISTBUDDY_BIN -c 'Print :CFBundleIdentifier' "$INFO_PLIST")"
BUNDLE_VERSION="$($PLISTBUDDY_BIN -c 'Print :CFBundleVersion' "$INFO_PLIST")"
SHORT_VERSION="$($PLISTBUDDY_BIN -c 'Print :CFBundleShortVersionString' "$INFO_PLIST")"

if [ "$BUNDLE_ID" != "$EXPECTED_BUNDLE_ID" ]; then
  printf 'Bundle identifier is %s, want %s\n' "$BUNDLE_ID" "$EXPECTED_BUNDLE_ID" >&2
  exit 1
fi
if [ "$BUNDLE_VERSION" != "$VERSION" ] || [ "$SHORT_VERSION" != "$VERSION" ]; then
  printf 'App bundle versions are CFBundleVersion=%s and CFBundleShortVersionString=%s, want %s\n' \
    "$BUNDLE_VERSION" "$SHORT_VERSION" "$VERSION" >&2
  exit 1
fi

printf '==> Signing app bundle with Developer ID\n'
"$CODESIGN_BIN" \
  --force \
  --options runtime \
  --timestamp \
  --sign "$SIGNING_IDENTITY" \
  "$APP_PATH"

"$CODESIGN_BIN" --verify --deep --strict --verbose=2 "$APP_PATH"
SIGNATURE_DETAILS="$($CODESIGN_BIN --display --verbose=4 "$APP_PATH" 2>&1)"
printf '%s\n' "$SIGNATURE_DETAILS"
if ! grep -Fq 'Authority=Developer ID Application:' <<<"$SIGNATURE_DETAILS"; then
  printf 'App signature is not a Developer ID Application signature.\n' >&2
  exit 1
fi
if ! grep -Fq "TeamIdentifier=$TEAM_ID" <<<"$SIGNATURE_DETAILS"; then
  printf 'App signature does not contain expected TeamIdentifier=%s.\n' "$TEAM_ID" >&2
  exit 1
fi
if ! grep -Eq 'flags=.*runtime' <<<"$SIGNATURE_DETAILS"; then
  printf 'App signature does not enable the hardened runtime.\n' >&2
  exit 1
fi

printf '==> Creating signed DMG\n'
mkdir -p "$(dirname "$DMG_PATH")"
rm -f "$DMG_PATH"
HDIUTIL_BIN="$HDIUTIL_BIN" bash "$ROOT_DIR/scripts/create-dmg.sh" \
  "$APP_PATH" "ssh-man" "$DMG_PATH"
"$CODESIGN_BIN" \
  --force \
  --timestamp \
  --sign "$SIGNING_IDENTITY" \
  "$DMG_PATH"
"$CODESIGN_BIN" --verify --strict --verbose=2 "$DMG_PATH"

SUBMISSION_OUTPUT="$(mktemp "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/ssh-man-notary-submit.XXXXXX")"
NOTARY_LOG="$(mktemp "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/ssh-man-notary-log.XXXXXX")"

cleanup() {
  rm -f "$SUBMISSION_OUTPUT" "$NOTARY_LOG"
}
trap cleanup EXIT

printf '==> Submitting DMG to Apple notary service\n'
set +e
"$XCRUN_BIN" notarytool submit "$DMG_PATH" \
  --keychain-profile "$NOTARY_KEYCHAIN_PROFILE" \
  --keychain "$NOTARY_KEYCHAIN_PATH" \
  --wait \
  --timeout 20m \
  --output-format json >"$SUBMISSION_OUTPUT"
submission_exit=$?
set -e
cat "$SUBMISSION_OUTPUT"

SUBMISSION_ID="$($PLUTIL_BIN -extract id raw -o - "$SUBMISSION_OUTPUT" 2>/dev/null || true)"
NOTARY_STATUS="$($PLUTIL_BIN -extract status raw -o - "$SUBMISSION_OUTPUT" 2>/dev/null || true)"

if [ -n "$SUBMISSION_ID" ]; then
  printf '==> Retrieving notarization log for %s\n' "$SUBMISSION_ID"
  "$XCRUN_BIN" notarytool log "$SUBMISSION_ID" "$NOTARY_LOG" \
    --keychain-profile "$NOTARY_KEYCHAIN_PROFILE" \
    --keychain "$NOTARY_KEYCHAIN_PATH"
  cat "$NOTARY_LOG"
fi

if [ "$submission_exit" -ne 0 ]; then
  printf 'Notarization submission failed with exit code %s.\n' "$submission_exit" >&2
  exit "$submission_exit"
fi
if [ "$NOTARY_STATUS" != "Accepted" ]; then
  printf 'Notarization finished with status %s.\n' "${NOTARY_STATUS:-unknown}" >&2
  exit 1
fi

printf '==> Stapling and validating notarization ticket\n'
"$XCRUN_BIN" stapler staple "$DMG_PATH"
"$XCRUN_BIN" stapler validate "$DMG_PATH"
"$CODESIGN_BIN" --verify --strict --verbose=2 "$DMG_PATH"
"$HDIUTIL_BIN" verify "$DMG_PATH"
"$SPCTL_BIN" --assess --type open --context context:primary-signature --verbose=4 "$DMG_PATH"

printf '==> Release artifact signed, notarized, and stapled: %s\n' "$DMG_PATH"
