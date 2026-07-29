//go:build windows

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

func defaultLauncher(context.Context) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}

	command := windowsDesktopCommand(executable)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start desktop executable %q: %w", executable, err)
	}
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("detach desktop executable %q: %w", executable, err)
	}
	return nil
}

func windowsDesktopCommand(executable string) *exec.Cmd {
	command := exec.Command(executable)
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	return command
}
