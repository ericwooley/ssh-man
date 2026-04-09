#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-}"
WAILS_VERSION="v2.12.0"
APP_PATH="$ROOT_DIR/build/bin/ssh-man.app"
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

require_command go
require_command pnpm
require_command hdiutil

cd "$ROOT_DIR"

printf '==> Downloading Go modules\n'
go mod download

printf '==> Installing frontend dependencies\n'
pnpm install --dir frontend --frozen-lockfile

printf '==> Building macOS app bundle\n'
go run github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION} build -platform darwin/universal -clean

if [ ! -d "$APP_PATH" ]; then
  printf 'Expected app bundle was not created: %s\n' "$APP_PATH" >&2
  exit 1
fi

printf '==> Updating app bundle version to %s\n' "$VERSION"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $VERSION" "$APP_PATH/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$APP_PATH/Contents/Info.plist"

printf '==> Creating DMG\n'
rm -rf "$ROOT_DIR/dist"
mkdir -p "$ROOT_DIR/dist"
rm -f "$DMG_PATH"
hdiutil create -volname "ssh-man" -srcfolder "$APP_PATH" -ov -format UDZO "$DMG_PATH"

printf '==> Release artifact ready: %s\n' "$DMG_PATH"
