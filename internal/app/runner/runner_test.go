package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"

	"ssh-man/internal/app/bindings"
	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	"ssh-man/internal/control"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/platform/menubar"
)

type fakeWindowRuntime struct {
	showCalls int
}

func (*fakeWindowRuntime) Hide(context.Context) {}
func (f *fakeWindowRuntime) Show(context.Context) {
	f.showCalls++
}
func (*fakeWindowRuntime) Quit(context.Context) {}

type fakeMenuBar struct {
	showResult bool
	startErr   error
	showCalls  int
	startCalls int
	stopCalls  int
	events     *[]string
}

func (f *fakeMenuBar) Start() error {
	f.startCalls++
	return f.startErr
}
func (f *fakeMenuBar) Show() bool {
	f.showCalls++
	return f.showResult
}

func (f *fakeMenuBar) ShowBrowserSwitcher() bool {
	return f.Show()
}

func (f *fakeMenuBar) CancelBrowserSwitchSession() {}

func (f *fakeMenuBar) SetBrowserShortcuts(string, string) error {
	return nil
}
func (f *fakeMenuBar) Stop() {
	f.stopCalls++
	if f.events != nil {
		*f.events = append(*f.events, "menu.stop")
	}
}

type fakeControlLifecycle struct {
	startErr   error
	stopErr    error
	startCalls int
	stopCalls  int
	events     *[]string
}

func (f *fakeControlLifecycle) Start() error {
	f.startCalls++
	if f.events != nil {
		*f.events = append(*f.events, "control.start")
	}
	return f.startErr
}

func (f *fakeControlLifecycle) Stop(context.Context) error {
	f.stopCalls++
	if f.events != nil {
		*f.events = append(*f.events, "control.stop")
	}
	return f.stopErr
}

type fakeOwnerLease struct {
	releaseCalls int
	releaseErr   error
	events       *[]string
}

func (f *fakeOwnerLease) Release() error {
	f.releaseCalls++
	if f.events != nil {
		*f.events = append(*f.events, "owner.release")
	}
	return f.releaseErr
}

func testLifecycle(bar menuBar) *applicationLifecycle {
	return newApplicationLifecycle(&fakeControlLifecycle{}, bar, nil, nil, func(context.Context) error { return nil })
}

func testLauncher() *bindings.ExplorerLauncherBindings {
	return bindings.NewExplorerLauncherBindingsWithDependencies(nil, nil)
}

func TestBrowserSwitchEventDispatcherPreservesSessionOrder(t *testing.T) {
	received := make(chan browserSwitchEvent, 5)
	dispatcher := newBrowserSwitchEventDispatcher(func(event browserSwitchEvent) {
		received <- event
	})
	t.Cleanup(dispatcher.Close)

	want := []browserSwitchEvent{
		{kind: browserSwitchAdvance, direction: menubar.BrowserSwitchForward, sessionID: 41},
		{kind: browserSwitchAdvance, direction: menubar.BrowserSwitchBackward, sessionID: 41},
		{kind: browserSwitchCommit, sessionID: 41},
		{kind: browserSwitchAdvance, direction: menubar.BrowserSwitchForward, sessionID: 42},
		{kind: browserSwitchCancel, sessionID: 42},
	}
	if !dispatcher.Dispatch(menubar.BrowserSwitchForward, 41) {
		t.Fatal("forward event rejected")
	}
	if !dispatcher.Dispatch(menubar.BrowserSwitchBackward, 41) {
		t.Fatal("backward event rejected")
	}
	if !dispatcher.Commit(41) {
		t.Fatal("commit event rejected")
	}
	if !dispatcher.Dispatch(menubar.BrowserSwitchForward, 42) {
		t.Fatal("next-session forward event rejected")
	}
	if !dispatcher.Cancel(42) {
		t.Fatal("cancel event rejected")
	}

	got := make([]browserSwitchEvent, 0, len(want))
	for range want {
		select {
		case event := <-received:
			got = append(got, event)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for browser switch event")
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
}

func TestBrowserSwitchEventDispatcherRejectsAfterClose(t *testing.T) {
	dispatcher := newBrowserSwitchEventDispatcher(func(browserSwitchEvent) {})
	dispatcher.Close()
	if dispatcher.Dispatch(menubar.BrowserSwitchForward, 1) {
		t.Fatal("expected dispatcher to reject events after close")
	}
	if dispatcher.Commit(1) {
		t.Fatal("expected dispatcher to reject commit after close")
	}
	if dispatcher.Cancel(1) {
		t.Fatal("expected dispatcher to reject cancel after close")
	}
}

func TestBrowserSwitchEventDispatcherRejectsZeroSessionID(t *testing.T) {
	dispatcher := newBrowserSwitchEventDispatcher(func(browserSwitchEvent) {})
	t.Cleanup(dispatcher.Close)

	if dispatcher.Dispatch(menubar.BrowserSwitchForward, 0) {
		t.Fatal("expected dispatcher to reject advance without a session")
	}
	if dispatcher.Commit(0) {
		t.Fatal("expected dispatcher to reject commit without a session")
	}
	if dispatcher.Cancel(0) {
		t.Fatal("expected dispatcher to reject cancel without a session")
	}
}

func TestBrowserSwitchEventDispatcherKeepsTerminalEventsWhenAdvanceQueueIsFull(t *testing.T) {
	const sessionID = uint64(77)
	started := make(chan struct{})
	release := make(chan struct{})
	received := make(chan browserSwitchEvent, browserSwitchEventQueueSize+3)
	first := true
	dispatcher := newBrowserSwitchEventDispatcher(func(event browserSwitchEvent) {
		if first {
			first = false
			close(started)
			<-release
		}
		received <- event
	})
	t.Cleanup(dispatcher.Close)

	if !dispatcher.Dispatch(menubar.BrowserSwitchForward, sessionID) {
		t.Fatal("initial advance rejected")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for emitter to block")
	}

	for i := 0; i < browserSwitchEventQueueSize; i++ {
		if !dispatcher.Dispatch(menubar.BrowserSwitchForward, sessionID) {
			t.Fatalf("advance %d rejected before queue reached capacity", i)
		}
	}
	if dispatcher.Dispatch(menubar.BrowserSwitchForward, sessionID) {
		t.Fatal("expected saturated advance queue to reject another advance")
	}
	if !dispatcher.Commit(sessionID) {
		t.Fatal("commit must remain non-droppable when advance queue is full")
	}
	if !dispatcher.Cancel(sessionID) {
		t.Fatal("cancel must remain non-droppable when advance queue is full")
	}
	close(release)

	wantCount := browserSwitchEventQueueSize + 3
	got := make([]browserSwitchEvent, 0, wantCount)
	for len(got) < wantCount {
		select {
		case event := <-received:
			got = append(got, event)
		case <-time.After(time.Second):
			t.Fatalf("timed out after %d dispatched events", len(got))
		}
	}
	if got[len(got)-2] != (browserSwitchEvent{kind: browserSwitchCommit, sessionID: sessionID}) {
		t.Fatalf("penultimate event = %#v, want commit", got[len(got)-2])
	}
	if got[len(got)-1] != (browserSwitchEvent{kind: browserSwitchCancel, sessionID: sessionID}) {
		t.Fatalf("last event = %#v, want cancel", got[len(got)-1])
	}
}

func TestBrowserSwitchEventDispatcherCloseDiscardsQueuedEvents(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	received := make(chan browserSwitchEvent, 2)
	first := true
	dispatcher := newBrowserSwitchEventDispatcher(func(event browserSwitchEvent) {
		if first {
			first = false
			close(started)
			<-release
		}
		received <- event
	})

	if !dispatcher.Dispatch(menubar.BrowserSwitchForward, 91) {
		t.Fatal("initial advance rejected")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for in-flight emitter")
	}
	if !dispatcher.Commit(91) {
		t.Fatal("queued commit rejected")
	}

	closed := make(chan struct{})
	go func() {
		dispatcher.Close()
		close(closed)
	}()
	deadline := time.Now().Add(time.Second)
	for !dispatcher.stopRequested.Load() && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if !dispatcher.stopRequested.Load() {
		t.Fatal("timed out waiting for dispatcher stop request")
	}
	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dispatcher close")
	}

	if len(received) != 1 {
		t.Fatalf("emitted events after close = %d, want only the in-flight event", len(received))
	}
}

func TestNewOptionsConfiguresCompactSingleInstanceApp(t *testing.T) {
	window := appwindow.NewWithRuntime(&fakeWindowRuntime{})
	app := &bindings.AppBindings{}
	bar := &fakeMenuBar{}
	launcher := testLauncher()
	got := newOptions(nil, app, launcher, window, bar, testLifecycle(bar))

	if got.Title != "SSH Man" {
		t.Fatalf("Title = %q, want SSH Man", got.Title)
	}
	if got.Width != 420 || got.Height != 720 {
		t.Fatalf("size = %dx%d, want 420x720", got.Width, got.Height)
	}
	if got.SingleInstanceLock == nil || got.SingleInstanceLock.UniqueId != singleInstanceID {
		t.Fatalf("SingleInstanceLock = %#v, want ID %q", got.SingleInstanceLock, singleInstanceID)
	}
	if got.OnStartup == nil || got.OnDomReady == nil || got.OnBeforeClose == nil || got.OnShutdown == nil {
		t.Fatal("expected complete Wails lifecycle hooks")
	}
	wantBindingCount := 2 + len(additionalBindingsForGeneration())
	if len(got.Bind) != wantBindingCount || got.Bind[0] != app || got.Bind[1] != launcher {
		t.Fatalf("Bind = %#v, want application bindings", got.Bind)
	}
}

func TestNewExplorerOptionsConfiguresIndependentResizableWindow(t *testing.T) {
	window := appwindow.NewWithRuntime(&fakeWindowRuntime{})
	explorer, middleware := bindings.NewExplorerBindings(&bootstrap.Application{}, serverdomain.Server{ID: "server-1", Name: "Production"}, window)

	got := newExplorerOptions(nil, explorer, middleware, window, "Production", "server-1")

	if got.Title != "Production — SSH Man Explorer" || got.Width != 1180 || got.Height != 760 {
		t.Fatalf("explorer window = %q %dx%d", got.Title, got.Width, got.Height)
	}
	if got.DisableResize || got.HideWindowOnClose || got.AlwaysOnTop {
		t.Fatal("explorer should be an ordinary persistent OS window")
	}
	if got.SingleInstanceLock == nil || got.SingleInstanceLock.UniqueId != singleInstanceID+".explorer.server-1" {
		t.Fatalf("explorer lock = %#v", got.SingleInstanceLock)
	}
	if len(got.Bind) != 1 || got.Bind[0] != explorer {
		t.Fatalf("explorer bindings = %#v", got.Bind)
	}
}

func TestSecondInstanceUsesNativeMenuBarWhenAvailable(t *testing.T) {
	runtime := &fakeWindowRuntime{}
	window := appwindow.NewWithRuntime(runtime)
	bar := &fakeMenuBar{showResult: true}
	got := newOptions(nil, &bindings.AppBindings{}, testLauncher(), window, bar, testLifecycle(bar))

	got.SingleInstanceLock.OnSecondInstanceLaunch(options.SecondInstanceData{})

	if bar.showCalls != 1 {
		t.Fatalf("menu-bar show calls = %d, want 1", bar.showCalls)
	}
	if runtime.showCalls != 0 {
		t.Fatalf("window show calls = %d, want native menu-bar show only", runtime.showCalls)
	}
}

func TestSecondInstanceDefersWindowShowUntilStartupWhenMenuBarUnavailable(t *testing.T) {
	runtime := &fakeWindowRuntime{}
	window := appwindow.NewWithRuntime(runtime)
	bar := &fakeMenuBar{}
	got := newOptions(nil, &bindings.AppBindings{}, testLauncher(), window, bar, testLifecycle(bar))

	got.SingleInstanceLock.OnSecondInstanceLaunch(options.SecondInstanceData{})
	if runtime.showCalls != 0 {
		t.Fatalf("window show calls before startup = %d, want 0", runtime.showCalls)
	}

	window.SetContext(context.Background())
	if runtime.showCalls != 1 {
		t.Fatalf("window show calls after startup = %d, want 1", runtime.showCalls)
	}
}

func TestDomReadyShowsFallbackWindowWhenMenuBarStartFails(t *testing.T) {
	runtime := &fakeWindowRuntime{}
	window := appwindow.NewWithRuntime(runtime)
	window.SetContext(context.Background())
	bar := &fakeMenuBar{startErr: errors.New("native unavailable")}
	got := newOptions(nil, &bindings.AppBindings{}, testLauncher(), window, bar, testLifecycle(bar))

	got.OnDomReady(context.Background())

	if bar.startCalls != 1 {
		t.Fatalf("menu-bar start calls = %d, want 1", bar.startCalls)
	}
	if runtime.showCalls != 1 {
		t.Fatalf("fallback window show calls = %d, want 1", runtime.showCalls)
	}
}

func TestStartupStartsConfiguredTunnelsOnceAndShutdownCancelsIt(t *testing.T) {
	started := make(chan struct{})
	startCalls := 0
	lifecycle := newApplicationLifecycle(
		&fakeControlLifecycle{},
		&fakeMenuBar{},
		func(ctx context.Context) error {
			startCalls++
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
		nil,
		func(context.Context) error { return nil },
	)
	window := appwindow.NewWithRuntime(&fakeWindowRuntime{})
	got := newOptions(nil, &bindings.AppBindings{}, testLauncher(), window, &fakeMenuBar{}, lifecycle)

	got.OnStartup(context.Background())
	got.OnStartup(context.Background())
	<-started
	got.OnShutdown(context.Background())

	if startCalls != 1 {
		t.Fatalf("start-on-launch calls = %d, want 1", startCalls)
	}
}

func TestLifecycleStartsControlAndStopsItBeforeApplication(t *testing.T) {
	events := []string{}
	window := appwindow.NewWithRuntime(&fakeWindowRuntime{})
	bar := &fakeMenuBar{events: &events}
	controlServer := &fakeControlLifecycle{events: &events}
	lifecycle := newApplicationLifecycle(controlServer, bar, func(context.Context) error {
		return nil
	}, func(context.Context) error {
		events = append(events, "explorers.shutdown")
		return nil
	}, func(context.Context) error {
		events = append(events, "application.shutdown")
		return nil
	})
	got := newOptions(nil, &bindings.AppBindings{}, testLauncher(), window, bar, lifecycle)

	if err := lifecycle.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	got.OnStartup(context.Background())
	got.OnShutdown(context.Background())
	got.OnShutdown(context.Background())

	want := []string{"control.start", "menu.stop", "control.stop", "explorers.shutdown", "application.shutdown"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("lifecycle events = %#v, want %#v", events, want)
	}
	if controlServer.startCalls != 1 || controlServer.stopCalls != 1 {
		t.Fatalf("control lifecycle calls = start %d, stop %d; want 1 each", controlServer.startCalls, controlServer.stopCalls)
	}
}

func TestApplicationLifecycleReturnsControlAndApplicationShutdownErrors(t *testing.T) {
	controlErr := errors.New("control stop failed")
	explorerErr := errors.New("explorer shutdown failed")
	applicationErr := errors.New("application shutdown failed")
	lifecycle := newApplicationLifecycle(
		&fakeControlLifecycle{stopErr: controlErr},
		&fakeMenuBar{},
		nil,
		func(context.Context) error { return explorerErr },
		func(context.Context) error { return applicationErr },
	)

	err := lifecycle.Shutdown(context.Background())
	if !errors.Is(err, controlErr) || !errors.Is(err, explorerErr) || !errors.Is(err, applicationErr) {
		t.Fatalf("Shutdown() error = %v, want all shutdown errors", err)
	}
}

func TestApplicationLifecycleShutsDownExplorersBeforeApplicationStorage(t *testing.T) {
	events := []string{}
	lifecycle := newApplicationLifecycle(
		&fakeControlLifecycle{},
		&fakeMenuBar{},
		nil,
		func(context.Context) error {
			events = append(events, "explorers.shutdown")
			return nil
		},
		func(context.Context) error {
			events = append(events, "application.shutdown")
			return nil
		},
	)

	if err := lifecycle.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"explorers.shutdown", "application.shutdown"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("shutdown events = %#v, want %#v", events, want)
	}
}

func TestPrepareOwnedApplicationAcquiresLeaseBeforeBootstrap(t *testing.T) {
	events := []string{}
	lease := &fakeOwnerLease{}
	application := &bootstrap.Application{}
	got, gotLease, handled, err := prepareOwnedApplication(context.Background(), ownershipDependencies{
		configDir: func() (string, error) {
			events = append(events, "config")
			return "/tmp/ssh-man-test", nil
		},
		acquireOwner: func(path string) (ownerLease, error) {
			events = append(events, "owner")
			if path != filepath.Join("/tmp/ssh-man-test", "controller.lock") {
				t.Fatalf("owner path = %q", path)
			}
			return lease, nil
		},
		bootstrap: func(context.Context) (*bootstrap.Application, error) {
			events = append(events, "bootstrap")
			return application, nil
		},
		showExisting: func(context.Context, string) error {
			t.Fatal("showExisting called for the owning process")
			return nil
		},
	})

	if err != nil {
		t.Fatalf("prepareOwnedApplication() error = %v", err)
	}
	if handled || got != application || gotLease != lease {
		t.Fatalf("prepareOwnedApplication() = (%p, %#v, %v), want provided application and lease", got, gotLease, handled)
	}
	want := []string{"config", "owner", "bootstrap"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("startup events = %#v, want %#v", events, want)
	}
}

func TestPrepareOwnedApplicationShowsExistingOwnerWithoutBootstrapping(t *testing.T) {
	bootstrapCalls := 0
	showCalls := 0
	application, lease, handled, err := prepareOwnedApplication(context.Background(), ownershipDependencies{
		configDir: func() (string, error) { return "/tmp/ssh-man-test", nil },
		acquireOwner: func(string) (ownerLease, error) {
			return nil, control.ErrOwnerActive
		},
		bootstrap: func(context.Context) (*bootstrap.Application, error) {
			bootstrapCalls++
			return &bootstrap.Application{}, nil
		},
		showExisting: func(_ context.Context, socketPath string) error {
			showCalls++
			if socketPath != filepath.Join("/tmp/ssh-man-test", "control.sock") {
				t.Fatalf("socket path = %q", socketPath)
			}
			return nil
		},
	})

	if err != nil {
		t.Fatalf("prepareOwnedApplication() error = %v", err)
	}
	if !handled || application != nil || lease != nil {
		t.Fatalf("prepareOwnedApplication() = (%p, %#v, %v), want existing owner handled", application, lease, handled)
	}
	if bootstrapCalls != 0 || showCalls != 1 {
		t.Fatalf("calls = bootstrap %d, show %d; want 0 and 1", bootstrapCalls, showCalls)
	}
}

func TestPrepareOwnedApplicationReleasesLeaseWhenBootstrapFails(t *testing.T) {
	bootstrapErr := errors.New("database unavailable")
	lease := &fakeOwnerLease{}
	_, _, _, err := prepareOwnedApplication(context.Background(), ownershipDependencies{
		configDir: func() (string, error) { return "/tmp/ssh-man-test", nil },
		acquireOwner: func(string) (ownerLease, error) {
			return lease, nil
		},
		bootstrap: func(context.Context) (*bootstrap.Application, error) {
			return nil, bootstrapErr
		},
		showExisting: func(context.Context, string) error { return nil },
	})

	if !errors.Is(err, bootstrapErr) {
		t.Fatalf("prepareOwnedApplication() error = %v, want bootstrap error", err)
	}
	if lease.releaseCalls != 1 {
		t.Fatalf("lease release calls = %d, want 1", lease.releaseCalls)
	}
}

func TestShowExistingOwnerSendsAppShow(t *testing.T) {
	socketDir, err := os.MkdirTemp("/tmp", "ssh-man-")
	if err != nil {
		t.Fatalf("create socket directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socketPath := filepath.Join(socketDir, "control.sock")
	shown := make(chan struct{}, 1)
	server := control.NewServer(socketPath, control.Backend{
		Show: func() error {
			shown <- struct{}{}
			return nil
		},
	})
	if err := server.Start(); err != nil {
		t.Fatalf("start control server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Stop(ctx)
	})

	if err := showExistingOwner(context.Background(), socketPath); err != nil {
		t.Fatalf("showExistingOwner() error = %v", err)
	}
	select {
	case <-shown:
	case <-time.After(time.Second):
		t.Fatal("app.show was not dispatched")
	}
}
