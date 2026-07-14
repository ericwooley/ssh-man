package menu

import (
	"fmt"
	"log"

	wmenu "github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-man/internal/app/bindings"
	appwindow "ssh-man/internal/app/window"
)

func Build(app *bindings.AppBindings, window *appwindow.Controller) *wmenu.Menu {
	root := wmenu.NewMenu()
	root.Append(wmenu.AppMenu())
	root.Append(wmenu.EditMenu())

	windowMenu := wmenu.WindowMenu()
	windowMenu.Append(wmenu.Separator())
	windowMenu.Append(wmenu.Text("Open DevTools", keys.Combo("f12", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *wmenu.CallbackData) {
		if err := app.OpenDevTools(); err != nil {
			ctx, contextErr := window.Context()
			if contextErr != nil {
				log.Printf("open devtools: %v", err)
				return
			}
			_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Title:   "Open DevTools",
				Message: fmt.Sprintf("%v\n\nIf you need packaged devtools support, build the app with the Wails `-devtools` flag.", err),
			})
		}
	}))

	root.Append(windowMenu)
	return root
}
