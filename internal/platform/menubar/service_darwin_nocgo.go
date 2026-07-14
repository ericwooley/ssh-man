//go:build darwin && !cgo

package menubar

import "errors"

type unavailableDarwinService struct{}

func New(Callbacks) Service {
	return unavailableDarwinService{}
}

func (unavailableDarwinService) Start() error {
	return errors.New("macOS menu-bar integration requires cgo")
}

func (unavailableDarwinService) Show() bool {
	return false
}

func (unavailableDarwinService) Stop() {}
