package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func launchDarwin(option BrowserOption, socksPort int) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s does not support proxy launch in this MVP", option.DisplayName)
	}
	if option.ID == "firefox" {
		return launchDarwinFirefox(option, socksPort)
	}
	profileDir, err := ensureDarwinBrowserProfile(option)
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

func launchDarwinFirefox(option BrowserOption, socksPort int) error {
	profileDir, err := ensureFirefoxProfile(option, socksPort)
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

func ensureDarwinBrowserProfile(option BrowserOption) (string, error) {
	baseDir := filepath.Join(os.TempDir(), "ssh-man-browser-profiles")
	profileDir := filepath.Join(baseDir, sanitizeBrowserID(option.ID))
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return "", err
	}
	return profileDir, nil
}
