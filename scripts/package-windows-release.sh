#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-}"
WAILS_VERSION="v2.13.0"
WINDOWS_BINARY="$ROOT_DIR/build/bin/ssh-man.exe"
DIST_EXECUTABLE="$ROOT_DIR/dist/ssh-man-windows-amd64.exe"
WAILS_TOOL_DIR=""

cleanup() {
  if [ -n "$WAILS_TOOL_DIR" ] && [ -d "$WAILS_TOOL_DIR" ]; then
    rm -rf "$WAILS_TOOL_DIR"
  fi
}

trap cleanup EXIT

require_command() {
  local command_name="$1"

  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$command_name" >&2
    exit 1
  fi
}

if [ -z "$VERSION" ]; then
  printf 'Usage: %s <version>\n' "$(basename "$0")" >&2
  exit 1
fi

if [[ ! "$VERSION" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  printf 'Version must be a plain semantic version such as 1.7.0: %s\n' "$VERSION" >&2
  exit 1
fi

require_command go
require_command x86_64-w64-mingw32-gcc

cd "$ROOT_DIR"
mkdir -p "$ROOT_DIR/dist"
rm -f "$DIST_EXECUTABLE"

WAILS_TOOL_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-wails-tool.XXXXXX")"
printf '==> Building host-side Wails tool\n'
CGO_ENABLED=0 \
  GOBIN="$WAILS_TOOL_DIR" \
  go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}"

printf '==> Building Windows release executable\n'
CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  "$WAILS_TOOL_DIR/wails" build \
    -platform windows/amd64 \
    -clean \
    -ldflags "-X ssh-man/internal/buildinfo.Version=$VERSION"

if [ ! -s "$WINDOWS_BINARY" ]; then
  printf 'Expected Windows executable was not created: %s\n' "$WINDOWS_BINARY" >&2
  exit 1
fi

cp "$WINDOWS_BINARY" "$DIST_EXECUTABLE"

printf '==> Windows release executable ready\n'
printf '    %s\n' "$DIST_EXECUTABLE"
