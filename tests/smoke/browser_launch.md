# Browser Launch Smoke Test

## Scope

Validate browser discovery, SOCKS-only launch eligibility, and theme readability/accessibility.

## Preconditions

- The app launches successfully.
- A saved SOCKS configuration exists and can be started.
- At least one supported browser is installed on the current platform.

## Steps

1. Start the saved SOCKS tunnel and confirm it reaches `connected`.
2. Open the browser-launch panel and activate `Refresh browsers`.
3. Confirm installed browsers appear in the selector.
4. If an unsupported browser appears, confirm it is described as unavailable for SOCKS launch in this MVP.
5. Select a supported browser and launch it through the running SOCKS session.
6. Confirm the app shows a success banner or clear launch failure message.
7. Stop the SOCKS tunnel and confirm browser launch controls become unavailable.
8. Toggle between dark and light theme.
9. Confirm button labels, banners, lists, and status pills remain readable in both themes.
10. Repeat the browser selection flow using only keyboard navigation and confirm visible focus states on the toggle, selector, refresh button, and launch button.

## Expected Results

- Browser launch remains disabled unless a SOCKS session is selected and connected.
- Unsupported browsers are communicated clearly and cannot be launched.
- Theme changes do not reduce contrast or hide focus indicators.
- Keyboard-only navigation can complete browser selection and launch.
- Linux and macOS may expose different browser names or install locations, but the user-visible discovery and launch flow should remain the same.
