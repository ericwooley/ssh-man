//go:build darwin

package browser

import (
	"fmt"
	"path/filepath"
)

func previewLaunchCommand(appDataDir string, serverID string, option BrowserOption, socksPort int) string {
	if option.ID == "firefox" {
		return formatCommand("open", "-na", option.LaunchReference, "--args", "-new-instance", "-profile", filepath.Join(profileScope(appDataDir, serverID, option), "firefox"))
	}
	return formatCommand(
		"open",
		"-na",
		option.LaunchReference,
		"--args",
		fmt.Sprintf("--proxy-server=socks5://localhost:%d", socksPort),
		fmt.Sprintf("--user-data-dir=%s", filepath.Join(profileScope(appDataDir, serverID, option), "chromium")),
		"--proxy-bypass-list=<-loopback>",
	)
}
