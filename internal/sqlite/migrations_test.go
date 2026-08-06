package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	preferencesdomain "ssh-man/internal/domain/preferences"
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

func TestRunMigrationsEnablesAutomaticUpdatesForLegacyPreferences(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	var enabled bool
	if err := db.QueryRow(`
		SELECT automatic_updates_enabled
		FROM user_preferences
		WHERE id = 1
	`).Scan(&enabled); err != nil {
		t.Fatalf("load automatic update preference: %v", err)
	}
	if !enabled {
		t.Fatal("automatic updates should default to enabled for existing users")
	}
}

func TestRunMigrationsKeepsLegacyUsersOnStableChannel(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	var experimental bool
	if err := db.QueryRow(`
		SELECT use_experimental_channel
		FROM user_preferences
		WHERE id = 1
	`).Scan(&experimental); err != nil {
		t.Fatalf("load update channel preference: %v", err)
	}
	if experimental {
		t.Fatal("legacy users should remain on the stable channel")
	}
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

func TestRunMigrationsSeparatesNewBrowserDataFromPreviousReaderColumns(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")
	if _, err := db.Exec(`
		ALTER TABLE user_preferences
			ADD COLUMN custom_browsers_json TEXT NOT NULL DEFAULT '[]';
		ALTER TABLE user_preferences
			ADD COLUMN url_rules_json TEXT NOT NULL DEFAULT '[]';
		UPDATE user_preferences
		SET custom_browsers_json = ?,
		    url_rules_json = ?
		WHERE id = 1
	`, `[
		{"id":"legacy-kagi","displayName":"Kagi Browser","launchReference":"/Applications/Kagi Browser.app","engine":"chromium"},
		{"id":"command-work","displayName":"Work Browser","command":"open -a \"Safari\" \"<URL>\"","icon":"icon:briefcase"}
	]`, `[
		{"id":"literal-bracket","matchMode":"contains","pattern":"[work]","action":"browser","browserId":"command-work","openDirect":true}
	]`); err != nil {
		t.Fatalf("seed previous browser storage: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	var legacyBrowsersJSON, commandBrowsersJSON, legacyRulesJSON, browserRulesJSON string
	if err := db.QueryRow(`
		SELECT custom_browsers_json, command_browsers_json,
		       url_rules_json, browser_routing_rules_json
		FROM user_preferences
		WHERE id = 1
	`).Scan(&legacyBrowsersJSON, &commandBrowsersJSON, &legacyRulesJSON, &browserRulesJSON); err != nil {
		t.Fatalf("load migrated browser storage: %v", err)
	}

	var legacyBrowsers []preferencesdomain.CustomBrowser
	if err := json.Unmarshal([]byte(legacyBrowsersJSON), &legacyBrowsers); err != nil {
		t.Fatalf("decode migrated legacy browsers: %v", err)
	}
	if len(legacyBrowsers) != 1 || legacyBrowsers[0].ID != "legacy-kagi" || legacyBrowsers[0].Command != "" {
		t.Fatalf("migrated legacy browsers = %#v", legacyBrowsers)
	}
	var commandBrowsers []preferencesdomain.CustomBrowser
	if err := json.Unmarshal([]byte(commandBrowsersJSON), &commandBrowsers); err != nil {
		t.Fatalf("decode migrated command browsers: %v", err)
	}
	if len(commandBrowsers) != 1 || commandBrowsers[0].ID != "command-work" {
		t.Fatalf("migrated command browsers = %#v", commandBrowsers)
	}

	var legacyRules []struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal([]byte(legacyRulesJSON), &legacyRules); err != nil {
		t.Fatalf("decode migrated legacy rules: %v", err)
	}
	if len(legacyRules) != 1 {
		t.Fatalf("migrated legacy rules = %#v", legacyRules)
	}
	if _, err := regexp.Compile(legacyRules[0].Pattern); err != nil {
		t.Fatalf("migrated compatibility pattern %q is not valid regex: %v", legacyRules[0].Pattern, err)
	}
	var browserRules []preferencesdomain.URLRule
	if err := json.Unmarshal([]byte(browserRulesJSON), &browserRules); err != nil {
		t.Fatalf("decode migrated browser rules: %v", err)
	}
	if len(browserRules) != 1 || browserRules[0].MatchMode != preferencesdomain.URLRuleMatchContains || !browserRules[0].OpenDirect {
		t.Fatalf("migrated browser rules = %#v", browserRules)
	}
}

func TestRunMigrationsPreservesMalformedBrowserPreferencesForRecovery(t *testing.T) {
	validCustomBrowsers := `[
		{"id":"command-work","displayName":"Work Browser","command":"open -a \"Safari\" \"<URL>\"","icon":"icon:briefcase"}
	]`
	validURLRules := `[
		{"id":"literal-work","matchMode":"contains","pattern":"[work]","action":"browser","browserId":"command-work","openDirect":true}
	]`
	tests := []struct {
		name                 string
		customBrowsersJSON   string
		urlRulesJSON         string
		wantCommandMigrated  bool
		wantURLRulesMigrated bool
		wantLoadError        string
	}{
		{
			name:                 "malformed custom browsers",
			customBrowsersJSON:   "{",
			urlRulesJSON:         validURLRules,
			wantURLRulesMigrated: true,
			wantLoadError:        "load custom browsers",
		},
		{
			name:                "malformed URL rules",
			customBrowsersJSON:  validCustomBrowsers,
			urlRulesJSON:        "{",
			wantCommandMigrated: true,
			wantLoadError:       "load URL rules",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openUnmigratedDatabase(t)
			createLegacyPreferences(t, db, false, "")
			if _, err := db.Exec(`
				ALTER TABLE user_preferences
					ADD COLUMN custom_browsers_json TEXT NOT NULL DEFAULT '[]';
				ALTER TABLE user_preferences
					ADD COLUMN url_rules_json TEXT NOT NULL DEFAULT '[]';
				UPDATE user_preferences
				SET custom_browsers_json = ?,
				    url_rules_json = ?
				WHERE id = 1
			`, test.customBrowsersJSON, test.urlRulesJSON); err != nil {
				t.Fatalf("seed browser preferences: %v", err)
			}

			if err := RunMigrations(db); err != nil {
				t.Fatalf("run migrations: %v", err)
			}

			var legacyCustomBrowsers, legacyURLRules string
			var commandBrowsers, browserRoutingRules sql.NullString
			if err := db.QueryRow(`
				SELECT custom_browsers_json, command_browsers_json,
				       url_rules_json, browser_routing_rules_json
				FROM user_preferences
				WHERE id = 1
			`).Scan(&legacyCustomBrowsers, &commandBrowsers, &legacyURLRules, &browserRoutingRules); err != nil {
				t.Fatalf("load migrated browser preferences: %v", err)
			}
			if test.wantCommandMigrated != commandBrowsers.Valid {
				t.Fatalf("command browser migration valid = %t, want %t", commandBrowsers.Valid, test.wantCommandMigrated)
			}
			if test.wantURLRulesMigrated != browserRoutingRules.Valid {
				t.Fatalf("browser rule migration valid = %t, want %t", browserRoutingRules.Valid, test.wantURLRulesMigrated)
			}
			if !test.wantCommandMigrated && legacyCustomBrowsers != test.customBrowsersJSON {
				t.Fatalf("legacy custom browsers = %q, want preserved malformed input %q", legacyCustomBrowsers, test.customBrowsersJSON)
			}
			if !test.wantURLRulesMigrated && legacyURLRules != test.urlRulesJSON {
				t.Fatalf("legacy URL rules = %q, want preserved malformed input %q", legacyURLRules, test.urlRulesJSON)
			}
			if _, err := NewPreferencesStore(db).Load(context.Background()); err == nil || !strings.Contains(err.Error(), test.wantLoadError) {
				t.Fatalf("preference load error = %v, want recoverable %q error", err, test.wantLoadError)
			}
		})
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

func TestRunMigrationsAddsURLPortAssignments(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	var assignments string
	if err := db.QueryRow(`
		SELECT url_port_assignments_json
		FROM user_preferences
		WHERE id = 1
	`).Scan(&assignments); err != nil {
		t.Fatalf("load migrated URL port assignments: %v", err)
	}
	if assignments != "[]" {
		t.Fatalf("migrated assignments = %q, want []", assignments)
	}
}

func TestRunMigrationsAddsDisabledBrowserIDs(t *testing.T) {
	db := openUnmigratedDatabase(t)
	createLegacyPreferences(t, db, false, "")

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	var disabled string
	if err := db.QueryRow(`
		SELECT disabled_browser_ids_json
		FROM user_preferences
		WHERE id = 1
	`).Scan(&disabled); err != nil {
		t.Fatalf("load migrated disabled browser ids: %v", err)
	}
	if disabled != "[]" {
		t.Fatalf("migrated disabled browser ids = %q, want []", disabled)
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
