package paths

import (
	"path/filepath"
	"testing"
)

func TestApplicationPathsShareTheConfigDirectory(t *testing.T) {
	dir := filepath.Join("tmp", "ssh-man")
	tests := map[string]string{
		"database": DatabasePath(dir),
		"control":  ControlSocketPath(dir),
		"owner":    OwnerLockPath(dir),
	}
	wants := map[string]string{
		"database": filepath.Join(dir, "ssh-man.db"),
		"control":  filepath.Join(dir, "control.sock"),
		"owner":    filepath.Join(dir, "controller.lock"),
	}
	for name, got := range tests {
		if got != wants[name] {
			t.Fatalf("%s path = %q, want %q", name, got, wants[name])
		}
	}
}
