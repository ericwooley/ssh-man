//go:build linux

package browser

import "fmt"

func previewLaunchCommand(option BrowserOption, socksPort int) string {
	if option.ID == "firefox" {
		return formatCommand(option.LaunchReference, "-new-instance", "-profile", fmt.Sprintf("/tmp/ssh-man-browser-profiles/%s-%d", sanitizeBrowserID(option.ID), socksPort))
	}
	return formatCommand(
		option.LaunchReference,
		fmt.Sprintf("--proxy-server=socks5://127.0.0.1:%d", socksPort),
		"--proxy-bypass-list=<-loopback>",
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1",
	)
}
