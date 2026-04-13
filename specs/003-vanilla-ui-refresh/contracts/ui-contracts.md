# UI Contracts

This folder documents the contract between the shared stylesheet and Svelte components for the refreshed visual system. It focuses on the class names, tokens, and markup expectations that Svelte components must respect.

## Shared Stylesheet Path

- `/Users/ericwooley/projects/ssh-man/frontend/src/app.css`

## Contract: Class & Token Usage

- Components must avoid adding new component-scoped styles. They should apply classes that correspond to tokens defined in `app.css`.
- Tokens to be provided (examples): `--bg`, `--bg-panel`, `--accent`, `--accent-strong`, `--border`, `--shadow`, spacing utilities (e.g., `.gap-sm`, `.gap-md`), and semantic state classes (e.g., `.is-selected`, `.is-disabled`, `.state-warning`, `.state-error`).

## Markup Expectations

- Form controls retain current markup (inputs, labels, buttons). The refresh may add recommended utility classes but must not rely on injected wrappers.
- Dialogs: `.dialog` root element with `.dialog-header`, `.dialog-body`, `.dialog-actions` sections expected.
- List items: `.list-item-shell` as primary container; selection uses `.selected` modifier class.

## Verification

- Unit and integration visual checks should assert that components render with the expected classes and tokens by default. End-to-end visual QA will validate appearance across platforms.
