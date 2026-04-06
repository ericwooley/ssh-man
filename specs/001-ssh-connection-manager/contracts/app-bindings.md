# Contract: App Bindings for SSH Connection Manager MVP

## Purpose

This contract defines the MVP boundary between the Wails backend bindings and the plain Svelte frontend. It documents the user-facing operations and payload shapes the UI needs, without prescribing internal package structure.

## Design Principles

- Bindings stay thin and delegate validation, persistence, SSH orchestration, and platform integration to internal Go packages.
- All payloads must be serializable across the Wails boundary and stable enough for frontend state management.
- Errors returned to the frontend must include actionable user-facing messages.

## Read Models

### ServerSummary

```text
id
name
host
username
configurationCount
```

### ConnectionConfigurationSummary

```text
id
serverId
label
connectionType
localPort
remoteHost
remotePort
socksPort
autoReconnectEnabled
runtimeStatus
runtimeStatusDetail
```

### BrowserOption

```text
id
displayName
supportsProxyLaunch
```

### UserPreference

```text
theme
lastSelectedServerId
```

## Commands

### LoadInitialState

**Purpose**: Load the saved server list, saved configurations, current runtime statuses, and persisted preferences needed for the main application view.

**Returns**
- Saved servers with nested configuration summaries.
- Current preferences.
- Current in-memory runtime session statuses.

### SaveServer

**Purpose**: Create or update a saved server.

**Input**
- Optional `id` for update.
- `name`, `host`, `port`, `username`, and authentication-related fields.

**Behavior**
- Validates required fields.
- Persists the server in SQLite.
- Returns the saved server summary.

### DeleteServer

**Purpose**: Delete a saved server and its child configurations after explicit confirmation in the UI.

**Input**
- `serverId`

**Behavior**
- Fails clearly if deletion is blocked by currently running child sessions.

### SaveConnectionConfiguration

**Purpose**: Create or update a saved local-forward or SOCKS configuration under a server.

**Input**
- Optional `id` for update.
- `serverId`, `label`, `connectionType`, bind settings, reconnect preference, and optional notes.

**Behavior**
- Validates required fields and conflicting bind settings.
- Persists the configuration in SQLite.
- Returns the saved configuration summary.

### DeleteConnectionConfiguration

**Purpose**: Delete a saved configuration.

**Input**
- `configurationId`

**Behavior**
- Fails clearly if the configuration is currently running.

### StartConfiguration

**Purpose**: Start a saved tunnel configuration.

**Input**
- `configurationId`

**Behavior**
- Loads the saved configuration.
- Starts the SSH-backed tunnel session.
- Returns the immediate runtime state (`starting`, `connected`, or `needs_attention`).

### StopConfiguration

**Purpose**: Stop a running tunnel configuration.

**Input**
- `configurationId`

**Behavior**
- Stops reconnect attempts and active binds.
- Returns the resulting stopped runtime state.

### RetryConfiguration

**Purpose**: Retry a configuration that is in `failed` or `needs_attention` state after the user has corrected the issue.

**Input**
- `configurationId`

### SubmitKeyUnlock

**Purpose**: Continue a manual start or retry flow that needs an encrypted-key passphrase.

**Input**
- `configurationId`
- user-provided unlock secret

**Behavior**
- Uses the secret only for the current flow.
- Does not persist the secret in SQLite.

### DiscoverBrowsers

**Purpose**: Discover currently installed supported browsers.

**Input**
- Optional refresh hint from the UI.

**Returns**
- Current runtime list of supported browsers.

### LaunchBrowserThroughSocks

**Purpose**: Launch a selected installed browser through a running SOCKS configuration.

**Input**
- `configurationId`
- `browserId`

**Behavior**
- Verifies the configuration is a running SOCKS session.
- Verifies the browser is still available.
- Attempts launch through the proxy path supported for that browser and platform.
- Returns success or a user-facing failure reason.

### SavePreferences

**Purpose**: Persist user presentation preferences such as theme and list selection.

**Input**
- `theme`
- optional `lastSelectedServerId`

## Events / Polling Contract

For the MVP, runtime session state may be delivered either through Wails events or short-interval frontend polling, but the contract exposed to the UI must include:

- configuration identifier
- current runtime status
- current status detail
- reconnect attempt count when reconnecting
- whether the session needs user input

The implementation choice may be finalized during tasks, but the frontend contract must remain stable regardless of transport.

## Error Contract

Each command must return or surface:

- a stable operation outcome (`success` or `error`)
- a user-facing message suitable for inline or toast display
- optional field-level validation errors for server/configuration forms
- optional recovery hint when the user can retry, unlock a key, free a port, or pick another browser
