//go:build darwin && !cgo

package defaultbrowser

import "fmt"

func defaultBrowserSupported() bool {
	return false
}

func isDefaultBrowser() (bool, error) {
	return false, nil
}

func setDefaultBrowser() error {
	return fmt.Errorf("setting the default browser requires cgo")
}
