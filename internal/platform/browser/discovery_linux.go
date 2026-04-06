package browser

import "os/exec"

func discoverLinuxBrowsers() []BrowserOption {
	candidates := []struct {
		ID          string
		DisplayName string
		Binary      string
	}{
		{ID: "google-chrome", DisplayName: "Google Chrome", Binary: "google-chrome"},
		{ID: "chromium", DisplayName: "Chromium", Binary: "chromium"},
		{ID: "brave-browser", DisplayName: "Brave", Binary: "brave-browser"},
		{ID: "firefox", DisplayName: "Firefox", Binary: "firefox"},
	}

	options := make([]BrowserOption, 0, len(candidates))
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate.Binary)
		if err != nil {
			continue
		}
		options = append(options, BrowserOption{ID: candidate.ID, DisplayName: candidate.DisplayName, LaunchReference: path, SupportsProxyLaunch: candidate.ID != "firefox"})
	}
	return options
}
