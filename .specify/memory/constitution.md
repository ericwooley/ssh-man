<!--
Sync Impact Report
Version change: 1.1.0 -> 1.2.0
Modified principles:
- I. Go and Wails Are the Product Runtime -> I. Go, Wails, and Svelte Define the Product Stack
- II. Linux and macOS Parity Is Mandatory -> II. Linux and macOS Parity Is Mandatory
- III. Core Logic Stays in Plain Go Packages -> III. Idiomatic Go Packages and Boundaries
- IV. Automated Verification Gates Every Change -> IV. Go Quality Gates Are Non-Negotiable
- V. Keep the Native Surface Area Small -> V. Keep the Native Surface Area Small
Added sections:
- Go Engineering Standards
- Frontend Constraints
Removed sections:
- None
Templates requiring updates:
- ✅ .specify/templates/plan-template.md
- ✅ .specify/templates/tasks-template.md
- ✅ .opencode/command/speckit.plan.md
- ✅ .opencode/command/speckit.tasks.md
- ✅ .opencode/command/speckit.implement.md
Follow-up TODOs:
- None
-->
# ssh-man Constitution

## Core Principles

### I. Go, Wails, and Svelte Define the Product Stack
Application features MUST ship as a Go application using Wails as the desktop
runtime and Svelte as the frontend framework. Core behavior MUST be implemented
in Go, the desktop shell MUST be the Wails application rather than a parallel
CLI, web server, or alternate GUI framework, and the frontend MUST use Svelte
without SvelteKit or another meta-framework layered on top. Any additional
runtime beyond the Go toolchain, Wails, and Svelte MUST be justified in the
plan and approved through the Complexity Tracking section. Rationale: a single,
explicit product stack keeps delivery, debugging, packaging, and frontend
architecture predictable for Linux and macOS.

### II. Linux and macOS Parity Is Mandatory
Every user-facing feature MUST behave consistently on Linux and macOS unless a
documented platform limitation makes parity impossible. Platform-specific
branches MUST be isolated behind explicit adapters, and each spec and plan MUST
call out filesystem, permissions, shell, packaging, and windowing differences
that can affect the feature. Rationale: cross-platform support is a product
requirement, not a best-effort goal.

### III. Idiomatic Go Packages and Boundaries
Business logic, state transitions, SSH orchestration, and data access MUST live
in small, focused Go packages that can be exercised without launching the Wails
runtime. `app.go`, Wails bindings, and frontend calls MUST stay thin and focused
on transport, presentation, and marshaling. Public APIs MUST be intentionally
small, exported identifiers MUST be documented when their behavior is not
obvious, and packages MUST avoid circular dependencies, hidden side effects, and
mutable global state. Rationale: idiomatic package boundaries keep the codebase
readable, testable, and easy to evolve.

### IV. Go Quality Gates Are Non-Negotiable
Every change MUST leave the Go codebase in a production-quality state. Changed
Go code MUST be formatted, MUST pass `go vet`, and MUST pass `go test ./...` or
an explicitly justified narrower test selection. When the repository configures
additional static analysis such as `staticcheck`, those checks MUST pass as
well. Error returns MUST be handled explicitly, errors crossing package
boundaries MUST preserve useful context, and concurrency-sensitive code MUST
make ownership, cancellation, and shutdown behavior explicit. Rationale: Go code
stays healthy when correctness and clarity are enforced continuously.

### V. Keep the Native Surface Area Small
New dependencies, background processes, native integrations, and shell-outs MUST
be minimized. Prefer the Go standard library and Wails-supported patterns before
adding third-party packages or OS-specific tooling. When shelling out is
unavoidable, the command contract, failure handling, and supported platforms
MUST be documented in the plan and verified in tests or smoke checks. Rationale:
small native surface area improves reliability and reduces platform drift.

## Go Engineering Standards

- Go packages MUST have a single clear responsibility and predictable ownership
  of state.
- `context.Context` MUST be the first parameter for request-scoped or
  cancellable operations, and context values MUST NOT be used as hidden optional
  parameters.
- Interfaces MUST be introduced only at the consumer boundary where substitution
  is required; do not create interface layers solely for future-proofing.
- Struct zero values, nil handling, and lifecycle expectations MUST be clear in
  code and tests.
- Exported functions and methods MUST return actionable errors; panic is
  reserved for unrecoverable programmer errors or impossible initialization
  states.
- Reviews MUST reject speculative abstractions, oversized packages, and helper
  layers that obscure straightforward Go control flow.

## Frontend Constraints

- Frontend implementation MUST use plain Svelte components and shared app-level
  services; do not introduce SvelteKit, CSS frameworks, CSS modules,
  component-scoped style blocks, CSS-in-JS, or runtime-generated styling.
- Visual styling MUST live in a single shared CSS file containing the project's
  class definitions. New UI work MUST reuse or extend that stylesheet rather
  than creating per-component or per-feature CSS files.
- Markup, class names, and interaction states MUST remain readable and explicit;
  avoid dynamic class composition patterns that hide the final styling contract.
- Plans, tasks, and reviews for frontend work MUST call out the shared
  stylesheet path and verify that any new UI respects the single-file CSS rule.

## Platform Constraints

- Supported desktop targets MUST include Linux and macOS.
- Windows support is out of scope unless this constitution is amended.
- Plans MUST declare the Go version, Wails version, Svelte version, shared CSS
  file path, and the build commands and quality commands used to validate Linux
  and macOS.
- Specs for desktop features MUST identify platform-sensitive edge cases when
  permissions, filesystems, SSH behavior, or OS integration can vary.
- Release-affecting changes MUST document packaging assumptions for each
  supported platform, including any signing or notarization expectations.

## Delivery Workflow

- Specifications MUST describe the user journey, acceptance scenarios, and
  platform-sensitive edge cases without leaking implementation detail.
- Implementation plans MUST pass the Constitution Check before research and
  again after design, with explicit notes for any justified complexity.
- Task lists MUST include work for Go domain logic, Wails bindings or frontend
  integration, shared stylesheet updates when UI changes, formatting, static
  analysis, and the automated verification needed for the affected scope.
- Pull requests and reviews MUST confirm constitution compliance, with explicit
  attention to platform parity, package boundaries, idiomatic Go, Svelte-only
  frontend scope, single-file CSS compliance, and verification coverage.
- Quickstart or validation docs MUST include the commands needed to run tests
  and confirm Linux and macOS builds for the feature.

## Governance

This constitution supersedes conflicting local practices for this repository.
Amendments MUST be made in `.specify/memory/constitution.md`, MUST include an
updated Sync Impact Report, and MUST propagate any resulting changes into the
plan, spec, task, and command templates before the amendment is considered
complete.

Versioning follows semantic versioning for governance documents: MAJOR for
backward-incompatible principle changes or removals, MINOR for new principles or
materially expanded obligations, and PATCH for clarifications that do not change
enforcement. Compliance review is mandatory during planning, task generation,
implementation, and code review. Reviews MUST verify that required formatting,
static analysis, tests, platform validation evidence, and the approved Svelte +
single-file CSS frontend constraints are present. Any deviation MUST be
documented in the relevant artifact's justification section rather than silently
accepted.

**Version**: 1.2.0 | **Ratified**: 2026-04-06 | **Last Amended**: 2026-04-06
