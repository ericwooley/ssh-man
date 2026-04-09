#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-}"
TAG=""

require_command() {
  local cmd="$1"

  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$cmd" >&2
    exit 1
  fi
}

if [ -z "$VERSION" ]; then
  printf 'Usage: %s <version>\n' "$(basename "$0")" >&2
  printf 'Example: %s 1.0.0\n' "$(basename "$0")" >&2
  exit 1
fi

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Version must match x.y.z, got: %s\n' "$VERSION" >&2
  exit 1
fi

TAG="$VERSION"

require_command git

cd "$ROOT_DIR"

if [ -n "$(git status --porcelain)" ]; then
  printf 'Working tree is not clean. Commit or stash changes before tagging.\n' >&2
  exit 1
fi

if git rev-parse "$TAG" >/dev/null 2>&1; then
  printf 'Tag already exists locally: %s\n' "$TAG" >&2
  exit 1
fi

if git ls-remote --tags origin "refs/tags/$TAG" | grep -q .; then
  printf 'Tag already exists on origin: %s\n' "$TAG" >&2
  exit 1
fi

printf '==> Building current OS\n'
./scripts/build-current-os.sh

printf '==> Creating tag %s\n' "$TAG"
git tag "$TAG"

printf '==> Pushing tag %s\n' "$TAG"
git push origin "$TAG"

printf '==> Done\n'
printf 'Pushed tag %s\n' "$TAG"
