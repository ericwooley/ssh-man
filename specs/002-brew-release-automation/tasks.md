---

description: "Task list for feature: Homebrew Installation and Automated Releases"
---

# Tasks: Homebrew Installation and Automated Releases

**Input**: Design documents from `/specs/002-brew-release-automation/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Automated verification, smoke-validation of all install/platform paths, Homebrew cask install/upgrade workflow, Linux support, Go/frontend static analysis, and release artifact validation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Validate and update project structure/dev environment for feature scope.

- [ ] T001 Verify tool and language prerequisites on dev systems for Go 1.22.2, Node.js, npm, Wails v2, Homebrew (macOS), GitHub Actions, and access/workflow tools (`wails doctor`, `brew`, `gh`, etc), updating quickstart.md as needed
- [ ] T002 Ensure scripts (build-current-os.sh, validate.sh, wails-build-linux.sh) are present and executable in scripts/
- [ ] T003 [P] Validate the presence, correctness, and execution of all quality gates: `gofmt`, `go vet`, `go test`, `npm run validate --prefix frontend`, and update any devdocs or scripts as needed

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure and validations needed before automating releases/user workflows.

- [ ] T004 Establish shared release domain types in `internal/domain/release.go` per data-model.md (ReleaseVersion, ReleaseWorkflowRun, ReleaseArtifact, HomebrewCaskDefinition, LinuxSupportPath)
- [ ] T005 [P] Add Go tests for release types and relationships in `internal/domain/release_test.go`
- [ ] T006 Extend repository build and validation scripts (scripts/build-current-os.sh, scripts/validate.sh, scripts/wails-build-linux.sh) to independently validate Linux and macOS build workflows, update quickstart.md
- [ ] T007 [P] Document a repeatable local validation workflow for maintainers in quickstart.md (including prerequisites and dry-run for release path)

---

## Phase 3: User Story 1 - Install the App with Homebrew (Priority: P1) 🎯 MVP

**Goal**: User can install the app on macOS via Homebrew cask from project-owned tap, using published command. Linux install remains documented as clone-and-build.

**Independent Test**: macOS user with Homebrew can execute the documented command and install the app from an official release (without manual build). Guidance for Linux is available and accurate.

### Tests for User Story 1 ⚠️
- [ ] T008 [P] [US1] Add integration test for cask-based installation scenario in `tests/smoke/homebrew_install.test.sh` covering install/upgrade edge cases
- [ ] T009 [P] [US1] Add documentation verification test that install guides (quickstart.md, README) render correct Homebrew commands and Linux guidance

### Implementation for User Story 1
- [ ] T010 [P] [US1] Create Homebrew cask metadata and initial tap repo (if missing), place/validate cask file in correct repo path (`homebrew-[project]/Casks/ssh-man.rb` or per tap conventions)
- [ ] T011 [P] [US1] Implement install/upgrade documentation flow for macOS and Linux in quickstart.md and project README
- [ ] T012 [US1] Add explicit documentation notes about unsigned app delivery for macOS in Homebrew context
- [ ] T013 [US1] Document edge/unsupported platform behavior for Homebrew and Linux

**Checkpoint**: At this point, a user can install and launch the app via the Homebrew cask; Linux users follow maintained build docs. All install guidance tested.

---

## Phase 4: User Story 2 - Publish Official Release Builds Automatically (Priority: P2)

**Goal**: Maintainer can trigger official macOS releases from GitHub Actions workflow, with released asset attached, properly tagged, and validated, minimizing manual steps and reducing error risk.

**Independent Test**: Triggering a release through the workflow creates a versioned, user-visible GitHub Release with the full downloadable macOS artifact. No manual asset packaging required.

### Tests for User Story 2 ⚠️
- [ ] T014 [P] [US2] Add release workflow integration test in `.github/workflows/release_automation_test.yml` to cover asset generation/attachment/validation on simulated release
- [ ] T015 [P] [US2] Add test for tag format and tag/release/Auth traceability edge cases (e.g. malformed/duplicate tags) in a smoke-sh workflow or unit test
- [ ] T016 [P] [US2] Add test to verify Linux build validation continues to run during/after release workflow in scripts/wails-build-linux.sh

### Implementation for User Story 2
- [ ] T017 [P] [US2] Implement GitHub Actions configuration for release build/tag workflow in `.github/workflows/release.yml`, triggered by semantic version tag and pushing macOS assets
- [ ] T018 [US2] Extend workflow to publish a user-visible GitHub Release with attached artifact and release notes/summary
- [ ] T019 [US2] Implement workflow error handling: if failure occurs, no incomplete release is published (cancel/tag rollback, asset cleanup)
- [ ] T020 [US2] Add explicit traceability linkages from published Release to tag, asset, workflow, cask, and update doc in quickstart.md/README
- [ ] T021 [US2] Document maintainer workflow, artifact expectations, and all manual/build exceptions in quickstart.md and docs/

**Checkpoint**: Maintainer can complete official release, validate all paths; Linux build testing runs and passes as expected.

---

## Phase 5: User Story 3 - Keep Homebrew Availability in Sync with Releases (Priority: P3)

**Goal**: Homebrew cask in project-owned tap updates automatically to latest released version, accurately referencing asset URL and checksum, so users always get the latest official app.

**Independent Test**: A new (simulated/actual) release triggers an update in the Homebrew cask so the install path references the new version and checksum within workflow SLA, with proper edge/failure handling.

### Tests for User Story 3 ⚠️
- [ ] T022 [P] [US3] Add test that fast-release cycles and multiple sequential releases sync correctly to cask in tap (update propagation, failed/incomplete runs)
- [ ] T023 [P] [US3] Add test that cask checksum and URL always match the published macOS artifact, checked in cask file and verified in smoke validation
- [ ] T024 [US3] Add test for Linux support docs to remain traceable and accurate after release update

### Implementation for User Story 3
- [ ] T025 [P] [US3] Automate/upsert cask file in project tap after successful GitHub release in tap repo (`homebrew-[project]/Casks/ssh-man.rb`)
- [ ] T026 [US3] Script cask checksum+url update (by artifact, version from release output), handle race/edge/error cases as per contract (no update for failed/incomplete release)
- [ ] T027 [US3] Add docs and CI badges for cask version/update status in README/quickstart.md, confirming sync and traceability
- [ ] T028 [US3] Update quickstart.md and supporting docs to clarify post-release sync SLA and traceability process

**Checkpoint**: Cask reliably matches the latest official release, user install/upgrade is trustable, docs reflect actual flow, Linux path still accurate

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Multi-story verification, documentation, and validation cleanup

- [ ] T029 [P] Full run-through of macOS and Linux build/validation scripts, checking for required output and workflow status in quickstart.md and tests/smoke/
- [ ] T030 Code/documentation cleanup, addressing edge cases, outdated notes, or workflow/validate.sh improvements
- [ ] T031 [P] Add security review of tap keys, workflows, secrets
- [ ] T032 [P] Final run: `gofmt`, `go vet`, `go test`, `npm run validate --prefix frontend` for repository root
- [ ] T033 Add/adjust Go/typescript domain unit tests or smoke tests for any missed/untested flows in internal/domain/release_test.go and tests/smoke/
- [ ] T034 [P] Validate README/quickstart.md clarity and install steps for all platforms

---

## Dependencies & Execution Order

### Phase Dependencies
- **Setup (Phase 1)**: None – can run immediately
- **Foundational (Phase 2)**: Depends on Setup; BLOCKS all user stories
- **User Stories (Phase 3+)**: Each depends on Foundational phase completion; stories P1–P3 can proceed sequentially or in parallel
- **Polish (Final Phase)**: Depends on completion of all user stories

### User Story Dependencies
- **US1 (P1)**: Needs Setup, Foundational. No downstream dependency.
- **US2 (P2)**: Needs Setup, Foundational. May use US1 cask/docs as input; must not break their testability.
- **US3 (P3)**: Needs 2 + Setup/Foundational. May depend on published releases (US2), must not break independent Homebrew install testability.

### Parallel Opportunities
- All [P] tasks within each phase can be executed in parallel.
- User stories can be partially staffed or sequenced as per team capacity.
- Test, doc, and some implementation tasks (e.g., cask+workflow config, parallel Linux+macOS builds/scripts) are parallelizable where no file/contract conflict.

---

## Parallel Example: User Story 1
```bash
# Verification in parallel:
Task: "Add integration test for cask-based installation scenario in tests/smoke/homebrew_install.test.sh"
Task: "Add documentation verification test for install guides (quickstart.md, README)"

# Domain/code/doc work in parallel:
Task: "Create cask metadata in homebrew-[project]/Casks/ssh-man.rb"
Task: "Implement install/upgrade documentation in quickstart.md and README"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)
1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL – blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test US1 independently
5. Deploy/demo if ready

### Incremental Delivery
1. Complete Setup + Foundational → Foundation ready
2. Add US1 → Test independently → Deploy/Demo (MVP!)
3. Add US2 → Test independently → Deploy/Demo
4. Add US3 → Test independently → Deploy/Demo
5. Each adds value without breaking earlier stories

### Parallel Team Strategy
- Complete Setup + Foundational as a team
- US1/US2/US3 can then progress in parallel
- Urgent doc/tests/infra can be run by any available dev as needed

---
