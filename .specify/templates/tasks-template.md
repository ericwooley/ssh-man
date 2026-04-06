---

description: "Task list template for feature implementation"
---

# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Include automated verification tasks required by the constitution and feature scope. Do not omit formatting, static analysis, test, smoke-check, or accessibility verification work for user stories or platform-sensitive changes.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `src/`, `tests/` at repository root
- **Web app**: `backend/src/`, `frontend/src/`
- **Mobile**: `api/src/`, `ios/src/` or `android/src/`
- **Desktop app (Go + Wails + Svelte)**: `cmd/`, `internal/`, `frontend/`, `build/`, `tests/`
- Paths shown below assume single project - adjust based on plan.md structure

<!-- 
  ============================================================================
  IMPORTANT: The tasks below are SAMPLE TASKS for illustration purposes only.
  
  The /speckit.tasks command MUST replace these with actual tasks based on:
  - User stories from spec.md (with their priorities P1, P2, P3...)
  - Feature requirements from plan.md
  - Entities from data-model.md
  - Endpoints from contracts/
  
  Tasks MUST be organized by user story so each story can be:
  - Implemented independently
  - Tested independently
  - Delivered as an MVP increment
  
  DO NOT keep these sample tasks in the generated tasks.md file.
  ============================================================================
-->

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create project structure per implementation plan
- [ ] T002 Initialize Go module, Wails application, and plain Svelte frontend dependencies
- [ ] T003 [P] Configure `gofmt`, `go vet`, configured static analysis, and test commands

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

Examples of foundational tasks (adjust based on your project):

- [ ] T004 Establish shared application configuration in `internal/app/`
- [ ] T005 [P] Implement platform abstraction helpers in `internal/platform/`
- [ ] T006 [P] Setup Wails bindings and plain Svelte frontend service integration in `app.go` and `frontend/src/`
- [ ] T007 Create shared domain models/services in `internal/domain/`
- [ ] T008 Configure error handling and logging infrastructure
- [ ] T009 Setup Linux and macOS build or smoke-check commands
- [ ] T010 [P] Document the Go quality gate commands used for local and CI validation
- [ ] T011 [P] Establish the single shared stylesheet in `frontend/src/app.css` and document its usage rules

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - [Title] (Priority: P1) 🎯 MVP

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T012 [P] [US1] Add Go unit tests for [user journey] in `internal/.../*_test.go`
- [ ] T013 [P] [US1] Add integration or smoke coverage for [user journey] in `tests/integration/` or `tests/smoke/`
- [ ] T014 [P] [US1] Add accessibility verification for [user journey] in `tests/smoke/` or frontend test coverage

### Implementation for User Story 1

- [ ] T015 [P] [US1] Create domain types in `internal/domain/`
- [ ] T016 [P] [US1] Implement supporting service logic in `internal/app/` or `internal/ssh/`
- [ ] T017 [US1] Expose the Wails binding or application entry point in `app.go`
- [ ] T018 [US1] Implement the user-facing flow in `frontend/src/` using classes defined in `frontend/src/app.css`
- [ ] T019 [US1] Add validation, error context, and cancellation handling
- [ ] T020 [US1] Add platform-specific handling only where Linux/macOS behavior differs

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - [Title] (Priority: P2)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 2 ⚠️

- [ ] T021 [P] [US2] Add Go unit tests for [user journey] in `internal/.../*_test.go`
- [ ] T022 [P] [US2] Add integration or smoke coverage for [user journey] in `tests/integration/` or `tests/smoke/`
- [ ] T023 [P] [US2] Add accessibility verification for [user journey] in `tests/smoke/` or frontend test coverage

### Implementation for User Story 2

- [ ] T024 [P] [US2] Create or extend domain types in `internal/domain/`
- [ ] T025 [US2] Implement supporting service logic in `internal/app/` or `internal/ssh/`
- [ ] T026 [US2] Implement the user-facing flow in `frontend/src/` and `app.go` as needed using the shared stylesheet only
- [ ] T027 [US2] Integrate with User Story 1 components without breaking independent verification

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - [Title] (Priority: P3)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 3 ⚠️

- [ ] T028 [P] [US3] Add Go unit tests for [user journey] in `internal/.../*_test.go`
- [ ] T029 [P] [US3] Add integration or smoke coverage for [user journey] in `tests/integration/` or `tests/smoke/`
- [ ] T030 [P] [US3] Add accessibility verification for [user journey] in `tests/smoke/` or frontend test coverage

### Implementation for User Story 3

- [ ] T031 [P] [US3] Create or extend domain types in `internal/domain/`
- [ ] T032 [US3] Implement supporting service logic in `internal/app/` or `internal/ssh/`
- [ ] T033 [US3] Implement the user-facing flow in `frontend/src/` and `app.go` as needed using the shared stylesheet only

**Checkpoint**: All user stories should now be independently functional

---

[Add more user story phases as needed, following the same pattern]

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] TXXX [P] Documentation updates in docs/ or quickstart.md
- [ ] TXXX Code cleanup and refactoring
- [ ] TXXX Performance optimization across all stories
- [ ] TXXX [P] Additional test coverage for shared Go or frontend code
- [ ] TXXX [P] Verify keyboard accessibility, focus states, and labels across supported UI flows
- [ ] TXXX [P] Run `gofmt`, `go vet`, configured static analysis, and fix findings
- [ ] TXXX Security hardening
- [ ] TXXX Run Linux and macOS validation commands from quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - May integrate with US1/US2 but should be independently testable

### Within Each User Story

- Tests and smoke checks MUST be written or updated before implementation and must fail when they cover new behavior
- Service and domain logic before Wails bindings and frontend integration
- Shared stylesheet updates before or alongside the Svelte markup that depends on them
- Formatting and static analysis before final validation
- Models before services
- Services before bindings or endpoints
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Models within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all verification for User Story 1 together:
Task: "Add Go unit tests for [user journey] in internal/.../*_test.go"
Task: "Add integration or smoke coverage for [user journey] in tests/integration/ or tests/smoke/"

# Launch all domain work for User Story 1 together:
Task: "Create domain types in internal/domain/"
Task: "Implement supporting service logic in internal/app/ or internal/ssh/"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests or smoke checks fail before implementing
- Verify `gofmt`, `go vet`, and configured static analysis pass before completion
- Verify frontend work stays within plain Svelte and the single shared CSS file
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
