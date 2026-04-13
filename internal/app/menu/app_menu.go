package menu

import (
	"context"
	"fmt"

	wmenu "github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-man/internal/app/bindings"
)

func Build(ctx context.Context, app *bindings.AppBindings) *wmenu.Menu {
	root := wmenu.NewMenu()
	root.Append(wmenu.AppMenu())
	root.Append(wmenu.EditMenu())

	windowMenu := wmenu.WindowMenu()
	windowMenu.Append(wmenu.Separator())
	windowMenu.Append(wmenu.Text("Open DevTools", keys.Combo("f12", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *wmenu.CallbackData) {
		if err := app.OpenDevTools(); err != nil {
			_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Title:   "Open DevTools",
				Message: fmt.Sprintf("%v\n\nIf you need packaged devtools support, build the app with the Wails `-devtools` flag.", err),
			})
		}
	}))

	root.Append(windowMenu)
	return root
}
