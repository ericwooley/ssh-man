//go:build darwin || linux

package previewwindow

import (
	"os"
	"syscall"
)

// FocusSignal is ignored by default, so an early focus request cannot
// terminate a preview process before its Wails window is ready.
func FocusSignal() os.Signal {
	return syscall.SIGWINCH
}
