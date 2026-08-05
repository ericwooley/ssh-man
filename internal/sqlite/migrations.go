package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"ssh-man/internal/domain/preferences"
)

const enableForeignKeys = `PRAGMA foreign_keys = ON;`

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS servers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		socks_port INTEGER NOT NULL DEFAULT 0,
		username TEXT NOT NULL,
		auth_mode TEXT NOT NULL,
		key_reference TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS connection_configurations (
		id TEXT PRIMARY KEY,
		server_id TEXT NOT NULL,
		label TEXT NOT NULL,
		connection_type TEXT NOT NULL,
		local_port INTEGER NOT NULL DEFAULT 0,
		remote_host TEXT NOT NULL DEFAULT '',
		remote_port INTEGER NOT NULL DEFAULT 0,
		socks_port INTEGER NOT NULL DEFAULT 0,
		auto_reconnect_enabled INTEGER NOT NULL DEFAULT 1,
		start_on_launch INTEGER NOT NULL DEFAULT 0,
		notes TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		FOREIGN KEY(server_id) REFERENCES servers(id) ON DELETE CASCADE,
		UNIQUE(server_id, label)
	);`,
	`CREATE TABLE IF NOT EXISTS user_preferences (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		theme TEXT NOT NULL,
		last_selected_server_id TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS session_history (
		id TEXT PRIMARY KEY,
		configuration_id TEXT NOT NULL,
		started_at TEXT NOT NULL,
		ended_at TEXT NOT NULL,
		outcome TEXT NOT NULL,
		message TEXT NOT NULL,
		FOREIGN KEY(configuration_id) REFERENCES connection_configurations(id) ON DELETE CASCADE
	);`,
	`CREATE TABLE IF NOT EXISTS command_history (
		id TEXT PRIMARY KEY,
		server_id TEXT NOT NULL,
		command TEXT NOT NULL,
		output TEXT NOT NULL DEFAULT '',
		exit_code INTEGER NOT NULL,
		started_at TEXT NOT NULL,
		ended_at TEXT NOT NULL,
		truncated INTEGER NOT NULL DEFAULT 0,
		error TEXT NOT NULL DEFAULT '',
		FOREIGN KEY(server_id) REFERENCES servers(id) ON DELETE CASCADE
	);`,
	`CREATE INDEX IF NOT EXISTS command_history_server_started_idx
		ON command_history(server_id, started_at DESC);`,
}

func RunMigrations(db *sql.DB) error {
	if _, err := db.Exec(enableForeignKeys); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, statement := range schemaStatements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}

	if _, err := ensureTableColumn(
		tx,
		"servers",
		"socks_port",
		`ALTER TABLE servers ADD COLUMN socks_port INTEGER NOT NULL DEFAULT 0;`,
	); err != nil {
		return err
	}

	if _, err := ensureUserPreferencesColumn(
		tx,
		"browser_switcher_shortcut",
		`ALTER TABLE user_preferences ADD COLUMN browser_switcher_shortcut TEXT NOT NULL DEFAULT 'Alt+X';`,
	); err != nil {
		return err
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"automatic_updates_enabled",
		`ALTER TABLE user_preferences ADD COLUMN automatic_updates_enabled INTEGER NOT NULL DEFAULT 1;`,
	); err != nil {
		return err
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"use_experimental_channel",
		`ALTER TABLE user_preferences ADD COLUMN use_experimental_channel INTEGER NOT NULL DEFAULT 0;`,
	); err != nil {
		return err
	}
	backwardAdded, err := ensureUserPreferencesColumn(
		tx,
		"browser_switcher_backward_shortcut",
		`ALTER TABLE user_preferences ADD COLUMN browser_switcher_backward_shortcut TEXT NOT NULL DEFAULT 'Alt+Z';`,
	)
	if err != nil {
		return err
	}
	if backwardAdded {
		if _, err := tx.Exec(`
			UPDATE user_preferences
			SET browser_switcher_shortcut = 'Alt+X'
			WHERE browser_switcher_shortcut IN ('Alt+;', 'Alt+]')
			   OR TRIM(browser_switcher_shortcut) = ''
		`); err != nil {
			return fmt.Errorf("upgrade browser switcher shortcut defaults: %w", err)
		}
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"browser_appearances_json",
		`ALTER TABLE user_preferences ADD COLUMN browser_appearances_json TEXT NOT NULL DEFAULT '{}';`,
	); err != nil {
		return err
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"default_browser_id",
		`ALTER TABLE user_preferences ADD COLUMN default_browser_id TEXT NOT NULL DEFAULT '';`,
	); err != nil {
		return err
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"proxy_browser_id",
		`ALTER TABLE user_preferences ADD COLUMN proxy_browser_id TEXT NOT NULL DEFAULT '';`,
	); err != nil {
		return err
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"url_rules_json",
		`ALTER TABLE user_preferences ADD COLUMN url_rules_json TEXT NOT NULL DEFAULT '[]';`,
	); err != nil {
		return err
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"custom_browsers_json",
		`ALTER TABLE user_preferences ADD COLUMN custom_browsers_json TEXT NOT NULL DEFAULT '[]';`,
	); err != nil {
		return err
	}
	commandBrowsersAdded, err := ensureUserPreferencesColumn(
		tx,
		"command_browsers_json",
		`ALTER TABLE user_preferences ADD COLUMN command_browsers_json TEXT;`,
	)
	if err != nil {
		return err
	}
	browserRoutingRulesAdded, err := ensureUserPreferencesColumn(
		tx,
		"browser_routing_rules_json",
		`ALTER TABLE user_preferences ADD COLUMN browser_routing_rules_json TEXT;`,
	)
	if err != nil {
		return err
	}
	if commandBrowsersAdded || browserRoutingRulesAdded {
		if err := migrateBrowserRoutingStorage(tx, commandBrowsersAdded, browserRoutingRulesAdded); err != nil {
			return err
		}
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"url_port_assignments_json",
		`ALTER TABLE user_preferences ADD COLUMN url_port_assignments_json TEXT NOT NULL DEFAULT '[]';`,
	); err != nil {
		return err
	}
	if _, err := ensureUserPreferencesColumn(
		tx,
		"disabled_browser_ids_json",
		`ALTER TABLE user_preferences ADD COLUMN disabled_browser_ids_json TEXT NOT NULL DEFAULT '[]';`,
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func migrateBrowserRoutingStorage(tx *sql.Tx, migrateCommandBrowsers, migrateBrowserRules bool) error {
	var customBrowsersJSON, urlRulesJSON string
	err := tx.QueryRow(`
		SELECT custom_browsers_json, url_rules_json
		FROM user_preferences
		WHERE id = 1
	`).Scan(&customBrowsersJSON, &urlRulesJSON)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load browser routing preferences for migration: %w", err)
	}

	if migrateCommandBrowsers {
		var customBrowsers []preferences.CustomBrowser
		// Invalid legacy JSON remains in place with a NULL modern partition so
		// normal preference loading can surface the existing recoverable error.
		if decodeErr := json.Unmarshal([]byte(customBrowsersJSON), &customBrowsers); decodeErr == nil {
			legacyBrowsers, commandBrowsers := splitCustomBrowsers(customBrowsers)
			legacyBrowsersJSON, err := json.Marshal(legacyBrowsers)
			if err != nil {
				return fmt.Errorf("encode legacy custom browsers for migration: %w", err)
			}
			commandBrowsersJSON, err := json.Marshal(commandBrowsers)
			if err != nil {
				return fmt.Errorf("encode command browsers for migration: %w", err)
			}
			if _, err := tx.Exec(`
				UPDATE user_preferences
				SET custom_browsers_json = ?,
				    command_browsers_json = ?
				WHERE id = 1
			`, string(legacyBrowsersJSON), string(commandBrowsersJSON)); err != nil {
				return fmt.Errorf("migrate command browser storage: %w", err)
			}
		}
	}

	if migrateBrowserRules {
		var rules []preferences.URLRule
		// Migrate each partition independently so one corrupt legacy value does
		// not prevent the other valid browser preferences from being upgraded.
		if decodeErr := json.Unmarshal([]byte(urlRulesJSON), &rules); decodeErr == nil {
			legacyRulesJSON, err := json.Marshal(projectLegacyURLRules(rules))
			if err != nil {
				return fmt.Errorf("encode legacy-compatible URL rules for migration: %w", err)
			}
			if _, err := tx.Exec(`
				UPDATE user_preferences
				SET url_rules_json = ?,
				    browser_routing_rules_json = ?
				WHERE id = 1
			`, string(legacyRulesJSON), urlRulesJSON); err != nil {
				return fmt.Errorf("migrate browser routing rule storage: %w", err)
			}
		}
	}

	return nil
}

func ensureUserPreferencesColumn(tx *sql.Tx, columnName, alterStatement string) (bool, error) {
	return ensureTableColumn(tx, "user_preferences", columnName, alterStatement)
}

func ensureTableColumn(tx *sql.Tx, tableName, columnName, alterStatement string) (bool, error) {
	rows, err := tx.Query(fmt.Sprintf(`PRAGMA table_info(%s);`, tableName))
	if err != nil {
		return false, fmt.Errorf("inspect %s columns: %w", tableName, err)
	}

	found := false
	for rows.Next() {
		var (
			columnID     int
			name         string
			dataType     string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)
		if err := rows.Scan(&columnID, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return false, fmt.Errorf("inspect %s column: %w", tableName, err)
		}
		if name == columnName {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return false, fmt.Errorf("close %s column inspection: %w", tableName, err)
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("inspect %s columns: %w", tableName, err)
	}
	if found {
		return false, nil
	}

	if _, err := tx.Exec(alterStatement); err != nil {
		return false, fmt.Errorf("add %s.%s: %w", tableName, columnName, err)
	}
	return true, nil
}
