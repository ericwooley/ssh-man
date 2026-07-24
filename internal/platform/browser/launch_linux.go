package browser

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func launchLinux(appDataDir string, serverID string, option BrowserOption, socksPort int, rawURL string) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s is configured for regular URL launches only", option.DisplayName)
	}
	if browserEngine(option) == BrowserEngineFirefox {
		return launchLinuxFirefox(appDataDir, serverID, option, socksPort, rawURL)
	}
	profileDir := filepath.Join(profileScope(appDataDir, serverID, option), "chromium")
	arguments := []string{
		fmt.Sprintf("--proxy-server=socks5://127.0.0.1:%d", socksPort),
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		"--proxy-bypass-list=<-loopback>",
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1",
	}
	if rawURL != "" {
		arguments = append(arguments, rawURL)
	}
	return exec.Command(option.LaunchReference, arguments...).Start()
}

func launchLinuxFirefox(appDataDir string, serverID string, option BrowserOption, socksPort int, rawURL string) error {
	profileDir, err := ensureFirefoxProfile(appDataDir, serverID, option, socksPort)
	if err != nil {
		return fmt.Errorf("prepare firefox profile: %w", err)
	}
	arguments := []string{"-new-instance", "-profile", profileDir}
	if rawURL != "" {
		arguments = append(arguments, rawURL)
	}
	return exec.Command(option.LaunchReference, arguments...).Start()
}

func openLinuxBrowserURL(option BrowserOption, rawURL string) error {
	return exec.Command(option.LaunchReference, rawURL).Start()
}
