//go:build linux

package browser

func discoverBrowsers() ([]BrowserOption, error) {
	return discoverLinuxBrowsers(), nil
}

func launchBrowser(appDataDir string, serverID string, option BrowserOption, socksPort int) error {
	return launchLinux(appDataDir, serverID, option, socksPort)
}
