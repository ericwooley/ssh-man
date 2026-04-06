# Implementation Plan: SSH Connection Manager MVP

**Branch**: `001-ssh-connection-manager` | **Date**: 2026-04-06 | **Spec**: [/home/ericwooley/ssh-man/specs/001-ssh-connection-manager/spec.md](/home/ericwooley/ssh-man/specs/001-ssh-connection-manager/spec.md)
**Input**: Feature specification from `/specs/001-ssh-connection-manager/spec.md`

## Summary

Build a greenfield Linux/macOS desktop SSH manager MVP using Go, Wails, plain Svelte, and a single shared CSS file. The MVP will let a single local user save SSH servers and nested connection configurations, start and stop local forward or SOCKS sessions, auto-reconnect after transient disconnects, unlock encrypted SSH keys during manual start, launch an installed browser through a running SOCKS proxy, persist app-managed data in a per-user SQLite database, and provide dark/light themes with platform-native baseline accessibility.

## Technical Context

**Language/Version**: Go 1.24.x  
**Primary Dependencies**: Wails v2, plain Svelte 5 (no SvelteKit), SQLite via `github.com/mattn/go-sqlite3`  
**Storage**: Embedded per-user SQLite database stored in the standard OS application configuration/data location; live session state remains in memory  
**Testing**: `go test ./...`, frontend unit checks for critical UI state and accessibility behavior, integration tests for persistence and session orchestration, smoke validation for Linux/macOS app startup and browser-launch flow  
**Quality Gates**: `gofmt -w .`, `go vet ./...`, `go test ./...`, `npm run build`, `wails doctor`, `wails build -clean`  
**Target Platform**: Linux and macOS  
**Project Type**: desktop-app (Wails)  
**Frontend Style Rules**: Plain Svelte only, shared global stylesheet at `frontend/src/app.css`, explicit classes only, no CSS modules, no scoped component styles, no runtime-generated styling  
**Performance Goals**: Primary screen renders and saved-list interactions feel immediate for up to 50 saved connection configurations; browser launch action begins within 15 seconds in successful cases; reconnect attempts start within 2 seconds of disconnect detection  
**Constraints**: MVP scope only; single-user local app; no cloud sync; no passphrase storage inside the app; no background daemons outside the app process; on-demand browser discovery; reconnect retries stop only on user stop or non-transient failure requiring user action  
**Scale/Scope**: One desktop app, one local user profile, dozens of saved servers/configurations, a small number of simultaneous active sessions, and only the flows required by spec priorities P1-P3

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Gate

- [x] Runtime stays Go + Wails; any alternate runtime or heavyweight dependency is justified.
- [x] Frontend stays on plain Svelte without SvelteKit or alternate UI frameworks.
- [x] Linux and macOS behavior, packaging, and platform-specific risks are explicitly addressed.
- [x] Core logic lives in small, focused Go packages; Wails bindings and UI adapters remain thin.
- [x] Go package boundaries, context usage, error handling, and concurrency behavior are explicit and idiomatic.
- [x] UI styling uses one shared CSS file with explicit classes; no CSS modules, scoped component CSS, or runtime-generated styling are introduced.
- [x] Verification includes formatting, static analysis, automated tests, plus build or smoke checks for affected platform-sensitive work.
- [x] New native integrations, shell-outs, or background services are minimized and documented.

### Post-Design Gate

- [x] Runtime remains Go + Wails with plain Svelte, and the design keeps browser discovery and launch behind narrow platform adapters.
- [x] Linux/macOS parity is addressed through OS-path and browser-launch adapters, while user-visible workflows stay equivalent on both platforms.
- [x] Core logic is separated into domain, persistence, SSH session orchestration, and platform integration packages; Wails bindings remain a thin transport layer.
- [x] Go design uses explicit context-aware session operations, actionable errors, and a small session state model suitable for reconnect and credential handling.
- [x] UI work is constrained to plain Svelte and one shared stylesheet at `frontend/src/app.css`.
- [x] Verification plan covers formatting, vet, automated tests, frontend build checks, packaging validation, and accessibility-focused smoke checks.
- [x] Native surface area stays small by using embedded SQLite, direct SSH orchestration in-process, and minimal OS-specific adapters for config paths and browser launch.

## Project Structure

### Documentation (this feature)

```text
specs/001-ssh-connection-manager/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── app-bindings.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
└── app/
    └── main.go

internal/
├── app/
│   ├── bootstrap/
│   └── bindings/
├── domain/
│   ├── config/
│   ├── preferences/
│   ├── server/
│   └── session/
├── platform/
│   ├── browser/
│   └── paths/
├── sqlite/
└── ssh/
    ├── auth/
    └── tunnel/

frontend/
├── src/
│   ├── components/
│   ├── lib/
│   ├── routes/
│   └── app.css
└── package.json

build/
└── [wails packaging assets]

tests/
├── integration/
└── smoke/
```

**Structure Decision**: Use the desktop-app structure above. Keep business rules and persistence orchestration in Go under `internal/`, expose only feature-focused Wails bindings from `internal/app/bindings`, keep Svelte limited to presentation/state wiring in `frontend/src/`, and route all styling through `frontend/src/app.css`.

## Phase 0 Research Decisions

- Embedded per-user SQLite is the persisted source of truth for saved servers, connection configurations, reconnect preferences, theme preference, and lightweight session history.
- Active tunnel runtime state stays in memory only and is not restored as a live session after restart.
- Installed browsers are discovered on demand and may be cached only in memory during the current app run.
- Auto-reconnect uses keepalive-driven disconnect detection and capped exponential backoff with jitter for transient failures.
- Encrypted SSH keys prompt for passphrases only during manual start or explicit retry; reconnect attempts reuse already unlocked credentials when available and otherwise transition to a user-attention state.
- Platform-native accessibility baseline means keyboard-complete primary workflows, visible focus states, discernible labels including icon actions, and readable light/dark themes.

## Phase 1 Design Overview

### MVP Scope

- Include: saved servers, nested saved connection configurations, local forwards, SOCKS proxy sessions, start/stop controls, session status display, reconnect behavior, encrypted key unlock flow, on-demand browser discovery, one-click browser launch through SOCKS, dark/light themes, app-managed persistence in SQLite, and accessibility baseline for primary workflows.
- Exclude: remote shell terminal UI, SFTP/file transfer, team sharing, sync across machines, configuration import/export, advanced SSH config parsing, jump hosts, agent management UI, per-browser launch profiles, and restoring active sessions after restart.

### Design Decisions

- Persist only durable app data in SQLite and keep runtime connection/session state in memory.
- Model saved tunnel definitions separately from live connection sessions so the UI can show saved data even when nothing is running.
- Treat each saved configuration as an independently startable unit; a given configuration may contain one SOCKS endpoint or one or more local forward rules, but a session only enters `Connected` when all requested binds succeed.
- Use explicit platform adapters for config-path resolution and browser discovery/launch to contain Linux/macOS differences.
- Keep the browser-launch feature behind validation that requires a running SOCKS session and an installed supported browser.
- Use one application shell with a server list, per-server configuration list, detail/editor panel, session status surface, browser-launch control, and theme toggle.

### Planned Verification

- Backend unit tests for domain validation, reconnect policy, persistence mapping, and adapter error handling.
- Integration tests for SQLite persistence, configuration CRUD, session lifecycle transitions, and unsupported/invalid launch conditions.
- Frontend tests for primary workflow rendering, keyboard reachability, focus visibility, labeling, and theme persistence.
- Smoke checks for Linux/macOS app startup, SQLite initialization in the OS-specific data location, and Wails/browser-launch happy-path validation.

## Complexity Tracking

No constitution violations or justified exceptions are required for this MVP plan.
