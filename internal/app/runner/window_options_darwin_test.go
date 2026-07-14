//go:build darwin

package runner

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestDarwinWindowOptionsCreateHiddenFixedPopup(t *testing.T) {
	app := &options.App{}
	applyPlatformWindowOptions(app)

	if !app.Frameless || !app.DisableResize || !app.StartHidden || !app.HideWindowOnClose || !app.AlwaysOnTop {
		t.Fatalf("unexpected Darwin window options: %#v", app)
	}
}
