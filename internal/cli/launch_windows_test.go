//go:build windows

package cli

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsDesktopCommandLaunchesSameExecutableDetached(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "SSH Man.exe")
	command := windowsDesktopCommand(executable)

	if command.Path != executable {
		t.Fatalf("command.Path = %q, want %q", command.Path, executable)
	}
	if len(command.Args) != 1 || command.Args[0] != executable {
		t.Fatalf("command.Args = %q, want only the executable", command.Args)
	}
	if command.SysProcAttr == nil {
		t.Fatal("command.SysProcAttr is nil")
	}
	if command.SysProcAttr.CreationFlags&windows.DETACHED_PROCESS == 0 {
		t.Fatalf("CreationFlags = %#x, want DETACHED_PROCESS", command.SysProcAttr.CreationFlags)
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatalf("CreationFlags = %#x, want CREATE_NEW_PROCESS_GROUP", command.SysProcAttr.CreationFlags)
	}
	if command.SysProcAttr.HideWindow {
		t.Fatal("HideWindow = true, which could suppress the visible Wails window")
	}
}
