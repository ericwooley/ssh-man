package bindings

import (
	"testing"

	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
)

func TestNewAppBindingsWithApplicationUsesProvidedApplicationAndWindow(t *testing.T) {
	application := &bootstrap.Application{}
	window := appwindow.New()

	got := NewAppBindingsWithApplication(application, window)

	if got.app != application {
		t.Fatal("bindings did not retain the provided application")
	}
	if got.window != window {
		t.Fatal("bindings did not retain the provided window")
	}
}
