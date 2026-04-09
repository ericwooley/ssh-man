//go:build darwin

package browser

func discoverBrowsers() ([]BrowserOption, error) {
	return discoverDarwinBrowsers(), nil
}

func launchBrowser(appDataDir string, serverID string, option BrowserOption, socksPort int) error {
	return launchDarwin(appDataDir, serverID, option, socksPort)
}
