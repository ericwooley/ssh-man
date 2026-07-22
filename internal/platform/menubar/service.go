package menubar

// Service owns the platform-specific menu-bar integration. Show reports
// whether the native popup was available and shown.
type Service interface {
	Start() error
	Show() bool
	ShowBrowserSwitcher() bool
	CancelBrowserSwitchSession()
	SetBrowserShortcuts(forward string, backward string) error
	Stop()
}

type BrowserSwitchDirection string

const (
	BrowserSwitchForward  BrowserSwitchDirection = "forward"
	BrowserSwitchBackward BrowserSwitchDirection = "backward"
)

type Callbacks struct {
	Quit                func()
	SwitchBrowsers      func(BrowserSwitchDirection, uint64)
	CommitBrowserSwitch func(uint64)
	CancelBrowserSwitch func(uint64)
}
