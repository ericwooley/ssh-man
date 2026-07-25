package runner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-man/internal/app/bindings"
	"ssh-man/internal/app/bootstrap"
	"ssh-man/internal/app/previewwindow"
	appwindow "ssh-man/internal/app/window"
)

const (
	previewShutdownTimeout      = 5 * time.Second
	previewWindowStateEventName = "preview-window:state"
)

type previewStateDispatcher struct {
	mu     sync.Mutex
	closed bool
	emit   func(previewwindow.State)
}

func newPreviewStateDispatcher(emit func(previewwindow.State)) *previewStateDispatcher {
	return &previewStateDispatcher{emit: emit}
}

func (d *previewStateDispatcher) Dispatch(state previewwindow.State) bool {
	if d == nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return false
	}
	if d.emit != nil {
		d.emit(state)
	}
	return true
}

func (d *previewStateDispatcher) Close() {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
}

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
	previewEvents := newPreviewStateDispatcher(func(state previewwindow.State) {
		if ctx, contextErr := window.Context(); contextErr == nil {
			wailsruntime.EventsEmit(ctx, previewWindowStateEventName, state)
		}
	})
	previewManager := previewwindow.NewManager()
	previewManager.SetStateListener(func(state previewwindow.State) {
		previewEvents.Dispatch(state)
	})
	previewLauncher := bindings.NewPreviewLauncherBindingsWithDependencies(
		server.ID,
		previewManager.Launch,
		previewManager.Focus,
		previewManager.IsOpen,
	)
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
	runErr := wails.Run(newExplorerOptions(
		assets,
		explorer,
		previewLauncher,
		middleware,
		window,
		server.Name,
		server.ID,
		previewEvents.Close,
		previewManager.Shutdown,
	))
	previewEvents.Close()
	close(finished)
	return runErr
}

func shutdownExplorer(
	parent context.Context,
	previewTimeout time.Duration,
	stopPreviewEvents func(),
	shutdownPreviews func(context.Context) error,
	shutdownApplication func(context.Context) error,
) error {
	shutdownContext := context.Background()
	if parent != nil {
		shutdownContext = context.WithoutCancel(parent)
	}
	if stopPreviewEvents != nil {
		stopPreviewEvents()
	}

	var previewErr error
	if shutdownPreviews != nil {
		previewContext, cancel := context.WithTimeout(shutdownContext, previewTimeout)
		previewErr = shutdownPreviews(previewContext)
		cancel()
	}

	var applicationErr error
	if shutdownApplication != nil {
		applicationErr = shutdownApplication(shutdownContext)
	}
	return errors.Join(previewErr, applicationErr)
}

func newExplorerOptions(
	assets fs.FS,
	explorer *bindings.ExplorerBindings,
	previewLauncher *bindings.PreviewLauncherBindings,
	middleware assetserver.Middleware,
	window *appwindow.Controller,
	serverName,
	serverID string,
	stopPreviewEvents func(),
	shutdownPreviews func(context.Context) error,
) *options.App {
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
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Bind: []interface{}{explorer, previewLauncher},
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
			if err := shutdownExplorer(ctx, previewShutdownTimeout, stopPreviewEvents, shutdownPreviews, explorer.Shutdown); err != nil {
				log.Printf("shutdown explorer: %v", err)
			}
		},
	}
}
