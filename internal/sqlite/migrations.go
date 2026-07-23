package sqlite

import (
	"database/sql"
	"fmt"
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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
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
