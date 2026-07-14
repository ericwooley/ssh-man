//go:build darwin || linux

package control

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireOwnerExcludesSecondController(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "controller.lock")
	first, err := AcquireOwner(path)
	if err != nil {
		t.Fatalf("first AcquireOwner() error = %v", err)
	}
	t.Cleanup(func() {
		if err := first.Release(); err != nil {
			t.Errorf("Release() error = %v", err)
		}
	})

	second, err := AcquireOwner(path)
	if second != nil {
		_ = second.Release()
		t.Fatal("second AcquireOwner() returned a lease")
	}
	if !errors.Is(err, ErrOwnerActive) {
		t.Fatalf("second AcquireOwner() error = %v, want ErrOwnerActive", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(lock) error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("lock permissions = %o, want 600", info.Mode().Perm())
	}
}
