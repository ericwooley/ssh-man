//go:build windows

package browser

func discoverBrowsers() ([]BrowserOption, error) {
	return discoverWindowsBrowsers(), nil
}

func launchBrowser(appDataDir string, serverID string, option BrowserOption, socksPort int, rawURL string) error {
	return launchWindows(appDataDir, serverID, option, socksPort, rawURL)
}

func openBrowserURL(option BrowserOption, rawURL string) error {
	return openWindowsBrowserURL(option, rawURL)
}
