//go:build darwin || linux

package previewwindow

import "testing"

func TestManagerFocusesExistingPreviewWithoutLaunchingAnotherProcess(t *testing.T) {
	processes := &fakeProcessManager{}
	manager := newManagerWithProcesses(processes)

	if err := manager.Launch("server-1", "/tmp/report.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Focus("/tmp/report.pdf"); err != nil {
		t.Fatal(err)
	}

	if len(processes.launches) != 1 {
		t.Fatalf("focus launched %d preview processes, want 1 total", len(processes.launches))
	}
	if signal := <-processes.process[0].signals; signal != FocusSignal() {
		t.Fatalf("focus signal = %v, want %v", signal, FocusSignal())
	}
}
