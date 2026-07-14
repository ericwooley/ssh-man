package bindings

import (
	"context"
	"testing"

	appwindow "ssh-man/internal/app/window"
)

type fakeWindowRuntime struct {
	hideCalls int
	quitCalls int
}

func (f *fakeWindowRuntime) Hide(context.Context) {
	f.hideCalls++
}

func (f *fakeWindowRuntime) Show(context.Context) {}

func (f *fakeWindowRuntime) Quit(context.Context) {
	f.quitCalls++
}

func TestWindowBindingsRouteThroughLifecycleController(t *testing.T) {
	runtime := &fakeWindowRuntime{}
	window := appwindow.NewWithRuntime(runtime)
	window.SetContext(context.Background())
	app := &AppBindings{window: window}

	if err := app.HideWindow(); err != nil {
		t.Fatalf("HideWindow() error = %v", err)
	}
	if err := app.Quit(); err != nil {
		t.Fatalf("Quit() error = %v", err)
	}

	if runtime.hideCalls != 1 {
		t.Fatalf("hide calls = %d, want 1", runtime.hideCalls)
	}
	if runtime.quitCalls != 1 {
		t.Fatalf("quit calls = %d, want 1", runtime.quitCalls)
	}
}

func TestWindowBindingsQueueQuitBeforeStartup(t *testing.T) {
	runtime := &fakeWindowRuntime{}
	window := appwindow.NewWithRuntime(runtime)
	app := &AppBindings{window: window}

	if err := app.HideWindow(); err == nil {
		t.Fatal("HideWindow() error = nil, want lifecycle readiness error")
	}
	if err := app.Quit(); err != nil {
		t.Fatalf("Quit() error = %v, want queued success", err)
	}
	if runtime.quitCalls != 0 {
		t.Fatalf("quit calls before startup = %d, want 0", runtime.quitCalls)
	}

	window.SetContext(context.Background())
	if runtime.quitCalls != 1 {
		t.Fatalf("quit calls after startup = %d, want 1", runtime.quitCalls)
	}
}
