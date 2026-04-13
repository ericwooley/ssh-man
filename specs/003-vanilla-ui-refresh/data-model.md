# Data Model: Vanilla Framework UI Refresh

This feature is a visual-system migration and does not introduce new persistent data models or domain entities. The data model for the application remains unchanged.

## Entities (No structural changes)

- Server: existing entity representing an SSH server and its metadata.
- Configuration/Tunnel: existing entity representing a tunnel or proxy configuration.
- Session/RuntimeSession: existing runtime view of active or historical sessions.

## Validation Rules (UI-driven)

- Field presence and format validations are unchanged; the visual refresh may relabel or reorder fields for clarity but must preserve the same validation rules declared in the frontend.

## State Transitions

- No new domain-state transitions are introduced by the visual refresh.
