package browser

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func launchLinux(appDataDir string, serverID string, option BrowserOption, socksPort int) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s does not support proxy launch in this MVP", option.DisplayName)
	}
	if option.ID == "firefox" {
		return launchLinuxFirefox(appDataDir, serverID, option, socksPort)
	}
	profileDir := filepath.Join(profileScope(appDataDir, serverID, option), "chromium")
	cmd := exec.Command(
		option.LaunchReference,
		fmt.Sprintf("--proxy-server=socks5://127.0.0.1:%d", socksPort),
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		"--proxy-bypass-list=<-loopback>",
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1",
	)
	return cmd.Start()
}

func launchLinuxFirefox(appDataDir string, serverID string, option BrowserOption, socksPort int) error {
	profileDir, err := ensureFirefoxProfile(appDataDir, serverID, option, socksPort)
	if err != nil {
		return fmt.Errorf("prepare firefox profile: %w", err)
	}
	return exec.Command(option.LaunchReference, "-new-instance", "-profile", profileDir).Start()
}
