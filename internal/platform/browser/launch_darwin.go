package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func launchDarwin(option BrowserOption, socksPort int) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s does not support proxy launch in this MVP", option.DisplayName)
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

func ensureDarwinBrowserProfile(option BrowserOption) (string, error) {
	baseDir := filepath.Join(os.TempDir(), "ssh-man-browser-profiles")
	profileDir := filepath.Join(baseDir, sanitizeBrowserID(option.ID))
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return "", err
	}
	return profileDir, nil
}

func sanitizeBrowserID(id string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	cleaned := replacer.Replace(strings.TrimSpace(id))
	if cleaned == "" {
		return "browser"
	}
	return cleaned
}
