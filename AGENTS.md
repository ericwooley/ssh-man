# ssh-man Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-04-08

## Active Technologies
- Go 1.22.2, Node/npm for frontend build tooling, shell scripts for local validation + Wails v2.12.0, plain Svelte 5.28.2, Vite 6, Vitest 3, SQLite via `github.com/mattn/go-sqlite3`, GitHub Actions, GitHub Releases, Homebrew cask/tap metadata (002-brew-release-automation)
- Existing app data remains unchanged; feature state lives in repository-managed workflow files, release metadata, packaged artifacts, and Homebrew tap cask metadata (002-brew-release-automation)

- Go 1.24.x + Wails v2, plain Svelte 5 (no SvelteKit), SQLite via `github.com/mattn/go-sqlite3` (001-ssh-connection-manager)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.24.x

## Code Style

Go 1.24.x: Follow standard conventions

## Recent Changes
- 002-brew-release-automation: Added Go 1.22.2, Node/npm for frontend build tooling, shell scripts for local validation + Wails v2.12.0, plain Svelte 5.28.2, Vite 6, Vitest 3, SQLite via `github.com/mattn/go-sqlite3`, GitHub Actions, GitHub Releases, Homebrew cask/tap metadata

- 001-ssh-connection-manager: Added Go 1.24.x + Wails v2, plain Svelte 5 (no SvelteKit), SQLite via `github.com/mattn/go-sqlite3`

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
