//go:build darwin || linux

package control

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

var ErrOwnerActive = errors.New("another SSH Man controller is already running")

type OwnerLease struct {
	file *os.File
}

func AcquireOwner(path string) (*OwnerLease, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open owner lock: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure owner lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrOwnerActive
		}
		return nil, fmt.Errorf("lock controller ownership: %w", err)
	}
	return &OwnerLease{file: file}, nil
}

func (l *OwnerLease) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	if err != nil {
		return err
	}
	return closeErr
}
