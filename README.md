# ssh-man

`ssh-man` is a desktop SSH tunnel manager built with Go, Wails, and Svelte.

It lets you:

- save SSH servers and tunnel configurations
- run local forwards and SOCKS proxies
- launch SOCKS-aware browsers through a selected tunnel
- use the local SSH agent by default for authentication
- keep app data, browser profiles, and session history under the normal OS config directory

## Install

## macOS

Use Homebrew for the normal macOS install path.

### Homebrew install

```bash
brew tap ericwooley/homebrew-apps
brew install --cask ssh-man
xattr -d com.apple.quarantine /Applications/ssh-man.app
```

The app is currently distributed unsigned, so remove the quarantine attribute after install before first launch. If Homebrew replaces the app bundle during a later upgrade, run the same `xattr -d com.apple.quarantine /Applications/ssh-man.app` command again before opening the updated copy.

### Requirements

- Go `1.22.2`
- Node.js and pnpm
- Xcode Command Line Tools

Install the Xcode tools if needed:

```bash
xcode-select --install
```

Install pnpm if needed:

```bash
npm install -g pnpm
```

### Build and run

If you want to build from source instead of using Homebrew:

```bash
git clone git@github.com:ericwooley/ssh-man.git
cd ssh-man
./scripts/build-current-os.sh
open build/bin/ssh-man.app
```

The packaged app bundle is written to `build/bin/ssh-man.app`.

### macOS notes

- If Gatekeeper warns because the app is unsigned, open it from Finder with `Open` and confirm once.
- `ssh-man` uses your local SSH agent by default, so make sure your agent is running and `SSH_AUTH_SOCK` is available to GUI apps.
- App data is stored under `~/Library/Application Support/ssh-man`.

### Upgrade

```bash
brew upgrade --cask ssh-man
xattr -d com.apple.quarantine /Applications/ssh-man.app
```

## Linux

Linux is currently supported through a clone-and-build workflow.

### Requirements

- Go `1.22.2`
- Node.js and pnpm
- `pkg-config`
- GTK 3 development headers
- WebKitGTK 4.1 development headers

Ubuntu or Debian example:

```bash
sudo apt update
sudo apt install -y golang-go pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
npm install -g pnpm
```

### Build and run

```bash
git clone git@github.com:ericwooley/ssh-man.git
cd ssh-man
./scripts/build-current-os.sh
./build/bin/ssh-man
```

If your distro needs the explicit Linux Wails build path, use:

```bash
./scripts/wails-build-linux.sh
./build/bin/ssh-man
```

### Linux notes

- This repo uses the `webkit2_41` Wails build tag for Linux builds.
- A plain `wails build -clean` may fail on systems that only expose the wrong WebKit package through `pkg-config`.
- App data is stored under `${XDG_CONFIG_HOME:-~/.config}/ssh-man`.

## Development

### Run in dev mode

```bash
./scripts/dev-current-os.sh
```

### Validate the repo

```bash
./scripts/validate.sh
```

### Frontend-only checks

```bash
pnpm install --dir frontend
pnpm --dir frontend run validate
```

## First-run tips

- New servers default to `SSH agent` auth.
- If you want file-based auth instead, switch the server to `Private key` and enter the key path.
- Browser profiles are persisted per server under the app config directory so bookmarks, extensions, and other browser state survive restarts.

## Project layout

```text
frontend/   Svelte UI
internal/   Go application code
scripts/    build, dev, and validation helpers
tests/      integration and smoke coverage
```

## Status

- macOS: supported via Homebrew cask and local source build
- Linux: supported via local source build
- Homebrew tap: `ericwooley/homebrew-apps`
