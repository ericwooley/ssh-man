package sqlite

import (
	"database/sql"
	"fmt"
	"testing"
)

func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	if err != nil {
		t.Fatalf("open test sqlite database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}
