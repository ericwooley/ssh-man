# Data Model: Homebrew Installation and Automated Releases

## Overview

This feature does not change the runtime data model of the desktop application. Instead, it introduces release-distribution entities that connect repository versions, automated workflow runs, published assets, and Homebrew metadata into a traceable release path.

## Entities

### ReleaseVersion

Represents a single user-visible application version intended for official distribution.

**Fields**
- `version`: Semantic version string used by users and release documentation.
- `tag`: Repository tag associated with the release version.
- `status`: Current distribution state such as `pending`, `building`, `published`, or `failed`.
- `release_notes_ref`: Link or reference to the generated release notes.
- `created_at`: Timestamp when release processing began.
- `published_at`: Timestamp when the release became publicly available.

**Validation Rules**
- `version` and `tag` must map to the same release.
- `tag` must be unique across official releases.
- `published_at` must exist only for successfully published releases.

**Relationships**
- One `ReleaseVersion` has one or more `ReleaseArtifact` records.
- One `ReleaseVersion` is produced by one authoritative `ReleaseWorkflowRun`.
- One `ReleaseVersion` may be referenced by one `HomebrewCaskVersion` update.

### ReleaseWorkflowRun

Represents one execution of the automated release pipeline.

**Fields**
- `run_id`: Unique workflow execution identifier.
- `trigger_ref`: Tag or manual trigger reference that started the run.
- `status`: `queued`, `running`, `succeeded`, `failed`, or `cancelled`.
- `started_at`: Execution start timestamp.
- `completed_at`: Execution completion timestamp.
- `failure_summary`: Human-readable failure detail when the run does not succeed.

**Validation Rules**
- `run_id` must be unique.
- `completed_at` must be present for terminal states.
- `failure_summary` must be present for failed runs that require maintainer action.

**Relationships**
- One `ReleaseWorkflowRun` produces at most one `ReleaseVersion`.
- One `ReleaseWorkflowRun` may produce multiple `ReleaseArtifact` records.

### ReleaseArtifact

Represents a downloadable platform-specific asset attached to an official release.

**Fields**
- `name`: Published artifact filename.
- `platform`: Distribution target such as `macos` or `linux`.
- `architecture`: Supported CPU target such as `amd64`, `arm64`, or `universal`.
- `package_format`: User-visible package type such as app archive or compressed binary archive.
- `checksum`: Published verification digest.
- `download_url`: Public release asset URL.
- `published`: Whether the artifact is attached to the user-visible release.

**Validation Rules**
- `name`, `platform`, `package_format`, `checksum`, and `download_url` are required for published artifacts.
- Artifact names must be unique within a release.
- The macOS Homebrew-targeted artifact must remain stable enough to reference from cask metadata.

**Relationships**
- Many `ReleaseArtifact` records belong to one `ReleaseVersion`.

### HomebrewCaskVersion

Represents the Homebrew metadata revision that points brew installs to a specific release.

**Fields**
- `cask_name`: Published Homebrew cask identifier.
- `tap_repository`: Repository that owns the cask metadata.
- `version`: Version exposed to Homebrew users.
- `asset_url`: Release asset URL used by the cask.
- `sha256`: Checksum published in the cask metadata.
- `status`: `pending`, `updated`, `verified`, or `failed`.
- `updated_at`: Timestamp of the latest metadata update.

**Validation Rules**
- `version`, `asset_url`, and `sha256` must match the referenced published release artifact.
- `cask_name` and `tap_repository` must remain stable across updates unless intentionally migrated.
- `verified` may only be set after the cask references a reachable published asset.

**Relationships**
- One `HomebrewCaskVersion` references one `ReleaseVersion` and one macOS `ReleaseArtifact`.

## State Transitions

### ReleaseVersion Status Flow

- `pending -> building`: A maintainer triggers an official release.
- `building -> published`: All required artifacts are generated, attached, and the release is made available.
- `building -> failed`: Workflow execution, tagging, or artifact publication fails.
- `failed -> building`: A maintainer retries the release with a corrected workflow or version.

### HomebrewCaskVersion Status Flow

- `pending -> updated`: The cask metadata is updated with the published version, asset URL, and checksum.
- `updated -> verified`: The cask references a reachable release asset and passes release validation.
- `updated -> failed`: The metadata update completes but points to an invalid or mismatched artifact.
- `failed -> updated`: Maintainers correct the cask metadata and rerun validation.

## Traceability Rules

- Every published `ReleaseVersion` must map to exactly one authoritative `ReleaseWorkflowRun`.
- Every Homebrew-facing version must map to a published macOS `ReleaseArtifact` with a matching checksum.
- Failed workflow runs must remain distinguishable from published releases so users cannot confuse incomplete automation with an official release.
