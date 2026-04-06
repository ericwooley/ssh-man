#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

gofmt -w main.go cmd/app internal tests
go vet ./...
go test ./...
npm run test --prefix frontend
npm run build --prefix frontend
