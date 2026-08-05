//go:build windows

package runner

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestWindowsWindowOptionsSupportTrayLifecycle(t *testing.T) {
	app := &options.App{}
	applyPlatformWindowOptions(app)

	if !app.StartHidden || !app.HideWindowOnClose {
		t.Fatalf("Windows tray lifecycle options are disabled: %#v", app)
	}
	if app.Frameless || app.DisableResize || app.AlwaysOnTop {
		t.Fatalf("Windows control window lost standard window behavior: %#v", app)
	}
}
