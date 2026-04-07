package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"ssh-man/internal/app/bindings"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app, err := bindings.NewAppBindings()
	if err != nil {
		log.Fatalf("bootstrap application: %v", err)
	}

	err = wails.Run(&options.App{
		Title:  "SSH Man",
		Width:  1280,
		Height: 840,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []interface{}{
			app,
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: true,
		},
	})
	if err != nil {
		log.Fatalf("run application: %v", err)
	}
}
