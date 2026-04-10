# Tasks: Vanilla Framework UI Refresh

**Feature**: Vanilla Framework UI Refresh  
**Branch**: `003-vanilla-ui-refresh`  
**Spec**: /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/spec.md

## Summary

This tasks file breaks the visual-system migration into discrete, independently testable tasks organized by phase and user story. Each task follows the required checklist format and references absolute file paths.

---

## Phase 1: Setup

- [X] T001 Initialize local frontend validation environment at /Users/ericwooley/projects/ssh-man/frontend (install dependencies)
- [X] T002 Verify frontend build and tests by running `pnpm --dir /Users/ericwooley/projects/ssh-man/frontend run validate` and record results in /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/validation-results.md
- [X] T003 Create a token-mapping document at /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/token-mapping.md listing proposed CSS variables and their Vanilla equivalents
- [X] T004 Create an inventory of frontend components and primary selectors at /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/component-inventory.md by scanning /Users/ericwooley/projects/ssh-man/frontend/src

## Phase 2: Foundational Tasks

- [X] T005 [P] Add base design tokens to the shared stylesheet in /Users/ericwooley/projects/ssh-man/frontend/src/app.css (create variable definitions and utility classes as documented in token-mapping.md)
- [X] T006 [P] Add semantic state classes (e.g., `.state-warning`, `.state-error`, `.is-selected`, `.is-disabled`) to /Users/ericwooley/projects/ssh-man/frontend/src/app.css
- [X] T007 [P] Add spacing utility classes (e.g., `.gap-sm`, `.gap-md`, `.gap-lg`) to /Users/ericwooley/projects/ssh-man/frontend/src/app.css
- [X] T008 Create migration guide at /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/migration-guide.md describing the incremental steps for component migration
- [X] T009 Update frontend CI validation script to include a CSS lint/build step and document the command in /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/quickstart.md

## Phase 3: User Story 1 (P1) - Complete core tunnel tasks in the refreshed interface

Goal: Ensure the main workspace (server list, configuration list, primary editor panels, active sessions) uses the new tokens and utility classes while preserving behavior.

Independent Test: Open the app and complete server selection, config edit, and session monitoring workflows with visual parity checked.

### Verification

- [X] T010 [US1] Create visual verification checklist for main workspace at /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/verification/us1-main-workspace.md
- [X] T011 [US1] Run `pnpm --dir /Users/ericwooley/projects/ssh-man/frontend run validate` after baseline changes and record results in /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/validation-results.md

### Implementation

- [X] T012 [US1] Update CSS rules for `.dashboard-shell`, `.dashboard-sidebar`, `.workspace-main`, and `.workspace-main-grid` in /Users/ericwooley/projects/ssh-man/frontend/src/app.css to use token variables and utility classes
- [X] T013 [US1] Update server list presentation styles for `.stack-list`, `.list-item-shell`, and `.list-card-topline` in /Users/ericwooley/projects/ssh-man/frontend/src/app.css to use token variables and semantic state classes
- [X] T014 [US1] Migrate selection and hover rules for list items to use `.is-selected` and spacing utilities in /Users/ericwooley/projects/ssh-man/frontend/src/app.css
- [X] T015 [US1] Verify selected and focused item keyboard accessibility for server list by performing keyboard navigation and documenting results in /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/accessibility/us1_keyboard.md
- [X] T016 [US1] Run manual QA: Start the development app (`./scripts/dev-current-os.sh`) and verify the P1 flows on macOS and Linux; document screenshots in /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/screenshots/us1/

## Phase 4: User Story 2 (P2) - Use dialogs, forms, and actions confidently

Goal: Ensure dialogs and forms (server editor, config editor, unlock key dialog) follow consistent patterns and validation states are visually clear.

Independent Test: Open each dialog, trigger validation errors, and confirm visually distinct error states and clear action buttons.

### Verification

- [X] T017 [US2] Create visual verification checklist for dialogs and forms at /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/verification/us2-dialogs.md

### Implementation

- [X] T018 [US2] Update dialog root styling and sections (`.dialog`, `.dialog-header`, `.dialog-body`, `.dialog-actions`) in /Users/ericwooley/projects/ssh-man/frontend/src/app.css
- [X] T019 [US2] Update server editor dialog styles in /Users/ericwooley/projects/ssh-man/frontend/src/components/ServerEditorDialog.svelte to use utility classes defined in app.css (no component-scoped styles)
- [X] T020 [US2] Update config editor styles in /Users/ericwooley/projects/ssh-man/frontend/src/components/ConfigEditor.svelte to use utility classes defined in app.css (no component-scoped styles)
- [X] T021 [US2] Ensure validation error styles are tied to form fields via classes (e.g., `.field-error`) and add those styles to /Users/ericwooley/projects/ssh-man/frontend/src/app.css
- [X] T022 [US2] Add automated visual unit tests (Vitest) that verify presence of expected classes on dialog markup in /Users/ericwooley/projects/ssh-man/frontend/src/components/__tests__/dialog-visual.test.js

## Phase 5: User Story 3 (P3) - Recognize connection state and app feedback quickly

Goal: Ensure banners, status badges, and session states are distinct and readable.

Independent Test: Populate the app with example sessions showing running, reconnecting, stopped, failed, and attention-required states and verify the UI distinguishes them.

### Verification

- [X] T023 [US3] Create verification scenarios and sample fixture data at /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/fixtures/session-states.json
- [X] T024 [US3] Create visual verification checklist for status states at /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/verification/us3-status.md

### Implementation

- [X] T025 [US3] Add semantic status classes (e.g., `.status-running`, `.status-reconnecting`, `.status-stopped`, `.status-failed`, `.status-attention`) to /Users/ericwooley/projects/ssh-man/frontend/src/app.css
- [X] T026 [US3] Update SessionStatus.svelte in /Users/ericwooley/projects/ssh-man/frontend/src/components/SessionStatus.svelte to apply status classes from the fixture data and ensure text labels/icons accompany color states
- [X] T027 [US3] Add Vitest unit tests to assert that SessionStatus renders expected classes and labels at /Users/ericwooley/projects/ssh-man/frontend/src/components/__tests__/sessionstatus.test.js

## Final Phase: Polish & Cross-Cutting Concerns

- [X] T028 Ensure accessibility: add contrast checks and ARIA attributes where needed; document fixes in /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/accessibility/report.md
- [X] T029 Run full verification: `pnpm --dir /Users/ericwooley/projects/ssh-man/frontend run validate` and run `go test ./...`; record final validation in /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/validation-final.md
- [X] T030 Clean up legacy CSS rules removed during migration in /Users/ericwooley/projects/ssh-man/frontend/src/app.css and record removed selectors in /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/removed-selectors.md

## Dependencies & Story Order

1. Phase 1 (Setup) tasks T001-T004 must run first.
2. Phase 2 (Foundational) tasks T005-T009 must complete before story phases that apply tokens and utility classes.
3. User Story phases are independent after foundational tasks complete; recommended order: US1 (T012-T016), US2 (T018-T022), US3 (T025-T027).

## Parallel Execution Examples

- Example: While T005-T007 add tokens and utilities to `app.css`, T004 component inventory and T003 token-mapping can be executed in parallel (T004, T003, T005 are [P] tasks).
- Example: US1 implementation tasks T012-T014 (CSS-only changes) are parallelizable with UI verification T010 provided CI is green.

## Implementation Strategy

- Deliver incrementally: implement tokens → update a small set of components → validate → expand. Prioritize P1 story as MVP.
- Aim to keep each story deployable and testable on its own: changes should be feature-flagged or revertible until cleanup tasks (T030) run.

---

**Generated**: 2026-04-10
