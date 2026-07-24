package runner

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-man/internal/app/bindings"
	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	"ssh-man/internal/control"
	preferencesdomain "ssh-man/internal/domain/preferences"
	"ssh-man/internal/platform/defaultbrowser"
	"ssh-man/internal/platform/paths"
)

const settingsOwnerRequestTimeout = 5 * time.Second

func RunSettings(assets fs.FS) error {
	application, err := bootstrap.New(context.Background())
	if err != nil {
		return fmt.Errorf("bootstrap settings: %w", err)
	}
	window := appwindow.New()
	app := bindings.NewAppBindingsWithApplication(application, window)
	marker := bindings.NewSettingsWindowBindings()
	owner := control.NewClient(paths.ControlSocketPath(application.ConfigDir), settingsOwnerRequestTimeout)
	app.SetPreferencesSaver(func(input preferencesdomain.UserPreference) (preferencesdomain.UserPreference, error) {
		ctx, cancel := context.WithTimeout(context.Background(), settingsOwnerRequestTimeout)
		defer cancel()
		var saved preferencesdomain.UserPreference
		if err := owner.Call(ctx, control.Request{Command: "preferences.save", Preferences: &input}, &saved); err != nil {
			return preferencesdomain.UserPreference{}, fmt.Errorf("save settings through SSH Man: %w", err)
		}
		return saved, nil
	})
	app.SetDefaultBrowserSetter(func() (defaultbrowser.Status, error) {
		ctx, cancel := context.WithTimeout(context.Background(), settingsOwnerRequestTimeout)
		defer cancel()
		var status defaultbrowser.Status
		if err := owner.Call(ctx, control.Request{Command: "browser.default.set"}, &status); err != nil {
			return defaultbrowser.Status{}, fmt.Errorf("set default browser through SSH Man: %w", err)
		}
		return status, nil
	})
	// The browser customizer is rendered inline in this regular window.
	app.SetBrowserSwitcherPresenter(func() bool { return true })

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	finished := make(chan struct{})
	go func() {
		select {
		case <-signalContext.Done():
			if err := window.Quit(); err != nil {
				log.Printf("quit settings after parent shutdown: %v", err)
			}
		case <-finished:
		}
	}()
	runErr := wails.Run(newSettingsOptions(assets, app, marker, window))
	close(finished)
	return runErr
}

func newSettingsOptions(assets fs.FS, app *bindings.AppBindings, marker *bindings.SettingsWindowBindings, window *appwindow.Controller) *options.App {
	return &options.App{
		Title:             "SSH Man Settings",
		Width:             900,
		Height:            760,
		MinWidth:          680,
		MinHeight:         560,
		Frameless:         false,
		DisableResize:     false,
		StartHidden:       false,
		HideWindowOnClose: false,
		AlwaysOnTop:       false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []interface{}{app, marker},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID + ".settings",
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
			app.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			if err := app.Shutdown(ctx); err != nil {
				log.Printf("shutdown settings: %v", err)
			}
		},
	}
}
