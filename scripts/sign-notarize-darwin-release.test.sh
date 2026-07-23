#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-sign-notarize-test.XXXXXX")"

cleanup() {
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

APP_PATH="$TEST_DIR/ssh-man.app"
DMG_PATH="$TEST_DIR/ssh-man.dmg"
FAKE_BIN="$TEST_DIR/bin"
TOOL_LOG="$TEST_DIR/tool.log"
mkdir -p "$APP_PATH/Contents/MacOS" "$FAKE_BIN"
touch "$APP_PATH/Contents/MacOS/ssh-man" "$TEST_DIR/signing.keychain-db"
chmod +x "$APP_PATH/Contents/MacOS/ssh-man"

cat >"$APP_PATH/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key>
  <string>tech.moonpixels.ssh-man</string>
  <key>CFBundleVersion</key>
  <string>1.2.3</string>
  <key>CFBundleShortVersionString</key>
  <string>1.2.3</string>
</dict>
</plist>
PLIST

cat >"$FAKE_BIN/codesign" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail

printf 'codesign' >>"$FAKE_TOOL_LOG"
printf ' <%s>' "$@" >>"$FAKE_TOOL_LOG"
printf '\n' >>"$FAKE_TOOL_LOG"

case " $* " in
  *" --display "*)
    printf 'Authority=Developer ID Application: Moonpixels (TEAM12345)\n' >&2
    printf 'TeamIdentifier=TEAM12345\n' >&2
    printf 'flags=0x10000(runtime)\n' >&2
    ;;
esac
FAKE

cat >"$FAKE_BIN/hdiutil" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail

printf 'hdiutil' >>"$FAKE_TOOL_LOG"
printf ' <%s>' "$@" >>"$FAKE_TOOL_LOG"
printf '\n' >>"$FAKE_TOOL_LOG"

case "${1:-}" in
  create)
    output_path="${!#}"
    printf 'fake dmg\n' >"$output_path"
    ;;
  verify) ;;
  *) exit 90 ;;
esac
FAKE

cat >"$FAKE_BIN/xcrun" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail

printf 'xcrun' >>"$FAKE_TOOL_LOG"
printf ' <%s>' "$@" >>"$FAKE_TOOL_LOG"
printf '\n' >>"$FAKE_TOOL_LOG"

tool="${1:-}"
command_name="${2:-}"
case "$tool:$command_name" in
  notarytool:submit)
    printf '{"id":"11111111-2222-3333-4444-555555555555","status":"%s"}\n' "${FAKE_NOTARY_STATUS:-Accepted}"
    ;;
  notarytool:log)
    printf '{"issues":[]}\n' >"${4:?missing notary log output path}"
    ;;
  stapler:staple|stapler:validate) ;;
  *) exit 91 ;;
esac
FAKE

cat >"$FAKE_BIN/spctl" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail

printf 'spctl' >>"$FAKE_TOOL_LOG"
printf ' <%s>' "$@" >>"$FAKE_TOOL_LOG"
printf '\n' >>"$FAKE_TOOL_LOG"
FAKE

chmod +x "$FAKE_BIN"/*

run_signing() {
  local notary_status="$1"
  local output_path="$2"

  APPLE_SIGNING_IDENTITY='Developer ID Application: Moonpixels (TEAM12345)' \
  APPLE_TEAM_ID='TEAM12345' \
  APPLE_NOTARY_KEYCHAIN_PROFILE='ssh-man-notary' \
  APPLE_NOTARY_KEYCHAIN_PATH="$TEST_DIR/signing.keychain-db" \
  CODESIGN_BIN="$FAKE_BIN/codesign" \
  HDIUTIL_BIN="$FAKE_BIN/hdiutil" \
  XCRUN_BIN="$FAKE_BIN/xcrun" \
  SPCTL_BIN="$FAKE_BIN/spctl" \
  FAKE_TOOL_LOG="$TOOL_LOG" \
  FAKE_NOTARY_STATUS="$notary_status" \
    bash "$ROOT_DIR/scripts/sign-notarize-darwin-release.sh" \
      1.2.3 "$APP_PATH" "$output_path"
}

run_signing Accepted "$DMG_PATH" >"$TEST_DIR/success.log" 2>&1
[ -f "$DMG_PATH" ] || fail "accepted notarization should produce a DMG"
grep -Fq 'Release artifact signed, notarized, and stapled' "$TEST_DIR/success.log" || fail "success output missing"

app_sign_line="$(grep -F 'codesign <--force> <--options> <runtime>' "$TOOL_LOG" | head -1)"
[ -n "$app_sign_line" ] || fail "app was not signed with hardened runtime"
case "$app_sign_line" in
  *'<--timestamp>'*'<--sign>'*'<Developer ID Application: Moonpixels (TEAM12345)>'*) ;;
  *) fail "app signing omitted the timestamp or Developer ID identity" ;;
esac
case "$app_sign_line" in
  *'<--deep>'*) fail "deprecated deep signing must not be used" ;;
esac

grep -Fq 'codesign <--force> <--timestamp> <--sign> <Developer ID Application: Moonpixels (TEAM12345)>' "$TOOL_LOG" || fail "DMG was not signed"
grep -Fq 'xcrun <notarytool> <submit>' "$TOOL_LOG" || fail "DMG was not submitted for notarization"
grep -Fq '<--wait>' "$TOOL_LOG" || fail "notarization submission did not wait for a result"
grep -Fq '<--timeout> <2h>' "$TOOL_LOG" || fail "notarization submission did not allow Apple up to two hours"
grep -Fq '<--keychain-profile> <ssh-man-notary>' "$TOOL_LOG" || fail "notary keychain profile was not used"
grep -Fq 'xcrun <stapler> <staple>' "$TOOL_LOG" || fail "notarization ticket was not stapled"
grep -Fq 'xcrun <stapler> <validate>' "$TOOL_LOG" || fail "stapled ticket was not validated"
grep -Fq 'spctl <--assess> <--type> <open>' "$TOOL_LOG" || fail "Gatekeeper assessment was not run"

: >"$TOOL_LOG"
INVALID_DMG_PATH="$TEST_DIR/invalid.dmg"
set +e
run_signing Invalid "$INVALID_DMG_PATH" >"$TEST_DIR/invalid.log" 2>&1
invalid_status=$?
set -e
[ "$invalid_status" -ne 0 ] || fail "invalid notarization must fail"
grep -Fq 'Notarization finished with status Invalid' "$TEST_DIR/invalid.log" || fail "invalid status was not reported"
if grep -Fq 'xcrun <stapler> <staple>' "$TOOL_LOG"; then
  fail "an invalid notarization must not be stapled"
fi

printf 'sign-notarize-darwin-release tests passed\n'
