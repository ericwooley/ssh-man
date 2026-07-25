package runner

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-man/internal/app/bindings"
	"ssh-man/internal/app/bootstrap"
	"ssh-man/internal/app/previewwindow"
	appwindow "ssh-man/internal/app/window"
)

func RunPreview(assets fs.FS, request previewwindow.Request) error {
	focusSignals := make(chan os.Signal, 1)
	if focusSignal := previewwindow.FocusSignal(); focusSignal != nil {
		signal.Notify(focusSignals, focusSignal)
		defer signal.Stop(focusSignals)
	}
	application, err := bootstrap.New(context.Background())
	if err != nil {
		return fmt.Errorf("bootstrap preview: %w", err)
	}
	server, err := application.ServerService.Get(context.Background(), request.ServerID)
	if err != nil {
		_ = application.Shutdown(context.Background())
		return fmt.Errorf("load preview server: %w", err)
	}
	window := appwindow.New()
	preview, middleware := bindings.NewPreviewBindings(application, server, request.RemotePath, window)
	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	finished := make(chan struct{})
	go func() {
		for {
			select {
			case <-signalContext.Done():
				if err := window.Quit(); err != nil {
					log.Printf("quit preview after parent shutdown: %v", err)
				}
				return
			case <-focusSignals:
				showPreviewWindow(window)
			case <-finished:
				return
			}
		}
	}()
	runErr := wails.Run(newPreviewOptions(assets, preview, middleware, window, server.Name, request))
	close(finished)
	return runErr
}

func showPreviewWindow(window *appwindow.Controller) {
	if ctx, err := window.Context(); err == nil {
		wailsruntime.WindowUnminimise(ctx)
		wailsruntime.WindowShow(ctx)
		return
	}
	window.ShowWhenReady()
}

func newPreviewOptions(
	assets fs.FS,
	preview *bindings.PreviewBindings,
	middleware assetserver.Middleware,
	window *appwindow.Controller,
	serverName string,
	request previewwindow.Request,
) *options.App {
	fileName := path.Base(request.RemotePath)
	return &options.App{
		Title:     fileName + " — " + serverName + " — SSH Man Preview",
		Width:     980,
		Height:    740,
		MinWidth:  560,
		MinHeight: 400,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: middleware,
		},
		Bind: []interface{}{preview},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID + ".preview." + previewwindow.InstanceID(request),
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				showPreviewWindow(window)
			},
		},
		OnStartup: func(ctx context.Context) {
			preview.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			if err := preview.Shutdown(ctx); err != nil {
				log.Printf("shutdown preview: %v", err)
			}
		},
	}
}
