//go:build linux

package browser

import (
	"path/filepath"
)

func previewLaunchCommand(appDataDir string, serverID string, option BrowserOption, socksPort int) string {
	if browserEngine(option) == BrowserEngineFirefox {
		return formatCommand(option.LaunchReference, "-new-instance", "-profile", filepath.Join(profileScope(appDataDir, serverID, option), "firefox"))
	}
	parts := []string{option.LaunchReference}
	parts = append(parts, chromiumProxyArguments(
		"127.0.0.1",
		filepath.Join(profileScope(appDataDir, serverID, option), "chromium"),
		socksPort,
	)...)
	return formatCommand(parts...)
}
