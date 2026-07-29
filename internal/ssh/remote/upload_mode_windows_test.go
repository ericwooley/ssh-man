//go:build windows

package remote

import (
	"os"
	"testing"
)

func prepareLocalUploadMode(t *testing.T, path string, mode os.FileMode) os.FileMode {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return safeUploadMode(info.Mode())
}
