package runner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-man/internal/app/bindings"
	"ssh-man/internal/app/bootstrap"
	"ssh-man/internal/app/explorerwindow"
	appmenu "ssh-man/internal/app/menu"
	appwindow "ssh-man/internal/app/window"
	"ssh-man/internal/control"
	"ssh-man/internal/platform/menubar"
	"ssh-man/internal/platform/paths"
)

const (
	singleInstanceID               = "tech.moonpixels.ssh-man"
	ownerStartupTimeout            = 5 * time.Second
	explorerShutdownTimeout        = 5 * time.Second
	shutdownTimeout                = 15 * time.Second
	browserSwitchEventQueueSize    = 64
	browserSwitcherOpenEventName   = "browser-switcher:open"
	browserSwitcherCommitEventName = "browser-switcher:commit"
	browserSwitcherCancelEventName = "browser-switcher:cancel"
)

type menuBar interface {
	Start() error
	Show() bool
	ShowBrowserSwitcher() bool
	CancelBrowserSwitchSession()
	SetBrowserShortcuts(string, string) error
	Stop()
}

type browserSwitchEventKind uint8

const (
	browserSwitchAdvance browserSwitchEventKind = iota + 1
	browserSwitchCommit
	browserSwitchCancel
)

type browserSwitchEvent struct {
	kind      browserSwitchEventKind
	direction menubar.BrowserSwitchDirection
	sessionID uint64
}

type browserSwitchEventDispatcher struct {
	mu            sync.Mutex
	emitMu        sync.Mutex
	ready         *sync.Cond
	events        []browserSwitchEvent
	done          chan struct{}
	emit          func(browserSwitchEvent)
	stopRequested atomic.Bool
	closeOnce     sync.Once
}

func newBrowserSwitchEventDispatcher(emit func(browserSwitchEvent)) *browserSwitchEventDispatcher {
	dispatcher := &browserSwitchEventDispatcher{
		events: make([]browserSwitchEvent, 0, browserSwitchEventQueueSize),
		done:   make(chan struct{}),
		emit:   emit,
	}
	dispatcher.ready = sync.NewCond(&dispatcher.mu)
	go dispatcher.run()
	return dispatcher
}

func (d *browserSwitchEventDispatcher) Dispatch(direction menubar.BrowserSwitchDirection, sessionID uint64) bool {
	return d.dispatch(browserSwitchEvent{kind: browserSwitchAdvance, direction: direction, sessionID: sessionID}, true)
}

func (d *browserSwitchEventDispatcher) Commit(sessionID uint64) bool {
	return d.dispatch(browserSwitchEvent{kind: browserSwitchCommit, sessionID: sessionID}, false)
}

func (d *browserSwitchEventDispatcher) Cancel(sessionID uint64) bool {
	return d.dispatch(browserSwitchEvent{kind: browserSwitchCancel, sessionID: sessionID}, false)
}

func (d *browserSwitchEventDispatcher) dispatch(event browserSwitchEvent, droppable bool) bool {
	if d == nil || event.sessionID == 0 || d.stopRequested.Load() {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopRequested.Load() {
		return false
	}
	if droppable && len(d.events) >= browserSwitchEventQueueSize {
		return false
	}
	d.events = append(d.events, event)
	d.ready.Signal()
	return true
}

func (d *browserSwitchEventDispatcher) Close() {
	d.closeOnce.Do(func() {
		d.stopRequested.Store(true)
		// Wait for an in-flight emit, then hold the barrier until the queue
		// is discarded. The worker rechecks stopRequested behind this lock.
		d.emitMu.Lock()
		d.mu.Lock()
		d.events = nil
		d.ready.Broadcast()
		d.mu.Unlock()
		d.emitMu.Unlock()
		<-d.done
	})
}

func (d *browserSwitchEventDispatcher) run() {
	defer close(d.done)
	for {
		d.mu.Lock()
		for len(d.events) == 0 && !d.stopRequested.Load() {
			d.ready.Wait()
		}
		if d.stopRequested.Load() {
			d.events = nil
			d.mu.Unlock()
			return
		}

		event := d.events[0]
		copy(d.events, d.events[1:])
		d.events = d.events[:len(d.events)-1]
		d.mu.Unlock()

		d.emitMu.Lock()
		if d.stopRequested.Load() {
			d.emitMu.Unlock()
			return
		}
		if d.emit != nil {
			d.emit(event)
		}
		d.emitMu.Unlock()
	}
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
	shutdownExplorers   func(context.Context) error
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

func newApplicationLifecycle(controlServer controlLifecycle, bar menuBar, startOnLaunch func(context.Context) error, shutdownExplorers func(context.Context) error, shutdownApplication func(context.Context) error) *applicationLifecycle {
	return &applicationLifecycle{
		control:             controlServer,
		bar:                 bar,
		startOnLaunch:       startOnLaunch,
		shutdownExplorers:   shutdownExplorers,
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
		var explorerErr error
		if l.shutdownExplorers != nil {
			explorerContext, cancelExplorers := context.WithTimeout(ctx, explorerShutdownTimeout)
			explorerErr = l.shutdownExplorers(explorerContext)
			cancelExplorers()
		}
		var applicationErr error
		if l.shutdownApplication != nil {
			applicationErr = l.shutdownApplication(ctx)
		}
		l.shutdownErr = errors.Join(controlErr, explorerErr, applicationErr)
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
	explorerManager := explorerwindow.NewManager()
	explorerLauncher := bindings.NewExplorerLauncherBindingsWithDependencies(application.ServerService, explorerManager.Launch)
	browserSwitchEvents := newBrowserSwitchEventDispatcher(func(event browserSwitchEvent) {
		ctx, contextErr := window.Context()
		if contextErr == nil {
			// Wails' Darwin JavaScript bridge must be entered from a Go
			// goroutine. Calling it directly from Carbon's AppKit callback can
			// over-release the bridge's dispatch block and crash in objc_release.
			payload := map[string]any{"sessionId": event.sessionID}
			switch event.kind {
			case browserSwitchAdvance:
				payload["direction"] = string(event.direction)
				wailsruntime.EventsEmit(ctx, browserSwitcherOpenEventName, payload)
			case browserSwitchCommit:
				wailsruntime.EventsEmit(ctx, browserSwitcherCommitEventName, payload)
			case browserSwitchCancel:
				wailsruntime.EventsEmit(ctx, browserSwitcherCancelEventName, payload)
			}
		}
	})

	var bar menuBar
	bar = menubar.New(menubar.Callbacks{
		Quit: func() {
			bar.Stop()
			if err := window.Quit(); err != nil {
				log.Printf("quit from menu bar: %v", err)
			}
		},
		SwitchBrowsers: func(direction menubar.BrowserSwitchDirection, sessionID uint64) {
			if !bar.ShowBrowserSwitcher() {
				bar.CancelBrowserSwitchSession()
				return
			}
			if !browserSwitchEvents.Dispatch(direction, sessionID) {
				log.Printf("browser switcher event queue is unavailable")
				bar.CancelBrowserSwitchSession()
			}
		},
		CommitBrowserSwitch: func(sessionID uint64) {
			if !browserSwitchEvents.Commit(sessionID) {
				log.Printf("browser switcher commit event queue is unavailable")
			}
		},
		CancelBrowserSwitch: func(sessionID uint64) {
			if !browserSwitchEvents.Cancel(sessionID) {
				log.Printf("browser switcher cancel event queue is unavailable")
			}
		},
	})
	app.SetBrowserShortcutsRegistrar(bar.SetBrowserShortcuts)
	app.SetBrowserSwitcherPresenter(bar.ShowBrowserSwitcher)
	show := func() error {
		return showApplication(bar, window)
	}
	controlServer := control.NewServer(
		paths.ControlSocketPath(application.ConfigDir),
		newControlBackend(application, window, show),
	)
	lifecycle := newApplicationLifecycle(controlServer, bar, application.SessionService.StartOnLaunch, explorerManager.Shutdown, app.Shutdown)
	defer func() {
		bar.Stop()
		browserSwitchEvents.Close()
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
				return
			}
			if shortcutErr := app.RegisterBrowserShortcuts(); shortcutErr != nil {
				log.Printf("register browser switcher shortcuts: %v", shortcutErr)
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
