package bindings

import "testing"

func TestSettingsLauncherStartsSettingsProcess(t *testing.T) {
	launchCalls := 0
	binding := NewSettingsLauncherBindingsWithDependency(func() error {
		launchCalls++
		return nil
	})

	if err := binding.Open(); err != nil {
		t.Fatal(err)
	}
	if launchCalls != 1 {
		t.Fatalf("launch calls = %d, want 1", launchCalls)
	}
}

func TestSettingsWindowCloseLifecycle(t *testing.T) {
	closeRequests := 0
	binding := NewSettingsWindowBindingsWithCloseRequester(func() {
		closeRequests++
	})

	if !binding.RequestCloseConfirmation() {
		t.Fatal("native settings close should wait for the frontend decision")
	}
	if closeRequests != 1 {
		t.Fatalf("close requests = %d, want 1", closeRequests)
	}

	binding.AllowClose()
	if binding.RequestCloseConfirmation() {
		t.Fatal("an explicitly allowed settings close should proceed")
	}
}
