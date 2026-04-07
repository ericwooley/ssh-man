package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"ssh-man/internal/platform/paths"

	_ "github.com/mattn/go-sqlite3"
)

func OpenDatabase(configDir string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL", paths.DatabasePath(configDir))
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if err := RunMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
