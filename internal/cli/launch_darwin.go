//go:build darwin

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func defaultLauncher(ctx context.Context) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}

	const marker = ".app/Contents/"
	index := strings.LastIndex(executable, marker)
	if index < 0 {
		return fmt.Errorf("the CLI is not inside an SSH Man app bundle; start the desktop app first")
	}
	bundle := executable[:index+len(".app")]
	// Force LaunchServices to create a desktop process even though this CLI is
	// already executing the app bundle's binary. The owner lease collapses a
	// racing extra instance onto the authoritative menu-bar process.
	command := exec.CommandContext(ctx, "/usr/bin/open", "-ngj", bundle)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("open %q: %w: %s", bundle, err, strings.TrimSpace(string(output)))
	}
	return nil
}
