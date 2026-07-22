//go:build darwin && !cgo

package browser

import "fmt"

func activateRunningBrowser(int) error {
	return fmt.Errorf("browser activation on macOS requires cgo")
}
