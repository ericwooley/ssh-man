//go:build linux

package browser

func discoverBrowsers() ([]BrowserOption, error) {
	return discoverLinuxBrowsers(), nil
}

func launchBrowser(appDataDir string, serverID string, option BrowserOption, socksPort int, rawURL string) error {
	return launchLinux(appDataDir, serverID, option, socksPort, rawURL)
}

func openBrowserURL(option BrowserOption, rawURL string) error {
	return openLinuxBrowserURL(option, rawURL)
}
