package menubar

// Service owns the platform-specific menu-bar integration. Show reports
// whether the native popup was available and shown.
type Service interface {
	Start() error
	Show() bool
	Stop()
}

type Callbacks struct {
	Quit func()
}
