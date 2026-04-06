package sqlite

import (
	"database/sql"
	"fmt"
)

var migrations = []string{
	`PRAGMA foreign_keys = ON;`,
	`CREATE TABLE IF NOT EXISTS servers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
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
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}

	return nil
}
