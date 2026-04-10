 # Implementation Plan: Vanilla Framework UI Refresh

**Branch**: `003-vanilla-ui-refresh` | **Date**: 2026-04-10 | **Spec**: /Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/spec.md
**Input**: Feature specification from `/Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Rework the application's visual system to use a cohesive set of component styles inspired by the provided Vanilla Framework reference. This is a visual-system migration affecting all Svelte frontend surfaces: main workspace, lists, dialogs, forms, banners, and status/diagnostic views. The plan preserves existing app behavior, data models, and Go/Wails runtime boundaries while replacing or mapping existing CSS classes to the new design tokens and component patterns.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.24+ for backend code; Svelte 5.x for frontend (repository currently lists Svelte ^5.28.2).  
**Primary Dependencies**: Wails (desktop runtime), Svelte (no SvelteKit), Node toolchain for frontend build (pnpm/npm and Vite).  
**Storage**: Local application data stored in OS-specific config directories (no change).  
**Testing**: Go: `go test ./...`; Frontend: Vitest as configured in `frontend/package.json` (`vitest run`).  
**Quality Gates**: gofmt/go vet/staticcheck for Go; `pnpm --dir frontend run validate` (build + test) and Vitest for frontend.  
**Target Platform**: macOS and Linux desktop builds (Windows out of scope).  
**Project Type**: Desktop app (Wails + Go backend, Svelte frontend).  
**Frontend Style Rules**: Svelte-only frontend; single shared stylesheet located at `/Users/ericwooley/projects/ssh-man/frontend/src/app.css`. No per-component scoped CSS, CSS-in-JS, or CSS modules will be introduced.  
**Performance Goals**: Visual refresh must not materially degrade runtime performance; interactive surfaces must remain responsive for typical desktop usage.  
**Constraints**: Maintain Linux/macOS parity; preserve accessibility cues and avoid communicating state by color alone.  
**Scale/Scope**: Visual system replacement touches all frontend components and the single shared CSS file; backend code and Go packages remain unchanged.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] Runtime stays Go + Wails; any alternate runtime or heavyweight dependency is justified.
- [x] Frontend stays on plain Svelte without SvelteKit or alternate UI frameworks.
- [x] Linux and macOS behavior, packaging, and platform-specific risks are explicitly addressed.
- [x] Core logic lives in small, focused Go packages; Wails bindings and UI adapters remain thin.
- [x] Go package boundaries, context usage, error handling, and concurrency behavior are explicit and idiomatic.
- [x] UI styling uses one shared CSS file with explicit classes; no CSS modules, scoped component CSS, or runtime-generated styling are introduced.
- [x] Verification includes formatting, static analysis, automated tests, plus build or smoke checks for affected platform-sensitive work.
- [x] New native integrations, shell-outs, or background services are minimized and documented.

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
