#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-}"
WAILS_VERSION="v2.13.0"
WINDOWS_BINARY="$ROOT_DIR/build/bin/ssh-man.exe"
DIST_EXECUTABLE="$ROOT_DIR/dist/ssh-man-windows-amd64.exe"
WAILS_CONFIG="$ROOT_DIR/wails.json"
WAILS_CONFIG_BACKUP=""
WAILS_TOOL_DIR=""

restore_wails_config() {
  if [ -n "$WAILS_CONFIG_BACKUP" ] && [ -f "$WAILS_CONFIG_BACKUP" ]; then
    cp "$WAILS_CONFIG_BACKUP" "$WAILS_CONFIG"
    WAILS_CONFIG_BACKUP=""
  fi
}

cleanup() {
  restore_wails_config

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
require_command node
require_command x86_64-w64-mingw32-gcc

if [ ! -f "$WAILS_CONFIG" ]; then
  printf 'Missing Wails project configuration: %s\n' "$WAILS_CONFIG" >&2
  exit 1
fi

cd "$ROOT_DIR"
mkdir -p "$ROOT_DIR/dist"
rm -f "$DIST_EXECUTABLE"

WAILS_TOOL_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-wails-tool.XXXXXX")"
printf '==> Building host-side Wails tool\n'
CGO_ENABLED=0 \
  GOBIN="$WAILS_TOOL_DIR" \
  go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}"

WAILS_CONFIG_BACKUP="$WAILS_TOOL_DIR/wails.json.original"
cp "$WAILS_CONFIG" "$WAILS_CONFIG_BACKUP"
node - "$WAILS_CONFIG" "$VERSION" <<'NODE'
const fs = require("fs");

const [configPath, version] = process.argv.slice(2);
const config = JSON.parse(fs.readFileSync(configPath, "utf8"));
config.info = config.info || {};
config.info.productVersion = version;
fs.writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`);
NODE

printf '==> Building Windows release executable\n'
CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  "$WAILS_TOOL_DIR/wails" build \
    -platform windows/amd64 \
    -clean \
    -ldflags "-X ssh-man/internal/buildinfo.Version=$VERSION"

restore_wails_config

if [ ! -s "$WINDOWS_BINARY" ]; then
  printf 'Expected Windows executable was not created: %s\n' "$WINDOWS_BINARY" >&2
  exit 1
fi

cp "$WINDOWS_BINARY" "$DIST_EXECUTABLE"

printf '==> Windows release executable ready\n'
printf '    %s\n' "$DIST_EXECUTABLE"
