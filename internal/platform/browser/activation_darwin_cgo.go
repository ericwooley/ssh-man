//go:build darwin && cgo

package browser

/*
#cgo LDFLAGS: -framework Cocoa
#include "activation_darwin.h"
*/
import "C"

import "fmt"

func activateRunningBrowser(pid int) error {
	if C.SSHManActivateBrowserProcess(C.int(pid)) == 0 {
		return fmt.Errorf("the selected browser is no longer running")
	}
	return nil
}
