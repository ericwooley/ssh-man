//go:build windows

package control

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	ownerHelperModeEnv = "SSH_MAN_OWNER_HELPER"
	ownerHelperPathEnv = "SSH_MAN_OWNER_LOCK_PATH"
)

func TestAcquireOwnerExcludesSecondWindowsController(t *testing.T) {
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

	command := exec.Command(os.Args[0], "-test.run=^TestWindowsOwnerLeaseHelper$")
	command.Env = append(os.Environ(),
		ownerHelperModeEnv+"=1",
		ownerHelperPathEnv+"="+path,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("helper process did not observe the owner lock: %v\n%s", err, output)
	}
}

func TestWindowsOwnerLeaseCanBeReacquiredAfterRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.lock")
	first, err := AcquireOwner(path)
	if err != nil {
		t.Fatalf("first AcquireOwner() error = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("first Release() error = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("second Release() error = %v", err)
	}

	second, err := AcquireOwner(path)
	if err != nil {
		t.Fatalf("reacquire error = %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("reacquired Release() error = %v", err)
	}
}

func TestWindowsOwnerLeaseHelper(t *testing.T) {
	if os.Getenv(ownerHelperModeEnv) != "1" {
		return
	}

	lease, err := AcquireOwner(os.Getenv(ownerHelperPathEnv))
	if lease != nil {
		_ = lease.Release()
		t.Fatal("helper process acquired an already-owned lock")
	}
	if !errors.Is(err, ErrOwnerActive) {
		t.Fatalf("helper AcquireOwner() error = %v, want ErrOwnerActive", err)
	}
}
