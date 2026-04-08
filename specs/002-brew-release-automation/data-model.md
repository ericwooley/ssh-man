# Data Model: Homebrew Installation and Automated Releases

## Overview

This feature does not change the runtime data model of the desktop application. It adds release-distribution entities that connect repository version tags, workflow executions, published macOS release assets, Linux support guidance, and Homebrew cask metadata.

## Entities

### ReleaseVersion

Represents one intended official application release.

**Fields**
- `version`: Canonical semantic version string in the format `1.2.3`.
- `tag`: Repository tag string, equal to `version`.
- `status`: `pending`, `building`, `published`, or `failed`.
- `release_notes_ref`: Optional reference to release summary content.
- `created_at`: Timestamp when release processing began.
- `published_at`: Timestamp when the GitHub Release became user-visible.

**Validation Rules**
- `version` and `tag` must be identical and follow the plain semantic version format.
- `published_at` is required when `status=published`.
- A duplicate `version` or `tag` is invalid for a new official release.

**Relationships**
- One `ReleaseVersion` maps to one authoritative `ReleaseWorkflowRun`.
- One `ReleaseVersion` may have one or more related `ReleaseArtifact` records.

### ReleaseWorkflowRun

Represents one automated execution of the official release workflow.

**Fields**
- `run_id`: Unique workflow execution identifier.
- `trigger_ref`: Tag or dispatch reference that started the run.
- `status`: `queued`, `running`, `succeeded`, or `failed`.
- `started_at`: Run start timestamp.
- `completed_at`: Run completion timestamp.
- `failure_summary`: Optional maintainer-facing summary when the run fails.

**Validation Rules**
- `trigger_ref` must map to exactly one intended `ReleaseVersion`.
- `completed_at` is required for terminal states.
- `failure_summary` should be present when `status=failed`.

**Relationships**
- One `ReleaseWorkflowRun` belongs to one `ReleaseVersion`.
- One `ReleaseWorkflowRun` produces zero or more `ReleaseArtifact` records.

### ReleaseArtifact

Represents a downloadable asset published from a release workflow run.

**Fields**
- `name`: Published asset filename.
- `platform`: `macos`.
- `architecture`: Target CPU architecture for the asset.
- `package_format`: Packaged archive or bundle format used for release distribution.
- `checksum`: Published checksum used for artifact verification.
- `download_url`: User-visible URL to the artifact.
- `published`: Boolean indicating whether the asset is attached to the GitHub Release.

**Validation Rules**
- `platform` is limited to `macos` for official automated artifacts in this feature.
- `checksum` and `download_url` are required when `published=true`.
- `name` should remain stable enough for release automation and Homebrew cask updates.

**Relationships**
- Many `ReleaseArtifact` records can belong to one `ReleaseVersion`.
- One published macOS `ReleaseArtifact` is referenced by one `HomebrewCaskDefinition` version update.

### HomebrewCaskDefinition

Represents the cask metadata published through the project-owned tap for one release.

**Fields**
- `cask_name`: Canonical Homebrew cask identifier.
- `tap_repository`: Repository location of the project-owned tap.
- `version`: Version string served to Homebrew users.
- `asset_url`: URL of the macOS release artifact used by the cask.
- `sha256`: Checksum corresponding to `asset_url`.
- `status`: `pending`, `updated`, `verified`, or `failed`.
- `updated_at`: Timestamp of the latest cask metadata change.

**Validation Rules**
- `version` must match the associated `ReleaseVersion.version`.
- `asset_url` and `sha256` must match the published macOS `ReleaseArtifact`.
- `tap_repository` must point to the project-owned tap, not `homebrew-core`.

**Relationships**
- One `HomebrewCaskDefinition` version update maps to one published `ReleaseVersion`.
- One cask update references one published macOS `ReleaseArtifact`.

### LinuxSupportPath

Represents the documented Linux support path maintained alongside release automation.

**Fields**
- `support_mode`: `clone_and_build`.
- `documentation_ref`: Reference to the user-facing Linux guidance.
- `validated_by`: Validation command or smoke step that confirms Linux support remains usable.
- `notes`: Optional explanatory text about why Linux does not share the macOS automated release cycle in this feature.

**Validation Rules**
- `support_mode` must remain `clone_and_build` for this feature.
- `documentation_ref` must identify where Linux users are directed.
- `validated_by` must reference an actual repository validation command or documented smoke step.

**Relationships**
- One `LinuxSupportPath` applies to the feature as a whole rather than to a single published artifact.

## State Transitions

### ReleaseVersion Status Flow

- `pending -> building`: Maintainer initiates an official tagged release.
- `building -> published`: Workflow completes successfully and publishes the release with assets.
- `building -> failed`: Workflow, tagging, or asset publication fails.
- `failed -> building`: Maintainer retries with a corrected release attempt.

### HomebrewCaskDefinition Status Flow

- `pending -> updated`: Cask metadata is changed to reference the successful macOS release artifact.
- `updated -> verified`: Install or checksum validation confirms the cask matches the published release.
- `updated -> failed`: Metadata update or validation fails.
- `failed -> updated`: Maintainer retries the cask update after correction.

## Traceability Rules

- Every published `ReleaseVersion` must map to one successful `ReleaseWorkflowRun`.
- Every Homebrew-served version must map to one published macOS `ReleaseArtifact` with matching URL and checksum.
- Failed workflow runs must remain distinguishable from published releases.
- Linux support documentation must remain traceable to a real clone-and-build validation path even when no official Linux artifact is published.
