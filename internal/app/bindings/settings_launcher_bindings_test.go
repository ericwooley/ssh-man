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
