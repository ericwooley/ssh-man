//go:build darwin

package browser

import (
	"path/filepath"
)

func previewLaunchCommand(appDataDir string, serverID string, option BrowserOption, socksPort int) string {
	if browserEngine(option) == BrowserEngineFirefox {
		return formatCommand("open", "-na", option.LaunchReference, "--args", "-new-instance", "-profile", filepath.Join(profileScope(appDataDir, serverID, option), "firefox"))
	}
	parts := []string{
		"open",
		"-na",
		option.LaunchReference,
		"--args",
	}
	parts = append(parts, chromiumProxyArguments(
		"127.0.0.1",
		filepath.Join(profileScope(appDataDir, serverID, option), "chromium"),
		socksPort,
	)...)
	return formatCommand(parts...)
}
