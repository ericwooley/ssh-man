package window

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type fakeRuntime struct {
	mu     sync.Mutex
	hidden context.Context
	shown  context.Context
	quit   context.Context
	quits  int
}

func (f *fakeRuntime) Hide(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hidden = ctx
}

func (f *fakeRuntime) Show(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = ctx
}

func (f *fakeRuntime) Quit(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quit = ctx
	f.quits++
}

func (f *fakeRuntime) snapshot() (context.Context, context.Context, context.Context, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hidden, f.shown, f.quit, f.quits
}

func (f *fakeRuntime) resetShown() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = nil
}

func TestControllerRequiresLifecycleContext(t *testing.T) {
	controller := NewWithRuntime(&fakeRuntime{})

	for name, call := range map[string]func() error{
		"hide": controller.Hide,
		"show": controller.Show,
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ErrNotReady) {
				t.Fatalf("error = %v, want %v", err, ErrNotReady)
			}
		})
	}
}

func TestControllerQuitRejectsMissingControllerOrRuntime(t *testing.T) {
	var missingController *Controller
	if err := missingController.Quit(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("nil controller Quit() error = %v, want %v", err, ErrNotReady)
	}

	controller := NewWithRuntime(nil)
	if err := controller.Quit(); !errors.Is(err, ErrRuntimeUnavailable) {
		t.Fatalf("missing runtime Quit() error = %v, want %v", err, ErrRuntimeUnavailable)
	}
}

func TestControllerRoutesOperationsThroughInjectedRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	controller := NewWithRuntime(runtime)
	ctx := context.WithValue(context.Background(), struct{}{}, "lifecycle")
	controller.SetContext(ctx)

	if err := controller.Hide(); err != nil {
		t.Fatalf("Hide() error = %v", err)
	}
	if err := controller.Show(); err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if err := controller.Quit(); err != nil {
		t.Fatalf("Quit() error = %v", err)
	}

	hidden, shown, quit, _ := runtime.snapshot()
	if hidden != ctx {
		t.Fatal("Hide() did not receive the lifecycle context")
	}
	if shown != ctx {
		t.Fatal("Show() did not receive the lifecycle context")
	}
	if quit != ctx {
		t.Fatal("Quit() did not receive the lifecycle context")
	}
}

func TestControllerDefersEarlyShowUntilStartup(t *testing.T) {
	runtime := &fakeRuntime{}
	controller := NewWithRuntime(runtime)
	controller.ShowWhenReady()
	_, shown, _, _ := runtime.snapshot()
	if shown != nil {
		t.Fatal("ShowWhenReady() ran before a lifecycle context was available")
	}

	ctx := context.Background()
	controller.SetContext(ctx)
	_, shown, _, _ = runtime.snapshot()
	if shown != ctx {
		t.Fatal("SetContext() did not deliver the deferred show request")
	}

	runtime.resetShown()
	controller.SetContext(ctx)
	_, shown, _, _ = runtime.snapshot()
	if shown != nil {
		t.Fatal("deferred show request ran more than once")
	}
}

func TestControllerDefersEarlyQuitUntilStartup(t *testing.T) {
	runtime := &fakeRuntime{}
	controller := NewWithRuntime(runtime)

	if err := controller.Quit(); err != nil {
		t.Fatalf("Quit() error = %v", err)
	}
	_, _, quit, quits := runtime.snapshot()
	if quit != nil || quits != 0 {
		t.Fatalf("Quit() ran before startup: context=%v calls=%d", quit, quits)
	}

	ctx := context.WithValue(context.Background(), struct{}{}, "lifecycle")
	controller.SetContext(ctx)
	_, _, quit, quits = runtime.snapshot()
	if quit != ctx || quits != 1 {
		t.Fatalf("deferred Quit() context=%v calls=%d, want context and one call", quit, quits)
	}

	controller.SetContext(ctx)
	if err := controller.Quit(); err != nil {
		t.Fatalf("repeated Quit() error = %v", err)
	}
	_, _, _, quits = runtime.snapshot()
	if quits != 1 {
		t.Fatalf("Quit() calls = %d, want exactly one", quits)
	}
}

func TestControllerEarlyQuitSupersedesDeferredShow(t *testing.T) {
	runtime := &fakeRuntime{}
	controller := NewWithRuntime(runtime)
	controller.ShowWhenReady()
	if err := controller.Quit(); err != nil {
		t.Fatalf("Quit() error = %v", err)
	}

	ctx := context.Background()
	controller.SetContext(ctx)
	_, shown, quit, quits := runtime.snapshot()
	if shown != nil {
		t.Fatal("deferred ShowWhenReady() ran while quitting")
	}
	if quit != ctx || quits != 1 {
		t.Fatalf("Quit() context=%v calls=%d", quit, quits)
	}
}

func TestControllerConcurrentQuitAndStartupDispatchExactlyOnce(t *testing.T) {
	runtime := &fakeRuntime{}
	controller := NewWithRuntime(runtime)
	start := make(chan struct{})
	const quitters = 32

	var wait sync.WaitGroup
	wait.Add(quitters + 1)
	for range quitters {
		go func() {
			defer wait.Done()
			<-start
			if err := controller.Quit(); err != nil {
				t.Errorf("Quit() error = %v", err)
			}
		}()
	}
	ctx := context.Background()
	go func() {
		defer wait.Done()
		<-start
		controller.SetContext(ctx)
	}()

	close(start)
	wait.Wait()
	_, _, quit, quits := runtime.snapshot()
	if quit != ctx || quits != 1 {
		t.Fatalf("Quit() context=%v calls=%d, want exactly one", quit, quits)
	}
}
