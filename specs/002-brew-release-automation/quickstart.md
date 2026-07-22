# Quickstart: Homebrew Installation and Automated Releases

## Purpose

This document describes how to validate the release-automation feature during development and rehearse the user-facing install paths for macOS and Linux.

## Preconditions

- Go 1.22.2 installed locally.
- Node.js and npm installed for the existing Wails frontend build.
- Wails v2 CLI installed and passing `wails doctor` where applicable.
- GitHub repository access sufficient to inspect or run release workflows.
- Access to the project-owned Homebrew tap or the automation target repository.
- A Developer ID Application certificate and Apple notarization credentials stored in the protected `release` environment.
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

## Protected Release Credentials

The workflow expects `TAP_GITHUB_TOKEN` plus the Apple signing secrets listed in the repository README. Store them as environment secrets in a protected GitHub environment named `release`, not as repository or organization secrets:

1. In GitHub, create a fine-grained personal access token owned by `ericwooley`.
2. Select **Only select repositories** and choose only `ericwooley/homebrew-apps`.
3. Grant repository **Contents: Read and write**. Leave every other permission read-only or unset.
4. Choose the shortest practical expiration, preferably 90 days or less.
5. In `ericwooley/ssh-man`, open **Settings → Environments → release**, allow deployments from `main`, then add `TAP_GITHUB_TOKEN` and the five documented Apple secrets under **Environment secrets**.
6. Remove any repository-level or organization-level copy of `TAP_GITHUB_TOKEN` after the environment secret is verified.

The unprivileged build and validation work completes before the protected job can read any credential. That job imports the Developer ID certificate into a temporary keychain, signs and notarizes the DMG, publishes it, updates the tap, and removes the temporary signing material.

Set a rotation reminder before expiration. Create and install the replacement token first, verify it with a release, and then revoke the old token. The preferred long-term credential is a GitHub App installed only on `homebrew-apps` with **Contents: Read and write**, provided the workflow is changed to generate a short-lived installation token during the protected publish job. The generated installation token itself must not be stored as `TAP_GITHUB_TOKEN`.

## Release Automation Rehearsal

1. Install the repository hooks with `./scripts/install-git-hooks.sh` and confirm a non-Conventional Commit message is rejected.
2. Confirm the latest release uses a plain semantic tag such as `1.2.3`.
3. Merge or push a `fix:`, `perf:`, `feat:`, or breaking Conventional Commit to `main` (or run the workflow manually against such an unreleased commit range).
4. Confirm GitHub Actions calculates the correct next version and creates one GitHub Release.
5. Confirm the GitHub Release exposes the signed and notarized macOS artifact and release notes or summary.
6. Record the published macOS artifact URL and checksum.
7. Confirm the Homebrew cask update in the project-owned tap uses the exact version, URL, and checksum from the successful macOS release.
8. Confirm a failed or partial workflow run does not leave Homebrew metadata pointing to incomplete artifacts.

## macOS Homebrew Rehearsal

1. Follow the documented tap and install instructions on a supported macOS environment.
2. Confirm the installation resolves to the latest successful official release.
3. Confirm `command -v ssh-man` resolves to the executable inside the installed `ssh-man.app` bundle.
4. Run `ssh-man version` and confirm it matches the cask and GitHub Release version.
5. Run `codesign --verify --deep --strict /Applications/ssh-man.app` and `spctl --assess --type execute --verbose=4 /Applications/ssh-man.app`, then launch the app normally.
6. Publish or simulate a newer successful release.
7. Run the documented Homebrew upgrade flow.
8. Confirm the installed app and `ssh-man version` both match the newer official release.

## Linux Support Rehearsal

1. Follow the documented Linux clone-and-build workflow from a fresh clone.
2. Run `./scripts/build-current-os.sh` on Linux or the documented Linux-specific build commands.
3. Run `./scripts/wails-build-linux.sh` to validate the packaged Linux build path still works where the host environment supports it.
4. Confirm the Linux guidance does not claim an official automated release artifact is available.

## Expected Outputs

- One tagged GitHub Release with the semantic version tag `1.2.3` style.
- One or more Developer ID signed, notarized, and stapled macOS artifacts suitable for Homebrew distribution.
- Homebrew cask metadata in the project-owned tap updated to the matching version and checksum.
- Linux support guidance that remains accurate for clone-and-build users.

## Documentation Expectations

Implementation should leave behind:

- User-facing Homebrew install and upgrade guidance for macOS.
- User-facing or maintainer-facing Linux clone-and-build guidance that remains accurate.
- Maintainer-facing release notes describing how tagged releases, macOS artifacts, and tap updates are validated.

## Platform Validation Notes

- macOS: this feature owns the official packaged install path, Homebrew flow, Developer ID signing, notarization, and stapled-ticket validation.
- Linux: this feature must keep clone-and-build support accurate and validated, even though Linux does not receive an official automated release artifact yet.
- Both platforms: product support remains documented, but release-channel parity is intentionally deferred.
