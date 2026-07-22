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

func (unavailableDarwinService) ShowBrowserSwitcher() bool {
	return false
}

func (unavailableDarwinService) CancelBrowserSwitchSession() {}

func (unavailableDarwinService) SetBrowserShortcuts(string, string) error {
	return errors.New("macOS global shortcuts require cgo")
}

func (unavailableDarwinService) Stop() {}
