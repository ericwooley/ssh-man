# Component Inventory

## Shared stylesheet target

- `/Users/ericwooley/projects/ssh-man/frontend/src/app.css`

## User-facing components

| Component | Purpose | Primary shared classes |
|---|---|---|
| `frontend/src/routes/App.svelte` | Main workspace shell and dialogs | `app-shell`, `hero`, `banner`, `dashboard-shell`, `workspace-main`, `workspace-main-grid`, `dialog-backdrop`, `dialog-card` |
| `frontend/src/components/ServerList.svelte` | Saved server list and row menu | `panel`, `panel-header`, `stack-list`, `list-item-shell`, `list-card-topline`, `list-card-main`, `list-card-tools`, `row-menu`, `pill` |
| `frontend/src/components/ConfigList.svelte` | Tunnel list and actions | `panel`, `panel-header`, `panel-actions`, `stack-list`, `list-item-shell`, `status-pill`, `row-menu` |
| `frontend/src/components/ActiveConnections.svelte` | Active runtime summary list | `panel`, `panel-header`, `stack-list`, `list-item-shell`, `status-pill`, `row-menu` |
| `frontend/src/components/ConfigEditor.svelte` | Tunnel editor form | `editor-card`, `editor-header`, `form-section`, `field-grid`, `checkbox-card`, `editor-actions`, `error-text` |
| `frontend/src/components/ServerEditorDialog.svelte` | Server editor dialog | `dialog-backdrop`, `dialog-card`, `editor-card`, `modal-editor-card`, `form-section`, `field-grid`, `editor-actions` |
| `frontend/src/components/SessionStatus.svelte` | Runtime controls and history | `panel`, `panel-header`, `status-pill`, `runtime-summary`, `status-callout`, `runtime-actions-grid`, `history-panel` |
| `frontend/src/components/BrowserLauncher.svelte` | SOCKS browser launch surface | `panel`, `panel-header`, `compact-form-grid`, `browser-command-block`, `banner-detail` |
| `frontend/src/components/DiagnosticsPanel.svelte` | Recovery and storage details | `panel`, `diagnostics-panel`, `diagnostics-block`, `diagnostics-issue`, `button-row` |
| `frontend/src/components/UnlockKeyDialog.svelte` | Key passphrase dialog | `dialog-backdrop`, `dialog-card`, `dialog-form`, `dialog-header`, `editor-actions` |
| `frontend/src/components/ThemeToggle.svelte` | Theme switcher | `theme-toggle` |

## Existing tests tied to refreshed surfaces

- `frontend/src/routes/App.test.js`
- `frontend/src/components/ServerList.test.js`
- `frontend/src/components/ConfigList.test.js`
- `frontend/src/components/ActiveConnections.test.js`
- `frontend/src/components/ConfigEditor.test.js`
- `frontend/src/components/ServerEditorDialog.test.js`
- `frontend/src/components/SessionStatus.test.js`
- `frontend/src/components/BrowserLauncher.test.js`
- `frontend/src/components/DiagnosticsPanel.test.js`
- `frontend/src/components/UnlockKeyDialog.test.js`
- `frontend/src/components/ThemeToggle.test.js`

## Refactor risks

- `panel`, `button`, `panel-header`, `empty-state`, `list-item-shell`, `status-pill`, and `dialog-*` are shared widely and must stay stable.
- Row-menu layering depends on `.menu-open` and `:has(...)` selectors in the shared stylesheet.
- The migration should preserve `.selected` behavior while introducing semantic aliases like `.is-selected`.
