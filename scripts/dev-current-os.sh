#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OS="$(uname -s)"
WAILS_VERSION="v2.12.0"

cd "$ROOT_DIR"

require_command() {
  local cmd="$1"

  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$cmd" >&2
    exit 1
  fi
}

require_command go
require_command npm

printf '==> Installing frontend dependencies\n'
npm install --prefix frontend

case "$OS" in
  Linux)
    printf '==> Starting Wails dev on Linux with webkit2_41 tag\n'
    go run github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION} dev -tags webkit2_41 "$@"
    ;;
  Darwin)
    printf '==> Starting Wails dev on macOS\n'
    go run github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION} dev "$@"
    ;;
  *)
    printf 'Unsupported OS for this project dev script: %s\n' "$OS" >&2
    exit 1
    ;;
esac
