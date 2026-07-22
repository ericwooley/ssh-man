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

func RunExplorer(assets fs.FS, serverID string) error {
	application, err := bootstrap.New(context.Background())
	if err != nil {
		return fmt.Errorf("bootstrap explorer: %w", err)
	}
	server, err := application.ServerService.Get(context.Background(), serverID)
	if err != nil {
		_ = application.Shutdown(context.Background())
		return fmt.Errorf("load explorer server: %w", err)
	}
	window := appwindow.New()
	explorer, middleware := bindings.NewExplorerBindings(application, server, window)
	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	finished := make(chan struct{})
	go func() {
		select {
		case <-signalContext.Done():
			if err := window.Quit(); err != nil {
				log.Printf("quit explorer after parent shutdown: %v", err)
			}
		case <-finished:
		}
	}()
	runErr := wails.Run(newExplorerOptions(assets, explorer, middleware, window, server.Name, server.ID))
	close(finished)
	return runErr
}

func newExplorerOptions(assets fs.FS, explorer *bindings.ExplorerBindings, middleware assetserver.Middleware, window *appwindow.Controller, serverName, serverID string) *options.App {
	return &options.App{
		Title:             serverName + " — SSH Man Explorer",
		Width:             1180,
		Height:            760,
		MinWidth:          820,
		MinHeight:         520,
		Frameless:         false,
		DisableResize:     false,
		StartHidden:       false,
		HideWindowOnClose: false,
		AlwaysOnTop:       false,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: middleware,
		},
		Bind: []interface{}{explorer},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID + ".explorer." + serverID,
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
			explorer.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			if err := explorer.Shutdown(ctx); err != nil {
				log.Printf("shutdown explorer: %v", err)
			}
		},
	}
}
