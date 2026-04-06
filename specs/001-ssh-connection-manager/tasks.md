# Tasks: SSH Connection Manager MVP

**Input**: Design documents from `/specs/001-ssh-connection-manager/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Include automated verification tasks required by the constitution and feature scope. Do not omit formatting, static analysis, test, smoke-check, or accessibility verification work for user stories or platform-sensitive changes.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Desktop app (Go + Wails + Svelte)**: `cmd/`, `internal/`, `frontend/`, `build/`, `tests/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and baseline tooling for a greenfield Wails desktop app.

- [ ] T001 Initialize the Go module and Wails entrypoint in `go.mod`, `cmd/app/main.go`, and `wails.json`
- [ ] T002 Initialize plain Svelte frontend dependencies and scripts in `frontend/package.json`, `frontend/src/main.js`, and `frontend/src/App.svelte`
- [ ] T003 [P] Establish the shared stylesheet and theme foundations in `frontend/src/app.css`
- [ ] T004 [P] Configure frontend build and validation scripts in `frontend/package.json`
- [ ] T005 [P] Add repository ignores for Wails, frontend, and build outputs in `.gitignore`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T006 Create application bootstrap and OS-specific config path resolution in `internal/app/bootstrap/bootstrap.go` and `internal/platform/paths/config_dir.go`
- [ ] T007 [P] Implement SQLite initialization and schema migration support in `internal/sqlite/db.go` and `internal/sqlite/migrations.go`
- [ ] T008 [P] Define shared domain models and validation rules in `internal/domain/server/models.go`, `internal/domain/config/models.go`, `internal/domain/preferences/models.go`, and `internal/domain/session/models.go`
- [ ] T009 [P] Implement SQLite repositories for servers, configurations, preferences, and session history in `internal/sqlite/server_store.go`, `internal/sqlite/config_store.go`, `internal/sqlite/preferences_store.go`, and `internal/sqlite/session_history_store.go`
- [ ] T010 [P] Implement SSH key-loading and tunnel session primitives in `internal/ssh/auth/keys.go`, `internal/ssh/auth/passphrase.go`, and `internal/ssh/tunnel/session.go`
- [ ] T011 [P] Implement Linux and macOS browser discovery and launch adapters in `internal/platform/browser/discovery_linux.go`, `internal/platform/browser/discovery_darwin.go`, `internal/platform/browser/launch_linux.go`, and `internal/platform/browser/launch_darwin.go`
- [ ] T012 Create thin shared Wails bindings and frontend API helpers in `internal/app/bindings/app_bindings.go` and `frontend/src/lib/api.js`
- [ ] T013 Create the main application shell and shared layout components in `frontend/src/routes/App.svelte`, `frontend/src/components/ServerList.svelte`, and `frontend/src/components/ConfigList.svelte`
- [ ] T014 [P] Add baseline integration and smoke test scaffolding in `tests/integration/README.md` and `tests/smoke/README.md`

**Checkpoint**: Foundation ready - user story implementation can now begin.

---

## Phase 3: User Story 1 - Organize Servers and Connection Configurations (Priority: P1) 🎯 MVP

**Goal**: Let the user create, edit, delete, persist, and browse saved servers with nested connection configurations.

**Independent Test**: Create multiple servers with multiple configurations, restart the app, and confirm the nested saved list reloads from the OS-appropriate app data location without starting any live sessions.

### Tests for User Story 1

- [ ] T015 [P] [US1] Add Go repository tests for server and configuration CRUD in `internal/sqlite/server_store_test.go` and `internal/sqlite/config_store_test.go`
- [ ] T016 [P] [US1] Add integration coverage for persisted nested server/configuration management in `tests/integration/server_config_crud_test.go`
- [ ] T017 [P] [US1] Add frontend accessibility and render coverage for list management in `frontend/src/components/ServerList.test.js`, `frontend/src/components/ConfigList.test.js`, and `frontend/src/components/ConfigEditor.test.js`

### Implementation for User Story 1

- [ ] T018 [P] [US1] Implement server CRUD application service in `internal/domain/server/service.go`
- [ ] T019 [P] [US1] Implement connection configuration CRUD service and conflict validation in `internal/domain/config/service.go`
- [ ] T020 [US1] Expose initial-state, server, and configuration CRUD bindings in `internal/app/bindings/server_bindings.go` and `internal/app/bindings/config_bindings.go`
- [ ] T021 [US1] Implement preference persistence for theme and last-selected server in `internal/domain/preferences/service.go` and `internal/app/bindings/preferences_bindings.go`
- [ ] T022 [US1] Build server/configuration management UI in `frontend/src/routes/App.svelte`, `frontend/src/components/ServerList.svelte`, `frontend/src/components/ConfigList.svelte`, and `frontend/src/components/ConfigEditor.svelte`
- [ ] T023 [US1] Add validation, empty states, and configuration-store error feedback in `frontend/src/components/ConfigEditor.svelte`, `frontend/src/routes/App.svelte`, and `frontend/src/app.css`
- [ ] T024 [US1] Handle configuration load/save failure recovery in `internal/app/bindings/app_bindings.go` and `frontend/src/routes/App.svelte`

**Checkpoint**: User Story 1 is independently functional and testable as the MVP slice.

---

## Phase 4: User Story 2 - Run and Recover SSH Tunnels Reliably (Priority: P2)

**Goal**: Let the user start and stop saved local-forward and SOCKS configurations, observe runtime status, recover from transient disconnects, and unlock encrypted keys when needed.

**Independent Test**: Start saved local-forward and SOCKS configurations, interrupt connectivity, and verify status transitions, reconnect attempts, and encrypted-key retry behavior without recreating saved configurations.

### Tests for User Story 2

- [ ] T025 [P] [US2] Add Go unit tests for runtime session state transitions and reconnect policy in `internal/domain/session/service_test.go` and `internal/ssh/tunnel/session_test.go`
- [ ] T026 [P] [US2] Add integration coverage for start, stop, reconnect, and unlock flows in `tests/integration/session_lifecycle_test.go`
- [ ] T027 [P] [US2] Add smoke and accessibility coverage for session controls and status messaging in `tests/smoke/session_management.md` and `frontend/src/components/SessionStatus.test.js`

### Implementation for User Story 2

- [ ] T028 [P] [US2] Implement runtime session orchestration and in-memory state tracking in `internal/domain/session/service.go` and `internal/domain/session/runtime_store.go`
- [ ] T029 [P] [US2] Implement reconnect handling and encrypted-key unlock behavior in `internal/ssh/auth/passphrase.go` and `internal/ssh/tunnel/reconnect.go`
- [ ] T030 [US2] Expose start, stop, retry, and unlock bindings in `internal/app/bindings/session_bindings.go`
- [ ] T031 [US2] Build session controls, status surfaces, and unlock prompts in `frontend/src/components/SessionStatus.svelte`, `frontend/src/components/UnlockKeyDialog.svelte`, and `frontend/src/routes/App.svelte`
- [ ] T032 [US2] Add actionable runtime error messaging for bind, auth, and reconnect failures in `internal/ssh/tunnel/session.go`, `internal/app/bindings/session_bindings.go`, and `frontend/src/app.css`

**Checkpoint**: User Stories 1 and 2 work together, and User Story 2 can be validated against saved configurations created through the MVP slice.

---

## Phase 5: User Story 3 - Launch a Browser Through a Saved SOCKS Proxy (Priority: P3)

**Goal**: Let the user discover installed browsers, select one for a running SOCKS session, launch it through the proxy, and use theme controls that remain readable and keyboard-accessible.

**Independent Test**: Start a saved SOCKS configuration, open the browser selector, confirm installed browsers are listed, launch one through the proxy, and verify theme switching preserves readable and keyboard-accessible UI.

### Tests for User Story 3

- [ ] T033 [P] [US3] Add Go unit tests for browser discovery and launch eligibility in `internal/platform/browser/service_test.go`
- [ ] T034 [P] [US3] Add integration coverage for SOCKS browser-launch validation in `tests/integration/browser_launch_test.go`
- [ ] T035 [P] [US3] Add smoke and accessibility coverage for browser selection and theme switching in `tests/smoke/browser_launch.md`, `frontend/src/components/BrowserLauncher.test.js`, and `frontend/src/components/ThemeToggle.test.js`

### Implementation for User Story 3

- [ ] T036 [P] [US3] Implement browser discovery and proxy-launch service in `internal/platform/browser/service.go`
- [ ] T037 [US3] Expose browser discovery and SOCKS launch bindings in `internal/app/bindings/browser_bindings.go`
- [ ] T038 [US3] Build browser selection and launch UI in `frontend/src/components/BrowserLauncher.svelte` and `frontend/src/routes/App.svelte`
- [ ] T039 [US3] Implement theme toggle behavior and accessible light/dark styles in `frontend/src/components/ThemeToggle.svelte`, `frontend/src/app.css`, and `internal/domain/preferences/service.go`

**Checkpoint**: All planned user stories are functional with the full MVP-plus browser-launch workflow.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements and validation that affect multiple user stories.

- [ ] T040 [P] Update developer validation and manual test guidance in `specs/001-ssh-connection-manager/quickstart.md`
- [ ] T041 [P] Add cross-story accessibility polish for focus states, labels, and status semantics in `frontend/src/app.css` and `frontend/src/routes/App.svelte`
- [ ] T042 [P] Add cross-story persistence and recovery coverage in `tests/integration/persistence_recovery_test.go` and `tests/smoke/platform_validation.md`
- [ ] T043 Run `gofmt -w .` and fix formatting issues in `cmd/app/`, `internal/`, and `tests/`
- [ ] T044 Run `go vet ./...` and fix findings in `cmd/app/`, `internal/`, and `tests/`
- [ ] T045 Run `go test ./...` and resolve failures in `internal/`, `tests/integration/`, and `frontend/src/`
- [ ] T046 Run `npm run build` and `wails build -clean`, then record Linux/macOS validation outcomes in `specs/001-ssh-connection-manager/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion - blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational completion.
- **User Story 2 (Phase 4)**: Depends on User Story 1 for saved configuration management and on Foundational completion.
- **User Story 3 (Phase 5)**: Depends on User Story 2 for running SOCKS sessions and on User Story 1 for saved configuration management.
- **Polish (Phase 6)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **User Story 1 (P1)**: First deliverable and MVP slice.
- **User Story 2 (P2)**: Requires saved servers and configurations from User Story 1.
- **User Story 3 (P3)**: Requires a running SOCKS session from User Story 2 and saved SOCKS configurations from User Story 1.

### Within Each User Story

- Tests and smoke checks MUST be written or updated before implementation and must fail when they cover new behavior.
- Service and domain logic before Wails bindings and frontend integration.
- Shared stylesheet updates before or alongside the Svelte markup that depends on them.
- Formatting and static analysis before final validation.

### Parallel Opportunities

- Setup tasks marked `[P]` can run in parallel.
- Foundational tasks marked `[P]` can run in parallel after project initialization.
- Within each user story, marked `[P]` test and backend tasks can run in parallel when they touch different files.

---

## Parallel Example: User Story 1

```bash
# Launch verification tasks together:
Task: "Add Go repository tests for server and configuration CRUD in internal/sqlite/server_store_test.go and internal/sqlite/config_store_test.go"
Task: "Add integration coverage for persisted nested server/configuration management in tests/integration/server_config_crud_test.go"
Task: "Add frontend accessibility and render coverage for list management in frontend/src/components/ServerList.test.js, frontend/src/components/ConfigList.test.js, and frontend/src/components/ConfigEditor.test.js"

# Launch backend implementation tasks together:
Task: "Implement server CRUD application service in internal/domain/server/service.go"
Task: "Implement connection configuration CRUD service and conflict validation in internal/domain/config/service.go"
```

## Parallel Example: User Story 2

```bash
# Launch verification tasks together:
Task: "Add Go unit tests for runtime session state transitions and reconnect policy in internal/domain/session/service_test.go and internal/ssh/tunnel/session_test.go"
Task: "Add integration coverage for start, stop, reconnect, and unlock flows in tests/integration/session_lifecycle_test.go"

# Launch backend implementation tasks together:
Task: "Implement runtime session orchestration and in-memory state tracking in internal/domain/session/service.go and internal/domain/session/runtime_store.go"
Task: "Implement reconnect handling and encrypted-key unlock behavior in internal/ssh/auth/passphrase.go and internal/ssh/tunnel/reconnect.go"
```

## Parallel Example: User Story 3

```bash
# Launch verification tasks together:
Task: "Add Go unit tests for browser discovery and launch eligibility in internal/platform/browser/service_test.go"
Task: "Add integration coverage for SOCKS browser-launch validation in tests/integration/browser_launch_test.go"

# Launch implementation tasks together:
Task: "Implement browser discovery and proxy-launch service in internal/platform/browser/service.go"
Task: "Implement theme toggle behavior and accessible light/dark styles in frontend/src/components/ThemeToggle.svelte, frontend/src/app.css, and internal/domain/preferences/service.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Stop and validate the saved server/configuration workflow independently.

### Incremental Delivery

1. Deliver User Story 1 as the MVP.
2. Add User Story 2 for live session management and reconnect behavior.
3. Add User Story 3 for browser launch and theme/accessibility completion.
4. Finish with cross-cutting validation and build checks.

### Parallel Team Strategy

1. Complete Setup and Foundational phases together.
2. Assign backend and frontend `[P]` tasks within each story in parallel.
3. Keep story-level delivery sequential because US2 depends on US1 and US3 depends on US2.

---

## Notes

- [P] tasks = different files, no dependencies.
- [Story] label maps each task to a specific user story for traceability.
- Every task follows the required checklist format with checkbox, task ID, optional parallel marker, optional story label, and explicit file path.
- Verify `gofmt`, `go vet`, `go test ./...`, `npm run build`, and `wails build -clean` before completion.
- Keep frontend work within plain Svelte and the single shared CSS file at `frontend/src/app.css`.
