package app

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"

	"ssh-man/internal/app/bindings"
	appmenu "ssh-man/internal/app/menu"
)

func Main() {
	app, err := bindings.NewAppBindings()
	if err != nil {
		log.Fatalf("bootstrap application: %v", err)
	}

	err = wails.Run(&options.App{
		Title:  "SSH Man",
		Width:  1280,
		Height: 840,
		Menu:   appmenu.Build(context.Background(), app),
		Bind: []interface{}{
			app,
		},
		OnStartup: func(ctx context.Context) {
			app.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			if err := app.Shutdown(ctx); err != nil {
				log.Printf("shutdown application: %v", err)
			}
		},
		EnableDefaultContextMenu: true,
	})
	if err != nil {
		log.Fatalf("run application: %v", err)
	}
}
