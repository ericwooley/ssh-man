//go:build !linux && !darwin

package browser

import (
	"fmt"
	"runtime"
)

func discoverBrowsers() ([]BrowserOption, error) {
	return nil, fmt.Errorf("browser discovery is not supported on %s", runtime.GOOS)
}

func launchBrowser(string, string, BrowserOption, int) error {
	return fmt.Errorf("browser launch is not supported on %s", runtime.GOOS)
}
