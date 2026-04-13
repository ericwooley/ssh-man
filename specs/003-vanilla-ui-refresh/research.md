# Research: Vanilla Framework UI Refresh

## Summary

This document captures decisions, rationale, and alternatives considered for migrating the application's visual system to a Vanilla-inspired design language while remaining compliant with the repository constitution (Svelte-only frontend and single shared CSS file at `frontend/src/app.css`).

## Decisions

- Decision: Use Vanilla Framework as visual inspiration and adapt its tokens and component patterns into the single shared CSS file rather than adding the framework as a runtime dependency.
  - Rationale: The constitution forbids introducing runtime-generated styling or external component frameworks that change the app runtime. Adapting the design vocabulary into the existing single CSS file preserves the styling contract while leveraging Vanilla's patterns.
  - Alternatives considered:
    - Import Vanilla Framework CSS as an external dependency: rejected because it would introduce third-party runtime CSS and potentially conflict with single-file CSS constraint.
    - Implement per-component CSS or CSS modules to map Vanilla classes: rejected due to constitution constraints and preference for single-file style consistency.

- Decision: Map existing high-level CSS tokens (colors, spacing, elevation) to Vanilla-like tokens inside `frontend/src/app.css` and create a clear mapping table in the design notes so Svelte components use stable class names.
  - Rationale: This keeps class names stable across components and avoids changing Svelte markup broadly. It also provides a single place to adjust theming and tokens for dark/light modes.
  - Alternatives considered:
    - Replace class names across components to Vanilla's canonical names: viable but higher-risk and less incremental. Chosen approach is incremental and reversible.

## Research Tasks

- Task: Create a token mapping table from Vanilla component tokens to our project's CSS variables (colors, spacing, radii, elevation).  
- Task: Inventory all frontend components and identify which selectors in `frontend/src/app.css` currently control their appearance.  
- Task: Create migration guide for incremental CSS refactor: (1) add tokens and utility classes to app.css, (2) adapt component markup to use the new tokens where safe, (3) remove legacy rules behind feature flags or in branches.

## Implementation Notes

- Keep interactions and markup unchanged where possible; prefer adding utility classes and token variables to `frontend/src/app.css` for parity.
- Ensure accessibility: do not rely on color alone; include iconography or text for critical status states.
- Validate at each step by running `pnpm --dir frontend run validate` and manual QA on macOS/Linux builds.

## Outcome

This research resolves all constitution-related clarifications by choosing an approach that respects Svelte-only and single-file CSS constraints, while allowing Vanilla-inspired visual language to be adopted incrementally.
