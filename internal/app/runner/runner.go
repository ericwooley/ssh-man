package runner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"ssh-man/internal/app/bindings"
	"ssh-man/internal/app/bootstrap"
	appmenu "ssh-man/internal/app/menu"
	appwindow "ssh-man/internal/app/window"
	"ssh-man/internal/control"
	"ssh-man/internal/platform/menubar"
	"ssh-man/internal/platform/paths"
)

const (
	singleInstanceID    = "com.wails.ssh-man"
	ownerStartupTimeout = 5 * time.Second
	shutdownTimeout     = 15 * time.Second
)

type menuBar interface {
	Start() error
	Show() bool
	Stop()
}

type controlLifecycle interface {
	Start() error
	Stop(context.Context) error
}

type ownerLease interface {
	Release() error
}

type ownershipDependencies struct {
	configDir    func() (string, error)
	acquireOwner func(string) (ownerLease, error)
	bootstrap    func(context.Context) (*bootstrap.Application, error)
	showExisting func(context.Context, string) error
}

type applicationLifecycle struct {
	control             controlLifecycle
	bar                 menuBar
	startOnLaunch       func(context.Context) error
	shutdownApplication func(context.Context) error

	startOnLaunchOnce sync.Once
	startOnLaunchWait sync.WaitGroup
	startOnLaunchStop context.CancelFunc
	shutdownOnce      sync.Once
	shutdownErr       error
}

func defaultOwnershipDependencies() ownershipDependencies {
	return ownershipDependencies{
		configDir: paths.ConfigDir,
		acquireOwner: func(path string) (ownerLease, error) {
			return control.AcquireOwner(path)
		},
		bootstrap: bootstrap.New,
		showExisting: func(ctx context.Context, socketPath string) error {
			return showExistingOwner(ctx, socketPath)
		},
	}
}

func prepareOwnedApplication(ctx context.Context, dependencies ownershipDependencies) (*bootstrap.Application, ownerLease, bool, error) {
	configDir, err := dependencies.configDir()
	if err != nil {
		return nil, nil, false, err
	}

	lease, err := dependencies.acquireOwner(paths.OwnerLockPath(configDir))
	if err != nil {
		if errors.Is(err, control.ErrOwnerActive) {
			if showErr := dependencies.showExisting(ctx, paths.ControlSocketPath(configDir)); showErr != nil {
				return nil, nil, false, fmt.Errorf("show the running SSH Man application: %w", showErr)
			}
			return nil, nil, true, nil
		}
		return nil, nil, false, fmt.Errorf("acquire controller ownership: %w", err)
	}

	application, err := dependencies.bootstrap(ctx)
	if err != nil {
		return nil, nil, false, errors.Join(
			fmt.Errorf("bootstrap application: %w", err),
			lease.Release(),
		)
	}
	return application, lease, false, nil
}

func showExistingOwner(parent context.Context, socketPath string) error {
	ctx, cancel := context.WithTimeout(parent, ownerStartupTimeout)
	defer cancel()

	client := control.NewClient(socketPath, ownerStartupTimeout)
	if err := client.Wait(ctx); err != nil {
		return err
	}
	if err := client.Call(ctx, control.Request{Command: "app.show"}, nil); err != nil {
		return fmt.Errorf("request app.show: %w", err)
	}
	return nil
}

func newApplicationLifecycle(controlServer controlLifecycle, bar menuBar, startOnLaunch func(context.Context) error, shutdownApplication func(context.Context) error) *applicationLifecycle {
	return &applicationLifecycle{
		control:             controlServer,
		bar:                 bar,
		startOnLaunch:       startOnLaunch,
		shutdownApplication: shutdownApplication,
	}
}

func (l *applicationLifecycle) Start() error {
	if l == nil || l.control == nil {
		return nil
	}
	return l.control.Start()
}

func (l *applicationLifecycle) StartConfiguredTunnels() {
	if l == nil || l.startOnLaunch == nil {
		return
	}
	l.startOnLaunchOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		l.startOnLaunchStop = cancel
		l.startOnLaunchWait.Add(1)
		go func() {
			defer l.startOnLaunchWait.Done()
			if err := l.startOnLaunch(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("start configured tunnels: %v", err)
			}
		}()
	})
}

func (l *applicationLifecycle) Shutdown(parent context.Context) error {
	if l == nil {
		return nil
	}

	l.shutdownOnce.Do(func() {
		ctx := context.Background()
		if parent != nil {
			ctx = context.WithoutCancel(parent)
		}
		ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
		defer cancel()
		if l.startOnLaunchStop != nil {
			l.startOnLaunchStop()
		}
		l.startOnLaunchWait.Wait()

		if l.bar != nil {
			l.bar.Stop()
		}

		var controlErr error
		if l.control != nil {
			controlErr = l.control.Stop(ctx)
		}
		var applicationErr error
		if l.shutdownApplication != nil {
			applicationErr = l.shutdownApplication(ctx)
		}
		l.shutdownErr = errors.Join(controlErr, applicationErr)
	})
	return l.shutdownErr
}

func showApplication(bar menuBar, window *appwindow.Controller) error {
	if bar != nil && bar.Show() {
		return nil
	}
	window.ShowWhenReady()
	return nil
}

func Run(assets fs.FS) (runErr error) {
	if handled, err := maybeRunBindingsGeneration(assets); handled {
		return err
	}
	application, lease, handled, err := prepareOwnedApplication(context.Background(), defaultOwnershipDependencies())
	if err != nil || handled {
		return err
	}

	window := appwindow.New()
	app := bindings.NewAppBindingsWithApplication(application, window)
	explorerLauncher := bindings.NewExplorerLauncherBindings(application)

	var bar menuBar
	bar = menubar.New(menubar.Callbacks{
		Quit: func() {
			bar.Stop()
			if err := window.Quit(); err != nil {
				log.Printf("quit from menu bar: %v", err)
			}
		},
	})
	show := func() error {
		return showApplication(bar, window)
	}
	controlServer := control.NewServer(
		paths.ControlSocketPath(application.ConfigDir),
		newControlBackend(application, window, show),
	)
	lifecycle := newApplicationLifecycle(controlServer, bar, application.SessionService.StartOnLaunch, app.Shutdown)
	defer func() {
		cleanupErr := lifecycle.Shutdown(context.Background())
		releaseErr := lease.Release()
		runErr = errors.Join(runErr, cleanupErr, releaseErr)
	}()
	if err := lifecycle.Start(); err != nil {
		return fmt.Errorf("start control service: %w", err)
	}

	runErr = wails.Run(newOptions(assets, app, explorerLauncher, window, bar, lifecycle))
	return runErr
}

func newOptions(assets fs.FS, app *bindings.AppBindings, explorerLauncher *bindings.ExplorerLauncherBindings, window *appwindow.Controller, bar menuBar, lifecycle *applicationLifecycle) *options.App {
	appOptions := &options.App{
		Title:  "SSH Man",
		Width:  420,
		Height: 720,
		Menu:   appmenu.Build(app, window),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: append([]interface{}{app, explorerLauncher}, additionalBindingsForGeneration()...),
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				_ = showApplication(bar, window)
			},
		},
		OnStartup: func(ctx context.Context) {
			app.SetContext(ctx)
			lifecycle.StartConfiguredTunnels()
		},
		OnDomReady: func(ctx context.Context) {
			// Set the context again to make this hook self-contained if lifecycle
			// scheduling changes in a future Wails release.
			app.SetContext(ctx)
			if err := bar.Start(); err != nil {
				log.Printf("start menu-bar integration: %v", err)
				if showErr := window.Show(); showErr != nil {
					log.Printf("show fallback application window: %v", showErr)
				}
			}
		},
		OnBeforeClose: func(context.Context) bool {
			// Wails releases its native window before OnShutdown. Remove the
			// status-item targets while the AppKit objects are still valid.
			bar.Stop()
			return false
		},
		OnShutdown: func(ctx context.Context) {
			if err := lifecycle.Shutdown(ctx); err != nil {
				log.Printf("shutdown application: %v", err)
			}
		},
	}

	applyPlatformWindowOptions(appOptions)
	return appOptions
}
