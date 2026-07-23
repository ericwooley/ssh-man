//go:build darwin && cgo

package defaultbrowser

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework CoreServices
#include "default_darwin.h"
*/
import "C"

import "fmt"

func defaultBrowserSupported() bool {
	return true
}

func isDefaultBrowser() (bool, error) {
	return C.sshManIsDefaultBrowser() == 1, nil
}

func setDefaultBrowser() error {
	if status := int(C.sshManSetDefaultBrowser()); status != 0 {
		return fmt.Errorf("Launch Services returned status %d", status)
	}
	return nil
}
