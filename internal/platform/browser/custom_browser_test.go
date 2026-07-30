package browser

import (
	"testing"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

func TestCustomExecutableBrowserOptionPropagatesAppearance(t *testing.T) {
	input := preferencesdomain.CustomBrowser{
		ID:          "custom-browser",
		DisplayName: "Custom Browser",
		Icon:        "icon:briefcase",
		Engine:      preferencesdomain.BrowserEngineFirefox,
	}

	got := customExecutableBrowserOption(input, "/browser", "/browser-bin")

	if got.Icon != input.Icon {
		t.Fatalf("custom browser icon = %q, want %q", got.Icon, input.Icon)
	}
	if got.LaunchReference != "/browser" || got.ExecutableReference != "/browser-bin" {
		t.Fatalf("custom browser executable references = %#v", got)
	}
	if got.Engine != BrowserEngineFirefox || !got.SupportsProxyLaunch || !got.Custom {
		t.Fatalf("custom browser metadata = %#v", got)
	}
}
