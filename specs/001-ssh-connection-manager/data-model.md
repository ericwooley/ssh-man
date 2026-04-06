# Data Model: SSH Connection Manager MVP

## Overview

The MVP separates durable app-managed data stored in SQLite from live runtime session state held in memory. Durable entities represent saved servers, saved connection configurations, preferences, and lightweight session history. Runtime session objects reflect currently active or recently failed tunnel activity and are not restored as live sessions after restart.

## Entities

### Server

Represents a saved SSH destination.

**Fields**
- `id`: Stable unique identifier.
- `name`: User-facing display name, unique within the local app data set.
- `host`: Target hostname or IP address.
- `port`: SSH port.
- `username`: Default username for the server.
- `auth_mode`: Preferred authentication mode for the server.
- `key_reference`: Optional pointer to a user-selected SSH key path or credential source.
- `created_at`: Creation timestamp.
- `updated_at`: Last modification timestamp.

**Validation Rules**
- `name`, `host`, and `username` are required.
- `port` must be a valid TCP port.
- `name` must remain distinct enough for users to identify the server in the list.

**Relationships**
- One `Server` has many `ConnectionConfiguration` records.

### ConnectionConfiguration

Represents a saved tunnel definition under a server.

**Fields**
- `id`: Stable unique identifier.
- `server_id`: Parent server identifier.
- `label`: User-facing configuration name.
- `connection_type`: `local_forward` or `socks_proxy`.
- `local_port`: Local listening port.
- `remote_host`: Remote destination host for local forwards only.
- `remote_port`: Remote destination port for local forwards only.
- `socks_port`: Local SOCKS listening port for SOCKS configurations.
- `auto_reconnect_enabled`: Whether reconnect should continue after transient disconnect.
- `start_on_launch`: Reserved MVP boolean, default false; not acted on in MVP.
- `notes`: Optional user-facing description.
- `created_at`: Creation timestamp.
- `updated_at`: Last modification timestamp.

**Validation Rules**
- `label` and `connection_type` are required.
- `connection_type=local_forward` requires `local_port`, `remote_host`, and `remote_port`.
- `connection_type=socks_proxy` requires `socks_port`.
- All port values must be valid TCP ports.
- A configuration cannot be saved if its bind settings conflict with another saved configuration in a way the product defines as invalid for MVP.

**Relationships**
- Many `ConnectionConfiguration` records belong to one `Server`.
- One `ConnectionConfiguration` may produce many `SessionHistoryEntry` records over time.

### UserPreference

Represents persisted user-facing settings.

**Fields**
- `theme`: `light` or `dark`.
- `last_selected_server_id`: Optional reference for restoring list focus.
- `updated_at`: Last modification timestamp.

**Validation Rules**
- `theme` must be one of the supported values.

**Relationships**
- Standalone per local profile.

### SessionHistoryEntry

Represents a lightweight persisted summary of a completed or interrupted connection attempt.

**Fields**
- `id`: Stable unique identifier.
- `configuration_id`: Associated saved configuration.
- `started_at`: When the attempt began.
- `ended_at`: When the attempt finished or was abandoned.
- `outcome`: `connected`, `stopped`, `reconnect_exhausted`, `failed_validation`, `failed_auth`, or `failed_runtime`.
- `message`: User-visible summary of the most important outcome detail.

**Validation Rules**
- `started_at` is required.
- `outcome` must use a known value.
- `message` should be concise and user-readable.

**Relationships**
- Many `SessionHistoryEntry` records belong to one `ConnectionConfiguration`.

### BrowserOption

Represents an installed browser discovered from the operating system at runtime.

**Fields**
- `id`: Runtime identifier stable for the current app session.
- `display_name`: User-facing browser name.
- `launch_reference`: OS-specific launch target.
- `supports_proxy_launch`: Whether the current adapter can attempt proxy-aware launch.

**Validation Rules**
- Not persisted in SQLite for MVP.
- Must only be presented if the browser is currently discoverable.

**Relationships**
- Referenced transiently by browser launch flows.

### RuntimeSession

Represents the live in-memory state of an active or recently interrupted tunnel session.

**Fields**
- `configuration_id`: Associated saved configuration.
- `status`: `stopped`, `starting`, `connected`, `reconnecting`, `needs_attention`, or `failed`.
- `status_detail`: User-facing detail string.
- `started_at`: Current session start timestamp.
- `last_state_change_at`: Last state transition timestamp.
- `reconnect_attempt_count`: Current reconnect attempt count.
- `last_error`: Most recent actionable runtime error.

**Validation Rules**
- Exists only while the app is running.
- `needs_attention` is used when the session requires user action, such as a passphrase prompt or unrecoverable credential state.

**Relationships**
- One active `RuntimeSession` maps to at most one `ConnectionConfiguration`.

## State Transitions

### RuntimeSession Status Flow

- `stopped -> starting`: User initiates a start action.
- `starting -> connected`: Authentication succeeds and all requested binds succeed.
- `starting -> needs_attention`: Manual credentials or passphrase input are required before the session can continue.
- `starting -> failed`: Validation, bind, host-key, or startup errors block a successful session.
- `connected -> reconnecting`: A transient disconnect is detected for a reconnect-enabled session.
- `reconnecting -> connected`: Recovery succeeds.
- `reconnecting -> needs_attention`: Recovery requires new user input, such as unlocking a key.
- `reconnecting -> failed`: Recovery reaches a terminal failure condition.
- `connected -> stopped`: User intentionally stops the session.
- `needs_attention -> starting`: User retries after providing the required input.
- `failed -> starting`: User retries after correcting the issue.

## Persistence Boundaries

- Persist in SQLite: `Server`, `ConnectionConfiguration`, `UserPreference`, `SessionHistoryEntry`.
- Discover on demand: `BrowserOption`.
- Keep in memory only: `RuntimeSession`, reconnect timers, active bind ownership, and any temporary credential or prompt state.
