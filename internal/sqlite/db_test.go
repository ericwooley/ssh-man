package sqlite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenDatabaseCreatesReusableDatabaseFile(t *testing.T) {
	configDir := t.TempDir()

	db, err := OpenDatabase(configDir)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	if _, err := os.Stat(filepath.Join(configDir, "ssh-man.db")); err != nil {
		t.Fatalf("stat database file: %v", err)
	}

	db, err = OpenDatabase(configDir)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := db.Ping(); err != nil {
		t.Fatalf("ping reopened database: %v", err)
	}
}
