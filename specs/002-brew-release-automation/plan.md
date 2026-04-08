# Implementation Plan: Homebrew Installation and Automated Releases

**Branch**: `002-brew-release-automation` | **Date**: 2026-04-08 | **Spec**: [/home/ericwooley/ssh-man/specs/002-brew-release-automation/spec.md](/home/ericwooley/ssh-man/specs/002-brew-release-automation/spec.md)
**Input**: Feature specification from `/specs/002-brew-release-automation/spec.md`

## Summary

Add an automated release pipeline for the existing Go + Wails desktop app that builds official Linux and macOS release artifacts from GitHub Actions, publishes tagged GitHub Releases, and keeps a Homebrew-installable macOS distribution path synchronized through a project-owned Homebrew tap and cask. The implementation stays out of the product UI, preserves the existing Wails/Svelte stack, and focuses on repeatable packaging, release traceability, and platform-aware validation.

## Technical Context

**Language/Version**: Go 1.22.2, Node/npm for frontend build tooling, shell scripts for local validation  
**Primary Dependencies**: Wails v2.12.0, plain Svelte 5.28.2, Vite 6, Vitest 3, SQLite via `github.com/mattn/go-sqlite3`, GitHub Actions, GitHub Releases, Homebrew cask/tap metadata  
**Storage**: Existing app data remains unchanged; feature state lives in repository-managed workflow files, release metadata, packaged artifacts, and Homebrew tap cask metadata  
**Testing**: `go test ./...`, `npm run validate --prefix frontend`, release workflow dry-run validation where possible, contract review for release assets, and Linux/macOS packaging smoke checks documented in `tests/smoke/`  
**Quality Gates**: `gofmt -w .`, `go vet ./...`, `go test ./...`, `npm run validate --prefix frontend`, `./scripts/validate.sh`, `./scripts/build-current-os.sh`, `./scripts/wails-build-linux.sh`, macOS Wails build on a macOS host, and release workflow verification through GitHub Actions runs  
**Target Platform**: Linux and macOS release artifacts; Homebrew installation path on macOS  
**Project Type**: desktop-app (Wails) with repository-hosted release automation  
**Frontend Style Rules**: Plain Svelte only, shared global stylesheet at `frontend/src/app.css`, explicit classes only, no CSS modules, no scoped component styles, no runtime-generated styling; no end-user UI changes are planned for this feature  
**Performance Goals**: Successful automated releases publish all required artifacts in a single workflow run, and Homebrew metadata reflects a successful release within 15 minutes in normal operation  
**Constraints**: Preserve Linux/macOS parity for official artifacts, minimize new native tooling, keep release logic mostly in GitHub-hosted workflows and repository scripts, use a Homebrew cask rather than a formula for the desktop app, and document macOS signing/notarization assumptions if unsigned release assets remain acceptable for the initial automated path  
**Scale/Scope**: One repository, one release workflow family, one project-owned Homebrew tap/cask definition, versioned releases for the existing desktop app, and no redesign of runtime application behavior

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

- [x] Runtime remains Go + Wails with plain Svelte; release automation does not introduce an alternate product runtime.
- [x] Linux/macOS parity is addressed by building and validating official artifacts for both platforms while clearly scoping Homebrew installation to macOS.
- [x] Core app package boundaries remain intact because the feature is centered on packaging, repository scripts, and workflow automation rather than new runtime orchestration in the app.
- [x] Go changes, if any, remain thin and release-focused, with explicit command contracts and actionable failures.
- [x] UI work remains constrained to plain Svelte and the single shared stylesheet at `frontend/src/app.css`; no new UI surface is required by this design.
- [x] Verification covers formatting, vet, automated tests, frontend validation, Linux/macOS build checks, and release/workflow smoke validation.
- [x] Native surface area stays small by preferring Wails-supported builds, GitHub-hosted workflow automation, and Homebrew cask metadata over custom installers or new background services.

## Project Structure

### Documentation (this feature)

```text
specs/002-brew-release-automation/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── release-distribution.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
└── app/
    └── main.go

internal/
├── app/
│   ├── bindings/
│   ├── bootstrap/
│   └── menu/
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
└── [wails packaging assets and output binaries]

scripts/
├── build-current-os.sh
├── dev-current-os.sh
├── validate.sh
└── wails-build-linux.sh

.github/
└── workflows/

tests/
├── integration/
└── smoke/
```

**Structure Decision**: Use the existing desktop-app structure. Keep application runtime code under `internal/` and `cmd/`, add release automation under `.github/workflows/`, add any release helper scripts under `scripts/`, keep end-user documentation updates in repository markdown files, and avoid any per-feature frontend assets because this feature does not require new UI.

## Phase 0 Research Decisions

- Use a Homebrew cask, not a formula, because the product is a desktop application distributed as packaged binaries rather than a CLI-first tool.
- Use a project-owned Homebrew tap and cask as the first automated distribution target so the repository can update release assets and Homebrew metadata in one controlled automation flow without waiting for external maintainers.
- Use GitHub Actions plus tagged GitHub Releases as the authoritative release pipeline and artifact source for official builds.
- Publish stable, versioned asset names per platform so release automation, Homebrew metadata, and user documentation all point at the same predictable release outputs.
- Treat macOS signing and notarization as an explicit packaging assumption to document during this phase; unsigned assets may be acceptable for initial automation if the release notes and validation docs make the user impact clear.

## Phase 1 Design Overview

### In Scope

- GitHub Actions workflows for tagged builds, release creation, and asset publication.
- Repository configuration or scripts needed to package official Linux and macOS release artifacts.
- Homebrew tap/cask metadata and an automated update path tied to successful releases.
- Documentation for installation, upgrade, validation, and release rehearsal.
- Release smoke checks and traceability between version tags, release assets, and Homebrew metadata.

### Out of Scope

- Redesigning app runtime behavior or adding end-user UI.
- Windows distribution.
- Replacing existing local development or validation scripts beyond what is needed to support release automation.
- Full Apple signing/notarization automation unless repository secrets and organizational requirements are already available.
- Publishing to `homebrew-core` as part of the first automation path.

### Design Decisions

- Keep release generation driven by version tags so the Git history, release version, and published assets stay aligned.
- Generate Linux and macOS artifacts from GitHub Actions and attach them to a single GitHub Release for each published version.
- Publish a macOS archive format that Homebrew cask can install directly and checksum reliably.
- Update the project-owned Homebrew tap only after release asset publication succeeds, using the exact published version and checksum.
- Preserve a stable naming convention for release assets so docs, workflows, and cask metadata do not need per-release manual edits.
- Document platform-specific build assumptions, especially Linux WebKit-related Wails tags and any macOS signing/notarization caveats.

### Planned Verification

- Validate Go and frontend quality gates with the existing repository commands.
- Verify Linux packaging through `./scripts/wails-build-linux.sh` and current validation helpers.
- Verify macOS packaging on a macOS GitHub Actions runner and document any unsigned/notarized behavior.
- Confirm a tagged release creates the expected GitHub Release entry, assets, and traceable version metadata.
- Confirm the Homebrew tap update references the published macOS asset and matching checksum.
- Rehearse install and upgrade flows from the generated Homebrew instructions against a released version.

## Complexity Tracking

No constitution violations or justified exceptions are required for this plan.
