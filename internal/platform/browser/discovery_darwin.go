package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

type darwinBrowserCandidate struct {
	ID             string
	DisplayName    string
	AppPaths       []string
	ExecutableName string
	Engine         BrowserEngine
}

func darwinBrowserCandidates() []darwinBrowserCandidate {
	return []darwinBrowserCandidate{
		{ID: "google-chrome", DisplayName: "Google Chrome", AppPaths: []string{"/Applications/Google Chrome.app"}, ExecutableName: "Google Chrome", Engine: BrowserEngineChromium},
		{ID: "google-chrome-canary", DisplayName: "Google Chrome Canary", AppPaths: []string{"/Applications/Google Chrome Canary.app"}, ExecutableName: "Google Chrome Canary", Engine: BrowserEngineChromium},
		{ID: "chromium", DisplayName: "Chromium", AppPaths: []string{"/Applications/Chromium.app"}, ExecutableName: "Chromium", Engine: BrowserEngineChromium},
		{ID: "brave-browser", DisplayName: "Brave", AppPaths: []string{"/Applications/Brave Browser.app"}, ExecutableName: "Brave Browser", Engine: BrowserEngineChromium},
		{ID: "microsoft-edge", DisplayName: "Microsoft Edge", AppPaths: []string{"/Applications/Microsoft Edge.app"}, ExecutableName: "Microsoft Edge", Engine: BrowserEngineChromium},
		{ID: "arc", DisplayName: "Arc", AppPaths: []string{"/Applications/Arc.app"}, ExecutableName: "Arc", Engine: BrowserEngineChromium},
		{ID: "vivaldi", DisplayName: "Vivaldi", AppPaths: []string{"/Applications/Vivaldi.app"}, ExecutableName: "Vivaldi", Engine: BrowserEngineChromium},
		{ID: "opera", DisplayName: "Opera", AppPaths: []string{"/Applications/Opera.app"}, ExecutableName: "Opera", Engine: BrowserEngineChromium},
		{ID: "opera-gx", DisplayName: "Opera GX", AppPaths: []string{"/Applications/Opera GX.app"}, ExecutableName: "Opera GX", Engine: BrowserEngineChromium},
		{ID: "firefox", DisplayName: "Firefox", AppPaths: []string{"/Applications/Firefox.app"}, ExecutableName: "firefox", Engine: BrowserEngineFirefox},
		{ID: "firefox-developer-edition", DisplayName: "Firefox Developer Edition", AppPaths: []string{"/Applications/Firefox Developer Edition.app"}, ExecutableName: "firefox", Engine: BrowserEngineFirefox},
		{ID: "zen", DisplayName: "Zen", AppPaths: []string{"/Applications/Zen.app", "/Applications/Zen Browser.app"}, ExecutableName: "zen", Engine: BrowserEngineFirefox},
		{ID: "librewolf", DisplayName: "LibreWolf", AppPaths: []string{"/Applications/LibreWolf.app"}, ExecutableName: "librewolf", Engine: BrowserEngineFirefox},
		{ID: "floorp", DisplayName: "Floorp", AppPaths: []string{"/Applications/Floorp.app"}, ExecutableName: "floorp", Engine: BrowserEngineFirefox},
		{ID: "waterfox", DisplayName: "Waterfox", AppPaths: []string{"/Applications/Waterfox.app"}, ExecutableName: "waterfox", Engine: BrowserEngineFirefox},
		{ID: "orion", DisplayName: "Orion", AppPaths: []string{"/Applications/Orion.app"}, ExecutableName: "Orion", Engine: BrowserEngineRegular},
		{ID: "duckduckgo", DisplayName: "DuckDuckGo", AppPaths: []string{"/Applications/DuckDuckGo.app"}, ExecutableName: "DuckDuckGo", Engine: BrowserEngineRegular},
		{ID: "safari", DisplayName: "Safari", AppPaths: []string{"/Applications/Safari.app", "/System/Applications/Safari.app"}, ExecutableName: "Safari", Engine: BrowserEngineRegular},
	}
}

func discoverDarwinBrowsers() []BrowserOption {
	candidates := darwinBrowserCandidates()
	options := make([]BrowserOption, 0, len(candidates))
	for _, candidate := range candidates {
		for _, appPath := range candidate.AppPaths {
			if _, err := os.Stat(appPath); err != nil {
				continue
			}
			options = append(options, BrowserOption{
				ID:                  candidate.ID,
				DisplayName:         candidate.DisplayName,
				LaunchReference:     appPath,
				ExecutableReference: filepath.Join(appPath, "Contents", "MacOS", candidate.ExecutableName),
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
		if _, err := os.Stat(browser.LaunchReference); err != nil {
			continue
		}
		engine := BrowserEngine(browser.Engine)
		executableReference := darwinBundleExecutable(browser.LaunchReference)
		options = append(options, BrowserOption{
			ID:                  browser.ID,
			DisplayName:         browser.DisplayName,
			LaunchReference:     browser.LaunchReference,
			ExecutableReference: executableReference,
			Engine:              engine,
			SupportsProxyLaunch: engine != BrowserEngineRegular,
			Custom:              true,
		})
	}
	return options
}

func darwinBundleExecutable(appPath string) string {
	infoPlist := filepath.Join(appPath, "Contents", "Info.plist")
	output, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleExecutable", "raw", "-o", "-", infoPlist).Output()
	if err != nil {
		return ""
	}
	executableName := strings.TrimSpace(string(output))
	if executableName == "" || filepath.Base(executableName) != executableName {
		return ""
	}
	return filepath.Join(appPath, "Contents", "MacOS", executableName)
}
