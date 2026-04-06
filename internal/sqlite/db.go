package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func OpenDatabase(configDir string) (*sql.DB, error) {
	dsn := filepath.Join(configDir, "ssh-man.db")
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := RunMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
