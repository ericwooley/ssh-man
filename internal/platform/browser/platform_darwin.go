//go:build darwin

package browser

func discoverBrowsers() ([]BrowserOption, error) {
	return discoverDarwinBrowsers(), nil
}

func launchBrowser(option BrowserOption, socksPort int) error {
	return launchDarwin(option, socksPort)
}
