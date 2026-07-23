//go:build !linux && !darwin

package browser

import (
	"fmt"
	"runtime"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

func discoverBrowsers() ([]BrowserOption, error) {
	return nil, fmt.Errorf("browser discovery is not supported on %s", runtime.GOOS)
}

func discoverCustomBrowsers([]preferencesdomain.CustomBrowser) []BrowserOption {
	return []BrowserOption{}
}

func launchBrowser(string, string, BrowserOption, int, string) error {
	return fmt.Errorf("browser launch is not supported on %s", runtime.GOOS)
}

func openBrowserURL(BrowserOption, string) error {
	return fmt.Errorf("opening browser URLs is not supported on %s", runtime.GOOS)
}
