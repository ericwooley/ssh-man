# Final Validation

## Automated checks

- `pnpm --dir /Users/ericwooley/projects/ssh-man/frontend run validate`: PASS
- `go test ./...`: PASS

## Manual QA

- macOS Chromium/browser-only verification completed against `http://127.0.0.1:4173` with populated mock data.
- Linux Chromium spot-check completed against the built frontend served locally from `frontend/dist`.
- Screenshots saved under `/Users/ericwooley/projects/ssh-man/specs/003-vanilla-ui-refresh/screenshots/us1/`, including `01-empty-state.png`, `07-row-menu-layering.png`, `16-linux-populated-workspace.png`, `18-linux-server-menu.png`, and `19-linux-narrow-layout.png`.

## Notes

- Frontend validation now includes the shared CSS contract check before tests and build.
- Residual follow-up: native Linux Wails desktop-shell smoke check and manual screen-reader verification.
