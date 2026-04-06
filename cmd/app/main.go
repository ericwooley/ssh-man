package app

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"

	"ssh-man/internal/app/bindings"
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
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatalf("run application: %v", err)
	}
}
