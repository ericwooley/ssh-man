//go:build darwin

package browser

import "testing"

func TestDiscoverDarwinBrowsersIncludesSafariAsUnsupportedWhenInstalled(t *testing.T) {
	browsers := discoverDarwinBrowsers()

	for _, browser := range browsers {
		if browser.ID != "safari" {
			continue
		}
		if browser.SupportsProxyLaunch {
			t.Fatal("expected safari to be detected but marked unsupported for proxy launch")
		}
		if browser.LaunchReference == "" {
			t.Fatal("expected safari launch reference to be populated")
		}
		return
	}

	t.Skip("Safari is not installed at a standard application path on this machine")
}
