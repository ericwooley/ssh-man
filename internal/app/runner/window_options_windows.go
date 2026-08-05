//go:build windows

package runner

import "github.com/wailsapp/wails/v2/pkg/options"

func applyPlatformWindowOptions(app *options.App) {
	app.Frameless = false
	app.DisableResize = false
	app.StartHidden = true
	app.HideWindowOnClose = true
	app.AlwaysOnTop = false
}
