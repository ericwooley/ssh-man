//go:build darwin

package browser

import "fmt"

func previewLaunchCommand(option BrowserOption, socksPort int) string {
	if option.ID == "firefox" {
		return formatCommand("open", "-na", option.LaunchReference, "--args", "-new-instance", "-profile", fmt.Sprintf("/tmp/ssh-man-browser-profiles/%s-%d", sanitizeBrowserID(option.ID), socksPort))
	}
	return formatCommand(
		"open",
		"-na",
		option.LaunchReference,
		"--args",
		fmt.Sprintf("--proxy-server=socks5://localhost:%d", socksPort),
		fmt.Sprintf("--user-data-dir=%s", fmt.Sprintf("/tmp/ssh-man-browser-profiles/%s", sanitizeBrowserID(option.ID))),
		"--proxy-bypass-list=<-loopback>",
	)
}
