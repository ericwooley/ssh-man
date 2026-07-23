package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func launchDarwin(appDataDir string, serverID string, option BrowserOption, socksPort int, rawURL string) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s is configured for regular URL launches only", option.DisplayName)
	}
	if browserEngine(option) == BrowserEngineFirefox {
		return launchDarwinFirefox(appDataDir, serverID, option, socksPort, rawURL)
	}
	profileDir, err := ensureDarwinBrowserProfile(appDataDir, serverID, option)
	if err != nil {
		return fmt.Errorf("prepare browser profile: %w", err)
	}
	arguments := []string{
		"open",
		"-na",
		option.LaunchReference,
		"--args",
		fmt.Sprintf("--proxy-server=socks5://localhost:%d", socksPort),
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		"--proxy-bypass-list=<-loopback>",
	}
	if rawURL != "" {
		arguments = append(arguments, rawURL)
	}
	return exec.Command(arguments[0], arguments[1:]...).Start()
}

func launchDarwinFirefox(appDataDir string, serverID string, option BrowserOption, socksPort int, rawURL string) error {
	profileDir, err := ensureFirefoxProfile(appDataDir, serverID, option, socksPort)
	if err != nil {
		return fmt.Errorf("prepare firefox profile: %w", err)
	}

	executable := option.ExecutableReference
	if executable == "" {
		executable = filepath.Join(option.LaunchReference, "Contents", "MacOS", "firefox")
	}
	if _, err := os.Stat(executable); err == nil {
		arguments := []string{"-new-instance", "-profile", profileDir}
		if rawURL != "" {
			arguments = append(arguments, rawURL)
		}
		return exec.Command(executable, arguments...).Start()
	}

	arguments := []string{
		"open",
		"-na",
		option.LaunchReference,
		"--args",
		"-new-instance",
		"-profile",
		profileDir,
	}
	if rawURL != "" {
		arguments = append(arguments, rawURL)
	}
	return exec.Command(arguments[0], arguments[1:]...).Start()
}

func openDarwinBrowserURL(option BrowserOption, rawURL string) error {
	return exec.Command("open", "-a", option.LaunchReference, rawURL).Start()
}

func ensureDarwinBrowserProfile(appDataDir string, serverID string, option BrowserOption) (string, error) {
	profileDir := filepath.Join(profileScope(appDataDir, serverID, option), "chromium")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return "", err
	}
	return profileDir, nil
}
