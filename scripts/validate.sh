#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_command() {
  local cmd="$1"

  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$cmd" >&2
    exit 1
  fi
}

cd "$ROOT_DIR"

require_command pnpm

gofmt -w main.go cmd/app internal tests
pnpm install --dir frontend --frozen-lockfile
pnpm --dir frontend run check:css
pnpm --dir frontend run build
go vet ./...
go test ./...
pnpm --dir frontend run test
