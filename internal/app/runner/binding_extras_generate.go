//go:build bindings

package runner

import (
	"io/fs"

	"github.com/wailsapp/wails/v2"

	"ssh-man/internal/app/bindings"
	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	serverdomain "ssh-man/internal/domain/server"
)

type bindingsMenuBar struct{}

func (bindingsMenuBar) Start() error { return nil }
func (bindingsMenuBar) Show() bool   { return false }
func (bindingsMenuBar) ShowBrowserSwitcher() bool {
	return false
}
func (bindingsMenuBar) CancelBrowserSwitchSession()              {}
func (bindingsMenuBar) SetBrowserShortcuts(string, string) error { return nil }
func (bindingsMenuBar) Stop()                                    {}

func additionalBindingsForGeneration() []interface{} {
	explorer, _ := bindings.NewExplorerBindings(
		&bootstrap.Application{},
		serverdomain.Server{},
		appwindow.New(),
	)
	return []interface{}{explorer}
}

func maybeRunBindingsGeneration(assets fs.FS) (bool, error) {
	window := appwindow.New()
	application := &bootstrap.Application{}
	app := bindings.NewAppBindingsWithApplication(application, window)
	launcher := bindings.NewExplorerLauncherBindingsWithDependencies(nil, nil)
	bar := bindingsMenuBar{}
	lifecycle := newApplicationLifecycle(nil, bar, nil, nil, nil)
	return true, wails.Run(newOptions(assets, app, launcher, window, bar, lifecycle))
}
