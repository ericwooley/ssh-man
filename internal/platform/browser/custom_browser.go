package browser

import preferencesdomain "ssh-man/internal/domain/preferences"

func customExecutableBrowserOption(
	browser preferencesdomain.CustomBrowser,
	launchReference string,
	executableReference string,
) BrowserOption {
	engine := BrowserEngine(browser.Engine)
	return BrowserOption{
		ID:                  browser.ID,
		DisplayName:         browser.DisplayName,
		LaunchReference:     launchReference,
		ExecutableReference: executableReference,
		Icon:                browser.Icon,
		Engine:              engine,
		SupportsProxyLaunch: engine != BrowserEngineRegular,
		Custom:              true,
	}
}
