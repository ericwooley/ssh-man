//go:build darwin

package browser

import (
	"os"
	"strings"
	"testing"
)

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

func TestDarwinBrowserCatalogIncludesZenAsFirefoxCompatible(t *testing.T) {
	for _, candidate := range darwinBrowserCandidates() {
		if candidate.ID != "zen" {
			continue
		}
		if candidate.Engine != BrowserEngineFirefox {
			t.Fatalf("Zen engine = %q, want Firefox-compatible", candidate.Engine)
		}
		if candidate.ExecutableName != "zen" {
			t.Fatalf("Zen executable = %q, want zen", candidate.ExecutableName)
		}
		if len(candidate.AppPaths) == 0 || candidate.AppPaths[0] != "/Applications/Zen.app" {
			t.Fatalf("Zen app paths = %#v", candidate.AppPaths)
		}
		return
	}
	t.Fatal("Zen is missing from the built-in macOS browser catalog")
}

func TestZenProxyPreviewUsesFirefoxProfileArguments(t *testing.T) {
	option := BrowserOption{
		ID:              "zen",
		DisplayName:     "Zen",
		LaunchReference: "/Applications/Zen.app",
		Engine:          BrowserEngineFirefox,
	}
	preview := previewLaunchCommand("/tmp/ssh-man", "bts", option, 41001)
	if !strings.Contains(preview, "-new-instance") || !strings.Contains(preview, "/zen/firefox") {
		t.Fatalf("Zen preview = %q, want Firefox profile launch", preview)
	}
	if strings.Contains(preview, "--proxy-server") || strings.Contains(preview, "--user-data-dir") {
		t.Fatalf("Zen preview = %q, should not use Chromium flags", preview)
	}
}

func TestDiscoverDarwinBrowsersIncludesInstalledZen(t *testing.T) {
	if _, err := os.Stat("/Applications/Zen.app"); err != nil {
		t.Skip("Zen is not installed at /Applications/Zen.app")
	}
	for _, option := range discoverDarwinBrowsers() {
		if option.ID != "zen" {
			continue
		}
		if option.DisplayName != "Zen" || option.Engine != BrowserEngineFirefox || !option.SupportsProxyLaunch {
			t.Fatalf("discovered Zen option = %#v", option)
		}
		if option.ExecutableReference != "/Applications/Zen.app/Contents/MacOS/zen" {
			t.Fatalf("Zen executable = %q", option.ExecutableReference)
		}
		return
	}
	t.Fatal("installed Zen browser was not discovered")
}
