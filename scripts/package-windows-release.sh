#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-}"
WAILS_VERSION="v2.13.0"
WINDOWS_BINARY="$ROOT_DIR/build/bin/ssh-man.exe"
INSTALLER_DIR="$ROOT_DIR/build/windows/installer"
STABLE_BUILD_INSTALLER="$ROOT_DIR/build/bin/ssh-man-windows-amd64-installer.exe"
EXPERIMENTAL_BUILD_INSTALLER="$ROOT_DIR/build/bin/ssh-man-experimental-windows-amd64-installer.exe"
STABLE_DIST_INSTALLER="$ROOT_DIR/dist/ssh-man-windows-amd64-installer.exe"
EXPERIMENTAL_DIST_INSTALLER="$ROOT_DIR/dist/ssh-man-experimental-windows-amd64-installer.exe"
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
require_command makensis
require_command x86_64-w64-mingw32-gcc

if [ ! -f "$INSTALLER_DIR/project.nsi" ]; then
  printf 'Missing Windows installer template: %s\n' "$INSTALLER_DIR/project.nsi" >&2
  exit 1
fi

cd "$ROOT_DIR"
mkdir -p "$ROOT_DIR/dist"
rm -f "$STABLE_DIST_INSTALLER" "$EXPERIMENTAL_DIST_INSTALLER"

WAILS_TOOL_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-wails-tool.XXXXXX")"
printf '==> Building host-side Wails tool\n'
CGO_ENABLED=0 \
  GOBIN="$WAILS_TOOL_DIR" \
  go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}"

printf '==> Building Windows application and preparing NSIS assets\n'
CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  SSH_MAN_CHANNEL=stable \
  SSH_MAN_VERSION="$VERSION" \
  "$WAILS_TOOL_DIR/wails" build \
    -platform windows/amd64 \
    -clean \
    -nsis \
    -ldflags "-X ssh-man/internal/buildinfo.Version=$VERSION"

if [ ! -s "$WINDOWS_BINARY" ]; then
  printf 'Expected Windows executable was not created: %s\n' "$WINDOWS_BINARY" >&2
  exit 1
fi

build_installer() {
  local channel="$1"
  local expected_installer="$2"

  printf '==> Building %s Windows installer\n' "$channel"
  (
    cd "$INSTALLER_DIR"
    SSH_MAN_CHANNEL="$channel" \
      SSH_MAN_VERSION="$VERSION" \
      makensis \
        "-DSSH_MAN_CHANNEL=$channel" \
        "-DSSH_MAN_VERSION=$VERSION" \
        "-DARG_WAILS_AMD64_BINARY=$WINDOWS_BINARY" \
        project.nsi
  )

  if [ ! -s "$expected_installer" ]; then
    printf 'Expected %s installer was not created: %s\n' "$channel" "$expected_installer" >&2
    exit 1
  fi
}

build_installer stable "$STABLE_BUILD_INSTALLER"
build_installer experimental "$EXPERIMENTAL_BUILD_INSTALLER"

cp "$STABLE_BUILD_INSTALLER" "$STABLE_DIST_INSTALLER"
cp "$EXPERIMENTAL_BUILD_INSTALLER" "$EXPERIMENTAL_DIST_INSTALLER"

printf '==> Windows release artifacts ready\n'
printf '    %s\n' "$STABLE_DIST_INSTALLER"
printf '    %s\n' "$EXPERIMENTAL_DIST_INSTALLER"
