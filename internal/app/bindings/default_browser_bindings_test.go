package bindings

import (
	"testing"

	"ssh-man/internal/platform/defaultbrowser"
)

func TestSetAsDefaultBrowserUsesOwnerSetter(t *testing.T) {
	binding := &AppBindings{}
	calls := 0
	binding.SetDefaultBrowserSetter(func() (defaultbrowser.Status, error) {
		calls++
		return defaultbrowser.Status{Supported: true, IsDefault: true}, nil
	})

	result, err := binding.SetAsDefaultBrowser()
	if err != nil {
		t.Fatal(err)
	}
	status, ok := result.(defaultbrowser.Status)
	if !ok || !status.IsDefault || calls != 1 {
		t.Fatalf("result = %#v, calls = %d", result, calls)
	}
}
