//go:build linux

package browser

func discoverBrowsers() ([]BrowserOption, error) {
	return discoverLinuxBrowsers(), nil
}

func launchBrowser(option BrowserOption, socksPort int) error {
	return launchLinux(option, socksPort)
}
