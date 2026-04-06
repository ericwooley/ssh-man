package integration

import (
	"database/sql"
	"fmt"
	"testing"

	"ssh-man/internal/sqlite"

	_ "github.com/mattn/go-sqlite3"
)

func sqliteTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}
