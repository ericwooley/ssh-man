# Quickstart: Homebrew Installation and Automated Releases

## Purpose

This document describes how to validate the release-automation feature during development and rehearse the user-facing install paths for macOS and Linux.

## Preconditions

- Go 1.22.2 installed locally.
- Node.js and npm installed for the existing Wails frontend build.
- Wails v2 CLI installed and passing `wails doctor` where applicable.
- GitHub repository access sufficient to inspect or run release workflows.
- Access to the project-owned Homebrew tap or the automation target repository.
- A macOS environment with Homebrew for install and upgrade rehearsal.
- A Linux or Linux-capable environment for clone-and-build validation.

## Baseline Validation Commands

Run these commands from the repository root before validating release automation:

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

1. Confirm the intended release version uses the semantic tag format `1.2.3`.
2. Confirm the GitHub Actions workflow is configured to build and publish the official macOS artifact.
3. Create or simulate the release trigger using the approved version tag.
4. Run the release workflow and confirm one GitHub Release is created.
5. Confirm the GitHub Release exposes the expected macOS artifact and release notes or summary.
6. Record the published macOS artifact URL and checksum.
7. Confirm the Homebrew cask update in the project-owned tap uses the exact version, URL, and checksum from the successful macOS release.
8. Confirm a failed or partial workflow run does not leave Homebrew metadata pointing to incomplete artifacts.

## macOS Homebrew Rehearsal

1. Follow the documented tap and install instructions on a supported macOS environment.
2. Confirm the installation resolves to the latest successful official release.
3. Launch the installed app and note any unsigned-app messaging that must remain in the documentation.
4. Publish or simulate a newer successful release.
5. Run the documented Homebrew upgrade flow.
6. Confirm the installed version matches the newer official release.

## Linux Support Rehearsal

1. Follow the documented Linux clone-and-build workflow from a fresh clone.
2. Run `./scripts/build-current-os.sh` on Linux or the documented Linux-specific build commands.
3. Run `./scripts/wails-build-linux.sh` to validate the packaged Linux build path still works where the host environment supports it.
4. Confirm the Linux guidance does not claim an official automated release artifact is available.

## Expected Outputs

- One tagged GitHub Release with the semantic version tag `1.2.3` style.
- One or more official macOS artifacts suitable for Homebrew distribution.
- Homebrew cask metadata in the project-owned tap updated to the matching version and checksum.
- Linux support guidance that remains accurate for clone-and-build users.

## Documentation Expectations

Implementation should leave behind:

- User-facing Homebrew install and upgrade guidance for macOS.
- User-facing or maintainer-facing Linux clone-and-build guidance that remains accurate.
- Maintainer-facing release notes describing how tagged releases, macOS artifacts, and tap updates are validated.

## Platform Validation Notes

- macOS: this feature owns the official packaged install path, Homebrew flow, and unsigned-app disclosure if signing remains disabled.
- Linux: this feature must keep clone-and-build support accurate and validated, even though Linux does not receive an official automated release artifact yet.
- Both platforms: product support remains documented, but release-channel parity is intentionally deferred.
