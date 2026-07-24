package browser

import (
	"os"
	"os/exec"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

func discoverLinuxBrowsers() []BrowserOption {
	candidates := []struct {
		ID          string
		DisplayName string
		Binaries    []string
		Engine      BrowserEngine
	}{
		{ID: "google-chrome", DisplayName: "Google Chrome", Binaries: []string{"google-chrome", "google-chrome-stable"}, Engine: BrowserEngineChromium},
		{ID: "chromium", DisplayName: "Chromium", Binaries: []string{"chromium", "chromium-browser"}, Engine: BrowserEngineChromium},
		{ID: "brave-browser", DisplayName: "Brave", Binaries: []string{"brave-browser"}, Engine: BrowserEngineChromium},
		{ID: "microsoft-edge", DisplayName: "Microsoft Edge", Binaries: []string{"microsoft-edge", "microsoft-edge-stable"}, Engine: BrowserEngineChromium},
		{ID: "vivaldi", DisplayName: "Vivaldi", Binaries: []string{"vivaldi", "vivaldi-stable"}, Engine: BrowserEngineChromium},
		{ID: "opera", DisplayName: "Opera", Binaries: []string{"opera"}, Engine: BrowserEngineChromium},
		{ID: "firefox", DisplayName: "Firefox", Binaries: []string{"firefox"}, Engine: BrowserEngineFirefox},
		{ID: "zen", DisplayName: "Zen", Binaries: []string{"zen", "zen-browser"}, Engine: BrowserEngineFirefox},
		{ID: "librewolf", DisplayName: "LibreWolf", Binaries: []string{"librewolf"}, Engine: BrowserEngineFirefox},
		{ID: "floorp", DisplayName: "Floorp", Binaries: []string{"floorp"}, Engine: BrowserEngineFirefox},
		{ID: "waterfox", DisplayName: "Waterfox", Binaries: []string{"waterfox"}, Engine: BrowserEngineFirefox},
	}

	options := make([]BrowserOption, 0, len(candidates))
	for _, candidate := range candidates {
		for _, binary := range candidate.Binaries {
			path, err := exec.LookPath(binary)
			if err != nil {
				continue
			}
			options = append(options, BrowserOption{
				ID:                  candidate.ID,
				DisplayName:         candidate.DisplayName,
				LaunchReference:     path,
				ExecutableReference: path,
				Engine:              candidate.Engine,
				SupportsProxyLaunch: candidate.Engine != BrowserEngineRegular,
			})
			break
		}
	}
	return options
}

func discoverCustomBrowsers(custom []preferencesdomain.CustomBrowser) []BrowserOption {
	options := make([]BrowserOption, 0, len(custom))
	for _, browser := range custom {
		info, err := os.Stat(browser.LaunchReference)
		if err != nil || info.IsDir() {
			continue
		}
		engine := BrowserEngine(browser.Engine)
		options = append(options, BrowserOption{
			ID:                  browser.ID,
			DisplayName:         browser.DisplayName,
			LaunchReference:     browser.LaunchReference,
			ExecutableReference: browser.LaunchReference,
			Engine:              engine,
			SupportsProxyLaunch: engine != BrowserEngineRegular,
			Custom:              true,
		})
	}
	return options
}
