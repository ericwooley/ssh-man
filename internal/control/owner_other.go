//go:build !darwin && !linux

package control

import "errors"

var ErrOwnerActive = errors.New("another SSH Man controller is already running")

type OwnerLease struct{}

func AcquireOwner(string) (*OwnerLease, error) {
	return &OwnerLease{}, nil
}

func (*OwnerLease) Release() error {
	return nil
}
