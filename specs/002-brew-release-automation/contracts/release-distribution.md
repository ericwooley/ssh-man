# Contract: Release Distribution and Homebrew Delivery

## Purpose

This contract defines the external release boundary for the project: how maintainers create official releases, what assets are published for users, and what information the Homebrew distribution path must consume.

## Design Principles

- GitHub Actions is the authoritative automation boundary for release execution.
- Git tags, GitHub Releases, release assets, and Homebrew metadata must reference the same version.
- Failed or partial automation must be visible to maintainers and must not appear as a fully available release to users.

## Release Trigger Contract

### Official Release Input

The automation must accept a maintainer-controlled release trigger based on a version tag.

**Required fields**
- `version_tag`: Repository tag identifying the release version.
- `release_channel`: Defaults to the project's official public release channel.

**Behavior**
- Starts the release workflow.
- Associates the workflow run with the requested version tag.
- Rejects duplicate or malformed release tags.

## Release Output Contract

### Published Release

Each successful official release must produce:

- One user-visible GitHub Release entry.
- One release tag matching the published version.
- One or more Linux release artifacts.
- One macOS release artifact suitable for Homebrew installation.
- User-visible release notes or generated release summary.

### Artifact Metadata

Every published artifact must expose:

```text
name
version
platform
architecture
packageFormat
checksum
downloadUrl
```

### Failure Behavior

If the release workflow does not complete successfully, the automation must:

- preserve failure details for maintainers
- prevent the failed release from being treated as fully published
- avoid updating Homebrew metadata to the incomplete version

## Homebrew Contract

### Distribution Model

The Homebrew install path is a cask published through a project-owned tap.

### Required Cask Metadata

The cask definition must include:

```text
caskName
version
sha256
url
homepage
appBundleReference
```

### Update Rules

- The cask version must match the GitHub Release version.
- The cask download URL must point to the published macOS release artifact.
- The cask checksum must match the published macOS release artifact checksum.
- The cask update must occur only after the release artifact is published successfully.

### User-Facing Install Contract

The documented brew flow must let users:

- install the latest official macOS release
- upgrade to a newer official release
- identify when their environment is outside the supported Homebrew scope

## Traceability Contract

Maintainers must be able to trace a Homebrew-installable version back to:

- the release tag
- the GitHub Actions workflow run that produced it
- the published macOS artifact
- the checksum recorded in the Homebrew cask

## Validation Contract

Before a release flow is considered acceptable for this feature, maintainers must be able to verify:

- repository quality gates passed for the release commit
- Linux and macOS artifacts were generated
- the GitHub Release contains the expected assets
- the Homebrew metadata references the intended version and checksum
- users can follow the documented brew install or upgrade path against a published release
