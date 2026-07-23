//go:build !darwin

package defaultbrowser

import "fmt"

func defaultBrowserSupported() bool {
	return false
}

func isDefaultBrowser() (bool, error) {
	return false, nil
}

func setDefaultBrowser() error {
	return fmt.Errorf("setting the default browser is only supported on macOS")
}
