# Contract: Release Distribution

## Purpose

This contract defines the externally visible release boundary for official macOS release automation, Homebrew cask distribution, and Linux support guidance.

## Design Principles

- GitHub Actions is the authoritative automation boundary for official releases.
- The release tag, GitHub Release, macOS asset, and Homebrew cask metadata must all reference the same semantic version.
- Linux remains a supported platform, but this feature does not require Linux to share the official automated release artifact cycle.
- Failed or partial automation must be visible and must not appear fully available to end users.

## Release Trigger Contract

Official releases are initiated by a maintainer-controlled workflow trigger bound to a semantic version tag.

**Inputs**
- `version_tag`: Required plain semantic version string in the format `1.2.3`.
- `release_channel`: Optional stable channel indicator if the workflow supports it.

**Behavior**
- Start one authoritative GitHub Actions release workflow run.
- Associate the workflow run to the requested `version_tag`.
- Reject malformed or duplicate tags.

## Release Output Contract

Each successful official release must produce:

- One user-visible GitHub Release.
- One matching repository tag in the format `1.2.3`.
- One or more macOS artifacts suitable for official distribution.
- Release notes or an equivalent generated summary.

This feature does not require official automated Linux artifacts.

## Artifact Metadata Contract

Each published official macOS artifact must expose:

- `name`
- `version`
- `platform`
- `architecture`
- `packageFormat`
- `checksum`
- `downloadUrl`

Artifact metadata must remain stable enough for Homebrew cask updates and release verification.

## Homebrew Contract

The official Homebrew path is a cask in a project-owned tap.

**Required cask fields**
- `caskName`
- `version`
- `sha256`
- `url`
- `homepage`
- `appBundleReference`

**Update rules**
- Update the cask only after the macOS artifact has been published successfully.
- Set `version`, `url`, and `sha256` from the actual published macOS artifact.
- Do not advance the cask to a version backed by a failed or partial release.

**User flow requirements**
- The documented Homebrew path must support install and upgrade on supported macOS environments.
- The documentation must clearly state if the macOS app is unsigned.

## Linux Support Contract

Linux remains a supported product platform through a documented clone-and-build workflow.

**Requirements**
- User-facing or maintainer-facing guidance must identify the Linux support path.
- Linux validation must continue to reference repository build commands or smoke checks.
- The release documentation must not imply that Linux users receive an official automated release artifact when they do not.

## Failure Behavior Contract

- Preserve failure details for maintainers when tagging, workflow execution, artifact publication, or cask updates fail.
- Prevent failed releases from being treated as fully published.
- Prevent Homebrew metadata from referencing incomplete or missing artifacts.

## Traceability Contract

Maintainers must be able to trace a brew-installable version back to:

- The semantic version tag.
- The GitHub Actions workflow run.
- The published macOS artifact.
- The checksum recorded in the Homebrew cask.

Maintainers must also be able to identify the Linux support path that remains valid for the same feature scope.

## Validation Contract

Validation for this feature must confirm:

- Repository quality gates pass for the release commit.
- The macOS artifact is generated and attached to the GitHub Release.
- The Homebrew cask metadata references the correct version and checksum.
- The documented Homebrew install and upgrade flow is usable on macOS.
- The documented Linux clone-and-build path remains accurate and reproducible.
