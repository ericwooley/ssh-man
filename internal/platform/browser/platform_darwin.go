//go:build darwin

package browser

func discoverBrowsers() ([]BrowserOption, error) {
	return discoverDarwinBrowsers(), nil
}

func launchBrowser(appDataDir string, serverID string, option BrowserOption, socksPort int, rawURL string) error {
	return launchDarwin(appDataDir, serverID, option, socksPort, rawURL)
}

func openBrowserURL(option BrowserOption, rawURL string) error {
	return openDarwinBrowserURL(option, rawURL)
}
