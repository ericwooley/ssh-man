#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-}"
WAILS_VERSION="v2.12.0"
APP_PATH="$ROOT_DIR/build/bin/ssh-man.app"
APP_EXECUTABLE="$APP_PATH/Contents/MacOS/ssh-man"
DMG_PATH="$ROOT_DIR/dist/ssh-man.dmg"

require_command() {
  local cmd="$1"

  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$cmd" >&2
    exit 1
  fi
}

if [ -z "$VERSION" ]; then
  printf 'Usage: %s <version>\n' "$(basename "$0")" >&2
  exit 1
fi

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Version must match x.y.z, got: %s\n' "$VERSION" >&2
  exit 1
fi

require_command go
require_command hdiutil
require_command codesign
require_command lipo

cd "$ROOT_DIR"

printf '==> Downloading Go modules\n'
go mod download

printf '==> Installing frontend dependencies\n'
"$ROOT_DIR/scripts/pnpm.sh" install --frozen-lockfile

printf '==> Building macOS app bundle\n'
go run github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION} build \
  -platform darwin/universal \
  -clean \
  -ldflags "-X ssh-man/internal/buildinfo.Version=$VERSION"

if [ ! -d "$APP_PATH" ]; then
  printf 'Expected app bundle was not created: %s\n' "$APP_PATH" >&2
  exit 1
fi

if [ ! -x "$APP_EXECUTABLE" ]; then
  printf 'Expected app executable was not created: %s\n' "$APP_EXECUTABLE" >&2
  exit 1
fi

printf '==> Updating app bundle version to %s\n' "$VERSION"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $VERSION" "$APP_PATH/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$APP_PATH/Contents/Info.plist"

printf '==> Re-signing and verifying app bundle\n'
codesign --force --deep --sign - "$APP_PATH"
codesign --verify --deep --strict --verbose=2 "$APP_PATH"

printf '==> Verifying universal app executable\n'
ARCHITECTURES="$(lipo -archs "$APP_EXECUTABLE")"
case " $ARCHITECTURES " in
  *" x86_64 "*) ;;
  *)
    printf 'App executable is missing x86_64 support: %s\n' "$ARCHITECTURES" >&2
    exit 1
    ;;
esac
case " $ARCHITECTURES " in
  *" arm64 "*) ;;
  *)
    printf 'App executable is missing arm64 support: %s\n' "$ARCHITECTURES" >&2
    exit 1
    ;;
esac

run_cli_smoke() {
  local label="$1"
  local expected="$2"
  shift 2

  local output_file
  local pid
  local status
  output_file="$(mktemp)"
  "$APP_EXECUTABLE" "$@" >"$output_file" 2>&1 &
  pid=$!

  for _ in {1..100}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      if wait "$pid"; then
        status=0
      else
        status=$?
      fi

      if [ "$status" -ne 0 ]; then
        printf 'Packaged CLI %s smoke check failed with exit code %s:\n' "$label" "$status" >&2
        cat "$output_file" >&2
        rm -f "$output_file"
        return 1
      fi

      if [ -n "$expected" ] && ! grep -Fq "$expected" "$output_file"; then
        printf 'Packaged CLI %s output did not contain %q:\n' "$label" "$expected" >&2
        cat "$output_file" >&2
        rm -f "$output_file"
        return 1
      fi

      rm -f "$output_file"
      return 0
    fi
    sleep 0.1
  done

  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  printf 'Packaged CLI %s smoke check timed out; CLI arguments may have launched the desktop runtime.\n' "$label" >&2
  cat "$output_file" >&2
  rm -f "$output_file"
  return 1
}

printf '==> Smoke testing packaged CLI\n'
run_cli_smoke "help" "Usage:" --help
run_cli_smoke "version" "$VERSION" version --output json

printf '==> Creating DMG\n'
mkdir -p "$ROOT_DIR/dist"
rm -f "$DMG_PATH"
bash "$ROOT_DIR/scripts/create-dmg.sh" "$APP_PATH" "ssh-man" "$DMG_PATH"

printf '==> Release artifact ready: %s\n' "$DMG_PATH"
