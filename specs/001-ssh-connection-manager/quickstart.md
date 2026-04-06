# Quickstart: SSH Connection Manager MVP

## Purpose

This document describes how to stand up, validate, and manually exercise the MVP feature scope on Linux and macOS during development.

## Preconditions

- Go 1.24.x installed.
- Node.js and npm installed for the plain Svelte frontend.
- Wails v2 CLI installed and passing `wails doctor`.
- SQLite available through the Go driver dependency during build.
- Access to at least one reachable SSH server for manual validation.
- At least one supported browser installed for SOCKS launch validation.

## Initial Setup

1. Initialize the Go module and Wails desktop app structure.
2. Initialize the plain Svelte frontend without SvelteKit.
3. Create the shared stylesheet at `frontend/src/app.css` and route all UI classes through it.
4. Add SQLite-backed persistence in the standard OS-specific application configuration or data location.
5. Configure Linux and macOS-specific path and browser adapters.

## Required Validation Commands

Run these commands from the repository root as the baseline quality gate:

```bash
wails doctor
gofmt -w .
go vet ./...
go test ./...
npm run build
wails build -clean
```

If frontend type or accessibility checks are added during implementation, include them in the local validation sequence before completion.

## Project Scripts

- `./scripts/build-current-os.sh`: bootstrap-from-clone build for the current host OS. It downloads Go modules, installs frontend dependencies, selects the correct Wails build invocation for the host platform, and writes the packaged binary to `build/bin/`.
- `./scripts/validate.sh`: runs `gofmt`, `go vet`, `go test`, `frontend` tests, and the frontend production build from the repo root.
- `./scripts/wails-build-linux.sh`: runs the Linux Wails package build with the required `webkit2_41` tag.
- `npm run validate --prefix frontend`: runs the frontend test and build steps only.

## Current Repository Status

- `gofmt -w main.go cmd/app internal tests`: passing as of 2026-04-06.
- `go vet ./...`: passing as of 2026-04-06.
- `go test ./...`: passing as of 2026-04-06.
- `npm run test` from `frontend/`: passing as of 2026-04-06.
- `npm run build` from `frontend/`: passing as of 2026-04-06.
- `./scripts/build-current-os.sh`: passing as of 2026-04-06. Verified from the current clone to `build/bin/ssh-man` in this workspace.
- `./scripts/validate.sh`: passing as of 2026-04-06.
- `wails doctor`: ran on Linux as of 2026-04-06 and still reports missing default WebKit packages unless the host has the GTK/WebKit development packages installed.
- `./scripts/wails-build-linux.sh`: passing as of 2026-04-06. This wraps `wails build -clean -tags webkit2_41` and produced `build/bin/ssh-man` in this workspace.
- macOS validation outcomes are still pending local execution on a macOS machine.

## Local Development Notes

1. Run `go mod tidy` from the repository root after dependency changes.
2. Run `npm install` from `frontend/` before starting the UI locally.
3. Start the frontend shell with `npm run dev` from `frontend/` for UI iteration.
4. Install the Wails CLI before desktop packaging or `wails dev` workflows.
5. From a fresh clone, run `./scripts/build-current-os.sh` to install project dependencies and produce the current platform binary in `build/bin/`.
6. On Linux, use `./scripts/wails-build-linux.sh` when you specifically want the raw Wails command with the required `webkit2_41` build tag.
7. If `wails doctor` is used as an environment check, install the GTK/WebKit development packages that match the target distro.

## Manual MVP Validation Flow

1. Launch the app.
2. Create a server entry with valid SSH connection details.
3. Add one local-forward configuration and one SOCKS configuration under that server.
4. Restart the app and confirm the saved data reloads from the OS-appropriate location.
5. Start the local-forward configuration and confirm the UI shows `Connected` only after the bind succeeds.
6. Start the SOCKS configuration and confirm the UI shows the running proxy endpoint.
7. Interrupt connectivity or suspend the connection source and confirm the UI transitions to `Reconnecting`, then back to `Connected` or to a clear failure state.
8. Use an encrypted SSH key during manual start and confirm the app requests an unlock step without requiring an unencrypted key.
9. Open the browser selector for the running SOCKS session and confirm installed supported browsers are listed.
10. Launch a selected browser through the SOCKS session and confirm the action succeeds or returns a clear failure reason.
11. Toggle between light and dark themes and verify readable text, visible focus states, and keyboard-complete access to primary actions.
12. Trigger invalid inputs such as duplicate/conflicting local ports, missing browser availability, or unavailable config storage, and confirm the app provides actionable error feedback.

## Platform Validation Notes

- Linux: `wails build -clean -tags webkit2_41` was executed successfully on Ubuntu 24.04 in this workspace and produced `build/bin/ssh-man`. The helper script `./scripts/wails-build-linux.sh` now captures that build flow. `wails doctor` may still report missing default WebKit packages depending on the installed distro packages.
- macOS: validate the equivalent flows, including app path resolution and browser launch behavior with the same user-visible workflow. This remains pending local execution on macOS.
- Both platforms must support the same saved-data, session-management, browser-selection, theme, and accessibility flows even if OS-native prompts or browser names differ.
