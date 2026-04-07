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

print_linux_dependency_help() {
  cat <<'EOF'
Linux desktop builds need the GTK/WebKit development packages required by Wails.

Ubuntu/Debian example:
  sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev

If your distro ships `webkit2gtk-4.0` instead, install the matching package and
adjust the build tags if needed.
EOF
}

require_command go
require_command npm

printf '==> Downloading Go modules\n'
go mod download

printf '==> Installing frontend dependencies\n'
npm install --prefix frontend

case "$OS" in
	Linux)
	    if ! pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
	      print_linux_dependency_help
	      exit 1
	    fi

	    printf '==> Building Linux desktop app with webkit2_41 tag and production devtools\n'
	    go run github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION} build -clean -devtools -tags webkit2_41 "$@"
	    ;;
	Darwin)
	    printf '==> Building macOS desktop app with production devtools\n'
	    go run github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION} build -clean -devtools "$@"
	    ;;
  *)
    printf 'Unsupported OS for this project build script: %s\n' "$OS" >&2
    exit 1
    ;;
esac

printf '==> Build complete: %s\n' "$ROOT_DIR/build/bin"
