# Research: SSH Connection Manager MVP

## Storage Decisions

Decision: Use an embedded local SQLite database as the primary persisted store for the app, located in the standard per-user application configuration or data path for the current operating system.

Rationale: The user explicitly requested SQLite, and it fits an offline-first desktop MVP with low operational overhead. It provides structured persistence for servers, saved connection configurations, reconnect preferences, theme preference, and lightweight session history without introducing external services or sync complexity.

Alternatives considered: JSON or YAML files were simpler initially but weaker for querying, migrations, and data integrity. A remote database was unnecessary for a single-user desktop MVP.

Decision: Keep active session state in memory only and persist only durable artifacts such as saved configurations, preferences, and lightweight session history.

Rationale: Live SSH sessions, bound local ports, reconnect timers, and authentication context are transient and cannot be safely restored as active runtime state after a restart. Keeping runtime state in memory creates a clear boundary between what the app remembers and what is actively running.

Alternatives considered: Persisting active sessions for crash recovery would increase complexity and produce stale or misleading restored state. Persisting nothing at all would lose useful historical and preference data.

Decision: Discover installed browsers on demand and optionally keep only an in-memory cache for the current app run.

Rationale: Installed browsers are external system state that may change outside the app. On-demand discovery keeps launch choices accurate without introducing stale persisted inventory.

Alternatives considered: Persisting browser inventory in SQLite or refreshing it periodically would add invalidation complexity without meaningful MVP benefit.

## SSH Session Decisions

Decision: Treat each saved configuration as a tunnel session that is only considered started after authentication succeeds and all requested local binds succeed.

Rationale: This gives users one predictable start or stop workflow and avoids confusing partial success when one bind works but another fails.

Alternatives considered: Separate session concepts for local forwarding and SOCKS, or allowing partially successful sessions, were rejected because they complicate the MVP user model.

Decision: Auto-reconnect after unexpected disconnects using keepalive-driven detection and capped exponential backoff with small jitter.

Rationale: This balances rapid recovery for short network interruptions with protection against tight retry loops that can overwhelm logs or remote hosts. It is practical for common desktop interruptions such as laptop sleep, Wi-Fi changes, and VPN reconnection.

Alternatives considered: Immediate tight-loop retries, fixed-interval retries only, or no reconnect support were rejected as either too noisy or too weak for the requested behavior.

Decision: Do not auto-reconnect after user-initiated stops, invalid configuration, host-key problems, local bind failures, or authentication failures that require new user input.

Rationale: Reconnect should address transient connectivity failure, not repeatedly retry conditions that need user intervention.

Alternatives considered: Retrying all failures indefinitely was rejected because it would create confusing loops and poor UX.

Decision: Use a compact user-visible status model of `Stopped`, `Starting`, `Connected`, `Reconnecting`, `Needs attention`, and `Failed`.

Rationale: These states are expressive enough for the MVP UI while remaining understandable and reusable across both SOCKS and local forward workflows.

Alternatives considered: A detailed internal state machine exposed directly to users would add noise. A simpler connected or disconnected model would hide important recovery and credential states.

Decision: Prompt for encrypted SSH key passphrases only during manual start or explicit user retry. During background reconnect, reuse already unlocked credentials if available and otherwise transition the session to `Needs attention`.

Rationale: This avoids surprising background prompts while still supporting encrypted keys and user-controlled recovery.

Alternatives considered: Storing passphrases in the app or always prompting during reconnect were rejected for security and UX reasons.

## Frontend and Accessibility Decisions

Decision: Use plain Svelte components with one shared stylesheet at `frontend/src/app.css` and keep all primary workflows keyboard-complete.

Rationale: This satisfies the constitution and keeps the MVP UI simple, explicit, and maintainable. Keyboard-complete flows are the practical baseline for the requested accessibility standard.

Alternatives considered: Component-scoped styles, CSS modules, runtime-generated styles, or delayed accessibility work were rejected because they violate project constraints or raise regression risk.

Decision: Define platform-native baseline accessibility as complete keyboard navigation for primary workflows, visible focus indication, discernible labels including icon-only actions, and readable presentation in both light and dark themes.

Rationale: This is a realistic MVP standard for a desktop app while still creating an enforceable accessibility scope for planning and testing.

Alternatives considered: A full WCAG audit target for the MVP was considered heavier than necessary; a vague best-effort baseline was too weak to test.

Decision: Make proxied browser launch an explicit user action that requires a running SOCKS session and an installed supported browser discovered at launch time.

Rationale: Browser proxy behavior varies by OS and browser. Tying launch to an explicit action keeps the MVP scope narrow and predictable.

Alternatives considered: Implicit browser launch, persistent browser inventory, or broader proxy configuration modes were rejected as out of scope for MVP.

## Concrete Stack Choices

Decision: Target Go 1.24.x, Wails v2, Svelte 5, and `github.com/mattn/go-sqlite3`.

Rationale: This combination is current, mature enough for a greenfield desktop app, and aligned with the user and constitution constraints.

Alternatives considered: Older Go or Svelte versions were more conservative but less forward-looking. Pure-Go SQLite drivers were considered but rejected in favor of the more battle-tested desktop choice.

Decision: Validate locally with `wails doctor`, `gofmt -w .`, `go vet ./...`, `go test ./...`, `npm run build`, and `wails build -clean`.

Rationale: These checks cover environment readiness, formatting, static analysis, backend tests, frontend build health, and desktop packaging validation for the supported platforms.

Alternatives considered: Heavier lint or end-to-end tooling may be added later but is unnecessary to define the MVP plan.
