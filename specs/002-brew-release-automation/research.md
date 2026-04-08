# Research: Homebrew Installation and Automated Releases

## Distribution Strategy

Decision: Use a Homebrew cask distributed through a project-owned Homebrew tap as the first automated brew installation path.

Rationale: The product is a desktop application, so a cask is the correct Homebrew distribution model instead of a formula. A project-owned tap keeps release publication and Homebrew metadata under the same automation boundary, which fits the requirement for builds to be automatically built and tagged from GitHub Actions and avoids waiting on a separate review process for every release.

Alternatives considered: A Homebrew formula was rejected because it is intended for CLI tools and source-oriented installs, not packaged desktop apps. Publishing directly to `homebrew-core` was considered better for discoverability, but it was rejected for the initial plan because it introduces an external review dependency and does not guarantee immediate sync with each automated release.

Decision: Treat the Homebrew package as a macOS-only install path while continuing to publish Linux artifacts through GitHub Releases.

Rationale: Homebrew is a natural install path for macOS users, while the constitution still requires Linux parity for official release artifacts. This split satisfies both the requested brew install path and the repository's cross-platform packaging obligations without pretending Linux will use the same install flow.

Alternatives considered: A Linux Homebrew path was considered but rejected as unnecessary scope for the initial release automation feature. macOS-only release automation was rejected because the constitution requires Linux and macOS parity for official user-facing releases.

## Release Automation

Decision: Use GitHub Actions as the authoritative release pipeline, triggered by version tags and responsible for building platform artifacts, creating the GitHub Release, and publishing release assets.

Rationale: The user explicitly requested that builds be automatically built and tagged from GitHub Actions. GitHub Actions is the closest control point to the repository, and GitHub Releases provides a stable public artifact source for users, release notes, and Homebrew metadata.

Alternatives considered: Manual release assembly was rejected because it breaks the requested automation and weakens traceability. External release infrastructure was rejected because it would increase operational complexity without adding user value for this project.

Decision: Use stable, versioned asset naming for every published platform artifact.

Rationale: Release automation, Homebrew metadata, user documentation, and troubleshooting all depend on predictable artifact names. Stable names also reduce the risk of cask update mistakes and make release runs easier to verify.

Alternatives considered: Ad hoc artifact names generated per runner or job were rejected because they make workflow outputs harder to consume and validate.

Decision: Publish the GitHub Release only when the required assets are available or clearly mark the release as failed/incomplete when publication cannot finish successfully.

Rationale: The specification requires that failed release creation or artifact publication must not appear fully available to users. Making release completeness explicit prevents broken Homebrew installs and misleading release pages.

Alternatives considered: Creating the release record before artifact verification and leaving it publicly available during partial failures was rejected because it conflicts with the spec's failure-handling requirement.

## Packaging Choices

Decision: Package a macOS archive format that Homebrew cask can install directly from GitHub Releases and checksum deterministically.

Rationale: Homebrew casks need a stable downloadable asset and checksum. A packaged macOS application archive fits that contract and can be referenced directly from the cask metadata without requiring users to build from source.

Alternatives considered: Source-based Homebrew installs were rejected because the spec requires installation from an official release. Custom installer logic outside standard Homebrew-supported artifact types was rejected because it would add maintenance overhead and reduce trust.

Decision: Preserve Linux packaging as a release artifact published alongside macOS, even though Homebrew installation targets macOS.

Rationale: Linux and macOS parity in official releases is a constitution requirement. Publishing Linux assets from the same automation pipeline keeps supported desktop targets aligned even when only macOS gets a Homebrew path.

Alternatives considered: Shipping only macOS artifacts for the automated release flow was rejected because it would create release drift between supported platforms.

## Security and Trust Assumptions

Decision: Document macOS signing and notarization as an explicit packaging assumption rather than making it a hard prerequisite for the first automated release workflow.

Rationale: Open-source desktop apps are commonly distributed without full Apple notarization automation, and the repository does not currently show an existing signing setup. The release design should make the assumption visible so users and maintainers understand the trust posture, while still allowing the initial automated release path to land.

Alternatives considered: Requiring signing and notarization immediately was rejected because it would add secrets, certificates, and Apple account dependencies that were not part of the original feature request. Ignoring signing expectations entirely was rejected because the constitution requires packaging assumptions to be documented for release-affecting work.

Decision: Update Homebrew metadata only after the release asset URL and checksum are known from the successful release output.

Rationale: This guarantees that the Homebrew install path points to a real, published release asset and prevents mismatches between the cask version, checksum, and downloadable file.

Alternatives considered: Updating the cask optimistically before release publication completes was rejected because it could expose broken installs. Manual checksum updates were rejected because they undermine the requested automation.

## Validation Strategy

Decision: Keep the existing repository quality commands as the baseline and add release-specific smoke validation around tagged builds, published assets, and Homebrew install or upgrade rehearsal.

Rationale: The repository already has a clear Go, frontend, and Wails validation path. Extending that path with release smoke checks keeps the feature aligned with current engineering standards without introducing a separate verification culture.

Alternatives considered: A release pipeline that skips repository tests and only checks workflow success was rejected because it weakens confidence in shipped binaries. Heavy end-to-end release test infrastructure was rejected as excessive for the current project scope.
