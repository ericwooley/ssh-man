# Session Management Smoke Test

## Scope

Validate saved tunnel startup, stop, reconnect feedback, and encrypted-key recovery on Linux and macOS.

## Preconditions

- The app launches successfully.
- At least one reachable SSH server is available.
- One saved local-forward configuration and one saved SOCKS configuration exist.
- An encrypted private key is available for the unlock flow.

## Steps

1. Launch the app and confirm the previously saved servers and configurations render without manual recreation.
2. Select the saved local-forward tunnel and start it.
3. Confirm the session status changes from `starting` to `connected` and the detail text names the localhost bind.
4. Stop the tunnel and confirm the status changes to `stopped` with a clear idle or stopped message.
5. Start the saved SOCKS tunnel and confirm the session shows `connected` with the SOCKS listener port.
6. Interrupt network connectivity or invalidate the SSH connection source.
7. Confirm the session status changes to `reconnecting` with actionable reconnect messaging.
8. Restore connectivity and confirm the session either returns to `connected` or ends in `failed` with a clear recovery message.
9. Start a configuration that uses an encrypted SSH key.
10. Confirm the status changes to `needs_attention` and the unlock form is available from keyboard-only navigation.
11. Submit an incorrect passphrase and confirm the UI keeps the session in a recoverable failure or attention state with actionable guidance.
12. Submit the correct passphrase and confirm the session transitions to `connected` without recreating the configuration.

## Expected Results

- Status pills and detail messages stay synchronized with runtime state changes.
- Keyboard users can reach Start, Stop, Retry, and Unlock controls with visible focus styling.
- Reconnect failures explain whether the issue is bind-related, SSH/auth-related, or connectivity-related.
- Unlock prompts are only required when encrypted-key startup actually needs user input.
