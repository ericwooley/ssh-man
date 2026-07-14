//go:build !darwin

package runner

import "github.com/wailsapp/wails/v2/pkg/options"

func applyPlatformWindowOptions(app *options.App) {
	// Without a native menu-bar entrypoint, other platforms retain a normal,
	// visible, resizable window so users can always get back into the app.
	app.Frameless = false
	app.DisableResize = false
	app.StartHidden = false
	app.HideWindowOnClose = false
	app.AlwaysOnTop = false
}
