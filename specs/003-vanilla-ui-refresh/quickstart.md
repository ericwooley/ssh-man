# Quickstart: Validate and Preview Visual Refresh

1. Install frontend dependencies (from repo root):

```bash
pnpm install --dir frontend
```

2. Run frontend tests and build validation:

```bash
pnpm --dir frontend run validate
```

The validate command now runs the shared CSS contract check before Vitest and the production build.

3. Run the app in development mode (desktop dev script):

```bash
./scripts/dev-current-os.sh
```

4. For visual QA: open the app on both macOS and Linux targets and exercise main flows: server list, config editor, active sessions, dialogs, and banners. Confirm parity and token usage.
