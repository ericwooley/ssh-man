# Research: Homebrew Installation and Automated Releases

## Distribution Decisions

Decision: Use a Homebrew cask as the official Homebrew installation model for the app.

Rationale: The feature distributes a packaged macOS desktop application, which maps cleanly to a cask and matches the user-facing install and upgrade flow expected from Homebrew for GUI apps.

Alternatives considered: A Homebrew formula was rejected because it better fits CLI or source-build-oriented tooling. Supporting either model would weaken automation and documentation clarity.

Decision: Publish the official cask through a project-owned Homebrew tap.

Rationale: A project-owned tap keeps version, URL, and checksum changes under repository-controlled automation and avoids external review latency that would break the requested automatic sync between releases and Homebrew availability.

Alternatives considered: `homebrew-core` offers broader discoverability but adds review and merge delays that conflict with the requested release-to-install synchronization. Supporting both immediately adds unnecessary scope.

Decision: Use GitHub Actions and GitHub Releases as the authoritative official release pipeline.

Rationale: The user explicitly requested builds and tags from GitHub Actions, and GitHub Releases provide a versioned, traceable source for release assets that Homebrew metadata can reference directly.

Alternatives considered: Local maintainer packaging or another CI platform would increase manual steps or conflict with the requested workflow authority.

## Versioning And Artifact Decisions

Decision: Use plain semantic version tags in the format `1.2.3` for official releases.

Rationale: The spec explicitly clarified this format, and a plain semantic version is easy to validate, easy to map into release metadata, and compatible with Homebrew version fields.

Alternatives considered: `v1.2.3` and other custom prefixes were rejected because the spec now defines the unprefixed semantic tag as the canonical release identifier.

Decision: Publish official automated release artifacts for macOS only in this feature.

Rationale: The Homebrew path is explicitly scoped to macOS, and narrowing artifact publication keeps the release automation small enough to deliver now while still satisfying the primary user-facing install goal.

Alternatives considered: Publishing Linux and macOS artifacts together would improve release-channel parity but exceeds the clarified feature scope. Publishing macOS, Linux, and Windows would further expand the feature without user need.

Decision: Keep Linux explicitly supported through documented clone-and-build workflows rather than matching the macOS automated release cycle.

Rationale: The product remains supported on Linux under the constitution, and the repository already has Linux build validation scripts. Treating Linux as a source-build-supported platform preserves product support and validation without forcing the release cycle to match macOS packaging work immediately.

Alternatives considered: Declaring Linux out of scope entirely would conflict with platform support expectations. Requiring Linux to ship official automated artifacts now would increase release automation scope beyond the clarified feature intent.

## Packaging Decisions

Decision: Treat macOS signing and notarization as not required for this feature, but document unsigned-app expectations in installation guidance.

Rationale: The spec explicitly clarified that signing is not required. Making the unsigned state visible in documentation reduces user confusion without expanding the feature into Apple credential and notarization automation.

Alternatives considered: Requiring signing or notarization now would add secrets management, Apple account coordination, and more failure modes. Leaving the unsigned state undocumented would create avoidable install friction.

Decision: Update Homebrew cask metadata only after the macOS asset URL and checksum are known from a successful release.

Rationale: This guarantees that Homebrew installation points only to complete, published artifacts and prevents stale or broken cask references when a release run fails partway through.

Alternatives considered: Precomputing metadata before release publication risks checksum mismatches or dead URLs. Manual cask edits after release would conflict with the automation goal.

## Validation Decisions

Decision: Keep the existing repository validation commands as the baseline quality gates and add release-specific smoke validation on top.

Rationale: The repository already has Go, frontend, and Linux build validation commands. Extending those checks with release workflow verification and Homebrew rehearsal provides confidence without inventing a separate validation stack.

Alternatives considered: A wholly separate release-only validation process would duplicate existing checks and increase maintenance cost.
