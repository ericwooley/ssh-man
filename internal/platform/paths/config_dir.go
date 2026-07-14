package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const appDirName = "ssh-man"
const databaseFileName = "ssh-man.db"
const controlSocketFileName = "control.sock"
const ownerLockFileName = "controller.lock"

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	dir := filepath.Join(base, appDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("secure config dir: %w", err)
	}

	return dir, nil
}

func DatabasePath(configDir string) string {
	return filepath.Join(configDir, databaseFileName)
}

func ControlSocketPath(configDir string) string {
	return filepath.Join(configDir, controlSocketFileName)
}

func OwnerLockPath(configDir string) string {
	return filepath.Join(configDir, ownerLockFileName)
}
