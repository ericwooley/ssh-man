package browser

import "os"

func discoverDarwinBrowsers() []BrowserOption {
	candidates := []struct {
		ID          string
		DisplayName string
		AppPaths    []string
	}{
		{ID: "google-chrome", DisplayName: "Google Chrome", AppPaths: []string{"/Applications/Google Chrome.app"}},
		{ID: "chromium", DisplayName: "Chromium", AppPaths: []string{"/Applications/Chromium.app"}},
		{ID: "brave-browser", DisplayName: "Brave", AppPaths: []string{"/Applications/Brave Browser.app"}},
		{ID: "firefox", DisplayName: "Firefox", AppPaths: []string{"/Applications/Firefox.app"}},
		{ID: "safari", DisplayName: "Safari", AppPaths: []string{"/Applications/Safari.app", "/System/Applications/Safari.app"}},
	}

	options := make([]BrowserOption, 0, len(candidates))
	for _, candidate := range candidates {
		for _, appPath := range candidate.AppPaths {
			if _, err := os.Stat(appPath); err != nil {
				continue
			}
			options = append(options, BrowserOption{ID: candidate.ID, DisplayName: candidate.DisplayName, LaunchReference: appPath, SupportsProxyLaunch: candidate.ID != "safari"})
			break
		}
	}
	return options
}
