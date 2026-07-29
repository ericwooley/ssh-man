//go:build !darwin && !linux

package previewwindow

import (
	"errors"
	"testing"
)

func TestManagerReportsFocusUnavailableWithoutLaunchingAnotherProcess(t *testing.T) {
	processes := &fakeProcessManager{}
	manager := newManagerWithProcesses(processes)

	if err := manager.Launch("server-1", "/tmp/report.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Focus("/tmp/report.pdf"); !errors.Is(err, ErrFocusUnavailable) {
		t.Fatalf("Focus() error = %v, want ErrFocusUnavailable", err)
	}
	if len(processes.launches) != 1 {
		t.Fatalf("focus launched %d preview processes, want 1 total", len(processes.launches))
	}
}
