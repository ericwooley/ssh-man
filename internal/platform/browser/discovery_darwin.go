package browser

import "os"

func discoverDarwinBrowsers() []BrowserOption {
	candidates := []struct {
		ID          string
		DisplayName string
		AppPath     string
	}{
		{ID: "google-chrome", DisplayName: "Google Chrome", AppPath: "/Applications/Google Chrome.app"},
		{ID: "chromium", DisplayName: "Chromium", AppPath: "/Applications/Chromium.app"},
		{ID: "brave-browser", DisplayName: "Brave", AppPath: "/Applications/Brave Browser.app"},
		{ID: "firefox", DisplayName: "Firefox", AppPath: "/Applications/Firefox.app"},
	}

	options := make([]BrowserOption, 0, len(candidates))
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate.AppPath); err != nil {
			continue
		}
		options = append(options, BrowserOption{ID: candidate.ID, DisplayName: candidate.DisplayName, LaunchReference: candidate.AppPath, SupportsProxyLaunch: candidate.ID != "firefox"})
	}
	return options
}
