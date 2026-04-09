# Platform Validation Smoke Test

## Scope

Record platform-sensitive validation for persistence, browser discovery, and packaged desktop behavior.

## Linux Checklist

1. Confirm saved data persists under the Linux user config directory used by the app.
2. Launch the app after restart and verify saved servers, tunnels, and theme preference reload.
3. Confirm browser discovery finds Linux browser executables on `PATH`.
4. Validate a packaged or local Wails build starts successfully.

## macOS Checklist

1. Confirm saved data persists under the macOS user config directory used by the app.
2. Launch the app after restart and verify saved servers, tunnels, and theme preference reload.
3. Confirm browser discovery finds supported applications in `/Applications`.
4. Validate a packaged or local Wails build starts successfully.

## Shared Expected Results

- The same saved data and recovery flows work on both platforms.
- Runtime status, reconnect messaging, and encrypted-key unlock behavior remain consistent.
- Browser discovery and launch match platform-specific expectations without changing the user-visible workflow.
- Build validation notes should be added below when each platform is exercised.

## Validation Notes

- Linux: `wails doctor` was run on Ubuntu 24.04 on 2026-04-09 and reported missing default `libwebkit` packages on this host. `wails build -clean` failed for the same `webkit2gtk-4.0` dependency gap. `./scripts/wails-build-linux.sh` succeeded and produced `build/bin/ssh-man` using the required `webkit2_41` tag.
- macOS: pending local execution notes.
