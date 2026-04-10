# Accessibility Report

## UI refresh checks

- Maintained text labels for runtime and validation states so meaning is not color-only.
- Preserved `aria-invalid`, dialog roles, menu roles, and `aria-live` usage already present in the components.
- Added field-level error styling hooks and semantic status classes to reinforce meaning.
- Browser-based macOS and Linux Chromium QA confirmed readable focus states, menu visibility, and main-workspace contrast after the refresh.

## Remaining manual checks

- Native Linux Wails shell parity, including window-chrome specific rendering.
- Manual screen-reader verification for dialog announcements and runtime state changes.
