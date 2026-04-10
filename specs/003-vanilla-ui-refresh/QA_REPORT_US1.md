# SSH Man Frontend - User Story 1 Manual QA Report
## Final Verification Summary (2026-04-10)

## Executive Summary

**Manual QA for User Story 1 is complete with 5 of 5 checklist items passing.**

The refreshed main workspace was verified in two browser-backed environments:
- macOS Chromium against the local dev server with the in-memory mock API
- Linux Chromium in Docker against the built frontend served from `frontend/dist`

## Checklist Results

1. Summary cards remain readable in dark and light themes: **PASS**
   - Evidence: `02-light-theme-workspace.png`, `03-dark-theme-workspace.png`, `16-linux-populated-workspace.png`

2. Server, tunnel, and active-session lists show clear hover, focus, and selected states: **PASS**
   - Evidence: `04-list-focus-and-selection.png`, `05-list-selected-state.png`, `06-keyboard-focus-visible.png`, `12-tunnel-list.png`

3. Row menus appear above neighboring cards without clipping: **PASS**
   - Evidence: `07-row-menu-layering.png`, `18-linux-server-menu.png`

4. Main workspace columns collapse cleanly on narrow windows: **PASS**
   - Evidence: `08-narrow-layout.png`, `09-mobile-layout.png`, `19-linux-narrow-layout.png`

5. Empty states remain readable and actionable: **PASS**
   - Evidence: `01-empty-state.png`

## Populated QA Data

- macOS server: `Demo Bastion`
- macOS tunnels: `App SOCKS`, `Admin UI`
- Linux spot-check server: `Linux Demo Server`
- Linux spot-check tunnel: `Linux SOCKS`

## Keyboard and Accessibility Notes

- Focus rings remained visible in populated list and form states.
- Selected rows retained contrast while focused.
- Row action menus dismissed correctly with `Escape` and outside click in live QA.

## Screenshots

Representative screenshots are stored in `/Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/screenshots/us1/`.

Core evidence files:
- `01-empty-state.png`
- `02-light-theme-workspace.png`
- `03-dark-theme-workspace.png`
- `04-list-focus-and-selection.png`
- `05-list-selected-state.png`
- `06-keyboard-focus-visible.png`
- `07-row-menu-layering.png`
- `08-narrow-layout.png`
- `09-mobile-layout.png`
- `16-linux-populated-workspace.png`
- `18-linux-server-menu.png`
- `19-linux-narrow-layout.png`

Additional exploratory captures from the QA runs remain in the same directory.

## Residual Risk

- Linux parity was spot-checked in Chromium against the built frontend rather than a full native Wails desktop shell.
- Manual screen-reader verification is still a follow-up item.

## Conclusion

**User Story 1 manual QA passes with no blockers.**

The refreshed workspace now has recorded evidence for theme readability, populated list states, row-menu layering, empty states, and responsive behavior across macOS and Linux browser-based verification.
