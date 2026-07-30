//go:build !linux && !darwin && !windows

package browser

import (
	"fmt"
	"runtime"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

func discoverBrowsers() ([]BrowserOption, error) {
	return nil, fmt.Errorf("browser discovery is not supported on %s", runtime.GOOS)
}

func discoverCustomBrowsers(custom []preferencesdomain.CustomBrowser) []BrowserOption {
	options := make([]BrowserOption, 0, len(custom))
	for _, browser := range custom {
		if browser.Command == "" {
			continue
		}
		options = append(options, BrowserOption{
			ID:              browser.ID,
			DisplayName:     browser.DisplayName,
			CommandTemplate: browser.Command,
			Icon:            browser.Icon,
			Engine:          BrowserEngineRegular,
			Custom:          true,
		})
	}
	return options
}

func launchBrowser(string, string, BrowserOption, int, string) error {
	return fmt.Errorf("browser launch is not supported on %s", runtime.GOOS)
}

func openBrowserURL(BrowserOption, string) error {
	return fmt.Errorf("opening browser URLs is not supported on %s", runtime.GOOS)
}
