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
