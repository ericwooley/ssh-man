package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func launchDarwin(appDataDir string, serverID string, option BrowserOption, socksPort int) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s does not support proxy launch in this MVP", option.DisplayName)
	}
	if option.ID == "firefox" {
		return launchDarwinFirefox(appDataDir, serverID, option, socksPort)
	}
	profileDir, err := ensureDarwinBrowserProfile(appDataDir, serverID, option)
	if err != nil {
		return fmt.Errorf("prepare browser profile: %w", err)
	}
	cmd := exec.Command(
		"open",
		"-na",
		option.LaunchReference,
		"--args",
		fmt.Sprintf("--proxy-server=socks5://localhost:%d", socksPort),
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		"--proxy-bypass-list=<-loopback>",
	)
	return cmd.Start()
}

func launchDarwinFirefox(appDataDir string, serverID string, option BrowserOption, socksPort int) error {
	profileDir, err := ensureFirefoxProfile(appDataDir, serverID, option, socksPort)
	if err != nil {
		return fmt.Errorf("prepare firefox profile: %w", err)
	}

	executable := filepath.Join(option.LaunchReference, "Contents", "MacOS", "firefox")
	if _, err := os.Stat(executable); err == nil {
		return exec.Command(executable, "-new-instance", "-profile", profileDir).Start()
	}

	return exec.Command(
		"open",
		"-na",
		option.LaunchReference,
		"--args",
		"-new-instance",
		"-profile",
		profileDir,
	).Start()
}

func ensureDarwinBrowserProfile(appDataDir string, serverID string, option BrowserOption) (string, error) {
	profileDir := filepath.Join(profileScope(appDataDir, serverID, option), "chromium")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return "", err
	}
	return profileDir, nil
}
