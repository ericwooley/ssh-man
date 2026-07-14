//go:build darwin

package runner

import "github.com/wailsapp/wails/v2/pkg/options"

func applyPlatformWindowOptions(app *options.App) {
	app.Frameless = true
	app.DisableResize = true
	app.StartHidden = true
	app.HideWindowOnClose = true
	app.AlwaysOnTop = true
}
