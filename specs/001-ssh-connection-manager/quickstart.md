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

- Linux: validate config/data storage path resolution, local port binding, browser discovery, and browser launch behavior for installed browsers on the test machine.
- macOS: validate the equivalent flows, including app path resolution and browser launch behavior with the same user-visible workflow.
- Both platforms must support the same saved-data, session-management, browser-selection, theme, and accessibility flows even if OS-native prompts or browser names differ.
