# Migration Guide

## Incremental path

1. Add semantic tokens, spacing utilities, and state helpers to `frontend/src/app.css` without removing existing aliases.
2. Update workspace and list surfaces to consume the new token layer while preserving current class names.
3. Update dialogs and forms to use shared structural classes such as `dialog`, `dialog-body`, `dialog-actions`, and `field-error`.
4. Update runtime and status surfaces to emit semantic status classes with text labels that do not rely on color alone.
5. Remove obsolete selectors only after automated validation and manual QA confirm parity.

## Guardrails

- Keep all styling in `frontend/src/app.css`.
- Prefer additive class changes in Svelte markup over structural rewrites.
- Preserve existing behavior, focus order, and Wails integration.
- Re-run `pnpm --dir frontend run validate` after each story-level slice.
