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

## Homebrew Tap Credential

The workflow expects the exact secret name `TAP_GITHUB_TOKEN`. Store it as an environment secret in a protected GitHub environment named `release`, not as a repository or organization secret:

1. In GitHub, create a fine-grained personal access token owned by `ericwooley`.
2. Select **Only select repositories** and choose only `ericwooley/homebrew-apps`.
3. Grant repository **Contents: Read and write**. Leave every other permission read-only or unset.
4. Choose the shortest practical expiration, preferably 90 days or less.
5. In `ericwooley/ssh-man`, open **Settings → Environments → release**, configure required reviewers and a version-tag deployment restriction, then add `TAP_GITHUB_TOKEN` under **Environment secrets**.
6. Remove any repository-level or organization-level copy of `TAP_GITHUB_TOKEN` after the environment secret is verified.

With the environment protection enabled, the unprivileged build and validation work can complete first. The tap-publishing job then waits for a reviewer; the credential is withheld until an authorized reviewer approves that known release tag and commit. Rejecting the deployment prevents the tap checkout and update.

Set a rotation reminder before expiration. Create and install the replacement token first, verify it with a release, and then revoke the old token. The preferred long-term credential is a GitHub App installed only on `homebrew-apps` with **Contents: Read and write**, provided the workflow is changed to generate a short-lived installation token during the protected publish job. The generated installation token itself must not be stored as `TAP_GITHUB_TOKEN`.

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
3. Confirm `command -v ssh-man` resolves to the executable inside the installed `ssh-man.app` bundle.
4. Run `ssh-man version` and confirm it matches the cask and GitHub Release version.
5. Launch the installed app and note any unsigned-app messaging that must remain in the documentation.
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
