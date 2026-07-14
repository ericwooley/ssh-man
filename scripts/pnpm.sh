#!/usr/bin/env sh

set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR/frontend"

if command -v corepack >/dev/null 2>&1; then
  exec corepack pnpm "$@"
fi

if command -v pnpm >/dev/null 2>&1; then
  exec pnpm "$@"
fi

printf 'Missing required command: install Corepack or pnpm\n' >&2
exit 127
