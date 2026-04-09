# Implementation Plan: Homebrew Installation and Automated Releases

**Branch**: `002-brew-release-automation` | **Date**: 2026-04-08 | **Spec**: [/home/ericwooley/ssh-man/specs/002-brew-release-automation/spec.md](/home/ericwooley/ssh-man/specs/002-brew-release-automation/spec.md)
**Input**: Feature specification from `/specs/002-brew-release-automation/spec.md`

## Summary

Add a repository-managed release pipeline for the existing Go + Wails desktop app that builds and publishes official macOS release artifacts from GitHub Actions, exposes the supported install and upgrade path through a Homebrew cask in a project-owned tap, and keeps Linux explicitly supported through clone-and-build workflows rather than matching the macOS automated release cycle.

## Technical Context

**Language/Version**: Go 1.22.2, Node/npm for frontend build tooling, shell scripts for local validation  
**Primary Dependencies**: Wails v2.12.0, plain Svelte 5.28.2, Vite 6, Vitest 3, SQLite via `github.com/mattn/go-sqlite3`, GitHub Actions, GitHub Releases, Homebrew cask/tap metadata  
**Storage**: Existing runtime app data is unchanged; feature state lives in repository-managed workflow files, release metadata, packaged artifacts, and Homebrew tap cask metadata  
**Testing**: `go test ./...`, `npm run validate --prefix frontend`, release workflow dry-run validation, contract review, Homebrew install or upgrade rehearsal on macOS, Linux clone-and-build smoke validation  
**Quality Gates**: `gofmt -w .`, `go vet ./...`, `go test ./...`, `npm run validate --prefix frontend`, `./scripts/validate.sh`, `./scripts/build-current-os.sh`, `./scripts/wails-build-linux.sh`, macOS Wails build on a macOS host, GitHub Actions release workflow verification  
**Target Platform**: Linux and macOS desktop product support; official automated release artifacts and Homebrew distribution on macOS only for this feature  
**Project Type**: desktop-app (Wails)  
**Frontend Style Rules**: Plain Svelte only, shared stylesheet at `frontend/src/app.css`, no SvelteKit, CSS modules, scoped styles, or runtime-generated styling; no end-user UI changes planned  
**Performance Goals**: Successful official macOS releases publish complete assets in one workflow run, and Homebrew cask metadata reflects the latest successful release within 15 minutes in 95% of validation runs  
**Constraints**: Linux and macOS product support must remain documented, release automation is GitHub Actions-based, Homebrew path must use a cask in a project-owned tap, release tags use `1.2.3`, macOS signing or notarization is not required for this feature, and native tooling should stay minimal  
**Scale/Scope**: One repository, one release workflow family, one project-owned Homebrew tap, one official macOS artifact path, Linux support via source-based workflow, and no runtime or UI redesign

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

- [x] Runtime remains Go + Wails with plain Svelte and keeps release automation outside the desktop runtime boundary.
- [x] Linux/macOS parity is addressed by preserving Linux support through clone-and-build validation while documenting macOS-specific packaged distribution and release-path differences.
- [x] Core logic impact stays limited to release scripts, workflow definitions, and packaging metadata; Wails bindings and frontend adapters remain unchanged.
- [x] Go design expectations remain idiomatic because any supporting automation helpers stay small, explicit, and error-aware.
- [x] UI scope remains plain Svelte with no new component styling beyond the existing shared stylesheet contract at `frontend/src/app.css`.
- [x] Verification covers formatting, vet, automated tests, Linux build validation, macOS packaging validation, Homebrew rehearsal, and release workflow verification.
- [x] Native surface area stays small by preferring GitHub-hosted workflows, existing repository scripts, and Homebrew cask metadata over new background services or heavyweight packaging systems.

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
├── domain/
├── platform/
├── sqlite/
└── ssh/

frontend/
├── src/
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

**Structure Decision**: Keep runtime application code under `cmd/` and `internal/`, keep frontend code unchanged under `frontend/` with the shared stylesheet at `frontend/src/app.css`, place release automation in `.github/workflows/`, use `scripts/` for reusable release-validation helpers, and limit documentation changes to feature docs and user-facing release or install guidance.

## Phase 0 Research Decisions

- Use a Homebrew cask, not a formula, for the official macOS install path.
- Publish that cask through a project-owned tap rather than targeting `homebrew-core` initially.
- Use GitHub Actions plus GitHub Releases as the authoritative official release channel.
- Use plain semantic version tags in the format `1.2.3`.
- Publish official automated release artifacts for macOS only in this feature while keeping Linux supported through documented clone-and-build workflows.
- Treat macOS signing and notarization as documented non-requirements for this feature and surface unsigned-app expectations in installation guidance.

## Phase 1 Design Overview

### In Scope

- GitHub Actions workflows for tagged official macOS release builds.
- Repository scripts or config updates needed to package and verify macOS release artifacts.
- GitHub Release publication with traceable version, tag, and attached macOS assets.
- Homebrew cask metadata and project-owned tap update flow tied to successful macOS releases.
- Documentation for Homebrew install and upgrade, unsigned macOS release expectations, and Linux clone-and-build support.
- Validation steps covering Linux source-build support, macOS packaged install path, and release traceability.

### Out of Scope

- Official automated Linux release artifacts.
- Windows packaging or distribution.
- Runtime application redesign, Wails binding changes, or frontend feature work.
- Full Apple signing or notarization automation.
- Publishing to `homebrew-core` in the initial release-automation feature.

### Design Decisions

- Drive official releases from maintainer-controlled plain semantic version tags.
- Build and publish the official macOS release artifact from GitHub Actions and attach it to one GitHub Release.
- Keep Linux product support documented through existing clone-and-build scripts and validation rather than adding Linux artifact publication in this feature.
- Update the Homebrew cask in a project-owned tap only after the macOS asset URL and checksum are known from a successful release.
- Preserve stable asset naming and release metadata so the Homebrew cask and GitHub Release remain traceable to the same version.
- Make release failure visible to maintainers and prevent incomplete automation runs from appearing as fully available releases.

### Planned Verification

- Run existing Go and frontend quality gates from the repository root.
- Validate Linux support with `./scripts/build-current-os.sh` and `./scripts/wails-build-linux.sh` on Linux-capable environments.
- Validate the packaged macOS build on a macOS host or runner.
- Confirm the GitHub Release contains the expected macOS asset and matching `1.2.3` tag.
- Confirm the Homebrew tap update references the exact macOS asset URL and checksum from the successful release.
- Rehearse macOS Homebrew install and upgrade flows and confirm Linux documentation still points users to clone-and-build workflows.

## Complexity Tracking

No constitution violations or justified exceptions are required for this plan.
