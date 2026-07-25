//go:build !darwin && !linux

package previewwindow

import "os"

func FocusSignal() os.Signal {
	return nil
}
