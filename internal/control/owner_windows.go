//go:build windows

package control

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
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

	var overlapped windows.Overlapped
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&overlapped,
	)
	if err != nil {
		_ = file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
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

	file := l.file
	l.file = nil
	var overlapped windows.Overlapped
	unlockErr := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
	closeErr := file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
