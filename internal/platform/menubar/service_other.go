//go:build !darwin && !windows

package menubar

type unsupportedService struct{}

func New(Callbacks) Service {
	return unsupportedService{}
}

func (unsupportedService) Start() error {
	return nil
}

func (unsupportedService) Show() bool {
	return false
}

func (unsupportedService) ShowBrowserSwitcher() bool {
	return false
}

func (unsupportedService) CancelBrowserSwitchSession() {}

func (unsupportedService) SetBrowserShortcuts(string, string) error {
	return nil
}

func (unsupportedService) ShouldHideWindowOnClose() bool {
	return false
}

func (unsupportedService) Stop() {}
