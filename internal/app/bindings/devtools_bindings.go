package bindings

import (
	"fmt"
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *AppBindings) OpenDevTools() error {
	ctx, err := a.window.Context()
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		wailsruntime.WindowExecJS(ctx, "window.WailsInvoke && window.WailsInvoke('wails:openInspector')")
		return nil
	case "linux":
		wailsruntime.WindowExecJS(ctx, "window.WailsInvoke && window.WailsInvoke('wails:showInspector')")
		return nil
	default:
		return fmt.Errorf("use Ctrl+Shift+F12 to open frontend devtools in this build")
	}
}
