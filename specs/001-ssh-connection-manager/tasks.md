---
description: "Task list for SSH Connection Manager MVP"
---

# Tasks: SSH Connection Manager MVP

**Input**: Design documents from `/specs/001-ssh-connection-manager/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Include automated verification tasks required by the constitution and feature scope. Do not omit formatting, static analysis, test, smoke-check, accessibility verification, shared stylesheet validation, or platform-sensitive validation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g. US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Desktop app (Go + Wails + Svelte)**: `cmd/`, `internal/`, `frontend/`, `build/`, `tests/`, `scripts/`
- **Frontend styling rule**: Keep all class-based styling in `frontend/src/app.css`; do not add component-scoped CSS, CSS modules, or runtime-generated styling

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the desktop app scaffold, validation entrypoints, and shared frontend styling baseline.

- [X] T001 Initialize the desktop app entrypoint and Wails configuration in `go.mod`, `cmd/app/main.go`, and `wails.json`
- [X] T002 Initialize the plain Svelte frontend entrypoints and package scripts in `frontend/package.json`, `frontend/src/main.js`, and `frontend/index.html`
- [X] T003 [P] Create the shared validation and build helpers in `scripts/validate.sh`, `scripts/build-current-os.sh`, `scripts/dev-current-os.sh`, and `scripts/wails-build-linux.sh`
- [X] T004 [P] Establish the shared theme and layout stylesheet foundation in `frontend/src/app.css`
- [X] T005 [P] Document repository bootstrap and required local quality gates in `specs/001-ssh-connection-manager/quickstart.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T006 Create application bootstrap and OS-specific configuration path setup in `internal/app/bootstrap/bootstrap.go` and `internal/platform/paths/config_dir.go`
- [X] T007 [P] Implement SQLite connection, schema creation, and migrations in `internal/sqlite/db.go` and `internal/sqlite/migrations.go`
- [X] T008 [P] Define shared domain models and validation primitives in `internal/domain/server/models.go`, `internal/domain/config/models.go`, `internal/domain/preferences/models.go`, and `internal/domain/session/models.go`
- [X] T009 [P] Implement SQLite stores for servers, configurations, preferences, and session history in `internal/sqlite/server_store.go`, `internal/sqlite/config_store.go`, `internal/sqlite/preferences_store.go`, and `internal/sqlite/session_history_store.go`
- [X] T010 [P] Implement SSH key loading and tunnel session primitives in `internal/ssh/auth/keys.go`, `internal/ssh/auth/passphrase.go`, `internal/ssh/tunnel/session.go`, and `internal/ssh/tunnel/socks.go`
- [X] T011 [P] Implement Linux and macOS browser discovery and launch adapters in `internal/platform/browser/discovery_linux.go`, `internal/platform/browser/discovery_darwin.go`, `internal/platform/browser/launch_linux.go`, and `internal/platform/browser/launch_darwin.go`
- [X] T012 Create the thin Wails binding surface and frontend API bridge in `internal/app/bindings/app_bindings.go` and `frontend/src/lib/api.js`
- [X] T013 Create the shared application shell and primary view layout in `frontend/src/routes/App.svelte`, `frontend/src/components/ServerList.svelte`, `frontend/src/components/ConfigList.svelte`, and `frontend/src/components/ActiveConnections.svelte`
- [X] T014 [P] Create integration and smoke test scaffolding in `tests/integration/README.md`, `tests/integration/test_helpers_test.go`, `tests/smoke/README.md`, and `tests/smoke/platform_validation.md`

**Checkpoint**: Foundation ready; user story work can now begin.

---

## Phase 3: User Story 1 - Organize Servers and Connection Configurations (Priority: P1) 🎯 MVP

**Goal**: Let the user create, edit, delete, persist, and browse saved servers with nested connection configurations.

**Independent Test**: Create multiple servers with multiple local-forward and SOCKS configurations, restart the app, and confirm the nested saved structure reloads from the OS-specific configuration location without starting live sessions.

### Tests for User Story 1 ⚠️

> **NOTE**: Write or update these tests before implementation and confirm they fail for the new behavior.

- [X] T015 [P] [US1] Add Go store and validation tests for server and configuration CRUD in `internal/sqlite/server_store_test.go`, `internal/sqlite/config_store_test.go`, and `internal/domain/config/service_test.go`
- [X] T016 [P] [US1] Add integration coverage for persisted nested server/configuration management and recovery from unreadable storage in `tests/integration/server_config_crud_test.go`
- [X] T017 [P] [US1] Add frontend rendering and keyboard-accessibility coverage for server and configuration management in `frontend/src/components/ServerList.test.js`, `frontend/src/components/ConfigList.test.js`, `frontend/src/components/ConfigEditor.test.js`, and `frontend/src/components/ServerEditorDialog.test.js`

### Implementation for User Story 1

- [X] T018 [P] [US1] Implement server CRUD application logic in `internal/domain/server/service.go`
- [X] T019 [P] [US1] Implement connection configuration CRUD logic and port-conflict validation in `internal/domain/config/service.go`
- [X] T020 [US1] Expose load, save, and delete bindings for servers and configurations in `internal/app/bindings/app_bindings.go`, `internal/app/bindings/server_bindings.go`, and `internal/app/bindings/config_bindings.go`
- [X] T021 [US1] Persist last-selected server restoration behavior in `internal/domain/preferences/service.go` and `internal/app/bindings/preferences_bindings.go`
- [X] T022 [US1] Implement the server and configuration editors and nested list workflow in `frontend/src/routes/App.svelte`, `frontend/src/components/ServerList.svelte`, `frontend/src/components/ConfigList.svelte`, `frontend/src/components/ServerEditorDialog.svelte`, and `frontend/src/components/ConfigEditor.svelte`
- [X] T023 [US1] Add inline validation, empty states, and configuration-store error feedback in `frontend/src/routes/App.svelte` and `frontend/src/app.css`

**Checkpoint**: User Story 1 should be independently functional and demoable as the MVP slice.

---

## Phase 4: User Story 2 - Run and Recover SSH Tunnels Reliably (Priority: P2)

**Goal**: Let the user start and stop saved local-forward and SOCKS configurations, detect stale tunnels, recover from transient disconnects, unlock encrypted keys when needed, and review user-accessible session logs.

**Independent Test**: Start saved local-forward and SOCKS configurations, interrupt connectivity or simulate sleep/resume-style staleness, and confirm status transitions, reconnect attempts, unlock prompts, and log visibility without recreating saved configurations.

### Tests for User Story 2 ⚠️

> **NOTE**: Write or update these tests before implementation and confirm they fail for the new behavior.

- [X] T024 [P] [US2] Add Go unit tests for runtime session state transitions, duplicate-start protection, and reconnect policy in `internal/domain/session/service_test.go`, `internal/ssh/tunnel/session_test.go`, and `internal/ssh/tunnel/reconnect_test.go`
- [X] T025 [P] [US2] Add integration coverage for start, stop, reconnect, stale-tunnel detection, and encrypted-key retry flows in `tests/integration/session_lifecycle_test.go`
- [X] T026 [P] [US2] Add frontend accessibility and status-message coverage for session controls, unlock prompts, and diagnostics visibility in `frontend/src/components/SessionStatus.test.js`, `frontend/src/components/UnlockKeyDialog.test.js`, and `frontend/src/components/DiagnosticsPanel.test.js`

### Implementation for User Story 2

- [X] T027 [P] [US2] Implement runtime session orchestration and in-memory session storage in `internal/domain/session/service.go` and `internal/domain/session/runtime_store.go`
- [X] T028 [P] [US2] Implement keepalive health checks, reconnect backoff, and user-stop cancellation handling in `internal/ssh/tunnel/session.go` and `internal/ssh/tunnel/reconnect.go`
- [X] T029 [P] [US2] Implement encrypted-key unlock and retry handling without persisting secrets in `internal/ssh/auth/passphrase.go` and `internal/ssh/auth/keys.go`
- [X] T030 [US2] Persist session history and user-visible connection event logs in `internal/sqlite/session_history_store.go` and `internal/domain/session/log_service.go`
- [X] T031 [US2] Expose start, stop, retry, unlock, runtime status, and diagnostics bindings in `internal/app/bindings/session_bindings.go`, `internal/app/bindings/app_bindings.go`, and `internal/app/bindings/devtools_bindings.go`
- [X] T032 [US2] Implement session controls, active-session status views, unlock prompts, and diagnostics export UI in `frontend/src/routes/App.svelte`, `frontend/src/components/ActiveConnections.svelte`, `frontend/src/components/SessionStatus.svelte`, `frontend/src/components/UnlockKeyDialog.svelte`, and `frontend/src/components/DiagnosticsPanel.svelte`
- [X] T033 [US2] Add actionable bind, authentication, reconnect, and storage failure messaging in `frontend/src/routes/App.svelte` and `frontend/src/app.css`

**Checkpoint**: User Stories 1 and 2 should work together, and User Story 2 should be testable using saved configurations created through User Story 1.

---

## Phase 5: User Story 3 - Launch a Browser Through a Saved SOCKS Proxy (Priority: P3)

**Goal**: Let the user discover installed browsers, launch one through a running SOCKS session, and switch between readable dark and light themes without losing usability.

**Independent Test**: Start a saved SOCKS configuration, verify installed supported browsers are listed, launch a selected browser through the proxy, then switch themes and confirm the UI remains readable and keyboard-operable.

### Tests for User Story 3 ⚠️

> **NOTE**: Write or update these tests before implementation and confirm they fail for the new behavior.

- [X] T034 [P] [US3] Add Go unit tests for browser discovery, unavailable-browser handling, and proxy-launch eligibility in `internal/platform/browser/service_test.go` and `internal/platform/browser/discovery_darwin_test.go`
- [X] T035 [P] [US3] Add integration coverage for SOCKS browser-launch success and failure cases in `tests/integration/browser_launch_test.go`
- [X] T036 [P] [US3] Add frontend accessibility and theme coverage for browser selection and theme switching in `frontend/src/components/BrowserLauncher.test.js`, `frontend/src/components/ThemeToggle.test.js`, and `frontend/src/routes/App.test.js`

### Implementation for User Story 3

- [X] T037 [P] [US3] Implement browser discovery and proxy-launch orchestration in `internal/platform/browser/service.go`, `internal/platform/browser/browser_profile.go`, and `internal/platform/browser/firefox_profile.go`
- [X] T038 [US3] Expose browser discovery and SOCKS launch bindings in `internal/app/bindings/browser_bindings.go` and `frontend/src/lib/api.js`
- [X] T039 [US3] Implement the browser selector and launch workflow in `frontend/src/components/BrowserLauncher.svelte` and `frontend/src/routes/App.svelte`
- [X] T040 [US3] Persist theme selection and restore it at startup in `internal/domain/preferences/service.go`, `internal/app/bindings/preferences_bindings.go`, and `frontend/src/components/ThemeToggle.svelte`
- [X] T041 [US3] Add accessible dark and light theme states plus clear browser-launch failure feedback in `frontend/src/app.css` and `frontend/src/routes/App.svelte`

**Checkpoint**: All planned user stories should now be independently verifiable with the full browser-launch workflow.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements and verification that affect multiple user stories.

- [X] T042 [P] Update developer setup, manual validation flows, and macOS/Linux notes in `specs/001-ssh-connection-manager/quickstart.md`, `tests/smoke/session_management.md`, and `tests/smoke/browser_launch.md`
- [X] T043 [P] Add cross-story persistence and recovery coverage in `tests/integration/persistence_recovery_test.go` and `tests/smoke/platform_validation.md`
- [X] T044 [P] Add cross-story focus, labeling, and responsive list polish in `frontend/src/app.css` and `frontend/src/routes/App.svelte`
- [X] T045 Run backend and frontend validation commands via `scripts/validate.sh`, `go test ./...`, and `npm run validate --prefix frontend`, then fix findings in `cmd/app/`, `internal/`, `frontend/src/`, and `tests/`
- [X] T046 Run `wails doctor`, `wails build -clean`, and `./scripts/wails-build-linux.sh`, then record Linux/macOS packaging and environment outcomes in `specs/001-ssh-connection-manager/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup; blocks all user story work
- **User Story 1 (Phase 3)**: Depends on Foundational completion
- **User Story 2 (Phase 4)**: Depends on User Story 1 saved configuration flows and Foundational completion
- **User Story 3 (Phase 5)**: Depends on User Story 2 running SOCKS sessions and User Story 1 saved SOCKS configurations
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependency Graph

```text
Setup -> Foundational -> US1 -> US2 -> US3 -> Polish
```

### Within Each User Story

- Write or update tests and smoke coverage before implementation and verify the new checks fail first
- Implement domain and service logic before Wails bindings and frontend integration
- Update `frontend/src/app.css` before or alongside Svelte markup that depends on new classes
- Complete story-specific verification before moving to the next priority story

### Parallel Opportunities

- Setup tasks marked `[P]` can run in parallel
- Foundational tasks marked `[P]` can run in parallel after T006
- Within US1, T015-T017 can run in parallel, and T018-T019 can run in parallel
- Within US2, T024-T026 can run in parallel, and T027-T029 can run in parallel
- Within US3, T034-T036 can run in parallel, and T037 can run in parallel with theme persistence work after browser-launch contracts are clear

---

## Parallel Example: User Story 1

```bash
# Launch verification together:
Task: "Add Go store and validation tests for server and configuration CRUD in internal/sqlite/server_store_test.go, internal/sqlite/config_store_test.go, and internal/domain/config/service_test.go"
Task: "Add integration coverage for persisted nested server/configuration management and recovery from unreadable storage in tests/integration/server_config_crud_test.go"
Task: "Add frontend rendering and keyboard-accessibility coverage for server and configuration management in frontend/src/components/ServerList.test.js, frontend/src/components/ConfigList.test.js, frontend/src/components/ConfigEditor.test.js, and frontend/src/components/ServerEditorDialog.test.js"

# Launch backend work together:
Task: "Implement server CRUD application logic in internal/domain/server/service.go"
Task: "Implement connection configuration CRUD logic and port-conflict validation in internal/domain/config/service.go"
```

## Parallel Example: User Story 2

```bash
# Launch verification together:
Task: "Add Go unit tests for runtime session state transitions, duplicate-start protection, and reconnect policy in internal/domain/session/service_test.go, internal/ssh/tunnel/session_test.go, and internal/ssh/tunnel/reconnect_test.go"
Task: "Add integration coverage for start, stop, reconnect, stale-tunnel detection, and encrypted-key retry flows in tests/integration/session_lifecycle_test.go"
Task: "Add frontend accessibility and status-message coverage for session controls, unlock prompts, and diagnostics visibility in frontend/src/components/SessionStatus.test.js, frontend/src/components/UnlockKeyDialog.test.js, and frontend/src/components/DiagnosticsPanel.test.js"

# Launch backend work together:
Task: "Implement runtime session orchestration and in-memory session storage in internal/domain/session/service.go and internal/domain/session/runtime_store.go"
Task: "Implement keepalive health checks, reconnect backoff, and user-stop cancellation handling in internal/ssh/tunnel/session.go and internal/ssh/tunnel/reconnect.go"
Task: "Implement encrypted-key unlock and retry handling without persisting secrets in internal/ssh/auth/passphrase.go and internal/ssh/auth/keys.go"
```

## Parallel Example: User Story 3

```bash
# Launch verification together:
Task: "Add Go unit tests for browser discovery, unavailable-browser handling, and proxy-launch eligibility in internal/platform/browser/service_test.go and internal/platform/browser/discovery_darwin_test.go"
Task: "Add integration coverage for SOCKS browser-launch success and failure cases in tests/integration/browser_launch_test.go"
Task: "Add frontend accessibility and theme coverage for browser selection and theme switching in frontend/src/components/BrowserLauncher.test.js, frontend/src/components/ThemeToggle.test.js, and frontend/src/routes/App.test.js"

# Launch implementation together:
Task: "Implement browser discovery and proxy-launch orchestration in internal/platform/browser/service.go, internal/platform/browser/browser_profile.go, and internal/platform/browser/firefox_profile.go"
Task: "Persist theme selection and restore it at startup in internal/domain/preferences/service.go, internal/app/bindings/preferences_bindings.go, and frontend/src/components/ThemeToggle.svelte"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Stop and validate the saved server/configuration workflow independently before continuing

### Incremental Delivery

1. Deliver Setup + Foundational to establish the platform, persistence, and binding surface
2. Deliver User Story 1 as the MVP for durable server/configuration management
3. Deliver User Story 2 for live tunnel operation, reconnect behavior, encrypted-key unlock, and diagnostics
4. Deliver User Story 3 for browser launch and theme completion
5. Finish with cross-story validation, accessibility, and packaging checks

### Parallel Team Strategy

1. Complete Setup and Foundational together
2. Split `[P]` backend, frontend-test, and platform-adapter tasks within each story across teammates
3. Keep story-level delivery in dependency order because US2 builds on US1 and US3 builds on US2

---

## Notes

- `[P]` tasks touch different files and do not depend on incomplete peer tasks
- `[US1]`, `[US2]`, and `[US3]` labels preserve traceability to the specification user stories
- Every task includes a checkbox, task ID, optional parallel marker, optional story label, and explicit file path
- Completion status reflects implemented feature behavior and validation in the current repository, even where the generated task paths were more specific than the final file layout
- Frontend tasks must keep markup in plain Svelte and route class-based styling changes through `frontend/src/app.css`
- Final validation must cover `gofmt -w .`, `go vet ./...`, `go test ./...`, `npm run build`, `wails doctor`, and `wails build -clean`
