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

gofmt -w main.go cmd/app internal tests
"$ROOT_DIR/scripts/pnpm.sh" install --frozen-lockfile
"$ROOT_DIR/scripts/pnpm.sh" run build
go vet ./...
go test ./...
"$ROOT_DIR/scripts/pnpm.sh" run test
bash "$ROOT_DIR/scripts/validate-commit-message.test.sh"
bash "$ROOT_DIR/scripts/release-plan.test.sh"
bash "$ROOT_DIR/scripts/create-dmg.test.sh"
bash "$ROOT_DIR/scripts/sign-notarize-darwin-release.test.sh"
