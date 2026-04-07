package browser

import (
	"fmt"
	"os/exec"
)

func launchLinux(option BrowserOption, socksPort int) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s does not support proxy launch in this MVP", option.DisplayName)
	}
	if option.ID == "firefox" {
		return launchLinuxFirefox(option, socksPort)
	}
	cmd := exec.Command(
		option.LaunchReference,
		fmt.Sprintf("--proxy-server=socks5://127.0.0.1:%d", socksPort),
		"--proxy-bypass-list=<-loopback>",
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1",
	)
	return cmd.Start()
}

func launchLinuxFirefox(option BrowserOption, socksPort int) error {
	profileDir, err := ensureFirefoxProfile(option, socksPort)
	if err != nil {
		return fmt.Errorf("prepare firefox profile: %w", err)
	}
	return exec.Command(option.LaunchReference, "-new-instance", "-profile", profileDir).Start()
}
