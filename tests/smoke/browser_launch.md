# Browser Launch Smoke Test

## Scope

Validate browser discovery, SOCKS-only launch eligibility, and theme readability/accessibility.

## Preconditions

- The app launches successfully.
- A saved SOCKS configuration exists and is stopped.
- At least one supported browser is installed on the current platform.

## Steps

1. Confirm the saved SOCKS tunnel is stopped.
2. Open the browser-launch panel and activate `Refresh browsers`.
3. Confirm installed browsers appear in the selector while the tunnel stays stopped.
4. If an unsupported browser appears, confirm it is described as unavailable for SOCKS launch.
5. Select a supported browser and launch it through the stopped SOCKS session.
6. Confirm the proxy connects before the browser opens.
7. Confirm the app shows a success banner or a clear launch failure message.
8. Toggle between dark and light theme.
9. Confirm button labels, banners, lists, and status pills remain readable in both themes.
10. Repeat the browser selection flow with only keyboard navigation and confirm visible focus states.

## Expected Results

- Browser launch remains available for a selected stopped SOCKS session and starts that session on demand.
- Unsupported browsers are communicated clearly and cannot be launched.
- Theme changes do not reduce contrast or hide focus indicators.
- Keyboard-only navigation can complete browser selection and launch.
- Linux and macOS may expose different browser names or install locations, but the user-visible discovery and launch flow should remain the same.
