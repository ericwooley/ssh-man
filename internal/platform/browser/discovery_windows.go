//go:build windows

package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

type windowsBrowserLocation struct {
	Environment  string
	RelativePath string
}

type windowsBrowserCandidate struct {
	ID              string
	DisplayName     string
	Locations       []windowsBrowserLocation
	PathExecutables []string
	Engine          BrowserEngine
}

func windowsBrowserCandidates() []windowsBrowserCandidate {
	return []windowsBrowserCandidate{
		{
			ID:          "google-chrome",
			DisplayName: "Google Chrome",
			Locations: windowsMachineAndUserLocations(
				`Google\Chrome\Application\chrome.exe`,
				`Google\Chrome\Application\chrome.exe`,
			),
			PathExecutables: []string{"chrome.exe"},
			Engine:          BrowserEngineChromium,
		},
		{
			ID:          "google-chrome-beta",
			DisplayName: "Google Chrome Beta",
			Locations: windowsMachineAndUserLocations(
				`Google\Chrome Beta\Application\chrome.exe`,
				`Google\Chrome Beta\Application\chrome.exe`,
			),
			Engine: BrowserEngineChromium,
		},
		{
			ID:          "google-chrome-dev",
			DisplayName: "Google Chrome Dev",
			Locations: windowsMachineAndUserLocations(
				`Google\Chrome Dev\Application\chrome.exe`,
				`Google\Chrome Dev\Application\chrome.exe`,
			),
			Engine: BrowserEngineChromium,
		},
		{
			ID:          "google-chrome-canary",
			DisplayName: "Google Chrome Canary",
			Locations: []windowsBrowserLocation{
				{Environment: "LOCALAPPDATA", RelativePath: `Google\Chrome SxS\Application\chrome.exe`},
			},
			Engine: BrowserEngineChromium,
		},
		{
			ID:          "chromium",
			DisplayName: "Chromium",
			Locations: windowsMachineAndUserLocations(
				`Chromium\Application\chrome.exe`,
				`Chromium\Application\chrome.exe`,
			),
			PathExecutables: []string{"chromium.exe"},
			Engine:          BrowserEngineChromium,
		},
		{
			ID:          "brave-browser",
			DisplayName: "Brave",
			Locations: windowsMachineAndUserLocations(
				`BraveSoftware\Brave-Browser\Application\brave.exe`,
				`BraveSoftware\Brave-Browser\Application\brave.exe`,
			),
			PathExecutables: []string{"brave.exe"},
			Engine:          BrowserEngineChromium,
		},
		{
			ID:          "microsoft-edge",
			DisplayName: "Microsoft Edge",
			Locations: windowsMachineAndUserLocations(
				`Microsoft\Edge\Application\msedge.exe`,
				`Microsoft\Edge\Application\msedge.exe`,
			),
			PathExecutables: []string{"msedge.exe"},
			Engine:          BrowserEngineChromium,
		},
		{
			ID:          "microsoft-edge-beta",
			DisplayName: "Microsoft Edge Beta",
			Locations: windowsMachineAndUserLocations(
				`Microsoft\Edge Beta\Application\msedge.exe`,
				`Microsoft\Edge Beta\Application\msedge.exe`,
			),
			Engine: BrowserEngineChromium,
		},
		{
			ID:          "microsoft-edge-dev",
			DisplayName: "Microsoft Edge Dev",
			Locations: windowsMachineAndUserLocations(
				`Microsoft\Edge Dev\Application\msedge.exe`,
				`Microsoft\Edge Dev\Application\msedge.exe`,
			),
			Engine: BrowserEngineChromium,
		},
		{
			ID:          "microsoft-edge-canary",
			DisplayName: "Microsoft Edge Canary",
			Locations: windowsMachineAndUserLocations(
				`Microsoft\Edge SxS\Application\msedge.exe`,
				`Microsoft\Edge SxS\Application\msedge.exe`,
			),
			Engine: BrowserEngineChromium,
		},
		{
			ID:          "vivaldi",
			DisplayName: "Vivaldi",
			Locations: windowsMachineAndUserLocations(
				`Vivaldi\Application\vivaldi.exe`,
				`Vivaldi\Application\vivaldi.exe`,
			),
			PathExecutables: []string{"vivaldi.exe"},
			Engine:          BrowserEngineChromium,
		},
		{
			ID:          "opera",
			DisplayName: "Opera",
			Locations: windowsMachineAndUserLocations(
				`Opera\launcher.exe`,
				`Programs\Opera\launcher.exe`,
			),
			PathExecutables: []string{"opera.exe"},
			Engine:          BrowserEngineChromium,
		},
		{
			ID:          "opera-gx",
			DisplayName: "Opera GX",
			Locations: windowsMachineAndUserLocations(
				`Opera GX\launcher.exe`,
				`Programs\Opera GX\launcher.exe`,
			),
			Engine: BrowserEngineChromium,
		},
		{
			ID:          "firefox",
			DisplayName: "Firefox",
			Locations: append(
				windowsMachineAndUserLocations(
					`Mozilla Firefox\firefox.exe`,
					`Programs\Mozilla Firefox\firefox.exe`,
				),
				windowsBrowserLocation{Environment: "LOCALAPPDATA", RelativePath: `Mozilla Firefox\firefox.exe`},
			),
			PathExecutables: []string{"firefox.exe"},
			Engine:          BrowserEngineFirefox,
		},
		{
			ID:          "firefox-developer-edition",
			DisplayName: "Firefox Developer Edition",
			Locations: append(
				windowsMachineAndUserLocations(
					`Firefox Developer Edition\firefox.exe`,
					`Programs\Firefox Developer Edition\firefox.exe`,
				),
				windowsBrowserLocation{Environment: "LOCALAPPDATA", RelativePath: `Firefox Developer Edition\firefox.exe`},
			),
			Engine: BrowserEngineFirefox,
		},
		{
			ID:          "firefox-nightly",
			DisplayName: "Firefox Nightly",
			Locations: append(
				windowsMachineAndUserLocations(
					`Firefox Nightly\firefox.exe`,
					`Programs\Firefox Nightly\firefox.exe`,
				),
				windowsBrowserLocation{Environment: "LOCALAPPDATA", RelativePath: `Firefox Nightly\firefox.exe`},
			),
			Engine: BrowserEngineFirefox,
		},
		{
			ID:          "zen",
			DisplayName: "Zen",
			Locations: append(
				windowsMachineAndUserLocations(
					`Zen Browser\zen.exe`,
					`Programs\Zen Browser\zen.exe`,
				),
				windowsMachineAndUserLocations(
					`Zen\zen.exe`,
					`Programs\Zen\zen.exe`,
				)...,
			),
			PathExecutables: []string{"zen.exe"},
			Engine:          BrowserEngineFirefox,
		},
		{
			ID:          "librewolf",
			DisplayName: "LibreWolf",
			Locations: windowsMachineAndUserLocations(
				`LibreWolf\librewolf.exe`,
				`Programs\LibreWolf\librewolf.exe`,
			),
			PathExecutables: []string{"librewolf.exe"},
			Engine:          BrowserEngineFirefox,
		},
		{
			ID:          "floorp",
			DisplayName: "Floorp",
			Locations: append(
				windowsMachineAndUserLocations(
					`Ablaze Floorp\floorp.exe`,
					`Programs\Ablaze Floorp\floorp.exe`,
				),
				windowsMachineAndUserLocations(
					`Floorp\floorp.exe`,
					`Programs\Floorp\floorp.exe`,
				)...,
			),
			PathExecutables: []string{"floorp.exe"},
			Engine:          BrowserEngineFirefox,
		},
		{
			ID:          "waterfox",
			DisplayName: "Waterfox",
			Locations: windowsMachineAndUserLocations(
				`Waterfox\waterfox.exe`,
				`Programs\Waterfox\waterfox.exe`,
			),
			PathExecutables: []string{"waterfox.exe"},
			Engine:          BrowserEngineFirefox,
		},
	}
}

func windowsMachineAndUserLocations(machinePath string, userPath string) []windowsBrowserLocation {
	return []windowsBrowserLocation{
		{Environment: "ProgramW6432", RelativePath: machinePath},
		{Environment: "ProgramFiles", RelativePath: machinePath},
		{Environment: "ProgramFiles(x86)", RelativePath: machinePath},
		{Environment: "LOCALAPPDATA", RelativePath: userPath},
	}
}

func discoverWindowsBrowsers() []BrowserOption {
	return discoverWindowsBrowsersWith(
		windowsBrowserCandidates(),
		os.Getenv,
		exec.LookPath,
		func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
	)
}

func discoverWindowsBrowsersWith(
	candidates []windowsBrowserCandidate,
	getenv func(string) string,
	lookPath func(string) (string, error),
	isFile func(string) bool,
) []BrowserOption {
	options := make([]BrowserOption, 0, len(candidates))
	seenPaths := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		path := findWindowsBrowser(candidate, getenv, lookPath, isFile)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		pathKey := strings.ToLower(path)
		if _, exists := seenPaths[pathKey]; exists {
			continue
		}
		seenPaths[pathKey] = struct{}{}
		options = append(options, BrowserOption{
			ID:                  candidate.ID,
			DisplayName:         candidate.DisplayName,
			LaunchReference:     path,
			ExecutableReference: path,
			Engine:              candidate.Engine,
			SupportsProxyLaunch: candidate.Engine != BrowserEngineRegular,
		})
	}
	return options
}

func findWindowsBrowser(
	candidate windowsBrowserCandidate,
	getenv func(string) string,
	lookPath func(string) (string, error),
	isFile func(string) bool,
) string {
	for _, location := range candidate.Locations {
		root := strings.TrimSpace(getenv(location.Environment))
		if root == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(location.RelativePath))
		if isFile(path) {
			return path
		}
	}
	for _, executable := range candidate.PathExecutables {
		path, err := lookPath(executable)
		if err == nil && isFile(path) {
			return path
		}
	}
	return ""
}

func discoverCustomBrowsers(custom []preferencesdomain.CustomBrowser) []BrowserOption {
	options := make([]BrowserOption, 0, len(custom))
	for _, browser := range custom {
		path := strings.TrimSpace(browser.LaunchReference)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		options = append(options, customExecutableBrowserOption(browser, path, path))
	}
	return options
}
