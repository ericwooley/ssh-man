package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestRunMigrationsUpgradesLegacyBrowserSwitcherDefaults(t *testing.T) {
	for _, legacyShortcut := range []string{"Alt+;", "Alt+]", ""} {
		t.Run(legacyShortcut, func(t *testing.T) {
			db := openUnmigratedDatabase(t)
			createLegacyPreferences(t, db, true, legacyShortcut)

			if err := RunMigrations(db); err != nil {
				t.Fatalf("run migrations: %v", err)
			}
			assertStoredBrowserSwitcherShortcuts(t, db, "Alt+X", "Alt+Z")
		})
	}
}

func TestRunMigrationsPreservesCustomLegacyBrowserSwitcherShortcut(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, true, "Ctrl+Alt+B")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	assertStoredBrowserSwitcherShortcuts(t, db, "Ctrl+Alt+B", "Alt+Z")
}

func TestRunMigrationsAddsBrowserSwitcherDefaultsToPreShortcutSchema(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	assertStoredBrowserSwitcherShortcuts(t, db, "Alt+X", "Alt+Z")
}

func TestRunMigrationsDoesNotOverwriteSavedBrowserSwitcherShortcuts(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, true, "Alt+]")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run first migrations: %v", err)
	}
	if _, err := db.Exec(`
		UPDATE user_preferences
		SET browser_switcher_shortcut = 'Ctrl+Alt+F8',
		    browser_switcher_backward_shortcut = 'Ctrl+Alt+F7'
		WHERE id = 1
	`); err != nil {
		t.Fatalf("customize shortcuts: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run repeated migrations: %v", err)
	}
	assertStoredBrowserSwitcherShortcuts(t, db, "Ctrl+Alt+F8", "Ctrl+Alt+F7")
}

func TestRunMigrationsAddsBrowserAppearancesDefaultToLegacySchema(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT browser_appearances_json FROM user_preferences WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("load migrated browser appearances: %v", err)
	}
	if stored != "{}" {
		t.Fatalf("migrated browser appearances = %q, want {}", stored)
	}
}

func TestRunMigrationsDoesNotOverwriteSavedBrowserAppearances(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run first migrations: %v", err)
	}
	want := `{"regular:google-chrome":{"icon":"X","primaryColor":"#00A651"}}`
	if _, err := db.Exec(`UPDATE user_preferences SET browser_appearances_json = ? WHERE id = 1`, want); err != nil {
		t.Fatalf("customize browser appearances: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run repeated migrations: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT browser_appearances_json FROM user_preferences WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("load browser appearances: %v", err)
	}
	if stored != want {
		t.Fatalf("browser appearances = %q, want %q", stored, want)
	}
}

func TestRunMigrationsAddsURLRoutingDefaultsToLegacySchema(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	var fallback, proxy, rules string
	if err := db.QueryRow(`
		SELECT default_browser_id, proxy_browser_id, url_rules_json
		FROM user_preferences
		WHERE id = 1
	`).Scan(&fallback, &proxy, &rules); err != nil {
		t.Fatalf("load migrated URL routing settings: %v", err)
	}
	if fallback != "" || proxy != "" || rules != "[]" {
		t.Fatalf("migrated URL routing = (%q, %q, %q)", fallback, proxy, rules)
	}
}

func TestRunMigrationsDoesNotOverwriteSavedURLRoutingSettings(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run first migrations: %v", err)
	}
	wantRules := `[{"id":"work","pattern":"github","action":"browser","browserId":"firefox"}]`
	if _, err := db.Exec(`
		UPDATE user_preferences
		SET default_browser_id = 'safari',
		    proxy_browser_id = 'firefox',
		    url_rules_json = ?
		WHERE id = 1
	`, wantRules); err != nil {
		t.Fatalf("customize URL routing: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run repeated migrations: %v", err)
	}
	var fallback, proxy, rules string
	if err := db.QueryRow(`
		SELECT default_browser_id, proxy_browser_id, url_rules_json
		FROM user_preferences
		WHERE id = 1
	`).Scan(&fallback, &proxy, &rules); err != nil {
		t.Fatalf("load URL routing settings: %v", err)
	}
	if fallback != "safari" || proxy != "firefox" || rules != wantRules {
		t.Fatalf("URL routing = (%q, %q, %q)", fallback, proxy, rules)
	}
}

func TestRunMigrationsAddsCustomBrowsersDefaultToLegacySchema(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT custom_browsers_json FROM user_preferences WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("load migrated custom browsers: %v", err)
	}
	if stored != "[]" {
		t.Fatalf("migrated custom browsers = %q, want []", stored)
	}

	want := `[{"id":"custom-kagi","displayName":"Kagi Browser","launchReference":"/Applications/Kagi Browser.app","engine":"chromium"}]`
	if _, err := db.Exec(`UPDATE user_preferences SET custom_browsers_json = ? WHERE id = 1`, want); err != nil {
		t.Fatalf("save custom browsers: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("run repeated migrations: %v", err)
	}
	if err := db.QueryRow(`SELECT custom_browsers_json FROM user_preferences WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("reload custom browsers: %v", err)
	}
	if stored != want {
		t.Fatalf("custom browsers = %q, want %q", stored, want)
	}
}

func TestRunMigrationsAddsServerSOCKSPortToLegacySchema(t *testing.T) {
	db := openUnmigratedDatabase(t)
	if _, err := db.Exec(`
		CREATE TABLE servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			host TEXT NOT NULL,
			port INTEGER NOT NULL,
			username TEXT NOT NULL,
			auth_mode TEXT NOT NULL,
			key_reference TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		INSERT INTO servers(
			id, name, host, port, username, auth_mode, key_reference, created_at, updated_at
		) VALUES(
			'server-1', 'Production', 'example.com', 22, 'deploy', 'agent', '',
			'2026-07-23T00:00:00Z', '2026-07-23T00:00:00Z'
		);
	`); err != nil {
		t.Fatalf("create legacy servers: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	var socksPort int
	if err := db.QueryRow(`SELECT socks_port FROM servers WHERE id = 'server-1'`).Scan(&socksPort); err != nil {
		t.Fatalf("load migrated SOCKS port: %v", err)
	}
	if socksPort != 0 {
		t.Fatalf("migrated SOCKS port = %d, want pending automatic assignment", socksPort)
	}
}

func openUnmigratedDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func createLegacyPreferences(t *testing.T, db *sql.DB, withShortcut bool, shortcut string) {
	t.Helper()
	statement := `CREATE TABLE user_preferences (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		theme TEXT NOT NULL,
		last_selected_server_id TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL
	);`
	if withShortcut {
		statement = `CREATE TABLE user_preferences (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			theme TEXT NOT NULL,
			last_selected_server_id TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL,
			browser_switcher_shortcut TEXT NOT NULL DEFAULT 'Alt+;'
		);`
	}
	if _, err := db.Exec(statement); err != nil {
		t.Fatalf("create legacy preferences: %v", err)
	}
	if withShortcut {
		if _, err := db.Exec(`
			INSERT INTO user_preferences(
				id, theme, last_selected_server_id, updated_at, browser_switcher_shortcut
			) VALUES(1, 'dark', '', '2026-07-19T00:00:00Z', ?)
		`, shortcut); err != nil {
			t.Fatalf("insert legacy preferences: %v", err)
		}
		return
	}
	if _, err := db.Exec(`
		INSERT INTO user_preferences(id, theme, last_selected_server_id, updated_at)
		VALUES(1, 'dark', '', '2026-07-19T00:00:00Z')
	`); err != nil {
		t.Fatalf("insert legacy preferences: %v", err)
	}
}

func assertStoredBrowserSwitcherShortcuts(t *testing.T, db *sql.DB, wantForward, wantBackward string) {
	t.Helper()
	var forward, backward string
	if err := db.QueryRow(`
		SELECT browser_switcher_shortcut, browser_switcher_backward_shortcut
		FROM user_preferences
		WHERE id = 1
	`).Scan(&forward, &backward); err != nil {
		t.Fatalf("load browser switcher shortcuts: %v", err)
	}
	if forward != wantForward || backward != wantBackward {
		t.Fatalf("shortcuts = (%q, %q), want (%q, %q)", forward, backward, wantForward, wantBackward)
	}
}
