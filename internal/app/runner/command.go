package runner

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-man/internal/app/bindings"
	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
)

func RunCommand(assets fs.FS, serverID string) error {
	application, err := bootstrap.New(context.Background())
	if err != nil {
		return fmt.Errorf("bootstrap quick command window: %w", err)
	}
	server, err := application.ServerService.Get(context.Background(), serverID)
	if err != nil {
		_ = application.Shutdown(context.Background())
		return fmt.Errorf("load quick command server: %w", err)
	}
	window := appwindow.New()
	command := bindings.NewCommandBindings(application, server, window)
	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	finished := make(chan struct{})
	go func() {
		select {
		case <-signalContext.Done():
			if err := window.Quit(); err != nil {
				log.Printf("quit command window after parent shutdown: %v", err)
			}
		case <-finished:
		}
	}()
	runErr := wails.Run(newCommandOptions(assets, command, window, server.Name, server.ID))
	close(finished)
	return runErr
}

func newCommandOptions(assets fs.FS, command *bindings.CommandBindings, window *appwindow.Controller, serverName, serverID string) *options.App {
	return &options.App{
		Title:             serverName + " — SSH Man Command",
		Width:             940,
		Height:            700,
		MinWidth:          700,
		MinHeight:         500,
		Frameless:         false,
		DisableResize:     false,
		StartHidden:       false,
		HideWindowOnClose: false,
		AlwaysOnTop:       false,
		AssetServer:       &assetserver.Options{Assets: assets},
		Bind:              []interface{}{command},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID + ".command." + serverID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				if ctx, err := window.Context(); err == nil {
					wailsruntime.WindowUnminimise(ctx)
					wailsruntime.WindowShow(ctx)
					return
				}
				window.ShowWhenReady()
			},
		},
		OnStartup: func(ctx context.Context) {
			command.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			if err := command.Shutdown(ctx); err != nil {
				log.Printf("shutdown command window: %v", err)
			}
		},
	}
}
