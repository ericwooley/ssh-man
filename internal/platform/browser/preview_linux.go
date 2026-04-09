//go:build linux

package browser

import (
	"fmt"
	"path/filepath"
)

func previewLaunchCommand(appDataDir string, serverID string, option BrowserOption, socksPort int) string {
	if option.ID == "firefox" {
		return formatCommand(option.LaunchReference, "-new-instance", "-profile", filepath.Join(profileScope(appDataDir, serverID, option), "firefox"))
	}
	return formatCommand(
		option.LaunchReference,
		fmt.Sprintf("--proxy-server=socks5://127.0.0.1:%d", socksPort),
		fmt.Sprintf("--user-data-dir=%s", filepath.Join(profileScope(appDataDir, serverID, option), "chromium")),
		"--proxy-bypass-list=<-loopback>",
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1",
	)
}
