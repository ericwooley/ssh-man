# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: [e.g., Go 1.24, Swift 5.9, Rust 1.75 or NEEDS CLARIFICATION]  
**Primary Dependencies**: [e.g., Wails, Svelte (no SvelteKit), FastAPI, UIKit or NEEDS CLARIFICATION]  
**Storage**: [if applicable, e.g., PostgreSQL, CoreData, files or N/A]  
**Testing**: [e.g., go test ./..., Vitest, XCTest or NEEDS CLARIFICATION]  
**Quality Gates**: [e.g., gofmt, go vet, staticcheck, frontend lint/build or NEEDS CLARIFICATION]  
**Target Platform**: [e.g., Linux, macOS, Linux server, iOS 15+ or NEEDS CLARIFICATION]
**Project Type**: [e.g., desktop-app (Wails), library, cli, web-service or NEEDS CLARIFICATION]  
**Frontend Style Rules**: [e.g., Svelte + single global CSS file at frontend/src/app.css, no CSS modules/dynamic styling, or NEEDS CLARIFICATION]  
**Performance Goals**: [domain-specific, e.g., 1000 req/s, 10k lines/sec, 60 fps or NEEDS CLARIFICATION]  
**Constraints**: [domain-specific, e.g., <200ms p95, <100MB memory, offline-capable or NEEDS CLARIFICATION]  
**Scale/Scope**: [domain-specific, e.g., 10k users, 1M LOC, 50 screens or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [ ] Runtime stays Go + Wails; any alternate runtime or heavyweight dependency is justified.
- [ ] Frontend stays on plain Svelte without SvelteKit or alternate UI frameworks.
- [ ] Linux and macOS behavior, packaging, and platform-specific risks are explicitly addressed.
- [ ] Core logic lives in small, focused Go packages; Wails bindings and UI adapters remain thin.
- [ ] Go package boundaries, context usage, error handling, and concurrency behavior are explicit and idiomatic.
- [ ] UI styling uses one shared CSS file with explicit classes; no CSS modules, scoped component CSS, or runtime-generated styling are introduced.
- [ ] Verification includes formatting, static analysis, automated tests, plus build or smoke checks for affected platform-sensitive work.
- [ ] New native integrations, shell-outs, or background services are minimized and documented.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
# [REMOVE IF UNUSED] Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# [REMOVE IF UNUSED] Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# [REMOVE IF UNUSED] Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure: feature modules, UI flows, platform tests]

# [REMOVE IF UNUSED] Option 4: Desktop app (Go + Wails)
cmd/
└── app/
    └── main.go

internal/
├── app/
├── domain/
├── platform/
└── ssh/

frontend/
├── src/
│   └── app.css
└── package.json

build/
└── [packaging assets]

tests/
├── integration/
└── smoke/
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
