# Quickstart: Homebrew Installation and Automated Releases

## Purpose

This document describes how to validate the release-automation feature during development and how to rehearse the user-facing Homebrew install path once official release assets are available.

## Preconditions

- Go 1.22.2 installed locally for repository validation.
- Node.js and npm installed for the plain Svelte frontend build.
- Wails v2 CLI installed and passing `wails doctor` on supported development machines.
- Access to the GitHub repository with permission to inspect workflow runs and releases.
- Access to the project-owned Homebrew tap repository or automation target for cask updates.
- A macOS environment with Homebrew installed for installation and upgrade rehearsal.
- A Linux or macOS environment available to validate packaged builds.

## Baseline Validation Commands

Run these commands from the repository root before rehearsing release automation:

```bash
wails doctor
gofmt -w .
go vet ./...
go test ./...
npm run validate --prefix frontend
./scripts/validate.sh
./scripts/build-current-os.sh
./scripts/wails-build-linux.sh
```

## Release Automation Rehearsal

1. Confirm the repository versioning approach for the release you want to test.
2. Verify the release workflow configuration points to the intended artifact names and supported platforms.
3. Create or simulate a version tag in a safe test context.
4. Run the GitHub Actions release workflow.
5. Confirm the workflow publishes a tagged GitHub Release.
6. Confirm the release includes the expected Linux and macOS assets.
7. Record the published macOS asset URL and checksum.
8. Confirm the Homebrew tap or cask update uses that exact version, URL, and checksum.
9. Verify a failed release path does not update the Homebrew metadata.

## Homebrew Install Rehearsal

Use a macOS machine with Homebrew installed.

1. Follow the published tap and install instructions for the project.
2. Confirm the install resolves to the latest successful official release.
3. Launch the installed application and verify it starts successfully.
4. Publish or simulate a newer successful release.
5. Run the documented Homebrew upgrade flow.
6. Confirm the installed version updates to the new release.

## Expected Artifacts to Verify

- One tagged GitHub Release entry for the target version.
- One macOS artifact suitable for Homebrew cask installation.
- One or more Linux release artifacts.
- Release notes or generated release summary.
- Homebrew cask metadata updated to the matching version and checksum.

## Platform Validation Notes

- Linux: verify the Wails Linux build flow remains valid and produces a releasable artifact from the automated pipeline.
- macOS: verify the build artifact is suitable for Homebrew installation and document whether the artifact is signed or unsigned.
- Both platforms: confirm the same release version is represented consistently in tags, release metadata, and asset names.

## Documentation Updates Expected from Implementation

- User-facing installation guidance must describe the official Homebrew install and upgrade flow.
- Maintainer-facing release documentation must describe how to trigger, observe, and troubleshoot automated releases.
- Smoke-validation notes should record at least one successful release rehearsal and one successful Homebrew install or upgrade rehearsal.
