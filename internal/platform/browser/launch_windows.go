//go:build windows

package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func launchWindows(appDataDir string, serverID string, option BrowserOption, socksPort int, rawURL string) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s is configured for regular URL launches only", option.DisplayName)
	}

	arguments, err := windowsProxyLaunchArguments(appDataDir, serverID, option, socksPort, true)
	if err != nil {
		return err
	}
	if rawURL != "" {
		arguments = append(arguments, rawURL)
	}
	return exec.Command(option.LaunchReference, arguments...).Start()
}

func openWindowsBrowserURL(option BrowserOption, rawURL string) error {
	return exec.Command(option.LaunchReference, rawURL).Start()
}

func windowsProxyLaunchArguments(appDataDir string, serverID string, option BrowserOption, socksPort int, prepareProfile bool) ([]string, error) {
	if browserEngine(option) == BrowserEngineFirefox {
		profileDir := filepath.Join(profileScope(appDataDir, serverID, option), "firefox")
		if prepareProfile {
			var err error
			profileDir, err = ensureFirefoxProfile(appDataDir, serverID, option, socksPort)
			if err != nil {
				return nil, fmt.Errorf("prepare firefox profile: %w", err)
			}
		}
		return []string{"-new-instance", "-profile", profileDir}, nil
	}

	profileDir := filepath.Join(profileScope(appDataDir, serverID, option), "chromium")
	if prepareProfile {
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			return nil, fmt.Errorf("prepare browser profile: %w", err)
		}
	}
	return chromiumProxyArguments("127.0.0.1", profileDir, socksPort), nil
}
