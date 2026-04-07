package bindings

import (
	"fmt"
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *AppBindings) OpenDevTools() error {
	if a.ctx == nil {
		return fmt.Errorf("the application window is not ready yet")
	}

	switch runtime.GOOS {
	case "darwin":
		wailsruntime.WindowExecJS(a.ctx, "window.WailsInvoke && window.WailsInvoke('wails:openInspector')")
		return nil
	case "linux":
		wailsruntime.WindowExecJS(a.ctx, "window.WailsInvoke && window.WailsInvoke('wails:showInspector')")
		return nil
	default:
		return fmt.Errorf("use Ctrl+Shift+F12 to open frontend devtools in this build")
	}
}
